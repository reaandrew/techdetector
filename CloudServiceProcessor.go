package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"strings"
)

//go:embed data/cloud_service_mappings/*.json
var servicesFS embed.FS

// CloudServiceProcessor processes files for CloudService findings.
type CloudServiceProcessor struct {
	serviceRegexes []CloudServiceRegex
}

func loadAllCloudServices() []CloudService {
	var allServices []CloudService

	entries, err := servicesFS.ReadDir("data/cloud_service_mappings")
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

		content, err := servicesFS.ReadFile(fmt.Sprintf("data/cloud_service_mappings/%s", entry.Name()))
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

func compileServicesRegexes(allServices []CloudService) []CloudServiceRegex {
	var serviceRegexes []CloudServiceRegex
	for _, service := range allServices {
		pattern := service.Reference
		re, err := regexp.Compile(pattern)
		if err != nil {
			log.Printf("Failed to compile regex pattern '%s' from service '%s': %v", pattern, service.CloudService, err)
			continue
		}
		serviceRegexes = append(serviceRegexes, CloudServiceRegex{
			Service: service,
			Regex:   re,
		})
	}
	return serviceRegexes
}

// NewServiceProcessor creates a new CloudServiceProcessor.
func NewServiceProcessor() *CloudServiceProcessor {
	services := loadAllCloudServices()
	serviceRegexes := compileServicesRegexes(services)
	return &CloudServiceProcessor{serviceRegexes: serviceRegexes}
}

func (csp *CloudServiceProcessor) Supports(filePath string) bool {
	ext := strings.TrimLeft(filepath.Ext(filePath), ".")
	for _, sre := range csp.serviceRegexes {
		if sre.Service.Language != "" && sre.Service.Language == ext {
			return true
		}
	}
	return false
}

// Process applies service regexes to the file content and returns findings.
func (csp *CloudServiceProcessor) Process(path string, repoName string, content string) []Finding {
	var findings []Finding
	ext := strings.TrimLeft(filepath.Ext(path), ".")

	for _, sre := range csp.serviceRegexes {
		// Match based on language (file extension) if specified
		if sre.Service.Language != "" && sre.Service.Language != ext {
			continue
		}

		matches := sre.Regex.FindAllString(content, -1)
		if len(matches) > 0 {
			for range matches {
				// Create a unique copy of the CloudService for each finding
				serviceCopy := sre.Service
				finding := Finding{
					Service:    &serviceCopy,
					Repository: repoName,
					Filepath:   path,
				}
				findings = append(findings, finding)
			}
		}
	}
	return findings
}

type CloudService struct {
	CloudVendor  string `json:"cloud_vendor"`
	CloudService string `json:"cloud_service"`
	Language     string `json:"language"`
	Reference    string `json:"reference"`
}

type CloudServiceRegex struct {
	Service CloudService
	Regex   *regexp.Regexp
}
