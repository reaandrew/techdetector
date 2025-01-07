package processors

import (
	"testing"
)

func TestDockerComposeProcessor_Supports(t *testing.T) {
	processor := DockerComposeProcessor{}

	tests := []struct {
		filePath string
		want     bool
	}{
		{"docker-compose.yml", true},
		{"docker-compose.yaml", true},
		{"compose.yml", true},
		{"compose.yaml", true},
		{"Docker-Compose.yaml", true}, // case-insensitive check
		{"random.yaml", false},
		{"config.yml", false},
		{"docker-compose.txt", false},
	}

	for _, tt := range tests {
		got := processor.Supports(tt.filePath)
		if got != tt.want {
			t.Errorf("Supports(%s) = %v; want %v", tt.filePath, got, tt.want)
		}
	}
}

func TestDockerComposeProcessor_Process(t *testing.T) {
	processor := DockerComposeProcessor{}

	content := `
version: '3.9'
services:
  web:
    image: nginx:latest
  db:
    image: postgres:14
`

	filePath := "docker-compose.yml"
	repoName := "test-repo"

	findings, err := processor.Process(filePath, repoName, content)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(findings) != 2 {
		t.Errorf("Expected 2 findings, got %d", len(findings))
	}

	// We expect one for "web" and one for "db"
	expectedNames := map[string]bool{
		"web": true,
		"db":  true,
	}

	for _, f := range findings {
		if !expectedNames[f.Name] {
			t.Errorf("Unexpected service name: %s", f.Name)
		}
		if f.Type != "Docker Compose Service" {
			t.Errorf("Expected type 'Docker Compose Service', got '%s'", f.Type)
		}
		if f.RepoName != repoName {
			t.Errorf("Expected RepoName '%s', got '%s'", repoName, f.RepoName)
		}
		if f.Path != filePath {
			t.Errorf("Expected Path '%s', got '%s'", filePath, f.Path)
		}
		// Just check "image" property presence
		imageVal, ok := f.Properties["image"]
		if !ok {
			t.Errorf("Expected 'image' property to exist for service %s", f.Name)
		}
		if f.Name == "web" && imageVal != "nginx:latest" {
			t.Errorf("Expected 'nginx:latest' for web service, got '%v'", imageVal)
		}
	}
}

func TestDockerComposeProcessor_Process_InvalidYaml(t *testing.T) {
	processor := DockerComposeProcessor{}

	invalidContent := `
	this is not valid:
  compose: data
  ???

`
	filePath := "docker-compose.yml"
	repoName := "test-repo"

	_, err := processor.Process(filePath, repoName, invalidContent)
	if err == nil {
		t.Error("Expected error due to invalid YAML, got nil")
	}
}
