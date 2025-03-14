package repositories

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/reaandrew/techdetector/core"
	log "github.com/sirupsen/logrus"
	"os"
)

// SqliteFindingRepository implements core.FindingRepository using SQLite
type SqliteFindingRepository struct {
	db   *sql.DB
	mu   sync.Mutex // For thread-safety
	stmt *sql.Stmt  // Cached prepared statement
}

// NewSqliteFindingRepository creates a new SQLite-backed repository.
func NewSqliteFindingRepository(dbPath string) (core.FindingRepository, error) {
	log.Debugf("Initializing SQLite repository at path: %s", dbPath)
	db, err := InitializeSQLiteDB(dbPath)
	if err != nil {
		log.Errorf("Failed to initialize SQLite DB: %v", err)
		return nil, err
	}

	log.Debug("Preparing INSERT statement for Findings table")
	stmt, err := db.Prepare(`
        INSERT INTO Findings (Name, Type, Category, Path, RepoName, Properties)
        VALUES (?, ?, ?, ?, ?, ?)
    `)
	if err != nil {
		log.Errorf("Failed to prepare INSERT statement: %v", err)
		db.Close()
		return nil, fmt.Errorf("failed to prepare statement: %w", err)
	}

	log.Info("SQLite repository initialized successfully")
	return &SqliteFindingRepository{
		db:   db,
		stmt: stmt,
	}, nil
}

// Store saves findings in a thread-safe manner with bulk insert optimization
func (r *SqliteFindingRepository) Store(matches []core.Finding) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	log.Debugf("Storing %d findings", len(matches))
	const maxRetries = 5
	for attempt := 0; attempt < maxRetries; attempt++ {
		err := r.insertMatches(matches)
		if err == nil {
			log.Infof("Successfully stored %d findings on attempt %d", len(matches), attempt+1)
			return nil
		}
		if !isRetryableError(err) {
			log.Errorf("Non-retryable error storing findings: %v", err)
			return err
		}
		// Exponential backoff
		delay := time.Millisecond * 100 * time.Duration(attempt+1)
		log.Warnf("Retry %d/%d for SQLite insert after %v delay due to: %v", attempt+1, maxRetries, delay, err)
		time.Sleep(delay)
	}
	err := fmt.Errorf("failed to store %d matches after %d retries", len(matches), maxRetries)
	log.Errorf("%v", err)
	return err
}

// insertMatches performs the actual insertion within a transaction
func (r *SqliteFindingRepository) insertMatches(matches []core.Finding) (err error) {
	log.Debug("Beginning transaction for inserting matches")
	tx, err := r.db.Begin()
	if err != nil {
		log.Errorf("Failed to begin transaction: %v", err)
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			log.Errorf("Panic during transaction: %v", p)
			tx.Rollback()
			panic(p)
		} else if err != nil {
			log.Warnf("Rolling back transaction due to error: %v", err)
			tx.Rollback()
		} else {
			err = tx.Commit()
			if err != nil {
				log.Errorf("Failed to commit transaction: %v", err)
			} else {
				log.Debug("Transaction committed successfully")
			}
		}
	}()

	// Use the cached prepared statement
	stmt := tx.Stmt(r.stmt)
	defer stmt.Close()

	for i, finding := range matches {
		log.Debugf("Processing finding %d: %s (Repo: %s)", i+1, finding.Name, finding.RepoName)
		jsonProps, err := json.Marshal(flattenProperties(finding.Properties))
		if err != nil {
			log.Warnf("Failed to marshal properties for finding '%s': %v, using empty object", finding.Name, err)
			jsonProps = []byte("{}")
		}

		_, err = stmt.Exec(
			finding.Name,
			finding.Type,
			finding.Category,
			finding.Path,
			finding.RepoName,
			string(jsonProps),
		)
		if err != nil {
			log.Errorf("Failed to insert finding '%s' at index %d: %v", finding.Name, i, err)
			return fmt.Errorf("failed to insert finding '%s': %w", finding.Name, err)
		}
	}
	log.Debugf("Inserted %d findings into transaction", len(matches))
	return nil
}

// Clear removes all findings
func (r *SqliteFindingRepository) Clear() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	log.Info("Clearing all findings from Findings table")
	result, err := r.db.Exec("DELETE FROM Findings")
	if err != nil {
		log.Errorf("Failed to clear findings: %v", err)
		return err
	}
	rows, _ := result.RowsAffected()
	log.Infof("Cleared %d rows from Findings table", rows)
	return nil
}

// NewIterator creates an iterator over findings
func (r *SqliteFindingRepository) NewIterator() core.FindingIterator {
	log.Debug("Creating new iterator for findings")
	return &SqliteFindingIterator{repo: r}
}

// Close cleans up resources
func (r *SqliteFindingRepository) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	log.Info("Closing SQLite repository")
	if r.stmt != nil {
		if err := r.stmt.Close(); err != nil {
			log.Errorf("Failed to close prepared statement: %v", err)
		} else {
			log.Debug("Prepared statement closed")
		}
	}
	if err := r.db.Close(); err != nil {
		log.Errorf("Failed to close database: %v", err)
		return err
	}
	log.Info("Database closed successfully")
	return nil
}

// SqliteFindingIterator iterates over findings
type SqliteFindingIterator struct {
	repo    *SqliteFindingRepository
	rows    *sql.Rows
	current core.FindingSet
}

