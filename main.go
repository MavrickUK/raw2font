package main

import (
	"Raw2Font/fontprocessor"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	// Specify input and output directories
	inputDir := "raw_fonts"     // Replace with your input directory path
	outputDir := "output_fonts" // Replace with your output directory path

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Set up logging
	logWriter, logFileName, err := setupLogging(outputDir)
	if err != nil {
		log.Fatalf("Failed to set up logging: %v", err)
	}
	defer logWriter.Close()

	// Process font files
	err = filepath.Walk(inputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("Error accessing path %s: %v", path, err)
			return nil
		}
		if info.IsDir() {
			return nil
		}
		// Filter for no-extension files or Type 1 fonts
		ext := strings.ToLower(filepath.Ext(path))
		if ext != "" && ext != ".pfa" && ext != ".pfb" {
			return nil
		}
		// Process font file
		if err := fontprocessor.ProcessFontFile(inputDir, path, outputDir, logWriter); err != nil {
			log.Printf("Failed to process file %s: %v", path, err)
			return nil
		}
		return nil
	})
	if err != nil {
		log.Printf("Error walking input directory: %v", err)
	}

	// Print log file creation message as last terminal output
	fmt.Fprintf(os.Stdout, "Log file created: %s\n", logFileName)
}

func setupLogging(outputDir string) (io.WriteCloser, string, error) {
	logFileName := filepath.Join(outputDir, fmt.Sprintf("Log_%s.txt", time.Now().Format("20060102150405")))
	logFile, err := os.Create(logFileName)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create log file %s: %v", logFileName, err)
	}

	// Write UTF-8 BOM
	if _, err := logFile.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		logFile.Close()
		return nil, "", fmt.Errorf("failed to write UTF-8 BOM: %v", err)
	}

	// Set up logging to file and stderr
	log.SetOutput(io.MultiWriter(logFile, os.Stderr))
	log.SetFlags(log.LstdFlags)
	return logFile, logFileName, nil
}
