package main

import (
	"encoding/json"
	"fmt"
	"github.com/gobwas/glob"
	"io/fs"
	"log"
	"path/filepath"
	"regexp"
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
			Framework:           framework,
			Regex:               re,
			PackageFilenameGlob: glob.MustCompile(framework.PackageFileName),
		})
	}
	return frameworkRegexes
}

// NewFrameworkProcessor creates a new FrameworkProcessor.
func NewFrameworkProcessor(loader FrameworksLoader) *FrameworkProcessor {
	frameworks, _ := loader.LoadAllFrameworks()
	frameworkRegexes := compileFrameworkRegexes(frameworks)
	return &FrameworkProcessor{frameworkRegexes: frameworkRegexes}
}

func hasWildcards(pattern string) bool {
	return strings.ContainsAny(pattern, "*?[]")
}

func (fp *FrameworkProcessor) Supports(filePath string) bool {
	base := filepath.Base(filePath)
	for _, fre := range fp.frameworkRegexes {
		if fre.Framework.PackageFileName != "" &&
			(fre.Framework.PackageFileName == base ||
				(hasWildcards(fre.Framework.PackageFileName) && fre.PackageFilenameGlob.Match(base))) {
			return true
		}
	}
	return false
}

// Process applies framework regexes to the file content and returns findings.
func (fp *FrameworkProcessor) Process(path string, repoName string, content string) ([]Finding, error) {
	var findings []Finding
	base := filepath.Base(path)
	for _, fre := range fp.frameworkRegexes {
		if fre.Framework.PackageFileName != "" &&
			!(fre.Framework.PackageFileName == base ||
				(hasWildcards(fre.Framework.PackageFileName) && fre.PackageFilenameGlob.Match(base))) {
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
	return findings, nil
}

type Framework struct {
	Name            string `json:"name,omitempty"`
	Category        string `json:"category,omitempty"`
	PackageFileName string `json:"package_file_name"`
	Pattern         string `json:"pattern"`
}

type FrameworkRegex struct {
	Framework           Framework
	Regex               *regexp.Regexp
	PackageFilenameGlob glob.Glob
}
