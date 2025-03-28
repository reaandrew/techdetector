package processors

import (
	"encoding/json"
	"fmt"
	"github.com/gobwas/glob"
	"github.com/reaandrew/techdetector/core"
	log "github.com/sirupsen/logrus"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
)

func isNilOrEmpty[T any](slice []T) bool {
	return slice == nil || len(slice) == 0
}

type Pattern struct {
	Name                 string   `json:"name,omitempty"`
	Type                 string   `json:"type,omitempty"`
	Category             string   `json:"category,omitempty"`
	Filenames            []string `json:"file_names,omitempty"`
	PathPatterns         []string `json:"path_patterns,omitempty"`
	FileExtensions       []string `json:"file_extensions,omitempty"`
	ContentPatterns      []string `json:"content_patterns,omitempty"`
	FilenameRegexs       []*regexp.Regexp
	ContentPatternRegexs []*regexp.Regexp
	PathPatternGlobs     []glob.Glob
	Properties           map[string]interface{} `json:"properties,omitempty"`
}

type FilePatternsProcessor struct {
	Patterns []Pattern
}

func (s *FilePatternsProcessor) CompilePatterns() {
	for i := range s.Patterns {
		for _, filename := range s.Patterns[i].Filenames {
			if containsRegexSpecialChars(filename) {
				// Treat as regex pattern as is
				s.Patterns[i].FilenameRegexs = append(s.Patterns[i].FilenameRegexs, regexp.MustCompile(filename))
			} else {
				// Treat as exact match by anchoring
				regexPattern := "^" + regexp.QuoteMeta(filename) + "$"
				s.Patterns[i].FilenameRegexs = append(s.Patterns[i].FilenameRegexs, regexp.MustCompile(regexPattern))
			}
		}
		for _, content := range s.Patterns[i].ContentPatterns {
			s.Patterns[i].ContentPatternRegexs = append(s.Patterns[i].ContentPatternRegexs, regexp.MustCompile(content))
		}
		for _, pathPattern := range s.Patterns[i].PathPatterns {
			s.Patterns[i].PathPatternGlobs = append(s.Patterns[i].PathPatternGlobs, glob.MustCompile(pathPattern))
		}
	}
}

// Helper function to check for regex special characters
func containsRegexSpecialChars(s string) bool {
	specialChars := `\.+*?()|[]{}^$`
	return strings.ContainsAny(s, specialChars)
}

func NewFilePatternsProcessor(fs fs.FS) *FilePatternsProcessor {
	patterns, _ := LoadAllPatterns(fs)
	processor := &FilePatternsProcessor{Patterns: patterns}
	processor.CompilePatterns()
	return processor
}

func LoadAllPatterns(f fs.FS) ([]Pattern, error) {
	var allPatterns []Pattern
	entries, err := fs.ReadDir(f, "data/patterns")
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

		content, err := fs.ReadFile(f, fmt.Sprintf("data/patterns/%s", entry.Name()))
		if err != nil {
			log.Printf("Failed to read file %s: %v", entry.Name(), err)
			continue
		}

		var patterns []Pattern
		err = json.Unmarshal(content, &patterns)
		if err != nil {
			log.Printf("Failed to unmarshal JSON from file %s: %v", entry.Name(), err)
			continue
		}

		allPatterns = append(allPatterns, patterns...)
	}
	return allPatterns, nil
}

func (s *FilePatternsProcessor) Supports(path string) bool {
	for _, pattern := range s.Patterns {
		if !isNilOrEmpty(pattern.Filenames) && !isNilOrEmpty(pattern.FileExtensions) {
			log.Errorf("Pattern error: Pattern '%s' specifies both file_names and file_extensions", pattern.Name)
			continue
		}

		if isNilOrEmpty(pattern.Filenames) && isNilOrEmpty(pattern.FileExtensions) && !isNilOrEmpty(pattern.ContentPatterns) {
			log.Errorf("Pattern error: Pattern '%s' has content_patterns but no file_names or file_extensions", pattern.Name)
			continue
		}

		var isFilenameMatch, isFileExtensionMatch, isPathMatch = false, false, false

		// Check filename match if specified
		if !isNilOrEmpty(pattern.Filenames) {
			isFilenameMatch = matchFilename(pattern, path)
		}

		// Check file extension match if specified
		if !isNilOrEmpty(pattern.FileExtensions) {
			isFileExtensionMatch = matchFileExtension(pattern, path)
		}

		// Check path pattern match if specified
		if !isNilOrEmpty(pattern.PathPatterns) {
			isPathMatch = matchPath(pattern, path)
		}

		// Ensure all criteria must match
		if isFilenameMatch || isFileExtensionMatch || isPathMatch {
			return true
		}
	}
	return false
}

