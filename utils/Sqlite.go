package utils

import (
	"database/sql"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"github.com/reaandrew/techdetector/core"
)

// PredefinedFieldsSlice contains the fields that always go in the findings table
var PredefinedFieldsSlice = []string{"Name", "Type", "Category", "Path", "RepoName"}

// InitializeSQLiteDB opens (or creates) the SQLite DB, applies a schema for findings,
// and optionally turns on performance PRAGMAs for faster bulk inserts.
func InitializeSQLiteDB(dbPath string) (*sql.DB, error) {

	DeleteDatabaseFileIfExists(dbPath)

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database: %w", err)
	}

	// ------------------------------------------------------------------
	// Optional performance tweaks, if you’re loading data one-shot:
	// WAL mode is typically faster for concurrent writes:
	_, _ = db.Exec("PRAGMA journal_mode = WAL;")
	// Reduces fsync calls for better performance, but if the system crashes,
	// you may lose the last few transactions:
	_, _ = db.Exec("PRAGMA synchronous = OFF;")
	// ------------------------------------------------------------------

	createStmt := `CREATE TABLE IF NOT EXISTS Findings (
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

func InsertMatches(db *sql.DB, matches []core.Finding) (err error) {
	// Begin a single transaction for all inserts
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	// Ensure we rollback if anything fails or panics
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		} else if err != nil {
			_ = tx.Rollback()
		} else {
			_ = tx.Commit()
		}
	}()

	// Prepare an INSERT statement once, rather than building a string each time
	stmt, err := tx.Prepare(`
		INSERT INTO Findings (Name, Type, Category, Path, RepoName, Properties)
		VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare insert statement: %w", err)
	}
	defer stmt.Close()

	// Walk through all findings from the repository

	for _, finding := range matches {
		flattenedProps := flattenProperties(finding.Properties)
		finding.Properties = flattenedProps

		jsonProps, jErr := json.Marshal(finding.Properties)
		if jErr != nil {
			log.Printf("Failed to marshal properties for finding '%s': %v", finding.Name, jErr)
			jsonProps = []byte("{}")
		}

		_, execErr := stmt.Exec(
			finding.Name,
			finding.Type,
			finding.Category,
			finding.Path,
			finding.RepoName,
			string(jsonProps),
		)
		if execErr != nil {
			return fmt.Errorf("failed to insert finding '%s': %w", finding.Name, execErr)
		}
	}

	return nil
}

// flattenProperties takes a potentially nested properties map, flattens or JSON-encodes
// the nested bits, and returns a top-level map of only strings and scalars.
func flattenProperties(properties map[string]interface{}) map[string]interface{} {
	flattened := make(map[string]interface{})
	for key, value := range properties {
		if isPredefinedField(key) {
			// If the property key is one of the standard columns, skip flattening
			// because we store it in a top-level column anyway
			continue
		}
		switch v := value.(type) {
		case map[string]interface{}:
			// Nested map => store it as a JSON string in flattened
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

// isPredefinedField checks if the key is in the typical top-level columns.
func isPredefinedField(key string) bool {
	for _, field := range PredefinedFieldsSlice {
		if strings.EqualFold(key, field) {
			return true
		}
	}
	return false
}
