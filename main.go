package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/font/sfnt"
)

func main() {
	// Specify the input and output directories
	inputDir := "./input_fonts"   // Replace with your input directory path
	outputDir := "./output_fonts" // Replace with your output directory path

	// Create the output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Read the input directory
	files, err := ioutil.ReadDir(inputDir)
	if err != nil {
		log.Fatalf("Failed to read input directory: %v", err)
	}

	// Iterate through each file in the input directory
	for _, file := range files {
		// Skip directories
		if file.IsDir() {
			continue
		}

		// Construct the full input file path
		inputPath := filepath.Join(inputDir, file.Name())

		// Read the font file
		fontData, err := ioutil.ReadFile(inputPath)
		if err != nil {
			log.Printf("Failed to read file %s: %v", file.Name(), err)
			continue
		}

		// Parse the font file
		parsedFont, err := sfnt.Parse(fontData)
		if err != nil {
			log.Printf("Failed to parse font file %s: %v", file.Name(), err)
			continue
		}

		// Extract the font metadata
		metadata, err := getFontMetadata(parsedFont)
		if err != nil {
			log.Printf("Failed to extract metadata for %s: %v", file.Name(), err)
			continue
		}

		// Get the FamilyName, FullName, and SubfamilyName
		familyName, ok := metadata["FamilyName"]
		if !ok || familyName == "" {
			log.Printf("No FamilyName found for %s, using original filename", file.Name())
			familyName = file.Name()
		}
		fullName, ok := metadata["FullName"]
		if !ok || fullName == "" {
			log.Printf("No FullName found for %s, using original filename", file.Name())
			fullName = file.Name()
		}
		subfamilyName, _ := metadata["SubfamilyName"] // May be empty

		// Log metadata for debugging
		log.Printf("File: %s, FamilyName: %s, SubfamilyName: %s, FullName: %s",
			file.Name(), familyName, subfamilyName, fullName)

		// Sanitize names for filesystem
		safeFamilyName := sanitizeFileName(familyName)
		safeFullName := sanitizeFileName(fullName)

		// Determine the font type and appropriate extension
		extension, err := getFontExtension(fontData)
		if err != nil {
			log.Printf("Could not determine font type for %s, defaulting to .ttf: %v", file.Name(), err)
			extension = ".ttf"
		}

		// Determine the output subdirectory based on style
		var subDir string
		// Include Regular, Italic, and Bold in the base family folder
		regularStyles := []string{"regular", "italic", "bold"}
		isRegularStyle := false
		for _, style := range regularStyles {
			if strings.Contains(strings.ToLower(subfamilyName), style) || subfamilyName == "" {
				isRegularStyle = true
				break
			}
		}
		if isRegularStyle {
			// Regular, Italic, and Bold fonts go in the FamilyName folder
			subDir = safeFamilyName
		} else {
			// Other styles (e.g., Black, Light) go in FamilyName_Style folder
			style := strings.ReplaceAll(subfamilyName, " ", "_")
			if style == "" {
				style = "Unknown"
			}
			subDir = fmt.Sprintf("%s_%s", safeFamilyName, sanitizeFileName(style))
		}

		// Create the subdirectory
		subDirPath := filepath.Join(outputDir, subDir)
		if err := os.MkdirAll(subDirPath, 0755); err != nil {
			log.Printf("Failed to create subdirectory %s: %v", subDirPath, err)
			continue
		}

		// Construct the output file path
		outputFileName := safeFullName + extension
		outputPath := filepath.Join(subDirPath, outputFileName)

		// Check for duplicate filenames
		outputPath, err = resolveDuplicate(outputPath)
		if err != nil {
			log.Printf("Failed to resolve output path for %s: %v", file.Name(), err)
			continue
		}

		// Copy the file to the output directory
		if err := copyFile(inputPath, outputPath); err != nil {
			log.Printf("Failed to copy file %s to %s: %v", file.Name(), outputPath, err)
			continue
		}

		fmt.Printf("Copied %s to %s\n", file.Name(), outputPath)
	}
}

// getFontMetadata extracts metadata like font family, full name, and subfamily
func getFontMetadata(f *sfnt.Font) (map[string]string, error) {
	metadata := make(map[string]string)

	// Iterate through the name table entries
	for _, nameID := range []sfnt.NameID{
		sfnt.NameIDFamily,
		sfnt.NameIDSubfamily,
		sfnt.NameIDFull,
		sfnt.NameIDPostScript,
	} {
		name, err := f.Name(nil, nameID)
		if err != nil {
			continue // Skip if the name ID is not found
		}
		switch nameID {
		case sfnt.NameIDFamily:
			metadata["FamilyName"] = name
		case sfnt.NameIDSubfamily:
			metadata["SubfamilyName"] = name
		case sfnt.NameIDFull:
			metadata["FullName"] = name
		case sfnt.NameIDPostScript:
			metadata["PostScriptName"] = name
		}
	}

	if len(metadata) == 0 {
		return nil, fmt.Errorf("no metadata found")
	}
	return metadata, nil
}

// getFontExtension determines the font file extension based on its header
func getFontExtension(fontData []byte) (string, error) {
	if len(fontData) < 4 {
		return "", fmt.Errorf("font data too short")
	}

	// Check the first 4 bytes of the font file
	header := string(fontData[:4])
	switch header {
	case "OTTO":
		return ".otf", nil // OpenType with PostScript outlines
	case "\x00\x01\x00\x00", "true":
		return ".ttf", nil // TrueType or OpenType with TrueType outlines
	default:
		return "", fmt.Errorf("unknown font signature: %x", header)
	}
}

// sanitizeFileName replaces invalid or unsafe characters in a filename
func sanitizeFileName(name string) string {
	// Replace invalid characters (e.g., /, \, :, *, ?, <, >, |) with underscores
	invalidChars := []string{"/", "\\", ":", "*", "?", "<", ">", "|", "\""}
	for _, char := range invalidChars {
		name = strings.ReplaceAll(name, char, "_")
	}
	// Replace spaces with underscores for consistency
	//name = strings.ReplaceAll(name, " ", "_")
	// Trim any leading/trailing underscores or spaces
	name = strings.Trim(name, "_ ")
	if name == "" {
		name = "unnamed"
	}
	return name
}

// resolveDuplicate ensures the output filename is unique by appending a counter if needed
func resolveDuplicate(outputPath string) (string, error) {
	ext := filepath.Ext(outputPath)
	baseName := strings.TrimSuffix(filepath.Base(outputPath), ext)
	dir := filepath.Dir(outputPath)
	for i := 1; ; i++ {
		if _, err := os.Stat(outputPath); os.IsNotExist(err) {
			return outputPath, nil
		}
		// Append a counter to the filename (e.g., Helvetica_Regular_1.ttf)
		outputPath = filepath.Join(dir, fmt.Sprintf("%s_%d%s", baseName, i, ext))
		if i > 1000 { // Arbitrary limit to prevent infinite loops
			return "", fmt.Errorf("too many duplicates for %s", baseName)
		}
	}
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	// Ensure the copied file has the same permissions as the source
	sourceInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, sourceInfo.Mode())
}
