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

func NewFileBasedMatchRepository() MatchRepository {
	return &FileBasedMatchRepository{
		path:  os.TempDir(),
		files: make([]string, 0),
	}
}

func (r *FileBasedMatchRepository) Store(matches []processors.Finding) error {
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

// NewIterator creates a new FileBasedMatchIterator for the Repository
func (r *FileBasedMatchRepository) NewIterator() MatchIterator {
	return &FileBasedMatchIterator{
		Repository:  r,
		currentFile: 0,
		matchSet:    MatchSet{Matches: nil},
	}
}

// FileBasedMatchIterator implements the Iterator pattern for Finding instances
type FileBasedMatchIterator struct {
	Repository  *FileBasedMatchRepository
	currentFile int
	matchSet    MatchSet
}

// HasNext checks if there are more Finding instances to iterate over
func (it *FileBasedMatchIterator) HasNext() bool {
	// Attempt to load the next file until a file with matchSet is found or all files are exhausted
	for it.currentFile < len(it.Repository.files) {
		err := it.loadNextFile()
		if err != nil {
			log.Printf("Error loading file %s: %v", it.Repository.files[it.currentFile], err)
			it.currentFile++
			continue
		}
		return true
	}
	return false
}

// Next retrieves the next Finding instance
func (it *FileBasedMatchIterator) Next() (MatchSet, error) {

	if it.matchSet.Matches == nil {
		return MatchSet{}, fmt.Errorf("no more matchSet available")
	}
	return it.matchSet, nil
}

// loadNextFile loads matchSet from the next file
func (it *FileBasedMatchIterator) loadNextFile() error {
	if it.currentFile >= len(it.Repository.files) {
		return fmt.Errorf("no more files to load")
	}

	filePath := it.Repository.files[it.currentFile]
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	var matches []processors.Finding
	err = json.Unmarshal(data, &matches)

	if err != nil {
		return fmt.Errorf("failed to parse JSON in file %s: %w", filePath, err)
	}

	it.matchSet = MatchSet{Matches: matches}
	it.currentFile++

	return nil
}
