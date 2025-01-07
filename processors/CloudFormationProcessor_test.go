package processors

import (
	"testing"
)

func TestCloudFormationProcessor_Supports(t *testing.T) {
	processor := CloudFormationProcessor{}

	tests := []struct {
		filePath string
		want     bool
	}{
		{"template.yaml", true},
		{"template.yml", true},
		{"template.json", true},
		{"main.tf", false}, // not CF
		{"docker-compose.yml", false},
		{"config.txt", false},
	}

	for _, tt := range tests {
		got := processor.Supports(tt.filePath)
		if got != tt.want {
			t.Errorf("Supports(%s) = %v; want %v", tt.filePath, got, tt.want)
		}
	}
}

func TestCloudFormationProcessor_Process_YamlSuccess(t *testing.T) {
	processor := CloudFormationProcessor{}
	filePath := "template.yaml"
	repoName := "test-repo"

	content := `
AWSTemplateFormatVersion: "2010-09-09"
Description: A sample template

Resources:
  MyBucket:
    Type: AWS::S3::Bucket
    Properties:
      BucketName: my-sample-bucket
  MyLambda:
    Type: AWS::Lambda::Function
    Properties:
      FunctionName: my-sample-lambda
`

	findings, err := processor.Process(filePath, repoName, content)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(findings) != 2 {
		t.Errorf("Expected 2 resources, got %d", len(findings))
	}

	expectedNames := map[string]bool{
		"MyBucket": true,
		"MyLambda": true,
	}

	for _, f := range findings {
		if !expectedNames[f.Name] {
			t.Errorf("Unexpected resource name: %s", f.Name)
		}
		if f.Type != "CloudFormation Resource" {
			t.Errorf("Expected 'CloudFormation Resource', got '%s'", f.Type)
		}
		resourceType, ok := f.Properties["resource_type"]
		if !ok {
			t.Errorf("Expected 'resource_type' property to exist for resource %s", f.Name)
		}
		if f.Name == "MyBucket" && resourceType != "AWS::S3::Bucket" {
			t.Errorf("Expected 'AWS::S3::Bucket' for MyBucket, got '%v'", resourceType)
		}
		if f.Name == "MyLambda" && resourceType != "AWS::Lambda::Function" {
			t.Errorf("Expected 'AWS::Lambda::Function' for MyLambda, got '%v'", resourceType)
		}
	}
}

func TestCloudFormationProcessor_Process_JsonSuccess(t *testing.T) {
	processor := CloudFormationProcessor{}
	filePath := "template.json"
	repoName := "test-repo"

	content := `
{
  "AWSTemplateFormatVersion": "2010-09-09",
  "Resources": {
    "MyBucket": {
      "Type": "AWS::S3::Bucket",
      "Properties": {
        "BucketName": "my-sample-bucket"
      }
    }
  }
}
`

	findings, err := processor.Process(filePath, repoName, content)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(findings) != 1 {
		t.Errorf("Expected 1 resource, got %d", len(findings))
	}

	if findings[0].Name != "MyBucket" {
		t.Errorf("Expected resource name 'MyBucket', got '%s'", findings[0].Name)
	}
}

func TestCloudFormationProcessor_Process_NotCloudFormation(t *testing.T) {
	processor := CloudFormationProcessor{}
	filePath := "any.yaml"
	repoName := "test-repo"

	// Missing AWSTemplateFormatVersion and Resources
	content := `
someKey:
  nested: thing
`
	findings, err := processor.Process(filePath, repoName, content)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(findings) != 0 {
		t.Errorf("Expected 0 findings, got %d", len(findings))
	}
}

func TestCloudFormationProcessor_Process_InvalidYamlOrJson(t *testing.T) {
	processor := CloudFormationProcessor{}

	invalidYaml := `
	??? This is invalid
`
	_, err := processor.Process("template.yaml", "test-repo", invalidYaml)
	if err != nil {
		t.Logf("Got expected error for invalid YAML: %v", err)
	} else {
		t.Error("Expected an error for invalid YAML, got nil")
	}
}
