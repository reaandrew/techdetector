package scanners

import (
	"fmt"
	"github.com/reaandrew/techdetector/core"
	"github.com/reaandrew/techdetector/reporters"
	"log"
	"path/filepath"
)

// DirectoryScanner struct
type DirectoryScanner struct {
	reporter        core.Reporter
	fileScanner     FileScanner
	matchRepository core.FindingRepository
}

// NewDirectoryScanner creates a new DirectoryScanner
func NewDirectoryScanner(reporter core.Reporter,
	processors []core.FileProcessor,
	matchRepository core.FindingRepository) *DirectoryScanner {
	return &DirectoryScanner{
		reporter:        reporter,
		fileScanner:     FileScanner{processors: processors},
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

	fmt.Printf("Number of matches in '%s': %d\n", directory, len(matches))

	// Store matches
	err = ds.matchRepository.Store(matches)
	if err != nil {
		log.Fatalf("Error storing matches in '%s': %v", directory, err)
	}

	// Generate summary report
	summaryReporter := reporters.XlsxSummaryReporter{}
	err = summaryReporter.Report(ds.matchRepository)
	if err != nil {
		log.Fatalf("Error generating summary report: %v", err)
	}

	// Generate main report
	err = ds.reporter.Report(ds.matchRepository)
	if err != nil {
		log.Fatalf("Error generating report: %v", err)
	}

	fmt.Println("Scan completed successfully.")
}
