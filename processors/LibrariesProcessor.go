package processors

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/reaandrew/techdetector/core"
	"path/filepath"
	"regexp"
	"strings"
)

type Library struct {
	Name     string `json:"library_name"`
	Language string `json:"language"`
	Version  string `json:"version"`
}

type LibrariesProcessor struct {
	csprojPatterns []*regexp.Regexp
}

func NewLibrariesProcessor() *LibrariesProcessor {
	return &LibrariesProcessor{}
}

func (mp *LibrariesProcessor) Supports(filePath string) bool {
	base := filepath.Base(filePath)
	supportedFiles := []string{
		"pom.xml",          // Java (Maven)
		"build.gradle",     // Java (Gradle) - Optional
		"go.mod",           // Go
		"package.json",     // Node.js
		"requirements.txt", // Python
		"pyproject.toml",   // Python
		"*.csproj",         // C#
	}

	for _, pattern := range supportedFiles {
		matched, err := filepath.Match(pattern, base)
		if err != nil {
			continue
		}
		if matched {
			return true
		}
	}
	return false
}

func (mp *LibrariesProcessor) Process(path string, repoName string, content string) ([]core.Finding, error) {
	var matches []core.Finding
	base := filepath.Base(path)

	switch base {
	case "pom.xml":
		fs, err := mp.parsePomXML(content, repoName, path)
		if err != nil {
			return nil, fmt.Errorf("failed to parse pom.xml: %w", err)
		}
		matches = append(matches, fs...)
	case "go.mod":
		fs, err := mp.parseGoMod(content, repoName, path)
		if err != nil {
			return nil, fmt.Errorf("failed to parse go.mod: %w", err)
		}
		matches = append(matches, fs...)
	case "package.json":
		fs, err := mp.parsePackageJSON(content, repoName, path)
		if err != nil {
			return nil, fmt.Errorf("failed to parse package.json: %w", err)
		}
		matches = append(matches, fs...)
	case "requirements.txt":
		fs, err := mp.parseRequirementsTXT(content, repoName, path)
		if err != nil {
			return nil, fmt.Errorf("failed to parse requirements.txt: %w", err)
		}
		matches = append(matches, fs...)
	case "pyproject.toml":
		fs, err := mp.parsePyProjectToml(content, repoName, path)
		if err != nil {
			return nil, fmt.Errorf("failed to parse pyproject.toml: %w", err)
		}
		matches = append(matches, fs...)
	default:
		if strings.HasSuffix(base, ".csproj") {
			fs, err := mp.parseCsProj(content, repoName, path)
			if err != nil {
				return nil, fmt.Errorf("failed to parse .csproj: %w", err)
			}
			matches = append(matches, fs...)
		} else {
			return nil, errors.New("unsupported package file")
		}
	}

	return matches, nil
}

func (mp *LibrariesProcessor) parsePomXML(content string, repoName string, path string) ([]core.Finding, error) {
	type Dependency struct {
		GroupID    string `xml:"groupId"`
		ArtifactID string `xml:"artifactId"`
		Version    string `xml:"version"`
	}

	type Project struct {
		XMLName      xml.Name     `xml:"project"`
		Dependencies []Dependency `xml:"dependencies>dependency"`
	}

	var project Project
	err := xml.Unmarshal([]byte(content), &project)
	if err != nil {
		return nil, err
	}

	var matches []core.Finding
	for _, dep := range project.Dependencies {
		libraryName := fmt.Sprintf("%s:%s", dep.GroupID, dep.ArtifactID)
		match := core.Finding{
			Name:     libraryName,
			Report:   "Library",
			Category: "",
			Properties: map[string]interface{}{
				"Language": "Java",
				"Version":  dep.Version,
			},
			Path:     path,
			RepoName: repoName,
		}
		matches = append(matches, match)
	}

	return matches, nil
}

func (mp *LibrariesProcessor) parseGoMod(content string, repoName string, path string) ([]core.Finding, error) {
	lines := strings.Split(content, "\n")
	var matches []core.Finding
	var inRequireBlock bool

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Check for start and end of "require" block
		if strings.HasPrefix(line, "require (") {
			inRequireBlock = true
			continue
		}
		if inRequireBlock && strings.HasPrefix(line, ")") {
			inRequireBlock = false
			continue
		}

		if inRequireBlock || strings.HasPrefix(line, "require ") {
			// Remove the "require" keyword for single-line entries
			if !inRequireBlock {
				line = strings.TrimPrefix(line, "require")
				line = strings.TrimSpace(line)
			}

			// Split the line to extract library name and version
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				libraryName := parts[0]
				version := parts[1]
				match := core.Finding{
					Name:     libraryName,
					Report:   "Library",
					Category: "",
					Properties: map[string]interface{}{
						"Language": "Go",
						"Version":  version,
					},
					Path:     path,
					RepoName: repoName,
				}
				matches = append(matches, match)
			}
		}
	}

	return matches, nil
}

