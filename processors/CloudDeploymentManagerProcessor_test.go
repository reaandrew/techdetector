package processors

import (
	"testing"
)

func TestCloudDeploymentManagerProcessor_Supports(t *testing.T) {
	processor := CloudDeploymentManagerProcessor{}

	tests := []struct {
		filePath string
		want     bool
	}{
		{"deployment.yaml", true},
		{"deployment.YML", true}, // check case insensitivity
		{"main.jinja", true},
		{"main.jinja2", false}, // not strictly in our Supports() check yet
		{"main.tf", false},
		{"template.json", false},
		{"random.txt", false},
	}

	for _, tt := range tests {
		got := processor.Supports(tt.filePath)
		if got != tt.want {
			t.Errorf("Supports(%q) = %v; want %v", tt.filePath, got, tt.want)
		}
	}
}

func TestCloudDeploymentManagerProcessor_Process_Basic(t *testing.T) {
	processor := CloudDeploymentManagerProcessor{}
	filePath := "deployment.yaml"
	repoName := "test-repo"

	// Minimal valid Deployment Manager YAML
	content := `
imports:
  - path: my-template.jinja
    name: MyTemplate
resources:
  - name: my-sample-resource
    type: MyTemplate
    properties:
      zone: us-central1-a
      machineType: n1-standard-1
      image: projects/debian-cloud/global/images/family/debian-10
  - name: my-another-resource
    type: compute.v1.instance
    properties:
      zone: us-central1-b
      tags:
        items:
          - web
`

	findings, err := processor.Process(filePath, repoName, content)
	if err != nil {
		t.Fatalf("Process() returned an error: %v", err)
	}

	// Expect 2 resources
	if len(findings) != 2 {
		t.Fatalf("Expected 2 findings, got %d", len(findings))
	}

	// Validate the first resource
	if findings[0].Name != "my-sample-resource" {
		t.Errorf("Expected Name to be 'my-sample-resource', got %q", findings[0].Name)
	}
	if findings[0].Type != "Deployment Manager Resource" {
		t.Errorf("Expected Type to be 'Deployment Manager Resource', got %q", findings[0].Type)
	}
	if findings[0].Category != "GCP" {
		t.Errorf("Expected Category to be 'GCP', got %q", findings[0].Category)
	}
	if findings[0].RepoName != repoName {
		t.Errorf("Expected RepoName to be %q, got %q", repoName, findings[0].RepoName)
	}
	if findings[0].Path != filePath {
		t.Errorf("Expected Path to be %q, got %q", filePath, findings[0].Path)
	}

	props, ok := findings[0].Properties["properties"].(map[string]interface{})
	if !ok {
		t.Errorf("Expected 'properties' to be a map, got %#v", findings[0].Properties["properties"])
	}
	if props["zone"] != "us-central1-a" {
		t.Errorf("Expected 'zone' property 'us-central1-a', got %v", props["zone"])
	}

	// Validate second resource
	if findings[1].Name != "my-another-resource" {
		t.Errorf("Expected resource name 'my-another-resource', got %q", findings[1].Name)
	}
}

func TestCloudDeploymentManagerProcessor_Process_NotDeploymentManager(t *testing.T) {
	processor := CloudDeploymentManagerProcessor{}

	// YAML but no "resources" key => not a valid DM template
	content := `
someKey:
  - nested: stuff
  - more: data
`

	findings, err := processor.Process("deployment.yaml", "test-repo", content)
	if err != nil {
		t.Fatalf("Did not expect an error, got %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("Expected 0 findings, got %d", len(findings))
	}
}

func TestCloudDeploymentManagerProcessor_Process_InvalidYaml(t *testing.T) {
	processor := CloudDeploymentManagerProcessor{}

	invalidContent := `
	??? invalid: [yaml
`
	findings, err := processor.Process("deployment.yaml", "test-repo", invalidContent)
	if err != nil {
		t.Fatalf("Expected no fatal error (we return nil for parse failure), got %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("Expected 0 findings for invalid YAML, got %d", len(findings))
	}
}
