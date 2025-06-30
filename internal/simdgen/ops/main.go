package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
)

const baseDir = "ops" // The main directory containing A, B, C, etc.

func main() {
	if err := mergeYamlFiles("categories.yaml"); err != nil {
		fmt.Printf("Error processing categories.yaml: %v\n", err)
		os.Exit(1)
	}
	if err := mergeYamlFiles("go.yaml"); err != nil {
		fmt.Printf("Error processing go.yaml: %v\n", err)
		os.Exit(1)
	}
}

func mergeYamlFiles(targetFileName string) error {
	outputFile, err := os.Create(targetFileName)
	if err != nil {
		return fmt.Errorf("failed to create output file %s: %w", targetFileName, err)
	}
	defer outputFile.Close()

	writer := bufio.NewWriter(outputFile)
	_, err = writer.WriteString("!sum\n")
	if err != nil {
		return fmt.Errorf("failed to write '!sum' to %s: %w", targetFileName, err)
	}

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return fmt.Errorf("failed to read base directory %s: %w", baseDir, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		subdirPath := filepath.Join(baseDir, entry.Name())
		sourceFilePath := filepath.Join(subdirPath, targetFileName)

		sourceFile, err := os.Open(sourceFilePath)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Printf("Skipping: %s not found in %s\n", targetFileName, subdirPath)
				continue
			}
			return fmt.Errorf("failed to open source file %s: %w", sourceFilePath, err)
		}
		defer sourceFile.Close()

		scanner := bufio.NewScanner(sourceFile)
		// Skip first line
		scanner.Scan()
		// Append the rest of the lines to the output file
		for scanner.Scan() {
			line := scanner.Text()
			_, err = writer.WriteString(line + "\n")
			if err != nil {
				return fmt.Errorf("failed to write line from %s to %s: %w", sourceFilePath, targetFileName, err)
			}
		}

		if err := scanner.Err(); err != nil {
			return fmt.Errorf("error reading lines from %s: %w", sourceFilePath, err)
		}
	}
	return writer.Flush()
}
