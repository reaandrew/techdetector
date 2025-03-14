package scanners

import (
	"context"
	"fmt"
	"github.com/reaandrew/techdetector/core"
	log "github.com/sirupsen/logrus"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	MaxWorkers     = 10
	MaxFileWorkers = 10
	CloneBaseDir   = "/tmp/techdetector" // You can make this configurable if needed
)

type FileScanner interface {
	TraverseAndSearch(repoPath, repoName string) ([]core.Finding, error)
}

// FsFileScanner implements FileScanner
type FsFileScanner struct {
	Processors []core.FileProcessor
}

func (fileScanner FsFileScanner) TraverseAndSearch(targetDir string, repoName string) ([]core.Finding, error) {
	log.Debugf("Starting TraverseAndSearch for %s at %s", repoName, targetDir)
	var Matches []core.Finding
	var mu sync.Mutex

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	info, err := os.Stat(targetDir)
	if os.IsNotExist(err) {
		log.Errorf("Target dir %s does not exist for %s", targetDir, repoName)
		return nil, fmt.Errorf("target directory '%s' does not exist", targetDir)
	}
	if !info.IsDir() {
		log.Errorf("%s is not a directory for %s", targetDir, repoName)
		return nil, fmt.Errorf("'%s' is not a directory", targetDir)
	}

	files := make(chan string, 100)
	fileMatches := make(chan core.Finding, 1000)
	errs := make(chan error, 10)
	var wg sync.WaitGroup

	for i := 0; i < MaxFileWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					log.Warnf("Worker %d timed out for %s", workerID, repoName)
					return
				case path, ok := <-files:
					if !ok {
						log.Debugf("Worker %d finished for %s", workerID, repoName)
						return
					}
					log.Debugf("Worker %d processing file %s in %s", workerID, path, repoName)
					for _, processor := range fileScanner.Processors {
						if processor.Supports(path) {
							content, err := os.ReadFile(path)
							if err != nil {
								log.Errorf("Worker %d failed to read %s: %v", workerID, path, err)
								errs <- fmt.Errorf("failed to read file %s: %v", path, err)
								continue
							}
							results, err := processor.Process(path, repoName, string(content))
							if err != nil {
								log.Warnf("Processor failed for %s: %v", path, err)
							}
							for _, match := range results {
								fileMatches <- match
							}
						}
					}
				}
			}
		}(i)
	}

	walkDone := make(chan struct{})
	go func() {
		log.Debugf("Walking dir %s for %s", targetDir, repoName)
		err := filepath.WalkDir(targetDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				log.Errorf("Walk error for %s at %s: %v", repoName, path, err)
				errs <- err
				return nil
			}
			if !d.IsDir() {
				select {
				case files <- path:
					log.Debugf("Sent file %s", path)
				case <-ctx.Done():
					log.Warnf("Walk aborted for %s due to timeout", repoName)
					return ctx.Err()
				}
			}
			return nil
		})
		if err != nil {
			log.Errorf("WalkDir failed for %s: %v", repoName, err)
		}
		close(files)
		close(walkDone)
	}()

	collectDone := make(chan struct{})
	go func() {
		wg.Wait()
		log.Debugf("All workers done for %s", repoName)
		close(fileMatches)
		close(errs)
		close(collectDone)
	}()

	for {
		select {
		case <-ctx.Done():
			log.Errorf("TraverseAndSearch timed out for %s", repoName)
			return Matches, fmt.Errorf("scan timed out")
		case match, ok := <-fileMatches:
			if !ok {
				log.Debugf("Matches channel closed for %s", repoName)
				goto Done
			}
			mu.Lock()
			Matches = append(Matches, match)
			mu.Unlock()
		}
	}
Done:
	<-walkDone
	<-collectDone

	var walkErrors []error
	for err := range errs {
		walkErrors = append(walkErrors, err)
	}
	if len(walkErrors) > 0 {
		log.Warnf("Encountered %d errors during scan of %s", len(walkErrors), repoName)
		return Matches, fmt.Errorf("some errors occurred during scanning: %v", walkErrors)
	}

	log.Infof("Completed scan for %s with %d findings", repoName, len(Matches))
	return Matches, nil
}
