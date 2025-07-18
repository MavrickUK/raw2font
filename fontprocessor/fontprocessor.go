package fontprocessor

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/image/font/sfnt"
)

type FontMetadata struct {
	FamilyName    string
	SubfamilyName string
	FullName      string
}

func ProcessFontFile(inputDir, inputPath, outputDir string, logWriter io.Writer) error {
	// Read font file
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %v", filepath.Base(inputPath), err)
	}

	// Parse font metadata
	metadata, ext, err := parseFont(data, inputPath, logWriter)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %v", filepath.Base(inputPath), err)
	}

	// Use filename as fallback
	filename := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	if metadata.FamilyName == "" {
		metadata.FamilyName = filename
	}
	if metadata.FullName == "" {
		metadata.FullName = filename
	}
	if metadata.SubfamilyName == "" {
		metadata.SubfamilyName = "Regular"
	}
	fmt.Fprintf(logWriter, "File: %s, FamilyName: %s, SubfamilyName: %s, FullName: %s\n",
		filepath.Base(inputPath), metadata.FamilyName, metadata.SubfamilyName, metadata.FullName)

	// Sanitize names
	safeFamilyName := sanitizeFileName(metadata.FamilyName)
	safeFullName := sanitizeFileName(metadata.FullName)

	// Determine output path (group by FamilyName)
	relPath, err := filepath.Rel(inputDir, filepath.Dir(inputPath))
	if err != nil {
		return fmt.Errorf("failed to compute relative path for %s: %v", inputPath, err)
	}
	outputSubDir := filepath.Join(relPath, safeFamilyName)
	if relPath == "." {
		outputSubDir = safeFamilyName
	}
	outputPath := filepath.Join(outputDir, outputSubDir, safeFullName+ext)

	// Create directory
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %v", filepath.Dir(outputPath), err)
	}

	// Resolve duplicates
	outputPath, isDuplicate, err := resolveDuplicate(outputPath, logWriter)
	if err != nil {
		return fmt.Errorf("failed to resolve output path for %s: %v", filepath.Base(inputPath), err)
	}
	if isDuplicate {
		return nil // Termina el procesamiento si es un duplicado
	}

	// Copy file
	if err := copyFile(inputPath, outputPath); err != nil {
		return fmt.Errorf("failed to copy %s to %s: %v", filepath.Base(inputPath), outputPath, err)
	}

	fmt.Fprintf(logWriter, "Copied %s to %s\n", filepath.Base(inputPath), outputPath)
	fmt.Fprintf(os.Stdout, "Copied %s to %s\n", filepath.Base(inputPath), outputPath)
	return nil
}

func parseFont(data []byte, inputPath string, logWriter io.Writer) (FontMetadata, string, error) {
	// Determine font type and extension
	ext, fontType, err := getFontType(data, inputPath)
	if err != nil {
		filename := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
		fmt.Fprintf(logWriter, "Invalid font signature for %s: %v\n", filepath.Base(inputPath), err)
		return inferMetadata(filename, fontType), ext, nil
	}

	switch fontType {
	case "type1":
		metadata, err := parseType1Font(data)
		if err != nil {
			filename := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
			fmt.Fprintf(logWriter, "Failed to parse Type 1 metadata for %s: %v\n", filepath.Base(inputPath), err)
			return inferMetadata(filename, fontType), ext, nil
		}
		return metadata, ext, nil
	case "truetype", "opentype":
		// Try sfnt parsing
		f, err := sfnt.Parse(data)
		if err == nil {
			metadata, err := getFontMetadata(f)
			if err == nil {
				return metadata, ext, nil
			}
			fmt.Fprintf(logWriter, "Failed to extract metadata for %s: %v\n", filepath.Base(inputPath), err)
		} else {
			fmt.Fprintf(logWriter, "Failed to parse font %s: %v\n", filepath.Base(inputPath), err)
		}

		// Try manual name table parsing
		metadata, err := parseNameTable(data, logWriter)
		if err == nil && metadata.FamilyName != "" {
			return metadata, ext, nil
		}
		fmt.Fprintf(logWriter, "Failed to parse name table for %s: %v\n", filepath.Base(inputPath), err)

		// Fallback to inferred metadata
		filename := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
		return inferMetadata(filename, fontType), ext, nil
	default:
		filename := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
		fmt.Fprintf(logWriter, "Unknown font type for %s\n", filepath.Base(inputPath))
		return inferMetadata(filename, fontType), ext, nil
	}
}

