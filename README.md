# Raw2Font

## Overview

Raw2Font is a Go application designed to organize and rename font files by extracting their metadata. It processes font files (TrueType, OpenType, and Type 1) with or without extensions, renaming them based on their `FamilyName`, `SubfamilyName`, and `FullName` metadata and organizing them into directories by `FamilyName`. The application handles variable fonts (e.g., `Chidori VF`, `Brevia Light`) without splitting them, supports filenames with spaces, and gracefully manages errors such as invalid `hmtx` or `CFF` tables.

## Features

- **Metadata Extraction**: Parses font metadata from TrueType (`*.ttf`), OpenType (`*.otf`), and Type 1 (`*.pfa`, `*.pfb`) fonts, even for files with numeric names (e.g., `60981`, `5458`).
- **File Organization**: Groups fonts into directories based on `FamilyName` (e.g., `output_fonts/Chidori VF/Chidori VF Condensed ExtraLight.otf`).
- **Error Handling**: Robustly handles parsing errors (e.g., `invalid hmtx table`, `invalid CFF table`) using fallback metadata extraction from the font’s `name` table or inferred metadata from filenames.
- **No Hardcoding**: Avoids hardcoded font names, ensuring flexibility for arbitrary input filenames.
- **Logging**: Outputs detailed logs to a UTF-8+BOM text file (e.g., `Log_20250718125100.txt`) and the terminal, with the final terminal output being `Log file created: ...`.
- **Spaces in Names**: Preserves spaces in font names and directories (e.g., `Brevia Light` instead of `Brevia_Light`).
- **Duplicate Handling**: Resolves filename conflicts by appending numeric suffixes (e.g., `FontName 1.otf`).

## Usage

### Prerequisites

- Go 1.21 or higher.
- Dependency: `golang.org/x/image/font/sfnt`.

### Setup

```bash
go mod tidy
go build -ldflags="-s -w" -o Raw2Font.exe main.go
```

### Run

```bash
./Raw2Font.exe
```

Specify `inputDir` and `outputDir` in `main.go` to point to your font directory and desired output location.

### Output

- Fonts are organized into `outputDir` (e.g., `output_fonts/Brevia Light/Brevia-LightItalic.otf`).
- A UTF-8+BOM log file is created in `outputDir` with processing details.

## Example

### Input

- `raw_fonts/60981` (variable font, 460KB)
- `raw_fonts/5458` (variable font)

### Output

```
output_fonts/
├── Myriad Devanagari/
│   ├── Myriad Devanagari Black Italic.otf
├── Chidori VF/
│   ├── Chidori VF Condensed ExtraLight.otf
├── Brevia Light/
│   ├── Brevia-LightItalic.otf
├── Log_20250718125100.txt
```

### Log File (UTF-8+BOM)

```
2025/07/18 12:51:00 File: 60981, FamilyName: Chidori VF, SubfamilyName: Condensed ExtraLight, FullName: Chidori VF Condensed ExtraLight
2025/07/18 12:51:00 Copied 60981 to output_fonts/Chidori VF/Chidori VF Condensed ExtraLight.otf
2025/07/18 12:51:00 Failed to parse font 5458: sfnt: invalid hmtx table, falling back to name table
2025/07/18 12:51:00 File: 5458, FamilyName: Brevia Light, SubfamilyName: Italic, FullName: Brevia-LightItalic
2025/07/18 12:51:00 Copied 5458 to output_fonts/Brevia Light/Brevia-LightItalic.otf
```

## Notes

- Variable fonts are kept as single files, preserving all styles (e.g., `Condensed ExtraLight`, `Italic`).
- The application is optimized for simplicity and reliability, with no external dependencies beyond the Go standard library and `sfnt`.
- For corrupted fonts, metadata is inferred from filenames or defaults to `Unknown VF` for numeric names.

## License

MIT License
