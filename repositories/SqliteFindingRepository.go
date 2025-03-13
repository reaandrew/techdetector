package repositories

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"github.com/reaandrew/techdetector/core"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
)

// SqliteFindingRepository implements core.FindingRepository using SQLite
type SqliteFindingRepository struct {
	db *sql.DB
}

// NewSqliteFindingRepository creates a new SQLite-backed repository.
// dbPath is the filename/path for your SQLite database (e.g. "findings.db").
func NewSqliteFindingRepository(dbPath string) (core.FindingRepository, error) {

	// Ensure the directory for the database file exists
	err := os.MkdirAll(filepath.Dir(dbPath), os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("failed to create DB directory: %w", err)
	}

	// Open (or create) the SQLite database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database: %w", err)
	}

	// Create the table to store JSON batches of findings
	schema := `
	CREATE TABLE IF NOT EXISTS finding_batches (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		json_data TEXT NOT NULL
	);
	`
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to create table: %w", err)
	}

	return &SqliteFindingRepository{db: db}, nil
}

// Store saves one array of core.Finding as a single JSON batch (like writing one JSON file).
func (r *SqliteFindingRepository) Store(matches []core.Finding) error {
	// Convert the slice of findings to JSON
	jsonData, err := json.MarshalIndent(matches, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal findings: %w", err)
	}

	// Insert a row into the finding_batches table
	_, err = r.db.Exec("INSERT INTO finding_batches (json_data) VALUES (?)", string(jsonData))
	if err != nil {
		return fmt.Errorf("failed to insert findings batch: %w", err)
	}

	return nil
}

// Clear removes all stored batches of findings (like removing all JSON files).
func (r *SqliteFindingRepository) Clear() error {
	_, err := r.db.Exec("DELETE FROM finding_batches")
	return err
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
