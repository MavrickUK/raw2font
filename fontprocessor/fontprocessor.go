package fontprocessor

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/font/sfnt"
)

type FontMetadata struct {
	FamilyName     string
	SubfamilyName  string
	FullName       string
	PostScriptName string
}

func ProcessFontFile(inputPath, outputDir string, logWriter io.Writer) error {
	baseName := filepath.Base(inputPath)

	fontData, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", baseName, err)
	}

	font, err := sfnt.Parse(fontData)
	if err != nil {
		return fmt.Errorf("parse %s: %w", baseName, err)
	}

	metadata, err := getFontMetadata(font)
	if err != nil {
		return fmt.Errorf("metadata %s: %w", baseName, err)
	}

	familyName := fallback(metadata.FamilyName, baseName, "FamilyName", logWriter)
	fullName := fallback(metadata.FullName, baseName, "FullName", logWriter)
	subfamilyName := metadata.SubfamilyName

	safeFamilyName := sanitizeFileName(familyName)
	safeFullName := sanitizeFileName(fullName)

	ext, err := getFontExtension(fontData)
	if err != nil {
		fmt.Fprintf(logWriter, "Unknown font type for %s; defaulting to .ttf: %v\n", baseName, err)
		ext = ".ttf"
	}

	subDir := determineSubDir(safeFamilyName, subfamilyName)

	destDir := filepath.Join(outputDir, subDir)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", destDir, err)
	}

	outputPath := filepath.Join(destDir, safeFullName+ext)
	outputPath, err = resolveDuplicate(outputPath)
	if err != nil {
		return fmt.Errorf("duplicate resolution %s: %w", baseName, err)
	}

	if err := copyFile(inputPath, outputPath); err != nil {
		return fmt.Errorf("copy %s to %s: %w", baseName, outputPath, err)
	}

	fmt.Fprintf(logWriter, "Copied %s → %s\n", baseName, outputPath)
	fmt.Fprintf(os.Stdout, "Copied %s → %s\n", baseName, outputPath)

	return nil
}

func getFontMetadata(f *sfnt.Font) (FontMetadata, error) {
	var meta FontMetadata

	for _, id := range []sfnt.NameID{
		sfnt.NameIDFamily,
		sfnt.NameIDSubfamily,
		sfnt.NameIDFull,
		sfnt.NameIDPostScript,
	} {
		if name, err := f.Name(nil, id); err == nil {
			switch id {
			case sfnt.NameIDFamily:
				meta.FamilyName = name
			case sfnt.NameIDSubfamily:
				meta.SubfamilyName = name
			case sfnt.NameIDFull:
				meta.FullName = name
			case sfnt.NameIDPostScript:
				meta.PostScriptName = name
			}
		}
	}

	if meta.FamilyName == "" && meta.FullName == "" && meta.SubfamilyName == "" && meta.PostScriptName == "" {
		return meta, fmt.Errorf("no metadata found")
	}
	return meta, nil
}

func getFontExtension(data []byte) (string, error) {
	if len(data) < 4 {
		return "", fmt.Errorf("font data too short")
	}
	switch sig := string(data[:4]); sig {
	case "OTTO":
		return ".otf", nil
	case "\x00\x01\x00\x00", "true":
		return ".ttf", nil
	default:
		return "", fmt.Errorf("unknown signature: %x", sig)
	}
}

func sanitizeFileName(name string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "<", "_", ">", "_", "|", "_", "\"", "_", " ", "_")
	name = replacer.Replace(name)
	name = strings.Trim(name, "_ ")
	if name == "" {
		return "unnamed"
	}
	return name
}

func resolveDuplicate(path string) (string, error) {
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(filepath.Base(path), ext)
	dir := filepath.Dir(path)

	for i := 1; ; i++ {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return path, nil
		}
		if i > 1000 {
			return "", fmt.Errorf("too many duplicates for %s", base)
		}
		path = filepath.Join(dir, fmt.Sprintf("%s_%d%s", base, i, ext))
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

	if _, err = io.Copy(out, in); err != nil {
		return err
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, srcInfo.Mode())
}

func fallback(value, fallback, name string, w io.Writer) string {
	if value == "" {
		fmt.Fprintf(w, "No %s found, using filename %s\n", name, fallback)
		return fallback
	}
	return value
}

func determineSubDir(family, subfamily string) string {
	styles := []string{"regular", "italic", "bold"}
	lowerSub := strings.ToLower(subfamily)

	for _, style := range styles {
		if strings.Contains(lowerSub, style) || subfamily == "" {
			return family
		}
	}
	style := sanitizeFileName(strings.ReplaceAll(subfamily, " ", "_"))
	if style == "" {
		style = "Unknown"
	}
	return fmt.Sprintf("%s_%s", family, style)
}
