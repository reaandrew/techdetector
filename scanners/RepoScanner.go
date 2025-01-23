package scanners

import (
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/reaandrew/techdetector/core"
	"github.com/reaandrew/techdetector/utils"
	"log"
	"os"
	"path/filepath"
	"time"
)

type RepoScanner struct {
	reporter        core.Reporter
	fileScanner     FileScanner
	matchRepository core.FindingRepository
}

func NewRepoScanner(
	reporter core.Reporter,
	processors []core.FileProcessor,
	matchRepository core.FindingRepository) *RepoScanner {
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

	// Collect Git metrics
	gitFindings, err := CollectGitMetrics(bareRepoPath, repoName)
	if err != nil {
		log.Fatalf("Error collecting Git metrics for '%s': %v", repoName, err)
	}

	fmt.Printf("Git Metrics: %+v\n", gitFindings)

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

	fmt.Printf("Number of matches: %d\n", len(matches)) // Debug statement

	// Generate report

	err = repoScanner.reporter.Report(repoScanner.matchRepository)
	if err != nil {
		log.Fatalf("Error generating report: %v", err)
	}
}

//func CollectGitMetrics(repoPath, repoName string) ([]core.Finding, error) {
//	var findings []core.Finding
//
//	repo, err := git.PlainOpen(repoPath)
//	if err != nil {
//		return nil, fmt.Errorf("failed to open git repository: %w", err)
//	}
//
//	// Collect all branch references (local and remote)
//	branches, err := repo.References()
//	if err != nil {
//		return nil, fmt.Errorf("failed to retrieve branches: %w", err)
//	}
//
//	var branchCount int
//	var branchNames []string
//
//	err = branches.ForEach(func(ref *plumbing.Reference) error {
//		if ref.Name().IsRemote() || ref.Name().IsBranch() {
//			branchNames = append(branchNames, ref.Name().String())
//			branchCount++
//		}
//		return nil
//	})
//
//	findings = append(findings, core.Finding{
//		Name:     "Branch Count",
//		Type:     "git_metric",
//		Category: "repository_analysis",
//		Properties: map[string]interface{}{
//			"value":    branchCount,
//			"branches": branchNames, // Store branch names for reference
//		},
//		RepoName: repoName,
//	})
//
//	// Count the number of tags using repo.Tags()
//	var tagCount int
//	var tagNames []string
//
//	tags, err := repo.Tags()
//	if err != nil {
//		return nil, fmt.Errorf("failed to retrieve tags: %w", err)
//	}
//
//	err = tags.ForEach(func(ref *plumbing.Reference) error {
//		tagNames = append(tagNames, ref.Name().Short())
//		tagCount++
//		return nil
//	})
//
//	findings = append(findings, core.Finding{
//		Name:     "Tag Count",
//		Type:     "git_metric",
//		Category: "repository_analysis",
//		Properties: map[string]interface{}{
//			"value": tagCount,
//			"tags":  tagNames, // Store tag names for reference
//		},
//		RepoName: repoName,
//	})
//
//	// Commit count across all branches
//	commitIter, err := repo.Log(&git.LogOptions{})
//	if err != nil {
//		return nil, fmt.Errorf("failed to retrieve commit history: %w", err)
//	}
//
//	commitCount := 0
//	authorSet := make(map[string]struct{})
//	err = commitIter.ForEach(func(c *object.Commit) error {
//		commitCount++
//		authorSet[c.Author.Email] = struct{}{}
//		return nil
//	})
//
//	findings = append(findings, core.Finding{
//		Name:     "Commit Count",
//		Type:     "git_metric",
//		Category: "repository_analysis",
//		Properties: map[string]interface{}{
//			"value": commitCount,
//		},
//		RepoName: repoName,
//	})
//
//	findings = append(findings, core.Finding{
//		Name:     "Unique Contributors",
//		Type:     "git_metric",
//		Category: "repository_analysis",
//		Properties: map[string]interface{}{
//			"value": len(authorSet),
//		},
//		RepoName: repoName,
//	})
//
//	return findings, nil
//}

func CollectGitMetrics(repoPath, repoName string) ([]core.Finding, error) {
	var findings []core.Finding

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open git repository: %w", err)
	}

	// Collect branch count findings
	branchFindings, err := getBranchMetrics(repo, repoName)
	if err != nil {
		return nil, err
	}
	findings = append(findings, branchFindings...)

	// Collect tag count findings
	tagFindings, err := getTagMetrics(repo, repoName)
	if err != nil {
		return nil, err
	}
	findings = append(findings, tagFindings...)

	// Collect commit statistics findings
	commitFindings, err := getCommitMetrics(repo, repoName)
	if err != nil {
		return nil, err
	}
	findings = append(findings, commitFindings...)

	return findings, nil
}

// ----------------- Branch Metrics -----------------
func getBranchMetrics(repo *git.Repository, repoName string) ([]core.Finding, error) {
	var findings []core.Finding
	branches, err := repo.References()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve branches: %w", err)
	}

	var branchCount int
	var branchNames []string

	err = branches.ForEach(func(ref *plumbing.Reference) error {
		if ref.Name().IsRemote() || ref.Name().IsBranch() {
			branchNames = append(branchNames, ref.Name().String())
			branchCount++
		}
		return nil
	})

	findings = append(findings, core.Finding{
		Name:     "Branch Count",
		Type:     "git_metric",
		Category: "repository_analysis",
		Properties: map[string]interface{}{
			"value":    branchCount,
			"branches": branchNames,
		},
		RepoName: repoName,
	})

	return findings, nil
}

