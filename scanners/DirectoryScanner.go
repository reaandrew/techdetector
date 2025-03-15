package scanners

import (
	"github.com/reaandrew/techdetector/core"
	log "github.com/sirupsen/logrus"
	"path/filepath"
)

// DirectoryScanner struct
type DirectoryScanner struct {
	reporter        core.Reporter
	fileScanner     FsFileScanner
	matchRepository core.FindingRepository
}

// NewDirectoryScanner creates a new DirectoryScanner
func NewDirectoryScanner(
	reporter core.Reporter,
	processors []core.FileProcessor,
	matchRepository core.FindingRepository) *DirectoryScanner {
	return &DirectoryScanner{
		reporter:        reporter,
		fileScanner:     FsFileScanner{Processors: processors},
		matchRepository: matchRepository,
	}
}

// Scan method for DirectoryScanner
func (ds *DirectoryScanner) Scan(directory string, reportFormat string) {
	// Traverse and search the root directory
	matches, err := ds.fileScanner.TraverseAndSearch(directory, filepath.Base(directory))
	if err != nil {
		log.Fatalf("Error scanning directory '%s': %v", directory, err)
	}

	log.Printf("Number of matches in '%s': %d\n", directory, len(matches))

	// Store matches
	err = ds.matchRepository.Store(matches)
	if err != nil {
		log.Fatalf("Error storing matches in '%s': %v", directory, err)
	}

	// Generate main report
	err = ds.reporter.Report(ds.matchRepository)
	if err != nil {
		log.Fatalf("Error generating report: %v", err)
	}

	log.Println("Scan completed successfully.")
}