func getFontType(data []byte, inputPath string) (string, string, error) {
	if len(data) < 4 {
		return ".otf", "opentype", fmt.Errorf("data too short")
	}
	header := string(data[:4])
	switch header {
	case "OTTO":
		return ".otf", "opentype", nil
	case "\x00\x01\x00\x00", "true":
		return ".ttf", "truetype", nil
	default:
		if len(data) >= 2 && data[0] == '%' && data[1] == '!' {
			return ".pfa", "type1", nil
		}
		if len(data) >= 2 && data[0] == 0x80 && data[1] == 0x01 {
			return ".pfb", "type1", nil
		}
		return ".otf", "opentype", fmt.Errorf("unknown font signature: %x", header[:min(len(header), 4)])
	}
}

func parseType1Font(data []byte) (FontMetadata, error) {
	var metadata FontMetadata
	if len(data) >= 2 && data[0] == '%' && data[1] == '!' {
		lines := strings.SplitN(string(data), "\n", 50)
		for _, line := range lines {
			if strings.HasPrefix(line, "/FontName") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					name := strings.TrimPrefix(parts[1], "/")
					metadata.FamilyName = name
					metadata.FullName = name
					if strings.Contains(strings.ToLower(name), "italic") {
						metadata.SubfamilyName = "Italic"
					} else if strings.Contains(strings.ToLower(name), "bold") {
						metadata.SubfamilyName = "Bold"
					} else {
						metadata.SubfamilyName = "Regular"
					}
					return metadata, nil
				}
			}
		}
	}
	return metadata, fmt.Errorf("no FontName found")
}

func parseNameTable(data []byte, logWriter io.Writer) (FontMetadata, error) {
	var metadata FontMetadata
	if len(data) < 12 {
		return metadata, fmt.Errorf("data too short for table directory")
	}

	numTables := binary.BigEndian.Uint16(data[4:6])
	if len(data) < int(12+numTables*16) {
		return metadata, fmt.Errorf("invalid table directory")
	}

	for i := 0; i < int(numTables); i++ {
		offset := 12 + i*16
		if string(data[offset:offset+4]) == "name" {
			tableOffset := binary.BigEndian.Uint32(data[offset+8 : offset+12])
			tableLength := binary.BigEndian.Uint32(data[offset+12 : offset+16])
			if int(tableOffset+tableLength) > len(data) {
				return metadata, fmt.Errorf("invalid name table offset")
			}

			nameTable := data[tableOffset:]
			if len(nameTable) < 6 {
				return metadata, fmt.Errorf("name table too short")
			}
			count := binary.BigEndian.Uint16(nameTable[2:4])
			stringOffset := binary.BigEndian.Uint16(nameTable[4:6])

			for j := 0; j < int(count); j++ {
				entryOffset := 6 + j*12
				if len(nameTable) < entryOffset+12 {
					fmt.Fprintf(logWriter, "Invalid name table entry at index %d\n", j)
					continue
				}
				platformID := binary.BigEndian.Uint16(nameTable[entryOffset : entryOffset+2])
				nameID := binary.BigEndian.Uint16(nameTable[entryOffset+6 : entryOffset+8])
				length := binary.BigEndian.Uint16(nameTable[entryOffset+8 : entryOffset+10])
				offset := binary.BigEndian.Uint16(nameTable[entryOffset+10 : entryOffset+12])

				if platformID == 0 || platformID == 1 || platformID == 3 {
					nameStart := int(stringOffset) + int(offset)
					nameEnd := nameStart + int(length)
					if nameEnd > len(nameTable) {
						fmt.Fprintf(logWriter, "Invalid name table string offset for nameID %d\n", nameID)
						continue
					}
					nameBytes := nameTable[nameStart:nameEnd]
					if len(nameBytes) == 0 {
						continue
					}
					var name string
					if platformID == 3 || platformID == 0 {
						if len(nameBytes)%2 != 0 {
							continue
						}
						nameRunes := make([]rune, 0, len(nameBytes)/2)
						for k := 0; k < len(nameBytes)-1; k += 2 {
							r := rune(binary.BigEndian.Uint16(nameBytes[k : k+2]))
							if r != 0 {
								nameRunes = append(nameRunes, r)
							}
						}
						name = string(nameRunes)
					} else {
						name = string(nameBytes)
						name = strings.Map(func(r rune) rune {
							if r < 32 || (r > 126 && r < 160) {
								return -1
							}
							return r
						}, name)
					}
					if name == "" {
						continue
					}

					switch nameID {
					case 1:
						metadata.FamilyName = name
					case 2:
						metadata.SubfamilyName = name
					case 4:
						metadata.FullName = name
					case 16: // Preferred Family
						if metadata.FamilyName == "" {
							metadata.FamilyName = name
						}
					case 17: // Preferred Subfamily
						if metadata.SubfamilyName == "" {
							metadata.SubfamilyName = name
						}
					}
				}
			}
			break
		}
	}

	if metadata.FamilyName == "" && metadata.SubfamilyName == "" && metadata.FullName == "" {
		return metadata, fmt.Errorf("no metadata found")
	}
	return metadata, nil
}

