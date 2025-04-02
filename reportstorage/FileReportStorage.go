package reportstorage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"

	_ "github.com/mattn/go-sqlite3"
	"github.com/reaandrew/techdetector/core"
)

type FileReportStorage struct {
	Queries          core.SqlQueries
	ArtifactPrefix   string
	SqliteDBFilename string
	OutputDir        string
}

const defaultJsonSummaryReport = "cloud_services_summary.json"

func (s *FileReportStorage) setDefaultOutputDir() {
	if s.OutputDir == "" {
		s.OutputDir = "."
	}
}

func (s FileReportStorage) Store(data []byte) error {
	s.setDefaultOutputDir()

	db, err := sql.Open("sqlite3", s.SqliteDBFilename)
	if err != nil {
		return fmt.Errorf("failed to open SQLite database: %w", err)
	}
	defer db.Close()

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM Findings").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to count records in Findings table: %w", err)
	}
	log.Printf("Total records in Findings table: %d\n", count)
	if count == 0 {
		log.Println("No records found in the Findings table. Summary report will be empty.")
	}

	if len(s.Queries.Queries) == 0 {
		log.Println("Warning: No SQL queries defined for summary report.")
		return nil
	}

	outputFilePath := fmt.Sprintf("%s/%s_%s", s.OutputDir, s.ArtifactPrefix, defaultJsonSummaryReport)
	outputFile, err := os.Create(outputFilePath)
	if err != nil {
		return fmt.Errorf("failed to create summary JSON output file: %w", err)
	}
	defer outputFile.Close()

	summaryData := make(map[string]interface{})

	for _, query := range s.Queries.Queries {
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

	cleanedSummaryData := removeNilValues(summaryData)
	summaryBytes, err := json.MarshalIndent(cleanedSummaryData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal summary data: %w", err)
	}

	_, err = outputFile.Write(summaryBytes)
	if err != nil {
		return fmt.Errorf("failed to write to summary output file: %v", err)
	}

	return nil
}

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

func removeNilValues(data map[string]interface{}) map[string]interface{} {
	cleanedData := make(map[string]interface{})
	for key, value := range data {
		if value == nil {
			continue
		}
		if slice, ok := value.([]map[string]interface{}); ok && len(slice) == 0 {
			continue
		}
		cleanedData[key] = value
	}
	return cleanedData
}
