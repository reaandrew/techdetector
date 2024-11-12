package repositories

import (
	"encoding/json"
	"fmt"
	"github.com/reaandrew/techdetector/processors"
	"github.com/reaandrew/techdetector/utils"
	"log"
	"os"
	"path"
)

type FileBasedMatchRepository struct {
	path  string
	files []string
}

func NewFileBasedMatchRepository() *FileBasedMatchRepository {
	return &FileBasedMatchRepository{
		path:  os.TempDir(),
		files: make([]string, 0),
	}
}

func (r *FileBasedMatchRepository) Store(matches []processors.Match) error {
	jsonData, err := json.MarshalIndent(matches, "", "  ") // Pretty-print with indentation
	if err != nil {
		return err
	}

	filePath := path.Join(r.path, utils.GenerateRandomFilename("json"))
	r.files = append(r.files, filePath)
	err = os.WriteFile(filePath, jsonData, 0644)
	if err != nil {
		return err
	}
	return nil
}

func (r *FileBasedMatchRepository) Clear() error {
	for _, filepath := range r.files {
		err := os.Remove(filepath)
		if err != nil {
			return err
		}
	}
	// Optionally, reset the files slice
	r.files = nil
	return nil
}

// NewIterator creates a new MatchIterator for the repository
func (r *FileBasedMatchRepository) NewIterator() *MatchIterator {
	return &MatchIterator{
		repository:   r,
		currentFile:  0,
		matches:      nil,
		currentMatch: 0,
	}
}

// MatchIterator implements the Iterator pattern for Match instances
type MatchIterator struct {
	repository   *FileBasedMatchRepository
	currentFile  int
	matches      []processors.Match
	currentMatch int
}

// HasNext checks if there are more Match instances to iterate over
func (it *MatchIterator) HasNext() bool {
	// Check if there are remaining matches in the current file
	if it.matches != nil && it.currentMatch < len(it.matches) {
		return true
	}

	// Attempt to load the next file until a file with matches is found or all files are exhausted
	for it.currentFile < len(it.repository.files) {
		err := it.loadNextFile()
		if err != nil {
			// Log the error and skip to the next file
			log.Printf("Error loading file %s: %v", it.repository.files[it.currentFile], err)
			it.currentFile++
			continue
		}

		// If the loaded file has matches, return true
		if len(it.matches) > 0 && it.currentMatch < len(it.matches) {
			return true
		}
	}

	// No more matches available
	return false
}

// Next retrieves the next Match instance
func (it *MatchIterator) Next() (processors.Match, error) {
	if !it.HasNext() {
		return processors.Match{}, fmt.Errorf("no more matches available")
	}

	match := it.matches[it.currentMatch]
	it.currentMatch++
	return match, nil
}

// loadNextFile loads matches from the next file
func (it *MatchIterator) loadNextFile() error {
	if it.currentFile >= len(it.repository.files) {
		return fmt.Errorf("no more files to load")
	}

	filePath := it.repository.files[it.currentFile]
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	var matches []processors.Match
	err = json.Unmarshal(data, &matches)
	if err != nil {
		return fmt.Errorf("failed to parse JSON in file %s: %w", filePath, err)
	}

	it.matches = matches
	it.currentMatch = 0
	it.currentFile++

	return nil
}
