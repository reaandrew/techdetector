package reporters

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/reaandrew/techdetector/core"
	"github.com/reaandrew/techdetector/utils"

	_ "github.com/mattn/go-sqlite3" // Import the SQLite driver
)

const (
	DefaultJsonReport        = "cloud_services_report.json"
	DefaultJsonSummaryReport = "cloud_services_summary.json"
)

type JsonReporter struct {
	Queries          core.SqlQueries
	ArtifactPrefix   string
	SqliteDBFilename string
	OutputDir        string
}

func (j *JsonReporter) setDefaultOutputDir() {
	if j.OutputDir == "" {
		j.OutputDir = "."
	}
}

// Report generates both detailed and summary JSON reports
func (j JsonReporter) Report(repository core.FindingRepository) error {
	dbPath := fmt.Sprintf("/tmp/%s_%s", j.ArtifactPrefix, j.SqliteDBFilename)

	// Initialize SQLite database
	db, err := utils.InitializeSQLiteDB(dbPath)
	if err != nil {
		return fmt.Errorf("failed to initialize SQLite database: %w", err)
	}
	defer db.Close()

	// Process findings incrementally and store them in SQLite
	err = utils.ProcessFindingsIncrementally(db, repository)
	if err != nil {
		return fmt.Errorf("failed to process findings: %w", err)
	}

	// Generate the detailed JSON report
	if err := j.generateDetailedReport(repository); err != nil {
		return fmt.Errorf("failed to generate detailed JSON report: %w", err)
	}

	// Check if Queries.Queries is populated
	if len(j.Queries.Queries) == 0 {
		log.Println("Warning: No SQL queries defined for summary report.")
	} else {
		log.Printf("Number of summary queries to execute: %d\n", len(j.Queries.Queries))
	}

	// Generate the summary JSON report
	if err := j.generateSummaryReport(dbPath); err != nil {
		return fmt.Errorf("failed to generate summary JSON report: %w", err)
	}

	return nil
}

// generateDetailedReport creates a detailed JSON report of all findings
func (j JsonReporter) generateDetailedReport(repository core.FindingRepository) error {
	j.setDefaultOutputDir()

	// Create the full path for the output file
	outputFilePath := fmt.Sprintf("%s/%s_%s", j.OutputDir, j.ArtifactPrefix, DefaultJsonReport)

	outputFile, err := os.Create(outputFilePath)

	if err != nil {
		return fmt.Errorf("failed to create detailed output file: %v", err)
	}
	defer outputFile.Close()

	iterator := repository.NewIterator()
	for iterator.HasNext() {
		match, err := iterator.Next()
		if err != nil {
			return fmt.Errorf("failed to retrieve next finding: %w", err)
		}

		jsonBytes, err := json.Marshal(match)
		if err != nil {
			return fmt.Errorf("failed to marshal finding to JSON: %w", err)
		}

		_, err = outputFile.Write(jsonBytes)
		if err != nil {
			return fmt.Errorf("failed to write to detailed output file: %v", err)
		}
		_, err = outputFile.WriteString("\n") // Add newline after each JSON object
		if err != nil {
			return fmt.Errorf("failed to write newline to detailed output file: %v", err)
		}
	}

	fmt.Printf("Detailed JSON report generated successfully: %s\n", outputFile.Name())
	return nil
}

// generateSummaryReport executes SQL queries and creates a summary JSON report
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

	j.setDefaultOutputDir()

	// Create the full path for the summary output file
	outputFilePath := fmt.Sprintf("%s/%s_%s", j.OutputDir, j.ArtifactPrefix, DefaultJsonSummaryReport)

	outputFile, err := os.Create(outputFilePath)

	if err != nil {
		return fmt.Errorf("failed to create summary JSON output file: %w", err)
	}
	defer outputFile.Close()

	summaryData := make(map[string]interface{})

	for _, query := range j.Queries.Queries {
		log.Printf("Executing query: %s\n", query.Query)
		results, err := executeSQLQuery(db, query.Query)
		if err != nil {
			log.Printf("Skipping query for '%s': %v", query.Name, err)
			continue
		}
		log.Printf("Query '%s' returned %d results.\n", query.Name, len(results))
		if len(results) == 0 {
			log.Printf("Warning: Query '%s' returned no results.\n", query.Name)
		}
		summaryData[query.Name] = results
	}

	// Debug: Print summaryData before marshaling
	log.Printf("Summary Data: %+v\n", summaryData)

	// Write summary data to JSON file
	summaryBytes, err := json.MarshalIndent(summaryData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal summary data: %w", err)
	}

	// Debug: Print the marshaled JSON string
	log.Printf("Marshaled Summary JSON: %s\n", string(summaryBytes))

	_, err = outputFile.Write(summaryBytes)
	if err != nil {
		return fmt.Errorf("failed to write to summary output file: %v", err)
	}

	fmt.Printf("Summary JSON report generated successfully: %s\n", outputFile.Name())
	return nil
}

// executeSQLQuery runs a SQL query and returns the results as a slice of maps
func executeSQLQuery(db *sql.DB, query string) ([]map[string]interface{}, error) {
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query '%s': %w", query, err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve columns for query '%s': %w", query, err)
	}

	var results []map[string]interface{}

	for rows.Next() {
		columnValues := make([]interface{}, len(columns))
		columnPointers := make([]interface{}, len(columns))

		for i := range columnValues {
			columnPointers[i] = &columnValues[i]
		}

		if err := rows.Scan(columnPointers...); err != nil {
			return nil, fmt.Errorf("failed to scan row for query '%s': %w", query, err)
		}

		rowData := make(map[string]interface{})
		for i, colName := range columns {
			value := columnValues[i]

			// Convert []byte to string for text columns
			if b, ok := value.([]byte); ok {
				rowData[colName] = string(b)
			} else {
				rowData[colName] = value
			}
		}

		results = append(results, rowData)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error for query '%s': %w", query, err)
	}

	return results, nil
}
