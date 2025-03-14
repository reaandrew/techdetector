package repositories

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"github.com/reaandrew/techdetector/core"
	"log"
	"os"
	"strings"
)

// SqliteFindingRepository implements core.FindingRepository using SQLite
type SqliteFindingRepository struct {
	db *sql.DB
}

// NewSqliteFindingRepository creates a new SQLite-backed repository.
// dbPath is the filename/path for your SQLite database (e.g. "findings.db").
func NewSqliteFindingRepository(dbPath string) (core.FindingRepository, error) {

	db, err := InitializeSQLiteDB(dbPath)
	if err != nil {
		return nil, err
	}

	return &SqliteFindingRepository{db: db}, nil
}

// Store saves one array of core.Finding as a single JSON batch (like writing one JSON file).
func (r *SqliteFindingRepository) Store(matches []core.Finding) error {
	return InsertMatches(r.db, matches)
}

// Clear removes all stored batches of findings (like removing all JSON files).
func (r *SqliteFindingRepository) Clear() error {
	return nil
}

// NewIterator creates an iterator that loads each JSON batch (file) one at a time
func (r *SqliteFindingRepository) NewIterator() core.FindingIterator {
	return &SqliteFindingIterator{
		repo:       r,
		currentID:  0,
		currentSet: core.FindingSet{Matches: nil},
	}
}

// Close closes the underlying SQLite database.
// Call this when you’re done with the repository (similar to not needing temp files).
func (r *SqliteFindingRepository) Close() error {
	return r.db.Close()
}

// -------------------------
//     ITERATOR
// -------------------------

// SqliteFindingIterator iterates over each row (batch) in finding_batches
type SqliteFindingIterator struct {
	repo       *SqliteFindingRepository
	currentID  int             // ID of the last row loaded
	currentSet core.FindingSet // The “current file’s” data
}

// HasNext tries to load the next row from the database.
// If successful, it returns true; if no more rows, returns false.
func (it *SqliteFindingIterator) HasNext() bool {

	for {
		// Attempt to load the next row after currentID
		err := it.loadNextBatch()
		if err != nil {
			if err.Error() != "no more batches" {
				// If it's a parse/read error, log it and keep going
				log.Printf("Error loading row with ID > %d: %v", it.currentID, err)
				it.currentID++ // skip this row and try the next
				continue
			}
			// truly no more
			return false
		}
		// Successfully loaded a batch
		return true
	}
}

// Next returns the last successfully loaded core.FindingSet
func (it *SqliteFindingIterator) Next() (core.FindingSet, error) {
	if it.currentSet.Matches == nil {
		return core.FindingSet{}, fmt.Errorf("no more matchSet available")
	}
	return it.currentSet, nil
}

// Reset starts iteration over from the beginning
func (it *SqliteFindingIterator) Reset() error {
	it.currentID = 0
	it.currentSet = core.FindingSet{}
	return nil
}

// loadNextBatch finds the row where id > it.currentID, in ascending order, and loads it into it.currentSet
func (it *SqliteFindingIterator) loadNextBatch() error {
	// Query the very next row (lowest id bigger than currentID)
	row := it.repo.db.QueryRow(`
		SELECT id, json_data 
		FROM finding_batches 
		WHERE id > ? 
		ORDER BY id ASC 
		LIMIT 1
	`, it.currentID)

	var id int
	var jsonData string
	if err := row.Scan(&id, &jsonData); err != nil {
		return fmt.Errorf("no more batches")
	}

	// Try to parse the JSON array into []core.Finding
	var matches []core.Finding
	if err := json.Unmarshal([]byte(jsonData), &matches); err != nil {
		return fmt.Errorf("failed to parse JSON for row %d: %w", id, err)
	}

	it.currentID = id
	it.currentSet = core.FindingSet{Matches: matches}
	return nil
}

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

func DeleteDatabaseFileIfExists(path string) error {
	// Check if the file exists
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		// File does not exist; nothing to delete
		return nil
	} else if err != nil {
		// An error occurred while trying to stat the file
		return fmt.Errorf("failed to check if file exists at path %s: %w", path, err)
	}

	// Ensure that the path is a file and not a directory
	if info.IsDir() {
		return fmt.Errorf("path %s is a directory, not a file", path)
	}

	// Attempt to delete the file
	err = os.Remove(path)
	if err != nil {
		return fmt.Errorf("failed to delete database file at path %s: %w", path, err)
	}

	return nil
}