func matchPath(pattern Pattern, path string) bool {
	normalizedPath := "/" + filepath.ToSlash(strings.TrimPrefix(path, "/"))
	for _, glob := range pattern.PathPatternGlobs {
		if glob.Match(normalizedPath) {
			log.Infof("Matched path: %s with pattern: %s", normalizedPath, glob)
			return true
		}
	}
	return false
}

func matchFilename(pattern Pattern, path string) bool {

	filename := filepath.Base(path) // Extract the base filename from the path

	for _, filenamePattern := range pattern.Filenames {
		if filename == filenamePattern {
			return true
		}
	}

	for _, filenameRegex := range pattern.FilenameRegexs {
		if filenameRegex.MatchString(filename) {
			return true
		}
	}

	return false
}

func matchFileExtension(pattern Pattern, path string) bool {
	if isNilOrEmpty(pattern.FileExtensions) {
		return true
	}
	for _, extension := range pattern.FileExtensions {
		if strings.TrimPrefix(filepath.Ext(path), ".") == strings.TrimPrefix(extension, ".") {
			return true
		}
	}
	return false
}

func copyProperties(properties map[string]interface{}) map[string]interface{} {
	if properties == nil {
		return nil
	}
	newProperties := make(map[string]interface{}, len(properties))
	for k, v := range properties {
		newProperties[k] = v
	}
	return newProperties
}

func (s *FilePatternsProcessor) Process(path string, repoName string, content string) ([]core.Finding, error) {
	var matches []core.Finding
	for _, pattern := range s.Patterns {
		// Skip patterns that specify both file_names and file_extensions (Rule 1)
		if !isNilOrEmpty(pattern.Filenames) && !isNilOrEmpty(pattern.FileExtensions) {
			log.Errorf("Pattern error: Pattern '%s' specifies both file_names and file_extensions", pattern.Name)
			continue
		}

		// Skip patterns that have content_patterns but no file_names or file_extensions (Rule 2)
		if isNilOrEmpty(pattern.Filenames) && isNilOrEmpty(pattern.FileExtensions) && !isNilOrEmpty(pattern.ContentPatterns) {
			log.Errorf("Pattern error: Pattern '%s' has content_patterns but no file_names or file_extensions", pattern.Name)
			continue
		}

		var isFilenameMatch bool = false
		var isFileExtensionMatch bool = false
		var isContentPatternMatch bool = false
		var isPathPatternMatch = false

		// Check path pattern match if specified
		if !isNilOrEmpty(pattern.PathPatterns) {
			isPathPatternMatch = matchPath(pattern, path)
		}

		// Check filename match if file_names are specified
		if !isNilOrEmpty(pattern.Filenames) {
			isFilenameMatch = matchFilename(pattern, path)
		}

		// Check file extension match if file_extensions are specified
		if !isNilOrEmpty(pattern.FileExtensions) {
			isFileExtensionMatch = matchFileExtension(pattern, path)
		}

		if (!isNilOrEmpty(pattern.Filenames) && !isFilenameMatch) ||
			(!isNilOrEmpty(pattern.FileExtensions) && !isFileExtensionMatch) ||
			(!isNilOrEmpty(pattern.PathPatterns) && !isPathPatternMatch) {
			continue
		}

		// If content_patterns are specified, check content match
		if !isNilOrEmpty(pattern.ContentPatterns) {
			isContentPatternMatch = false
			for _, contentPatternRegex := range pattern.ContentPatternRegexs {
				if contentPatternRegex.MatchString(content) {
					isContentPatternMatch = true
					break
				}
			}
			if !isContentPatternMatch {
				continue // Content pattern didn't match; skip to next pattern
			}
		}

		matches = append(matches, createMatch(pattern, path, repoName))
	}
	return matches, nil
}

func createMatch(pattern Pattern, path string, repoName string) core.Finding {
	return core.Finding{
		Name:       pattern.Name,
		Type:       pattern.Type,
		Category:   pattern.Category,
		Properties: copyProperties(pattern.Properties),
		Path:       path,
		RepoName:   repoName,
	}
}
