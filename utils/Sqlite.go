package utils

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"github.com/reaandrew/techdetector/core"
)

// Predefined fields that will always be stored in the findings table
var PredefinedFieldsSlice = []string{"Name", "Type", "Category", "Path", "RepoName"}

// InitializeSQLiteDB creates the findings table if it doesn't exist
func InitializeSQLiteDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database: %w", err)
	}

	createStmt := `CREATE TABLE IF NOT EXISTS findings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		Name TEXT,
		Type TEXT,
		Category TEXT,
		Path TEXT,
		RepoName TEXT,
		Properties TEXT
	);`

	if _, err := db.Exec(createStmt); err != nil {
		return nil, fmt.Errorf("failed to create findings table: %w", err)
	}

	return db, nil
}

// ProcessFindingsIncrementally iterates over findings and inserts them directly into SQLite
func ProcessFindingsIncrementally(db *sql.DB, repo core.FindingRepository) error {
	iterator := repo.NewIterator()
	for iterator.HasNext() {
		set, _ := iterator.Next()
		for _, finding := range set.Matches {
			flattenedProps := flattenProperties(finding.Properties)
			finding.Properties = flattenedProps

			if err := InsertFinding(db, finding); err != nil {
				log.Printf("Error inserting finding: %v", err)
			}
		}
	}
	return nil
}

// InsertFinding inserts a single finding into the SQLite database
func InsertFinding(db *sql.DB, finding core.Finding) error {
	fields := []string{"Name", "Type", "Category", "Path", "RepoName", "Properties"}
	placeholders := []string{"?", "?", "?", "?", "?", "?"}
	args := []interface{}{
		finding.Name,
		finding.Type,
		finding.Category,
		finding.Path,
		finding.RepoName,
	}

	// Convert properties to JSON
	jsonProperties, err := json.Marshal(finding.Properties)
	if err != nil {
		log.Printf("Failed to marshal properties for finding '%s': %v", finding.Name, err)
		args = append(args, "{}")
	} else {
		args = append(args, string(jsonProperties))
	}

	insertStmt := fmt.Sprintf(
		"INSERT INTO findings (%s) VALUES (%s);",
		strings.Join(fields, ", "),
		strings.Join(placeholders, ", "),
	)

	_, err = db.Exec(insertStmt, args...)
	if err != nil {
		return fmt.Errorf("failed to insert finding '%s': %w", finding.Name, err)
	}

	return nil
}

// Flatten properties by converting nested structures to JSON strings
func flattenProperties(properties map[string]interface{}) map[string]interface{} {
	flattened := make(map[string]interface{})
	for key, value := range properties {
		if isPredefinedField(key) {
			continue
		}

		switch v := value.(type) {
		case map[string]interface{}:
			jsonBytes, err := json.Marshal(v)
			if err != nil {
				log.Printf("Failed to marshal nested map for key '%s': %v", key, err)
				flattened[key] = nil
			} else {
				flattened[key] = string(jsonBytes)
			}
		default:
			flattened[key] = value
		}
	}
	return flattened
}

// Check if a field is predefined
func isPredefinedField(key string) bool {
	for _, field := range PredefinedFieldsSlice {
		if strings.EqualFold(key, field) {
			return true
		}
	}
	return false
}
