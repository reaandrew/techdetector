package main

import (
	"github.com/aws/aws-lambda-go/lambda"
	"log"
	"os"
)

const (
	MaxWorkers        = 10
	MaxFileWorkers    = 10
	CloneBaseDir      = "/tmp/techdetector" // You can make this configurable if needed
	XlsxSummaryReport = "summary_report.xlsx"
	XlsxSQLiteDB      = "findings.db"
)

var Version string

func main() {
	if _, exists := os.LookupEnv("AWS_LAMBDA_FUNCTION_NAME"); exists {
		// Running in Lambda mode
		log.Println("Starting in Lambda mode")
		lambda.Start(Handler)
	} else {
		// Running in CLI mode
		log.Println("Starting in CLI mode")
		cli := &Cli{}
		if err := cli.Execute(); err != nil {
			log.Fatalf("Error executing command: %v", err)
		}
	}
}
