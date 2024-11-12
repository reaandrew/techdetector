package scanners

import (
	"fmt"
	"github.com/reaandrew/techdetector/processors"
	"github.com/reaandrew/techdetector/reporters"
	"github.com/reaandrew/techdetector/utils"
	"log"
	"os"
	"path/filepath"
)

type RepoScanner struct {
	reporter    reporters.Reporter
	fileScanner FileScanner
}

func NewRepoScanner(reporter reporters.Reporter, processors []processors.FileProcessor) *RepoScanner {
	return &RepoScanner{
		reporter:    reporter,
		fileScanner: FileScanner{processors: processors},
	}
}

func (repoScanner RepoScanner) Scan(repoURL string, reportFormat string) {
	// Ensure clone base directory exists
	err := os.MkdirAll(CloneBaseDir, os.ModePerm)
	if err != nil {
		log.Fatalf("Failed to create clone base directory '%s': %v", CloneBaseDir, err)
	}

	repoName, err := utils.ExtractRepoName(repoURL)
	if err != nil {
		log.Fatalf("Invalid repository URL '%s': %v", repoURL, err)
	}

	repoPath := filepath.Join(CloneBaseDir, utils.SanitizeRepoName(repoName))
	fmt.Printf("Cloning repository: %s\n", repoName)
	err = utils.CloneRepository(repoURL, repoPath)
	if err != nil {
		log.Fatalf("Failed to clone repository '%s': %v", repoName, err)
	}

	// Traverse and search with processors
	findings, err := repoScanner.fileScanner.TraverseAndSearch(repoPath, repoName)
	if err != nil {
		log.Fatalf("Error searching repository '%s': %v", repoName, err)
	}

	fmt.Printf("Number of findings: %d\n", len(findings)) // Debug statement

	// Generate report
	err = repoScanner.reporter.GenerateReport(findings, reportFormat)
	if err != nil {
		log.Fatalf("Error generating report: %v", err)
	}
}
