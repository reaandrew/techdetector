package main

import (
	"log"
	"path/filepath"
	"regexp"
	"strings"
)

// CloudServiceProcessor processes files for CloudService findings.
type CloudServiceProcessor struct {
	serviceRegexes []CloudServiceRegex
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
func NewServiceProcessor(loader CloudServicesLoader) *CloudServiceProcessor {
	services := loader.LoadAllCloudServices()
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
