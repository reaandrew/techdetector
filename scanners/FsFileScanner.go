package scanners

import (
	"fmt"
	"github.com/reaandrew/techdetector/core"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

const (
	MaxWorkers     = 10
	MaxFileWorkers = 10
	CloneBaseDir   = "/tmp/techdetector" // You can make this configurable if needed
)

type FileScanner interface {
	TraverseAndSearch(repoPath, repoName string) ([]core.Finding, error)
}

type FsFileScanner struct {
	Processors []core.FileProcessor
}

func (fileScanner FsFileScanner) TraverseAndSearch(targetDir string, repoName string) ([]core.Finding, error) {
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

	// File processing workers
	for i := 0; i < MaxFileWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range files {
				for _, processor := range fileScanner.Processors {
					if processor.Supports(path) {
						content, err := os.ReadFile(path)
						if err != nil {
							errs <- fmt.Errorf("failed to read file %s: %v", path, err)
							continue
						}
						results, _ := processor.Process(path, repoName, string(content))
						for _, Match := range results {
							fileMatches <- Match
						}
					}
				}
			}
		}()
	}

	// Walk through directory and send files to the Worker channel
	go func() {
		_ = filepath.WalkDir(targetDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				errs <- err
				return nil
			}
			if !d.IsDir() {
				files <- path
			}
			return nil
		})
		close(files)
	}()

	// Collect results
	go func() {
		wg.Wait()
		close(fileMatches)
		close(errs)
	}()

	for match := range fileMatches {
		mu.Lock()
		Matches = append(Matches, match)
		mu.Unlock()
	}

	if len(errs) > 0 {
		return Matches, fmt.Errorf("some errors occurred during scanning")
	}

	return Matches, nil
}
