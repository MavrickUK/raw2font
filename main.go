package main

import (
	"OrganizedFonts/fontprocessor" // Replace with your module name
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

func main() {
	// Specify the input and output directories
	inputDir := "input_fonts"   // Replace with your input directory path
	outputDir := "output_fonts" // Replace with your output directory path

	// Create the output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Set up logging to a timestamped file with UTF-8+BOM and stderr
	logWriter, err := setupLogging(outputDir)
	if err != nil {
		log.Fatalf("Failed to set up logging: %v", err)
	}
	defer logWriter.Close()

	// Read the input directory
	files, err := os.ReadDir(inputDir)
	if err != nil {
		log.Fatalf("Failed to read input directory: %v", err)
	}

	// Process each file
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		inputPath := filepath.Join(inputDir, file.Name())
		if err := fontprocessor.ProcessFontFile(inputPath, outputDir, logWriter); err != nil {
			log.Printf("Failed to process file %s: %v", file.Name(), err)
			continue
		}
	}
}

// setupLogging creates a timestamped log file with UTF-8+BOM and configures logging
func setupLogging(outputDir string) (io.WriteCloser, error) {
	logFileName := filepath.Join(outputDir, fmt.Sprintf("Log_%s.txt", time.Now().Format("20060102150405")))
	logFile, err := os.Create(logFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file %s: %v", logFileName, err)
	}

	// Write UTF-8 BOM
	_, err = logFile.Write([]byte{0xEF, 0xBB, 0xBF})
	if err != nil {
		logFile.Close()
		return nil, fmt.Errorf("failed to write UTF-8 BOM to log file: %v", err)
	}

	// Set up logging to both file and stderr
	multiWriter := io.MultiWriter(logFile, os.Stderr)
	log.SetOutput(multiWriter)
	log.SetFlags(log.LstdFlags) // Include timestamp in logs

	return logFile, nil
}
