package repositories

import (
	"encoding/json"
	"fmt"
	"github.com/reaandrew/techdetector/core"
	"github.com/reaandrew/techdetector/utils"
	log "github.com/sirupsen/logrus"
	"os"
	"path"
)

type FileBasedFindingRepository struct {
	path  string
	files []string
}

func (r *FileBasedFindingRepository) Close() error {
	return nil
}

func NewFileBasedMatchRepository() core.FindingRepository {
	return &FileBasedFindingRepository{
		path:  os.TempDir(),
		files: make([]string, 0),
	}
}

func (r *FileBasedFindingRepository) Store(matches []core.Finding) error {
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

func (r *FileBasedFindingRepository) Clear() error {
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
func (r *FileBasedFindingRepository) NewIterator() core.FindingIterator {
	return &FileBasedMatchIterator{
		Repository:  r,
		currentFile: 0,
		matchSet:    core.FindingSet{Matches: nil},
	}
}

// FileBasedMatchIterator implements the Iterator pattern for Finding instances
type FileBasedMatchIterator struct {
	Repository  *FileBasedFindingRepository
	currentFile int
	matchSet    core.FindingSet
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
func (it *FileBasedMatchIterator) Next() (core.FindingSet, error) {

	if it.matchSet.Matches == nil {
		return core.FindingSet{}, fmt.Errorf("no more matchSet available")
	}
	return it.matchSet, nil
}

func (it *FileBasedMatchIterator) Reset() error {
	it.currentFile = 0
	it.matchSet = core.FindingSet{}
	return nil
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

	var matches []core.Finding
	err = json.Unmarshal(data, &matches)

	if err != nil {
		return fmt.Errorf("failed to parse JSON in file %s: %w", filePath, err)
	}

	it.matchSet = core.FindingSet{Matches: matches}
	it.currentFile++

	return nil
}