func (mp *LibrariesProcessor) parsePackageJSON(content string, repoName string, path string) ([]core.Finding, error) {
	type PackageJSON struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}

	var pkg PackageJSON
	err := json.Unmarshal([]byte(content), &pkg)
	if err != nil {
		return nil, err
	}

	var matches []core.Finding

	combined := make(map[string]string)
	for k, v := range pkg.Dependencies {
		combined[k] = v
	}
	for k, v := range pkg.DevDependencies {
		combined[k] = v
	}

	for lib, ver := range combined {
		match := core.Finding{
			Name:     lib,
			Report:   "Library",
			Category: "",
			Properties: map[string]interface{}{
				"Language": "Node.js",
				"Version":  ver,
			},
			Path:     path,
			RepoName: repoName,
		}
		matches = append(matches, match)
	}

	return matches, nil
}

func (mp *LibrariesProcessor) parseRequirementsTXT(content string, repoName string, path string) ([]core.Finding, error) {
	lines := strings.Split(content, "\n")
	matches := make([]core.Finding, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		var libraryName, version string
		specifiers := []string{"==", ">=", "<=", "~=", ">", "<"}
		found := false

		for _, spec := range specifiers {
			if strings.Contains(line, spec) {
				parts := strings.SplitN(line, spec, 2)
				libraryName = strings.TrimSpace(parts[0])
				version = spec + strings.TrimSpace(parts[1])
				found = true
				break
			}
		}

		if !found {
			libraryName = line
			version = "N/A"
		}
		match := core.Finding{
			Name:     libraryName,
			Report:   "Library",
			Category: "",
			Properties: map[string]interface{}{
				"Language": "Python",
				"Version":  version,
			},
			Path:     path,
			RepoName: repoName,
		}
		matches = append(matches, match)
	}

	return matches, nil
}

func (mp *LibrariesProcessor) parsePyProjectToml(content string, repoName string, path string) ([]core.Finding, error) {
	type PyProject struct {
		Tool struct {
			Poetry struct {
				Dependencies    map[string]interface{} `toml:"dependencies"`
				DevDependencies map[string]interface{} `toml:"dev-dependencies"`
			} `toml:"poetry"`
		} `toml:"tool"`
	}

	var py PyProject
	if _, err := toml.Decode(content, &py); err != nil {
		return nil, err
	}

	var matches []core.Finding

	combined := make(map[string]string)
	for k, v := range py.Tool.Poetry.Dependencies {
		if ver, ok := v.(string); ok {
			combined[k] = ver
		}
	}
	for k, v := range py.Tool.Poetry.DevDependencies {
		if ver, ok := v.(string); ok {
			combined[k] = ver
		}
	}

	for lib, ver := range combined {
		match := core.Finding{
			Name:     lib,
			Report:   "Library",
			Category: "",
			Properties: map[string]interface{}{
				"Language": "Python",
				"Version":  ver,
			},
			Path:     path,
			RepoName: repoName,
		}
		matches = append(matches, match)
	}

	return matches, nil
}

func (mp *LibrariesProcessor) parseCsProj(content string, repoName string, path string) ([]core.Finding, error) {
	type PackageReference struct {
		Include string `xml:"Include,attr"`
		Version string `xml:"Version,attr"`
	}

	type Reference struct {
		Include string `xml:"Include,attr"`
		Version string `xml:"Version,attr,omitempty"` // Version is optional
	}

	type Project struct {
		XMLName           xml.Name           `xml:"Project"`
		PackageReferences []PackageReference `xml:"ItemGroup>PackageReference"`
		References        []Reference        `xml:"ItemGroup>Reference"`
	}

	var project Project
	err := xml.Unmarshal([]byte(content), &project)
	if err != nil {
		return nil, err
	}

	var matches []core.Finding

	for _, pr := range project.PackageReferences {
		if strings.TrimSpace(pr.Include) == "" {
			continue
		}

		libraryName := pr.Include
		version := pr.Version

		if strings.TrimSpace(version) == "" {
			version = "N/A" // Or any default value you prefer
		}

		match := core.Finding{
			Name:     libraryName,
			Report:   "Library",
			Category: "",
			Properties: map[string]interface{}{
				"Language": "C#",
				"Version":  version,
			},
			Path:     path,
			RepoName: repoName,
		}
		matches = append(matches, match)
	}

	for _, ref := range project.References {
		if strings.TrimSpace(ref.Include) == "" {
			continue
		}

		libraryName, embeddedVersion := ParseReferenceInclude(ref.Include)
		version := embeddedVersion
		if strings.TrimSpace(version) == "" {
			version = ref.Version // Use Version attribute if present
		}
		if strings.TrimSpace(version) == "" {
			version = "N/A" // Default if both are missing
		}
		match := core.Finding{
			Name:     libraryName,
			Report:   "Library",
			Category: "",
			Properties: map[string]interface{}{
				"Language": "C#",
				"Version":  version,
			},
			Path:     path,
			RepoName: repoName,
		}
		matches = append(matches, match)
	}

	return matches, nil
}

func ParseReferenceInclude(include string) (string, string) {
	// Regular expression to match key-value pairs
	re := regexp.MustCompile(`\s*,\s*`) // Split by comma and optional whitespace

	// Split the Include string by commas
	parts := re.Split(include, -1)

	// The first part is always the library name
	libraryName := strings.TrimSpace(parts[0])

	// Initialize version as empty
	version := ""

	// Iterate over the remaining parts to find the Version
	for _, part := range parts[1:] {
		if strings.HasPrefix(part, "Version=") {
			version = strings.TrimPrefix(part, "Version=")
			version = strings.TrimSpace(version)
			break
		}
	}

	return libraryName, version
}
