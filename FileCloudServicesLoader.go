package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"strings"
)

type FileCloudServicesLoader struct {
	fs fs.FS
}

func NewFileCloudServicesLoader(fs fs.FS) *FileCloudServicesLoader {
	return &FileCloudServicesLoader{fs: fs}
}

func (f FileCloudServicesLoader) LoadAllCloudServices() []CloudService {
	var allServices []CloudService

	entries, err := fs.ReadDir(f.fs, "data/cloud_service_mappings")
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

		content, err := fs.ReadFile(f.fs, fmt.Sprintf("data/cloud_service_mappings/%s", entry.Name()))
		if err != nil {
			log.Printf("Failed to read file %s: %v", entry.Name(), err)
			continue
		}

		var services []CloudService
		err = json.Unmarshal(content, &services)
		if err != nil {
			log.Printf("Failed to unmarshal JSON from file %s: %v", entry.Name(), err)
			continue
		}

		allServices = append(allServices, services...)
	}
	return allServices
}
