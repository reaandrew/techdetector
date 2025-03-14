package scanners

import (
	"github.com/reaandrew/techdetector/core"
	"github.com/reaandrew/techdetector/utils"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
)

type RepoScanner struct {
	reporter        core.Reporter
	fileScanner     FsFileScanner
	matchRepository core.FindingRepository
	Cutoff          string
	GitMetrics      utils.GitMetrics
}

func NewRepoScanner(
	reporter core.Reporter,
	processors []core.FileProcessor,
	matchRepository core.FindingRepository,
	cutoff string) *RepoScanner {
	return &RepoScanner{
		reporter:        reporter,
		fileScanner:     FsFileScanner{Processors: processors},
		matchRepository: matchRepository,
		Cutoff:          cutoff,
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
	log.Printf("Cloning repository: %s\n", repoName)
	err = utils.CloneRepository(repoURL, repoPath, false)
	if err != nil {
		log.Fatalf("Failed to clone repository '%s': %v", repoName, err)
	}

	// Perform bare clone to extract metadata
	bareRepoPath := filepath.Join(CloneBaseDir, utils.SanitizeRepoName(repoName)+"_bare")
	err = utils.CloneRepository(repoURL, bareRepoPath, true)
	if err != nil {
		log.Fatalf("Failed to perform bare clone for '%s': %v", repoName, err)
	}

	log.Println("Fetching Git Metrics")
	// Collect Git metrics
	gitFindings, err := repoScanner.GitMetrics.CollectGitMetrics(bareRepoPath, repoName, repoScanner.Cutoff)
	if err != nil {
		log.Fatalf("Error collecting Git metrics for '%s': %v", repoName, err)
	}

	// Traverse and search with processors
	matches, err := repoScanner.fileScanner.TraverseAndSearch(repoPath, repoName)
	if err != nil {
		log.Fatalf("Error storing matches in '%s': %v", repoName, err)
	}

	matches = append(matches, gitFindings...)

	err = repoScanner.matchRepository.Store(matches)
	if err != nil {
		log.Fatalf("Error searching repository '%s': %v", repoName, err)
	}

	log.Printf("Number of matches: %d\n", len(matches)) // Debug statement

	// Generate report

	err = repoScanner.reporter.Report(repoScanner.matchRepository)
	if err != nil {
		log.Println("Dumping Schema!!!")
		log.Fatalf("Error generating report: %v", err)
	}
}
