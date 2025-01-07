package processors

import (
	"encoding/json"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/reaandrew/techdetector/core"
)

// CloudFormationTemplate is a partial representation of a CloudFormation template.
type CloudFormationTemplate struct {
	AWSTemplateFormatVersion string                            `yaml:"AWSTemplateFormatVersion,omitempty" json:"AWSTemplateFormatVersion,omitempty"`
	Resources                map[string]CloudFormationResource `yaml:"Resources,omitempty" json:"Resources,omitempty"`
}

// CloudFormationResource is a partial representation of a CloudFormation resource.
type CloudFormationResource struct {
	Type       string                 `yaml:"Report,omitempty" json:"Report,omitempty"`
	Properties map[string]interface{} `yaml:"Properties,omitempty" json:"Properties,omitempty"`
}

// CloudFormationProcessor parses files with .yml, .yaml, or .json
// and checks if they appear to be AWS CloudFormation templates.
// It then emits a Finding for each resource.
type CloudFormationProcessor struct {
}

func (c CloudFormationProcessor) Supports(filePath string) bool {
	lower := strings.ToLower(filePath)
	if !(strings.HasSuffix(lower, "template.yml") || strings.HasSuffix(lower, "template.yaml") || strings.HasSuffix(lower, "template.json")) {
		return false
	}
	// Quick heuristic: we might read partial content to see if "AWSTemplateFormatVersion" or "Resources" exist
	// but you can also attempt a parse in Process. We'll just let Process handle that.
	return true
}

func (c CloudFormationProcessor) Process(path string, repoName string, content string) ([]core.Finding, error) {
	// Attempt YAML parse first
	var template CloudFormationTemplate
	yamlErr := yaml.Unmarshal([]byte(content), &template)
	if yamlErr != nil {
		// If YAML fails, attempt JSON parse
		jsonErr := json.Unmarshal([]byte(content), &template)
		if jsonErr != nil {
			// Not recognized as valid YAML or JSON for CloudFormation
			return nil, jsonErr
		}
	}

	// If AWSTemplateFormatVersion is empty, there's a chance
	// it's not a real CF template. We'll just do a quick check:
	if template.AWSTemplateFormatVersion == "" && len(template.Resources) == 0 {
		return nil, nil // Doesn't look like a CloudFormation template
	}

	var matches []core.Finding
	for resourceName, resource := range template.Resources {
		matches = append(matches, core.Finding{
			Name:     resourceName,
			Report:   "CloudFormation Resource",
			Category: "AWS",
			Properties: map[string]interface{}{
				"resource_type": resource.Type,
				"properties":    resource.Properties,
			},
			Path:     path,
			RepoName: repoName,
		})
	}

	return matches, nil
}
