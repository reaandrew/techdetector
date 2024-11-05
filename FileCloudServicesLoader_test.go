package main

import (
	"io"
	"log"
	"testing"
	"testing/fstest"
)

func TestFileCloudServicesLoader_LoadAllCloudServices(t *testing.T) {
	mockFS := fstest.MapFS{
		"data/cloud_service_mappings/aws_test.json": &fstest.MapFile{
			Data: []byte(`[
                {
                    "cloud_vendor": "AWS",
                    "cloud_service": "S3",
                    "language": "go",
                    "reference": "github.com/aws/aws-sdk-go/service/s3"
                },
                {
                    "cloud_vendor": "AWS",
                    "cloud_service": "EC2",
                    "language": "python",
                    "reference": "boto3.client('ec2')"
                }
            ]`),
		},
		"data/cloud_service_mappings/gcp_test.json": &fstest.MapFile{
			Data: []byte(`[
                {
                    "cloud_vendor": "GCP",
                    "cloud_service": "AIPlatform",
                    "language": "python",
                    "reference": "from google.ai import generativelanguage_v1"
                }
            ]`),
		},
	}

	loader := NewFileCloudServicesLoader(mockFS)
	services := loader.LoadAllCloudServices()

	expectedCount := 3
	if len(services) != expectedCount {
		t.Errorf("Expected %d services, got %d", expectedCount, len(services))
	}

	expectedServices := []CloudService{
		{
			CloudVendor:  "AWS",
			CloudService: "S3",
			Language:     "go",
			Reference:    "github.com/aws/aws-sdk-go/service/s3",
		},
		{
			CloudVendor:  "AWS",
			CloudService: "EC2",
			Language:     "python",
			Reference:    "boto3.client('ec2')",
		},
		{
			CloudVendor:  "GCP",
			CloudService: "AIPlatform",
			Language:     "python",
			Reference:    "from google.ai import generativelanguage_v1",
		},
	}

	for i, service := range expectedServices {
		if services[i] != service {
			t.Errorf("Expected service %v, got %v", service, services[i])
		}
	}
}

func TestFileCloudServicesLoader_LoadAllCloudServices_WithMalformedJSON(t *testing.T) {
	mockFS := fstest.MapFS{
		"data/cloud_service_mappings/malformed.json": &fstest.MapFile{
			Data: []byte(`[
                {
                    "cloud_vendor": "AWS",
                    "cloud_service": "S3",
                    "language": "go",
                    "reference": "github.com/aws/aws-sdk-go/service/s3",
                } // Trailing comma and missing closing bracket
            `),
		},
	}

	originalOutput := log.Writer()
	defer log.SetOutput(originalOutput)

	log.SetOutput(io.Discard)

	loader := NewFileCloudServicesLoader(mockFS)
	services := loader.LoadAllCloudServices()

	expectedCount := 0
	if len(services) != expectedCount {
		t.Errorf("Expected %d services due to malformed JSON, got %d", expectedCount, len(services))
	}
}

func TestFileCloudServicesLoader_LoadAllCloudServices_WithExtraFiles(t *testing.T) {
	mockFS := fstest.MapFS{
		"data/cloud_service_mappings/aws_test.json": &fstest.MapFile{
			Data: []byte(`[
                {
                    "cloud_vendor": "AWS",
                    "cloud_service": "S3",
                    "language": "go",
                    "reference": "github.com/aws/aws-sdk-go/service/s3"
                }
            ]`),
		},
		"data/cloud_service_mappings/extra.txt": &fstest.MapFile{
			Data: []byte("This is an extra file that should be ignored by the loader."),
		},
	}

	originalOutput := log.Writer()
	defer log.SetOutput(originalOutput)

	log.SetOutput(io.Discard)

	loader := NewFileCloudServicesLoader(mockFS)
	services := loader.LoadAllCloudServices()

	expectedCount := 1 // Only aws_test.json is valid
	if len(services) != expectedCount {
		t.Errorf("Expected %d services, got %d", expectedCount, len(services))
	}
}

func TestCompileServicesRegexes(t *testing.T) {
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
			Language:     "python",
			Reference:    "from google.ai import generativelanguage_v1",
		},
		{
			CloudVendor:  "AWS",
			CloudService: "EC2",
			Language:     "python",
			Reference:    "boto3.client('ec2')",
		},
	}

	serviceRegexes := compileServicesRegexes(services)

	if len(serviceRegexes) != 3 {
		t.Errorf("Expected 3 service regexes, got %d", len(serviceRegexes))
	}

	for i, sre := range serviceRegexes {
		if sre.Service != services[i] {
			t.Errorf("Service at index %d does not match. Got %v, want %v", i, sre.Service, services[i])
		}
		if sre.Regex.String() != services[i].Reference {
			t.Errorf("Regex at index %d does not match. Got %s, want %s", i, sre.Regex.String(), services[i].Reference)
		}
	}
}