func getFontMetadata(f *sfnt.Font) (FontMetadata, error) {
	var metadata FontMetadata
	for _, nameID := range []sfnt.NameID{sfnt.NameIDFamily, sfnt.NameIDSubfamily, sfnt.NameIDFull, 16, 17} {
		name, err := f.Name(nil, nameID)
		if err != nil {
			continue
		}
		switch nameID {
		case sfnt.NameIDFamily:
			metadata.FamilyName = name
		case sfnt.NameIDSubfamily:
			metadata.SubfamilyName = name
		case sfnt.NameIDFull:
			metadata.FullName = name
		case 16: // Preferred Family
			if metadata.FamilyName == "" {
				metadata.FamilyName = name
			}
		case 17: // Preferred Subfamily
			if metadata.SubfamilyName == "" {
				metadata.SubfamilyName = name
			}
		}
	}
	if metadata.FamilyName == "" && metadata.SubfamilyName == "" && metadata.FullName == "" {
		return metadata, fmt.Errorf("no metadata found")
	}
	return metadata, nil
}

func inferMetadata(filename, fontType string) FontMetadata {
	metadata := FontMetadata{
		FamilyName:    filename,
		SubfamilyName: "Regular",
		FullName:      filename,
	}

	// Check if filename is numeric (e.g., "60981")
	_, err := strconv.Atoi(filename)
	isNumeric := err == nil

	// Normalize filename
	clean := strings.ReplaceAll(strings.ToLower(filename), "-", " ")
	clean = strings.ReplaceAll(clean, "_", " ")
	clean = strings.Join(strings.Fields(clean), " ")
	words := strings.Fields(clean)
	if len(words) == 0 {
		return metadata
	}

	// Detect variable font: numeric filename or "vf" in name or OpenType font
	isVF := isNumeric || strings.Contains(clean, "vf") || fontType == "opentype"
	if isVF {
		styles := []string{
			"condensed extralight italics", "condensed extralight", "condensed light italics", "condensed light",
			"extralight italics", "extralight", "light italics", "light", "medium italics", "medium",
			"bold italics", "bold", "italic", "regular",
		}
		for _, style := range styles {
			if strings.Contains(clean, strings.ReplaceAll(style, " ", "")) {
				metadata.SubfamilyName = style
				break
			}
		}
		if metadata.SubfamilyName == "Regular" && len(words) > 1 {
			metadata.SubfamilyName = strings.Join(words[max(0, len(words)-2):], " ")
		}
		if isNumeric {
			metadata.FamilyName = "Unknown VF"
		} else {
			metadata.FamilyName = strings.Join(words[:max(0, len(words)-2)], " ")
			if metadata.FamilyName == "" {
				metadata.FamilyName = "Unknown VF"
			}
		}
		metadata.FullName = metadata.FamilyName + " " + metadata.SubfamilyName
	}

	metadata.FullName = strings.TrimSpace(metadata.FullName)
	if metadata.FamilyName == "" {
		metadata.FamilyName = "Unknown"
	}
	if metadata.FullName == "" {
		metadata.FullName = metadata.FamilyName + " Regular"
	}
	return metadata
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func sanitizeFileName(name string) string {
	invalidChars := []string{"/", "\\", ":", "*", "?", "<", ">", "|", "\""}
	for _, char := range invalidChars {
		name = strings.ReplaceAll(name, char, " ")
	}
	name = strings.Join(strings.Fields(name), " ")
	if name == "" {
		name = "Unnamed"
	}
	return name
}

func resolveDuplicate(path string, logWriter io.Writer) (string, bool, error) {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	dir := filepath.Dir(path)
	ext := filepath.Ext(path)
	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(logWriter, "La fuente '%s' ya existe\n", filepath.Base(path))
		return "", true, nil // Retorna true para indicar que es un duplicado
	}
	for i := 1; ; i++ {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return path, false, nil
		}
		path = filepath.Join(dir, fmt.Sprintf("%s %d%s", base, i, ext))
		if i > 100 {
			return "", false, fmt.Errorf("too many duplicates for %s", base)
		}
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, info.Mode())
}
