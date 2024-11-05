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

//go:embed data/frameworks/*.json
var frameworksFS embed.FS

// FrameworkProcessor processes files for Framework findings.
type FrameworkProcessor struct {
	frameworkRegexes []FrameworkRegex
}

func compileFrameworkRegexes(allFrameworks []Framework) []FrameworkRegex {
	var frameworkRegexes []FrameworkRegex
	for _, framework := range allFrameworks {
		pattern := framework.Pattern
		re, err := regexp.Compile(pattern)
		if err != nil {
			log.Printf("Failed to compile regex pattern '%s' from framework '%s': %v", pattern, framework.Name, err)
			continue
		}
		frameworkRegexes = append(frameworkRegexes, FrameworkRegex{
			Framework: framework,
			Regex:     re,
		})
	}
	return frameworkRegexes
}

func loadAllFrameworks() []Framework {
	var allFrameworks []Framework

	entries, err := frameworksFS.ReadDir("data/frameworks") // Corrected from servicesFS to frameworksFS
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

		content, err := frameworksFS.ReadFile(fmt.Sprintf("data/frameworks/%s", entry.Name())) // Corrected path
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
	return allFrameworks
}

// NewFrameworkProcessor creates a new FrameworkProcessor.
func NewFrameworkProcessor() *FrameworkProcessor {
	frameworks := loadAllFrameworks()
	frameworkRegexes := compileFrameworkRegexes(frameworks)
	return &FrameworkProcessor{frameworkRegexes: frameworkRegexes}
}

func (fp *FrameworkProcessor) Supports(filePath string) bool {
	base := filepath.Base(filePath)
	for _, fre := range fp.frameworkRegexes {
		if fre.Framework.PackageFileName != "" && fre.Framework.PackageFileName == base {
			return true
		}
	}
	return false
}

// Process applies framework regexes to the file content and returns findings.
func (fp *FrameworkProcessor) Process(path string, repoName string, content string) []Finding {
	var findings []Finding
	base := filepath.Base(path)

	for _, fre := range fp.frameworkRegexes {
		// If PackageFileName is specified, match the exact file name
		if fre.Framework.PackageFileName != "" && fre.Framework.PackageFileName != base {
			continue
		}

		matches := fre.Regex.FindAllString(content, -1)
		if len(matches) > 0 {
			for range matches {
				// Create a unique copy of the Framework for each finding
				frameworkCopy := fre.Framework
				finding := Finding{
					Framework:  &frameworkCopy,
					Repository: repoName,
					Filepath:   path,
				}
				findings = append(findings, finding)
			}
		}
	}
	return findings
}

type Framework struct {
	Name            string `json:"name,omitempty"`
	Category        string `json:"category,omitempty"`
	PackageFileName string `json:"package_file_name"`
	Pattern         string `json:"pattern"`
}

type FrameworkRegex struct {
	Framework Framework
	Regex     *regexp.Regexp
}
