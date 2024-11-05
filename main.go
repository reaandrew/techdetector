package main

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	MaxWorkers     = 10
	MaxFileWorkers = 10
	CloneBaseDir   = "/tmp/techdetector" // You can make this configurable if needed
)

func main() {
	cli := &Cli{}
	if err := cli.Execute(); err != nil {
		log.Fatalf("Error executing command: %v", err)
	}
}

// traverseAndSearch traverses the target directory and applies all processors to each file.
func traverseAndSearch(targetDir string, repoName string, processors []Processor) ([]Finding, error) {
	var findings []Finding

	info, err := os.Stat(targetDir)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("target directory '%s' does not exist", targetDir)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("'%s' is not a directory", targetDir)
	}

	// Collect supported file extensions and specific file names from all processors
	supportedExtMap := make(map[string]struct{})
	supportedFileNames := make(map[string]struct{})
	for _, processor := range processors {
		switch p := processor.(type) {
		case *CloudServiceProcessor:
			for _, sre := range p.serviceRegexes {
				if sre.Service.Language != "" {
					supportedExtMap[sre.Service.Language] = struct{}{}
				}
			}
		case *FrameworkProcessor:
			for _, fre := range p.frameworkRegexes {
				if fre.Framework.PackageFileName != "" {
					supportedFileNames[fre.Framework.PackageFileName] = struct{}{}
				}
			}
		}
	}

	files := make(chan string, 100)
	fileFindings := make(chan Finding, 100)

	var wg sync.WaitGroup

	// Start file workers
	for i := 0; i < MaxFileWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range files {
				ext := strings.TrimLeft(filepath.Ext(path), ".")
				base := filepath.Base(path)

				// Check if the file extension or name is supported
				if _, ok := supportedExtMap[ext]; !ok {
					if _, nameOk := supportedFileNames[base]; !nameOk {
						continue
					}
				}

				content, err := os.ReadFile(path)
				if err != nil {
					log.Printf("Failed to read file '%s': %v", path, err)
					continue
				}

				text := string(content)

				// Apply all processors
				for _, processor := range processors {
					results := processor.Process(path, repoName, text)
					for _, finding := range results {
						fileFindings <- finding
					}
				}
			}
		}()
	}

	// Walk the directory and send file paths to the workers
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

	// Collect findings in a separate goroutine
	go func() {
		wg.Wait()
		close(fileFindings)
	}()

	for finding := range fileFindings {
		findings = append(findings, finding)
	}

	return findings, nil
}
