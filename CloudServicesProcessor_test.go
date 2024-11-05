package main

import (
	"testing"
)

// MockCloudServicesLoader is a mock implementation of CloudServicesLoader for testing.
type MockCloudServicesLoader struct {
	Services []CloudService
}

func (m MockCloudServicesLoader) LoadAllCloudServices() ([]CloudService, error) {
	return m.Services, nil
}

// TestCloudServiceProcessor_Supports tests the Supports method of CloudServiceProcessor.
func TestCloudServiceProcessor_Supports(t *testing.T) {
	services := []CloudService{
		{
			CloudVendor:  "AWS",
			CloudService: "S3",
			Language:     "go",
			Reference:    "github.com/aws/aws-sdk-go/service/s3",
		},
		{
			CloudVendor:  "GCP",
			CloudService: "AIPlatform",
			Language:     "py",
			Reference:    "from google.ai import generativelanguage_v1",
		},
	}

	serviceRegexes := compileServicesRegexes(services)
	processor := &CloudServiceProcessor{serviceRegexes: serviceRegexes}

	testCases := []struct {
		filePath string
		expected bool
	}{
		{"main.go", true},
		{"app.py", true},
		{"README.md", false},
		{"service.go", true},
		{"script.js", false},
		{"model.py", true},
		{"deploy.yaml", false},
		{"lib.go", true},
		{"test.PYTHON", false}, // Case-sensitive
		{"setup.Go", false},    // Case-sensitive
		{"configure.go", true},
	}

	for _, tc := range testCases {
		result := processor.Supports(tc.filePath)
		if result != tc.expected {
			t.Errorf("Supports(%s) = %v; want %v", tc.filePath, result, tc.expected)
		}
	}
}

// TestCloudServiceProcessor_Process tests the Process method of CloudServiceProcessor.
func TestCloudServiceProcessor_Process(t *testing.T) {
	services := []CloudService{
		{
			CloudVendor:  "AWS",
			CloudService: "S3",
			Language:     "go",
			Reference:    "github.com/aws/aws-sdk-go/service/s3",
		},
		{
			CloudVendor:  "GCP",
			CloudService: "AIPlatform",
			Language:     "py",
			Reference:    "from google.ai import generativelanguage_v1",
		},
	}

	serviceRegexes := compileServicesRegexes(services)
	processor := &CloudServiceProcessor{serviceRegexes: serviceRegexes}

	testCases := []struct {
		filePath string
		repoName string
		content  string
		expected []Finding
	}{
		{
			filePath: "main.go",
			repoName: "test-repo",
			content:  `import "github.com/aws/aws-sdk-go/service/s3"`,
			expected: []Finding{
				{
					Service: &CloudService{
						CloudVendor:  "AWS",
						CloudService: "S3",
						Language:     "go",
						Reference:    "github.com/aws/aws-sdk-go/service/s3",
					},
					Repository: "test-repo",
					Filepath:   "main.go",
				},
			},
		},
		{
			filePath: "app.py",
			repoName: "test-repo",
			content:  `from google.ai import generativelanguage_v1`,
			expected: []Finding{
				{
					Service: &CloudService{
						CloudVendor:  "GCP",
						CloudService: "AIPlatform",
						Language:     "py",
						Reference:    "from google.ai import generativelanguage_v1",
					},
					Repository: "test-repo",
					Filepath:   "app.py",
				},
			},
		},
		{
			filePath: "README.md",
			repoName: "test-repo",
			content:  `This is a README file.`,
			expected: []Finding{},
		},
		{
			filePath: "service.go",
			repoName: "test-repo",
			content: `// Using AWS S3 client
import "github.com/aws/aws-sdk-go/service/s3"`,
			expected: []Finding{
				{
					Service: &CloudService{
						CloudVendor:  "AWS",
						CloudService: "S3",
						Language:     "go",
						Reference:    "github.com/aws/aws-sdk-go/service/s3",
					},
					Repository: "test-repo",
					Filepath:   "service.go",
				},
			},
		},
		{
			filePath: "script.js",
			repoName: "test-repo",
			content:  `// JavaScript file`,
			expected: []Finding{},
		},
		{
			filePath: "model.py",
			repoName: "test-repo",
			content: `from google.ai import generativelanguage_v1
# Another reference
from google.ai import generativelanguage_v1`,
			expected: []Finding{
				{
					Service: &CloudService{
						CloudVendor:  "GCP",
						CloudService: "AIPlatform",
						Language:     "py",
						Reference:    "from google.ai import generativelanguage_v1",
					},
					Repository: "test-repo",
					Filepath:   "model.py",
				},
				{
					Service: &CloudService{
						CloudVendor:  "GCP",
						CloudService: "AIPlatform",
						Language:     "py",
						Reference:    "from google.ai import generativelanguage_v1",
					},
					Repository: "test-repo",
					Filepath:   "model.py",
				},
			},
		},
	}

	for _, tc := range testCases {
		findings, _ := processor.Process(tc.filePath, tc.repoName, tc.content)
		if len(findings) != len(tc.expected) {
			t.Errorf("Process(%s) = %d findings; want %d", tc.filePath, len(findings), len(tc.expected))
			continue
		}
		for i, finding := range findings {
			expected := tc.expected[i]
			if finding.Service == nil || expected.Service == nil {
				t.Errorf("Finding %d has nil Service; expected %v", i, expected.Service)
				continue
			}
			if *finding.Service != *expected.Service {
				t.Errorf("Finding %d Service = %v; want %v", i, *finding.Service, *expected.Service)
			}
			if finding.Repository != expected.Repository {
				t.Errorf("Finding %d Repository = %s; want %s", i, finding.Repository, expected.Repository)
			}
			if finding.Filepath != expected.Filepath {
				t.Errorf("Finding %d Filepath = %s; want %s", i, finding.Filepath, expected.Filepath)
			}
		}
	}
}

// TestCloudServiceProcessor_WithMockLoader tests CloudServiceProcessor using a mock loader.
func TestCloudServiceProcessor_WithMockLoader(t *testing.T) {
	mockLoader := MockCloudServicesLoader{
		Services: []CloudService{
			{
				CloudVendor:  "AWS",
				CloudService: "S3",
				Language:     "go",
				Reference:    "github.com/aws/aws-sdk-go/service/s3",
			},
			{
				CloudVendor:  "GCP",
				CloudService: "AIPlatform",
				Language:     "py",
				Reference:    "from google.ai import generativelanguage_v1",
			},
		},
	}

	processor := NewServiceProcessor(mockLoader)

	// Verify Supports method
	testCases := []struct {
		filePath string
		expected bool
	}{
		{"service.go", true},
		{"app.py", true},
		{"README.md", false},
	}

	for _, tc := range testCases {
		result := processor.Supports(tc.filePath)
		if result != tc.expected {
			t.Errorf("Supports(%s) = %v; want %v", tc.filePath, result, tc.expected)
		}
	}

	// Verify Process method
	findings, err := processor.Process("service.go", "test-repo", `import "github.com/aws/aws-sdk-go/service/s3"`)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(findings) != 1 {
		t.Errorf("Expected 1 finding, got %d", len(findings))
	}
	if findings[0].Service.CloudService != "S3" {
		t.Errorf("Expected CloudService 'S3', got '%s'", findings[0].Service.CloudService)
	}
}
