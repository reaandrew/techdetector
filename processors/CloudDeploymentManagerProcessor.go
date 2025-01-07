package processors

import (
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/reaandrew/techdetector/core"
)

// CloudDeploymentManagerTemplate is a simplified representation of a Cloud Deployment Manager template.
// Typically, you’ll see keys: "imports", "resources", etc.
type CloudDeploymentManagerTemplate struct {
	Imports   []CDMImport   `yaml:"imports,omitempty"`
	Resources []CDMResource `yaml:"resources,omitempty"`
}

type CDMImport struct {
	Path string `yaml:"path,omitempty"`
	Name string `yaml:"name,omitempty"`
}

type CDMResource struct {
	Name       string                 `yaml:"name,omitempty"`
	Type       string                 `yaml:"type,omitempty"`
	Properties map[string]interface{} `yaml:"properties,omitempty"`
}

// CloudDeploymentManagerProcessor attempts to detect and parse GCP Cloud Deployment Manager templates.
type CloudDeploymentManagerProcessor struct{}

// Supports checks file extensions that are commonly used for Deployment Manager templates.
func (p CloudDeploymentManagerProcessor) Supports(filePath string) bool {
	lower := strings.ToLower(filePath)
	return strings.HasSuffix(lower, "deployment.yaml") ||
		strings.HasSuffix(lower, "deployment.yml") ||
		strings.HasSuffix(lower, "main.jinja")
}

// Process tries to parse the file as YAML (common for Deployment Manager).
// If "resources" is present and non-empty, we produce a Finding for each resource.
func (p CloudDeploymentManagerProcessor) Process(path string, repoName string, content string) ([]core.Finding, error) {
	var template CloudDeploymentManagerTemplate
	err := yaml.Unmarshal([]byte(content), &template)
	if err != nil {
		// If YAML fails to parse at all, return no findings (or error, if you prefer).
		return nil, nil
	}

	// If no resources are present, it might not be a Deployment Manager file.
	if len(template.Resources) == 0 {
		return nil, nil
	}

	var findings []core.Finding

	for _, resource := range template.Resources {
		finding := core.Finding{
			Name:     resource.Name,
			Type:     "Deployment Manager Resource",
			Category: "GCP",
			Properties: map[string]interface{}{
				"resource_type": resource.Type,
				"properties":    resource.Properties,
			},
			Path:     path,
			RepoName: repoName,
		}
		findings = append(findings, finding)
	}

	return findings, nil
}
