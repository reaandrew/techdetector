package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"strings"
)

type FileFrameworksLoader struct {
	fs fs.FS
}

func (f FileFrameworksLoader) LoadAllFrameworks() ([]Framework, error) {
	var allFrameworks []Framework

	entries, err := fs.ReadDir(f.fs, "data/frameworks") // Corrected from servicesFS to frameworksFS
	if err != nil {
		log.Fatalf("Failed to read embedded directory: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		content, err := fs.ReadFile(f.fs, fmt.Sprintf("data/frameworks/%s", entry.Name())) // Corrected path
		if err != nil {
			log.Printf("Failed to read file %s: %v", entry.Name(), err)
			continue
		}

		var frameworks []Framework
		err = json.Unmarshal(content, &frameworks)
		if err != nil {
			log.Printf("Failed to unmarshal JSON from file %s: %v", entry.Name(), err)
			continue
		}

		allFrameworks = append(allFrameworks, frameworks...)
	}
	return allFrameworks, nil
}

func NewFileFrameworkLoader(fs fs.FS) *FileFrameworksLoader {
	return &FileFrameworksLoader{fs: fs}
}
