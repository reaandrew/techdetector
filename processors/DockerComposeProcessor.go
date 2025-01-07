package processors

import (
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/reaandrew/techdetector/core"
)

// DockerComposeService is a minimal struct to parse just enough from a docker-compose file.
type DockerComposeService struct {
	Image string `yaml:"image,omitempty"`
}

// DockerComposeFile represents the top-level structure of a docker-compose.yml file.
type DockerComposeFile struct {
	Services map[string]DockerComposeService `yaml:"services,omitempty"`
}

// DockerComposeProcessor scans Docker Compose files (docker-compose.yml / .yaml)
// and reports the discovered services/images.
type DockerComposeProcessor struct {
}

// Supports checks if the file is named (or symlinked) as a docker-compose file.
func (d DockerComposeProcessor) Supports(filePath string) bool {
	base := filepath.Base(filePath)
	lower := strings.ToLower(base)
	// Common Docker Compose filenames
	return lower == "docker-compose.yml" || lower == "docker-compose.yaml" ||
		lower == "compose.yml" || lower == "compose.yaml"
}

func (d DockerComposeProcessor) Process(path string, repoName string, content string) ([]core.Finding, error) {
	var compose DockerComposeFile
	err := yaml.Unmarshal([]byte(content), &compose)
	if err != nil {
		return nil, fmt.Errorf("failed to parse docker-compose file '%s': %w", path, err)
	}

	var matches []core.Finding
	for serviceName, service := range compose.Services {
		// Create a Finding for each service
		matches = append(matches, core.Finding{
			Name:     serviceName,
			Type:     "Docker Compose Service",
			Category: "",
			Properties: map[string]interface{}{
				"image": service.Image,
			},
			Path:     path,
			RepoName: repoName,
		})
	}

	return matches, nil
}
