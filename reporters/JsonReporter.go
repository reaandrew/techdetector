package reporters

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/mattn/go-sqlite3" // Import the SQLite driver
	"github.com/reaandrew/techdetector/core"
	"github.com/reaandrew/techdetector/utils"
	log "github.com/sirupsen/logrus"
)

const (
	DefaultJsonReport        = "cloud_services_report.json"
	DefaultJsonSummaryReport = "cloud_services_summary.json"
)

type JsonReporter struct {
	Queries          core.SqlQueries
	SqliteDBFilename string
	ReportStorage    core.ReportStorage
}

func (j JsonReporter) Report(repository core.FindingRepository) error {
	if len(j.Queries.Queries) == 0 {
		log.Println("Warning: No SQL queries defined for summary report.")
	} else {
		log.Printf("Number of summary queries to execute: %d\n", len(j.Queries.Queries))
	}

	// Generate the summary JSON report
	if err := j.generateSummaryReport(j.SqliteDBFilename); err != nil {
		return fmt.Errorf("failed to generate summary JSON report: %w", err)
	}

	return nil
}

func (j JsonReporter) generateSummaryReport(dbPath string) error {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open SQLite database: %w", err)
	}
	defer db.Close()

	// Verify that the Findings table has data
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM Findings").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to count records in Findings table: %w", err)
	}
	log.Printf("Total records in Findings table: %d\n", count)
	if count == 0 {
		log.Println("No records found in the Findings table. Summary report will be empty.")
	}

	// Check if there are any SQL queries defined
	if len(j.Queries.Queries) == 0 {
		log.Println("Warning: No SQL queries defined for summary report.")
		return nil
	}

	summaryData, err := utils.ExecuteQueries(db, j.Queries.Queries)
	if err != nil {
		// You can decide if this is fatal or if you want to log and continue
		return fmt.Errorf("failed to execute queries: %w", err)
	}

	// Optionally filter out queries with empty (nil) results
	cleanedSummaryData := removeNilOrEmptyResults(summaryData)

	// Write summary data to JSON file
	summaryBytes, err := json.MarshalIndent(cleanedSummaryData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal summary data: %w", err)
	}

	if err = j.ReportStorage.Store(summaryBytes); err != nil {
		return fmt.Errorf("failed to write to summary output file: %v", err)
	}

	return nil
}

// removeNilOrEmptyResults filters out empty query results before writing JSON
func removeNilOrEmptyResults(data map[string][]map[string]interface{}) map[string][]map[string]interface{} {
	cleaned := make(map[string][]map[string]interface{})
	for queryName, rows := range data {
		if rows != nil && len(rows) > 0 {
			cleaned[queryName] = rows
		}
	}
	return cleaned
}
