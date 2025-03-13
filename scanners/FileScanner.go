package scanners

import (
	"fmt"
	"github.com/reaandrew/techdetector/core"
	log "github.com/sirupsen/logrus"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var (
	// MaxWorkers sets the number of parallel workers that handle repositories
	MaxWorkers = runtime.NumCPU()
	// MaxFileWorkers sets the number of parallel workers that handle files in each repo
	MaxFileWorkers = runtime.NumCPU()
	// CloneBaseDir is where all repositories get cloned to
	CloneBaseDir = "/tmp/techdetector"
)

// FileScanner is responsible for walking through a directory and passing files
// to all configured FileProcessors.
type FileScanner struct {
	processors []core.FileProcessor
}

// TraverseAndSearch walks through the specified directory, sends each *actual file*
// to the provided processors, and returns aggregated matches.
func (fileScanner FileScanner) TraverseAndSearch(targetDir string, repoName string) ([]core.Finding, error) {
	var Matches []core.Finding
	var mu sync.Mutex

	info, err := os.Stat(targetDir)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("target directory '%s' does not exist", targetDir)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("'%s' is not a directory", targetDir)
	}

	files := make(chan string, 100)
	fileMatches := make(chan core.Finding, 1000)
	errs := make(chan error, 10)

	var wg sync.WaitGroup

	// Spawn workers to process files
	for i := 0; i < MaxFileWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range files {
				for _, processor := range fileScanner.processors {
					if processor.Supports(path) {
						content, err := os.ReadFile(path)
						if err != nil {
							errs <- fmt.Errorf("failed to read file %s: %v", path, err)
							continue
						}
						results, procErr := processor.Process(path, repoName, string(content))
						if procErr != nil {
							errs <- fmt.Errorf("processing error in file %s: %v", path, procErr)
						}
						for _, Match := range results {
							fileMatches <- Match
						}
					}
				}
			}
		}()
	}

	// Walk the directory, enqueue only actual files (regular files) to "files"
	go func() {
		_ = filepath.WalkDir(targetDir, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				errs <- fmt.Errorf("error walking path %s: %v", path, walkErr)
				return nil
			}
			// Stat the path to detect directories or symlinks
			info, statErr := os.Stat(path)
			if statErr != nil {
				errs <- fmt.Errorf("error stating path %s: %v", path, statErr)
				return nil
			}
			// Only enqueue if it's a regular file (not a directory or symlink to a dir)
			if info.Mode().IsRegular() {
				files <- path
			}
			return nil
		})
		close(files)
	}()

	// Once all files are processed, close channels
	go func() {
		wg.Wait()
		close(fileMatches)
		close(errs)
	}()

	// Collect processed matches
	for match := range fileMatches {
		mu.Lock()
		Matches = append(Matches, match)
		mu.Unlock()
	}

	// Collect all errors
	var errorMessages []string
	for err := range errs {
		log.Errorf("Error encountered: %v", err)
		errorMessages = append(errorMessages, err.Error())
	}

	// Return errors if any
	if len(errorMessages) > 0 {
		return Matches, fmt.Errorf("errors encountered during scanning:\n%s",
			strings.Join(errorMessages, "\n"))
	}

	return Matches, nil
}