// ----------------- Tag Metrics -----------------
func getTagMetrics(repo *git.Repository, repoName string) ([]core.Finding, error) {
	var findings []core.Finding
	tags, err := repo.Tags()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve tags: %w", err)
	}

	var tagCount int
	var tagNames []string

	err = tags.ForEach(func(ref *plumbing.Reference) error {
		tagNames = append(tagNames, ref.Name().Short())
		tagCount++
		return nil
	})

	findings = append(findings, core.Finding{
		Name:     "Tag Count",
		Type:     "git_metric",
		Category: "repository_analysis",
		Properties: map[string]interface{}{
			"value": tagCount,
			"tags":  tagNames,
		},
		RepoName: repoName,
	})

	return findings, nil
}

// ----------------- Commit Metrics -----------------
func getCommitMetrics(repo *git.Repository, repoName string) ([]core.Finding, error) {
	var findings []core.Finding

	commitIter, err := repo.Log(&git.LogOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve commit history: %w", err)
	}

	commitsPerDay := make(map[string]int)
	commitsPerWeek := make(map[string]int)
	commitsPerMonth := make(map[string]int)

	var firstCommitDate, lastCommitDate time.Time
	var totalCommits int
	authorSet := make(map[string]struct{})

	err = commitIter.ForEach(func(c *object.Commit) error {
		commitDate := c.Committer.When

		// Fixing the ISOWeek error by capturing both return values
		year, week := commitDate.ISOWeek()
		weekKey := fmt.Sprintf("%d-W%d", year, week)
		monthKey := commitDate.Format("2006-01")
		dayKey := commitDate.Format("2006-01-02")

		commitsPerDay[dayKey]++
		commitsPerWeek[weekKey]++
		commitsPerMonth[monthKey]++
		authorSet[c.Author.Email] = struct{}{}

		if firstCommitDate.IsZero() || commitDate.Before(firstCommitDate) {
			firstCommitDate = commitDate
		}
		if commitDate.After(lastCommitDate) {
			lastCommitDate = commitDate
		}

		totalCommits++
		return nil
	})

	maxCommitsPerDay, avgCommitsPerDay := calculateMaxAndAvg(commitsPerDay)
	maxCommitsPerWeek, avgCommitsPerWeek := calculateMaxAndAvg(commitsPerWeek)
	maxCommitsPerMonth, avgCommitsPerMonth := calculateMaxAndAvg(commitsPerMonth)

	// Add findings
	findings = append(findings, core.Finding{
		Name:     "Total Commits",
		Type:     "git_metric",
		Category: "repository_analysis",
		Properties: map[string]interface{}{
			"value": totalCommits,
		},
		RepoName: repoName,
	})

	findings = append(findings, core.Finding{
		Name:     "First Commit Date",
		Type:     "git_metric",
		Category: "repository_analysis",
		Properties: map[string]interface{}{
			"value": firstCommitDate.Format(time.RFC3339),
		},
		RepoName: repoName,
	})

	findings = append(findings, core.Finding{
		Name:     "Last Commit Date",
		Type:     "git_metric",
		Category: "repository_analysis",
		Properties: map[string]interface{}{
			"value": lastCommitDate.Format(time.RFC3339),
		},
		RepoName: repoName,
	})

	// Add max and average commits per day/week/month to findings
	findings = append(findings, core.Finding{
		Name:     "Max Commits Per Day",
		Type:     "git_metric",
		Category: "repository_analysis",
		Properties: map[string]interface{}{
			"value": maxCommitsPerDay,
		},
		RepoName: repoName,
	})

	findings = append(findings, core.Finding{
		Name:     "Average Commits Per Day",
		Type:     "git_metric",
		Category: "repository_analysis",
		Properties: map[string]interface{}{
			"value": avgCommitsPerDay,
		},
		RepoName: repoName,
	})

	findings = append(findings, core.Finding{
		Name:     "Max Commits Per Week",
		Type:     "git_metric",
		Category: "repository_analysis",
		Properties: map[string]interface{}{
			"value": maxCommitsPerWeek,
		},
		RepoName: repoName,
	})

	findings = append(findings, core.Finding{
		Name:     "Average Commits Per Week",
		Type:     "git_metric",
		Category: "repository_analysis",
		Properties: map[string]interface{}{
			"value": avgCommitsPerWeek,
		},
		RepoName: repoName,
	})

	findings = append(findings, core.Finding{
		Name:     "Max Commits Per Month",
		Type:     "git_metric",
		Category: "repository_analysis",
		Properties: map[string]interface{}{
			"value": maxCommitsPerMonth,
		},
		RepoName: repoName,
	})

	findings = append(findings, core.Finding{
		Name:     "Average Commits Per Month",
		Type:     "git_metric",
		Category: "repository_analysis",
		Properties: map[string]interface{}{
			"value": avgCommitsPerMonth,
		},
		RepoName: repoName,
	})

	findings = append(findings, core.Finding{
		Name:     "Unique Contributors",
		Type:     "git_metric",
		Category: "repository_analysis",
		Properties: map[string]interface{}{
			"value": len(authorSet),
		},
		RepoName: repoName,
	})

	return findings, nil
}

// ----------------- Utility Function -----------------
func calculateMaxAndAvg(commitStats map[string]int) (int, float64) {
	var maxCommits int
	var totalCommits int

	for _, count := range commitStats {
		totalCommits += count
		if count > maxCommits {
			maxCommits = count
		}
	}

	avgCommits := 0.0
	if len(commitStats) > 0 {
		avgCommits = float64(totalCommits) / float64(len(commitStats))
	}

	return maxCommits, avgCommits
}
