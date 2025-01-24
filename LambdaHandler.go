package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/reaandrew/techdetector/core"
	"github.com/reaandrew/techdetector/processors"
	"github.com/reaandrew/techdetector/reporters"
	"github.com/reaandrew/techdetector/repositories"
	"github.com/reaandrew/techdetector/scanners"
	"gopkg.in/yaml.v3"
	"log"
	"os"
)

const (
	queriesFilePath = "/var/task/queries.yaml" // Standard path inside Lambda
	prefix          = "techdetector"
)

type LambdaRequest struct {
	Repo string `json:"repo"`
}

type LambdaResponse struct {
	StatusCode      int               `json:"statusCode"`
	Headers         map[string]string `json:"headers,omitempty"`
	Body            string            `json:"body,omitempty"`
	IsBase64Encoded bool              `json:"isBase64Encoded"`
}

// Lambda handler function compatible with AWS Lambda Function URLs
func Handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var lambdaReq LambdaRequest

	// Parse JSON body
	err := json.Unmarshal([]byte(request.Body), &lambdaReq)
	if err != nil {
		log.Printf("Error parsing request body: %v", err)
		return toAPIGatewayResponse(400, `{"error": "Invalid JSON format."}`), nil
	}

	if lambdaReq.Repo == "" {
		errMsg := "The 'repo' field is required in the JSON request."
		log.Println(errMsg)
		return toAPIGatewayResponse(400, fmt.Sprintf(`{"error": "%s"}`, errMsg)), nil
	}

	// Perform the scan
	jsonReport, err := ScanRepo(lambdaReq.Repo, queriesFilePath, prefix)
	if err != nil {
		log.Printf("Error scanning repository: %v", err)
		errorBody, _ := json.Marshal(map[string]string{"error": err.Error()})
		return toAPIGatewayResponse(500, string(errorBody)), nil
	}

	// Successful response
	return toAPIGatewayResponse(200, jsonReport), nil
}

// toAPIGatewayResponse converts LambdaResponse to events.APIGatewayProxyResponse
func toAPIGatewayResponse(statusCode int, body string) events.APIGatewayProxyResponse {
	return events.APIGatewayProxyResponse{
		StatusCode:      statusCode,
		Headers:         map[string]string{"Content-Type": "application/json"},
		Body:            body,
		IsBase64Encoded: false,
	}
}

func ScanRepo(repoURL string, queriesPath string, prefix string) (string, error) {
	queries, err := loadQueries(queriesPath)
	if err != nil {
		return "", fmt.Errorf("failed to load queries: %v", err)
	}

	reporter, err := createJSONReporter(queries, prefix)
	if err != nil {
		return "", fmt.Errorf("failed to create reporter: %v", err)
	}

	repository := repositories.NewFileBasedMatchRepository()
	defer func() {
		if err := repository.Clear(); err != nil {
			log.Fatalf("Error clearing repository: %v", err)
		}
	}()

	scanner := scanners.NewRepoScanner(
		reporter,
		processors.InitializeProcessors(),
		repository,
	)

	scanner.Scan(repoURL, "json")

	jsonReport := fmt.Sprintf(`{"repo": "%s", "status": "Scan completed successfully."}`, repoURL)
	return jsonReport, nil
}

func loadQueries(queriesPath string) (core.SqlQueries, error) {
	var queries core.SqlQueries

	fileData, err := os.ReadFile(queriesPath)
	if err != nil {
		return queries, fmt.Errorf("failed to read YAML file '%s': %w", queriesPath, err)
	}

	err = yaml.Unmarshal(fileData, &queries)
	if err != nil {
		return queries, fmt.Errorf("failed to unmarshal YAML data: %w", err)
	}

	return queries, nil
}

func createJSONReporter(queries core.SqlQueries, prefix string) (core.Reporter, error) {
	return reporters.JsonReporter{
		Queries:          queries,
		ArtifactPrefix:   prefix,
		SqliteDBFilename: "findings.db",
	}, nil
}
