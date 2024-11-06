package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

type RepoScanner struct {
	reporter    Reporter
	fileScanner FileScanner
}

func NewRepoScanner(reporter Reporter, processors []Processor) *RepoScanner {
	return &RepoScanner{
		reporter:    Reporter{},
		fileScanner: FileScanner{processors: processors},
	}
}

func (repoScanner RepoScanner) scan(repoURL string, reportFormat string) {
	// Ensure clone base directory exists
	err := os.MkdirAll(CloneBaseDir, os.ModePerm)
	if err != nil {
		log.Fatalf("Failed to create clone base directory '%s': %v", CloneBaseDir, err)
	}

	repoName, err := ExtractRepoName(repoURL)
	if err != nil {
		log.Fatalf("Invalid repository URL '%s': %v", repoURL, err)
	}

	repoPath := filepath.Join(CloneBaseDir, SanitizeRepoName(repoName))
	fmt.Printf("Cloning repository: %s\n", repoName)
	err = CloneRepository(repoURL, repoPath)
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
