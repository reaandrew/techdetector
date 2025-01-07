package processors

import (
	"github.com/reaandrew/techdetector/core"
	"reflect"
	"testing"
)

func TestDockerProcessor_Supports(t *testing.T) {
	d := DockerProcessor{}

	testCases := []struct {
		filePath string
		expected bool
	}{
		{"Dockerfile", true},
		{"path/to/Dockerfile", true},
		{"Dockerfile.dev", true},
		{"Dockerfile.prod", true},
		{"Dockerfile.", true},
		{"dockerfile", false},
		{"Dockerfile_backup", false},
		{"NotADockerfile", false},
		{"path/to/NotADockerfile", false},
	}

	for _, tc := range testCases {
		result := d.Supports(tc.filePath)
		if result != tc.expected {
			t.Errorf("Supports(%q) = %v; want %v", tc.filePath, result, tc.expected)
		}
	}
}

func TestDockerProcessor_Process(t *testing.T) {
	d := DockerProcessor{}

	path := "Dockerfile"
	repoName := "test-repo"
	content := `
# Sample Dockerfile
FROM ubuntu:20.04

# Install packages
RUN apt-get update && apt-get install -y \
    curl \
    wget

# Set environment variable
ENV APP_ENV=production

# Expose port
EXPOSE 8080

# Define volume
VOLUME /data

# Set user
USER appuser

# Entry point
ENTRYPOINT ["bash", "-c", "echo Hello World"]
`

	expectedMatches := []core.Finding{
		{
			Name:     "FROM",
			Type:     "Docker Directive",
			Category: "",
			Properties: map[string]interface{}{
				"arguments": "ubuntu:20.04",
			},
			Path:     path,
			RepoName: repoName,
		},
		{
			Name:     "EXPOSE",
			Type:     "Docker Directive",
			Category: "",
			Properties: map[string]interface{}{
				"arguments": "8080",
			},
			Path:     path,
			RepoName: repoName,
		},
	}

	matches, err := d.Process(path, repoName, content)
	if err != nil {
		t.Fatalf("Process returned an error: %v", err)
	}

	if !reflect.DeepEqual(matches, expectedMatches) {
		t.Errorf("Process returned unexpected matches.\nGot:\n%v\nExpected:\n%v", matches, expectedMatches)
	}
}
