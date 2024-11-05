package main

import (
	"path/filepath"
	"strings"
)

// CloudServiceProcessor processes files for Service findings.
type CloudServiceProcessor struct {
	serviceRegexes []ServiceRegex
}

// NewServiceProcessor creates a new CloudServiceProcessor.
func NewServiceProcessor(serviceRegexes []ServiceRegex) *CloudServiceProcessor {
	return &CloudServiceProcessor{serviceRegexes: serviceRegexes}
}

// Process applies service regexes to the file content and returns findings.
func (sp *CloudServiceProcessor) Process(path string, repoName string, content string) []Finding {
	var findings []Finding
	ext := strings.TrimLeft(filepath.Ext(path), ".")

	for _, sre := range sp.serviceRegexes {
		// Match based on language (file extension) if specified
		if sre.Service.Language != "" && sre.Service.Language != ext {
			continue
		}

		matches := sre.Regex.FindAllString(content, -1)
		if len(matches) > 0 {
			for range matches {
				// Create a unique copy of the Service for each finding
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
