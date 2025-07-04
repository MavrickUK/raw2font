package main

import (
	"fmt"
	"golang.org/x/image/font/sfnt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	const (
		inputDir  = "./input_fonts"
		outputDir = "./output_fonts"
	)

	// Create output directory
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create output directory: %v\n", err)
		os.Exit(1)
	}

	// Walk directory for better handling of subdirectories
	err := filepath.WalkDir(inputDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error accessing %s: %v\n", path, err)
			return nil // Continue processing other files
		}
		if d.IsDir() {
			return nil
		}

		return processFontFile(path, outputDir)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to walk input directory: %v\n", err)
		os.Exit(1)
	}
}

func processFontFile(inputPath, outputDir string) error {
	// Open font file as a stream
	fontFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %v", inputPath, err)
	}
	defer fontFile.Close()

	// Read only the necessary bytes for parsing
	fontData := io.LimitReader(fontFile, 64*1024*1024) // limit to 64MB or as needed
	buf, err := io.ReadAll(fontData)
	if err != nil {
		return fmt.Errorf("failed to read font data from %s: %v", inputPath, err)
	}

	// Parse font
	parsedFont, err := sfnt.Parse(buf)
	if err != nil {
		return fmt.Errorf("failed to parse font %s: %v", inputPath, err)
	}

	// Get metadata
	metadata, err := getFontMetadata(parsedFont)
	if err != nil {
		return fmt.Errorf("failed to extract metadata for %s: %v", inputPath, err)
	}

	// Extract and validate names
	familyName := metadata["FamilyName"]
	if familyName == "" {
		familyName = filepath.Base(inputPath)
		fmt.Fprintf(os.Stderr, "No FamilyName for %s, using filename\n", inputPath)
	}
	fullName := metadata["FullName"]
	if fullName == "" {
		fullName = filepath.Base(inputPath)
		fmt.Fprintf(os.Stderr, "No FullName for %s, using filename\n", inputPath)
	}
	subfamilyName := metadata["SubfamilyName"]

	// Sanitize names
	safeFamilyName := sanitizeFileName(familyName)
	safeFullName := sanitizeFileName(fullName)

	// Determine extension
	extension, err := getFontExtension(buf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unknown font type for %s, defaulting to .ttf: %v\n", inputPath, err)
		extension = ".ttf"
	}

	// Determine subdirectory
	subDir := safeFamilyName
	if subfamilyName != "" && !isRegularStyle(subfamilyName) {
		subDir = fmt.Sprintf("%s_%s", safeFamilyName, sanitizeFileName(strings.ReplaceAll(subfamilyName, " ", "_")))
	}

	// Create subdirectory
	subDirPath := filepath.Clean(filepath.Join(outputDir, subDir))
	if err := os.MkdirAll(subDirPath, 0700); err != nil {
		return fmt.Errorf("failed to create subdirectory %s: %v", subDirPath, err)
	}

	// Construct and resolve output path
	outputPath := filepath.Clean(filepath.Join(subDirPath, safeFullName+extension))
	outputPath, err = resolveDuplicate(outputPath)
	if err != nil {
		return fmt.Errorf("failed to resolve output path for %s: %v", inputPath, err)
	}

	// Copy file
	if err := copyFile(inputPath, outputPath); err != nil {
		return fmt.Errorf("failed to copy %s to %s: %v", inputPath, outputPath, err)
	}

	fmt.Fprintf(os.Stderr, "Processed %s to %s\n", filepath.Base(inputPath), outputPath)
	return nil
}

func getFontMetadata(f *sfnt.Font) (map[string]string, error) {
	metadata := make(map[string]string, 4)
	nameIDs := map[sfnt.NameID]string{
		sfnt.NameIDFamily:     "FamilyName",
		sfnt.NameIDSubfamily:  "SubfamilyName",
		sfnt.NameIDFull:       "FullName",
		sfnt.NameIDPostScript: "PostScriptName",
	}

	for nameID, key := range nameIDs {
		if name, err := f.Name(nil, nameID); err == nil && name != "" {
			metadata[key] = name
		}
	}

	if len(metadata) == 0 {
		return nil, fmt.Errorf("no metadata found")
	}
	return metadata, nil
}

func getFontExtension(fontData []byte) (string, error) {
	if len(fontData) < 4 {
		return "", fmt.Errorf("font data too short")
	}

	switch string(fontData[:4]) {
	case "OTTO":
		return ".otf", nil
	case "\x00\x01\x00\x00", "true":
		return ".ttf", nil
	default:
		return "", fmt.Errorf("unknown font signature: %x", fontData[:4])
	}
}

func sanitizeFileName(name string) string {
	invalidChars := []rune{'/', '\\', ':', '*', '?', '<', '>', '|', '"'}
	for _, char := range invalidChars {
		name = strings.ReplaceAll(name, string(char), "_")
	}
	return strings.Trim(name, "_ ") + func() string {
		if name == "" {
			return "unnamed"
		}
		return ""
	}()
}

func resolveDuplicate(outputPath string) (string, error) {
	const maxAttempts = 1000
	ext := filepath.Ext(outputPath)
	baseName := strings.TrimSuffix(filepath.Base(outputPath), ext)
	dir := filepath.Dir(outputPath)

	for i := 0; i < maxAttempts; i++ {
		path := outputPath
		if i > 0 {
			path = filepath.Join(dir, fmt.Sprintf("%s_%d%s", baseName, i, ext))
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return path, nil
		}
	}
	return "", fmt.Errorf("too many duplicates for %s", baseName)
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}
	return os.Chmod(dst, srcInfo.Mode())
}

func isRegularStyle(subfamilyName string) bool {
	regularStyles := []string{"regular", "italic", "bold"}
	lowerSubfamily := strings.ToLower(subfamilyName)
	for _, style := range regularStyles {
		if strings.Contains(lowerSubfamily, style) {
			return true
		}
	}
	return false
}
