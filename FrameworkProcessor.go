package main

import (
	"github.com/gobwas/glob"
	"log"
	"path/filepath"
	"regexp"
	"strings"
)

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
