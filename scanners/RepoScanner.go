package scanners

import (
	"fmt"
	"github.com/reaandrew/techdetector/processors"
	"github.com/reaandrew/techdetector/reporters"
	"github.com/reaandrew/techdetector/repositories"
	"github.com/reaandrew/techdetector/utils"
	"log"
	"os"
	"path/filepath"
)

type RepoScanner struct {
	reporter        reporters.Reporter
	fileScanner     FileScanner
	matchRepository repositories.MatchRepository
}

func NewRepoScanner(
	reporter reporters.Reporter,
	processors []processors.FileProcessor,
	matchRepository repositories.MatchRepository) *RepoScanner {
	return &RepoScanner{
		reporter:        reporter,
		fileScanner:     FileScanner{processors: processors},
		matchRepository: matchRepository,
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
	matches, err := repoScanner.fileScanner.TraverseAndSearch(repoPath, repoName)
	if err != nil {
		log.Fatalf("Error storing matches in '%s': %v", repoName, err)
	}
	err = repoScanner.matchRepository.Store(matches)
	if err != nil {
		log.Fatalf("Error searching repository '%s': %v", repoName, err)
	}

	fmt.Printf("Number of matches: %d\n", len(matches)) // Debug statement

	// Generate report

	err = repoScanner.reporter.Report(repoScanner.matchRepository)
	if err != nil {
		log.Fatalf("Error generating report: %v", err)
	}
}
