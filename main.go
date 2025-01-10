package main

import (
	"log"
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
	cli := &Cli{}
	if err := cli.Execute(); err != nil {
		log.Fatalf("Error executing command: %v", err)
	}
}
