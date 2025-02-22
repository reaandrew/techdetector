package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"gopkg.in/yaml.v3"

	"github.com/aws/aws-lambda-go/events"
	"github.com/reaandrew/techdetector/core"
	"github.com/reaandrew/techdetector/processors"
	"github.com/reaandrew/techdetector/reporters"
	"github.com/reaandrew/techdetector/repositories"
	"github.com/reaandrew/techdetector/scanners"

	"log"
	"os"
)

// LambdaRequest represents the expected JSON structure in the request body
type LambdaRequest struct {
	Repo   string `json:"repo"`
	Cutoff string `json:"cutoff"`
}

// LambdaResponse represents the structure of the response
type LambdaResponse struct {
	StatusCode      int               `json:"statusCode"`
	Headers         map[string]string `json:"headers,omitempty"`
	Body            string            `json:"body,omitempty"`
	IsBase64Encoded bool              `json:"isBase64Encoded"`
}

// Handler is the Lambda function handler
func Handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	//log.Println("Received request:", request)
	//
	//// Step 1: Authenticate the request
	//authHeader := request.Headers["authorization"]
	//if authHeader == "" {
	//	log.Println("Missing Authorization header")
	//	return toAPIGatewayResponse(401, `{"error": "Missing Authorization header."}`), nil
	//}
	//
	//// Expecting format: "Bearer tdt_<userid>_<token>"
	//parts := strings.SplitN(authHeader, " ", 2)
	//if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
	//	log.Println("Invalid Authorization header format")
	//	return toAPIGatewayResponse(401, `{"error": "Invalid Authorization header format."}`), nil
	//}
	//
	//token := parts[1]
	//if !strings.HasPrefix(token, "tdt_") {
	//	log.Println("Invalid token prefix")
	//	return toAPIGatewayResponse(401, `{"error": "Invalid token format."}`), nil
	//}
	//
	//// Split the token to extract userid and token
	//tokenParts := strings.SplitN(token, "_", 3)
	//if len(tokenParts) != 3 {
	//	log.Println("Invalid token structure")
	//	return toAPIGatewayResponse(401, `{"error": "Invalid token structure."}`), nil
	//}
	//
	//userID := strings.TrimSpace(tokenParts[1])
	//providedToken := strings.TrimSpace(tokenParts[2])
	//storedToken, err := getStoredToken(ctx, userID)
	//expectedToken := fmt.Sprintf("tdt_%s_%s", userID, providedToken)
	//
	////log.Printf("Provided Token: '%s'", providedToken)
	////log.Printf("Stored Token: '%s'", storedToken)
	//
	//if err != nil {
	//	log.Printf("Error retrieving token for user '%s': %v", userID, err)
	//	// To prevent user enumeration, return a generic error message
	//	return toAPIGatewayResponse(401, `{"error": "Unauthorized."}`), nil
	//}
	//
	//// Step 3: Compare the provided token with the stored token
	//if expectedToken != storedToken {
	//	log.Printf("Token mismatch for user '%s'", userID)
	//	return toAPIGatewayResponse(401, `{"error": "Unauthorized."}`), nil
	//}

	// Step 4: Parse the request body
	var lambdaReq LambdaRequest
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

	fmt.Printf("Cut off: %s", lambdaReq.Cutoff)

	// Step 5: Perform the scan
	jsonReport, err := ScanRepo(lambdaReq.Repo, "/var/task/queries.yaml", "techdetector", lambdaReq.Cutoff)
	if err != nil {
		log.Printf("Error scanning repository: %v", err)
		errorBody, _ := json.Marshal(map[string]string{"error": err.Error()})
		return toAPIGatewayResponse(500, string(errorBody)), nil
	}

	// Successful response with the JSON report
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

// getStoredToken retrieves the stored token for a given userID from SSM Parameter Store
func getStoredToken(ctx context.Context, userID string) (string, error) {
	// Load the AWS configuration
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return "", fmt.Errorf("unable to load AWS SDK config: %w", err)
	}

	// Create an SSM client
	svc := ssm.NewFromConfig(cfg)

	// Retrieve the SSM parameter prefix from environment variables
	paramPrefix := os.Getenv("SSM_PARAMETER_PREFIX")
	if paramPrefix == "" {
		return "", fmt.Errorf("SSM_PARAMETER_PREFIX environment variable is not set")
	}

	// Construct the parameter name for the given userID
	paramName := fmt.Sprintf("%s%s", paramPrefix, userID)

	// Fetch the parameter value (token)
	input := &ssm.GetParameterInput{
		Name:           aws.String(paramName),
		WithDecryption: aws.Bool(true),
	}

	result, err := svc.GetParameter(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve parameter '%s': %w", paramName, err)
	}

	if result.Parameter == nil || result.Parameter.Value == nil {
		return "", fmt.Errorf("parameter '%s' has no value", paramName)
	}

	return *result.Parameter.Value, nil
}

// ScanRepo performs the repository scan and returns the JSON report
func ScanRepo(repoURL string, queriesPath string, prefix string, cutoff string) (string, error) {
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
		cutoff,
	)

	scanner.Scan(repoURL, "json")

	// Read the generated detailed JSON report
	reportFilePath := fmt.Sprintf("/tmp/%s_%s", prefix, reporters.DefaultJsonSummaryReport)
	log.Printf("Attempting to read detailed JSON report from: %s", reportFilePath)
	reportData, err := os.ReadFile(reportFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to read JSON report: %v", err)
	}

	// Log the size of the report for debugging
	log.Printf("Read JSON report of size: %d bytes", len(reportData))

	// Return the full JSON report as a string
	return string(reportData), nil
}

// loadQueries loads SQL queries from a YAML file
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

// createJSONReporter initializes a JsonReporter
func createJSONReporter(queries core.SqlQueries, prefix string) (core.Reporter, error) {
	return reporters.JsonReporter{
		Queries:          queries,
		ArtifactPrefix:   prefix,
		SqliteDBFilename: "findings.db",
		OutputDir:        "/tmp",
	}, nil
}
