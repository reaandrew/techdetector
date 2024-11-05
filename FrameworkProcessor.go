package main

import "path/filepath"

// FrameworkProcessor processes files for Framework findings.
type FrameworkProcessor struct {
	frameworkRegexes []FrameworkRegex
}

// NewFrameworkProcessor creates a new FrameworkProcessor.
func NewFrameworkProcessor(frameworkRegexes []FrameworkRegex) *FrameworkProcessor {
	return &FrameworkProcessor{frameworkRegexes: frameworkRegexes}
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
