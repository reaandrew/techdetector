package main

import (
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	log "github.com/sirupsen/logrus"
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

func setupLogging() {
	// Create or open the error log file
	logFile, err := os.OpenFile("techdetector.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println("Failed to open error log file:", err)
		return
	}

	// Set Logrus output to both stdout and the error log file
	log.SetOutput(logFile)

	// Set log level to capture errors and above (i.e., error, fatal, panic)
	log.SetLevel(log.InfoLevel)

	// Format log output (optional)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})
}

func main() {
	setupLogging()
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