// HasNext checks if there are more findings
func (it *SqliteFindingIterator) HasNext() bool {
	if it.rows == nil {
		log.Debug("Querying findings for iterator")
		rows, err := it.repo.db.Query("SELECT Name, Type, Category, Path, RepoName, Properties FROM Findings")
		if err != nil {
			log.Errorf("Failed to query findings for iterator: %v", err)
			return false
		}
		it.rows = rows
	}

	if !it.rows.Next() {
		log.Debug("No more findings to iterate")
		if err := it.rows.Close(); err != nil {
			log.Errorf("Failed to close rows: %v", err)
		}
		it.rows = nil
		return false
	}

	var f core.Finding
	var props string
	err := it.rows.Scan(&f.Name, &f.Type, &f.Category, &f.Path, &f.RepoName, &props)
	if err != nil {
		log.Errorf("Failed to scan finding: %v", err)
		return false
	}
	log.Debugf("Scanned finding: %s (Repo: %s)", f.Name, f.RepoName)
	if err := json.Unmarshal([]byte(props), &f.Properties); err != nil {
		log.Errorf("Failed to unmarshal properties for finding '%s': %v", f.Name, err)
	}
	it.current = core.FindingSet{Matches: []core.Finding{f}}
	return true
}

// Next returns the current finding set
func (it *SqliteFindingIterator) Next() (core.FindingSet, error) {
	if it.current.Matches == nil {
		log.Warn("No more findings available in iterator")
		return core.FindingSet{}, fmt.Errorf("no more findings available")
	}
	log.Debugf("Returning finding set with %d matches", len(it.current.Matches))
	return it.current, nil
}

// Reset restarts the iteration
func (it *SqliteFindingIterator) Reset() error {
	log.Debug("Resetting iterator")
	if it.rows != nil {
		if err := it.rows.Close(); err != nil {
			log.Errorf("Failed to close rows during reset: %v", err)
		}
	}
	it.rows = nil
	it.current = core.FindingSet{}
	return nil
}

// PredefinedFieldsSlice contains standard fields
var PredefinedFieldsSlice = []string{"Name", "Type", "Category", "Path", "RepoName"}

// InitializeSQLiteDB sets up the SQLite database
func InitializeSQLiteDB(dbPath string) (*sql.DB, error) {
	log.Infof("Setting up SQLite database at %s", dbPath)
	if err := DeleteDatabaseFileIfExists(dbPath); err != nil {
		log.Errorf("Failed to delete existing database file: %v", err)
		return nil, err
	}

	log.Debug("Opening SQLite database")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Errorf("Failed to open SQLite database: %v", err)
		return nil, fmt.Errorf("failed to open SQLite database: %w", err)
	}

	log.Debug("Applying performance optimizations")
	_, _ = db.Exec("PRAGMA journal_mode = WAL;")
	_, _ = db.Exec("PRAGMA synchronous = NORMAL;")
	_, _ = db.Exec("PRAGMA cache_size = -20000;")

	createStmt := `
        CREATE TABLE IF NOT EXISTS Findings (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            Name TEXT,
            Type TEXT,
            Category TEXT,
            Path TEXT,
            RepoName TEXT,
            Properties TEXT
        );
        CREATE INDEX IF NOT EXISTS idx_repo ON Findings (RepoName);
    `
	log.Debug("Creating Findings table and index")
	if _, err := db.Exec(createStmt); err != nil {
		log.Errorf("Failed to create findings table: %v", err)
		db.Close()
		return nil, fmt.Errorf("failed to create findings table: %w", err)
	}

	log.Info("SQLite database initialized successfully")
	return db, nil
}

// flattenProperties optimizes property flattening
func flattenProperties(properties map[string]interface{}) map[string]interface{} {
	log.Debugf("Flattening properties with %d keys", len(properties))
	flattened := make(map[string]interface{}, len(properties))
	for key, value := range properties {
		if isPredefinedField(key) {
			continue
		}
		if nested, ok := value.(map[string]interface{}); ok {
			jsonBytes, err := json.Marshal(nested)
			if err != nil {
				log.Warnf("Failed to marshal nested map for key '%s': %v", key, err)
				flattened[key] = nil
			} else {
				flattened[key] = string(jsonBytes)
			}
		} else {
			flattened[key] = value
		}
	}
	log.Debug("Properties flattened successfully")
	return flattened
}

// isPredefinedField checks standard fields
func isPredefinedField(key string) bool {
	for _, field := range PredefinedFieldsSlice {
		if strings.EqualFold(key, field) {
			return true
		}
	}
	return false
}

// isRetryableError checks if an error is worth retrying
func isRetryableError(err error) bool {
	retryable := err != nil && (strings.Contains(err.Error(), "database is locked") || strings.Contains(err.Error(), "SQLITE_BUSY"))
	if retryable {
		log.Debugf("Detected retryable error: %v", err)
	}
	return retryable
}

// DeleteDatabaseFileIfExists removes the database file if it exists
func DeleteDatabaseFileIfExists(path string) error {
	log.Debugf("Checking if database file exists at %s", path)
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		log.Debug("Database file does not exist, no deletion needed")
		return nil
	}
	if err != nil {
		log.Errorf("Failed to stat file %s: %v", path, err)
		return fmt.Errorf("failed to stat file %s: %w", path, err)
	}
	if info.IsDir() {
		log.Errorf("Path %s is a directory, not a file", path)
		return fmt.Errorf("path %s is a directory, not a file", path)
	}
	log.Info("Deleting existing database file")
	if err = os.Remove(path); err != nil {
		log.Errorf("Failed to delete database file %s: %v", path, err)
		return fmt.Errorf("failed to delete database file %s: %w", path, err)
	}
	log.Debug("Database file deleted successfully")
	return nil
}
