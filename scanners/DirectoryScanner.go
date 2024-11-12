package scanners

import (
	"fmt"
	"github.com/reaandrew/techdetector/processors"
	"github.com/reaandrew/techdetector/reporters"
	"github.com/reaandrew/techdetector/repositories"
	"log"
	"os"
	"path/filepath"
)

// DirectoryScanner struct
type DirectoryScanner struct {
	reporter        reporters.Reporter
	fileScanner     FileScanner
	matchRepository repositories.MatchRepository
}

// NewDirectoryScanner creates a new DirectoryScanner
func NewDirectoryScanner(reporter reporters.Reporter,
	processors []processors.FileProcessor,
	matchRepository repositories.MatchRepository) *DirectoryScanner {
	return &DirectoryScanner{
		reporter:        reporter,
		fileScanner:     FileScanner{processors: processors},
		matchRepository: matchRepository,
	}
}

// Scan method for DirectoryScanner
func (ds *DirectoryScanner) Scan(directory string, reportFormat string) {
	// List top-level directories
	dirs, err := listTopLevelDirectories(directory)
	if err != nil {
		log.Fatalf("Failed to list directories in '%s': %v", directory, err)
	}

	if len(dirs) == 0 {
		log.Println("No top-level directories found to Scan.")
		return
	}

	//var allMatches []processors.Match

	for _, dir := range dirs {
		fmt.Printf("Processing directory: %s\n", dir)

		// Traverse and search
		matches, err := ds.fileScanner.TraverseAndSearch(dir, filepath.Base(dir))
		if err != nil {
			log.Printf("Error searching directory '%s': %v", dir, err)
			continue // Proceed with the next directory
		}

		fmt.Printf("Number of matches in '%s': %d\n", dir, len(matches))
		err = ds.matchRepository.Store(matches)
		if err != nil {
			log.Fatalf("Error storing matches in '%s': %v", dir, err)
		}
		//allMatches = append(allMatches, matches...)
	}

	//// Generate consolidated report
	//if len(allMatches) == 0 {
	//	fmt.Println("No findings detected across all directories.")
	//	return
	//}

	err = ds.reporter.GenerateReport(ds.matchRepository, reportFormat)
	if err != nil {
		log.Fatalf("Error generating report: %v", err)
	}

	fmt.Println("Report generation completed successfully.")
}

// Helper function to list top-level directories in a given path
func listTopLevelDirectories(path string) ([]string, error) {
	var directories []string

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			dirPath := filepath.Join(path, entry.Name())
			directories = append(directories, dirPath)
		}
	}

	return directories, nil
}
