package main

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sync"
)

type FileScanner struct {
	processors []Processor
}

func (fileScanner FileScanner) TraverseAndSearch(targetDir string, repoName string) ([]Match, error) {
	var Matches []Match

	info, err := os.Stat(targetDir)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("target directory '%s' does not exist", targetDir)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("'%s' is not a directory", targetDir)
	}

	files := make(chan string, 100)
	fileMatches := make(chan Match, 100)

	var wg sync.WaitGroup

	// Start file workers
	for i := 0; i < MaxFileWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for path := range files {
				// Check if any processor supports this file
				supported := false
				for _, processor := range fileScanner.processors {
					if processor.Supports(path) {
						supported = true
						break
					}
				}

				if !supported {
					continue // Skip files not supported by any processor
				}

				content, err := os.ReadFile(path)
				if err != nil {
					log.Printf("Failed to read file '%s': %v", path, err)
					continue
				}

				text := string(content)

				// Apply all processors that support this file
				for _, processor := range fileScanner.processors {
					if processor.Supports(path) {
						results, _ := processor.Process(path, repoName, text)
						for _, Match := range results {
							fileMatches <- Match
						}
					}
				}
			}
		}(i)
	}

	go func() {
		err := filepath.WalkDir(targetDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				log.Printf("Error accessing path '%s': %v", path, err)
				return nil // Continue walking.
			}

			if d.IsDir() {
				return nil
			}

			files <- path
			return nil
		})
		if err != nil {
			log.Printf("Error walking the directory: %v", err)
		}
		close(files)
	}()

	// Collect Matches in a separate goroutine
	go func() {
		wg.Wait()
		close(fileMatches)
	}()

	for Match := range fileMatches {
		Matches = append(Matches, Match)
	}

	return Matches, nil
}
