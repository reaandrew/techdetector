package utils

import (
	"fmt"
	"sort"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/markusmobius/go-dateparser"
	"github.com/reaandrew/techdetector/core"
)

// GitMetrics defines the behavior for collecting Git metrics.
type GitMetrics interface {
	CollectGitMetrics(repoPath, repoName, cutoffDate string) ([]core.Finding, error)
}

// GitMetricsClient is the default implementation of GitMetrics.
type GitMetricsClient struct{}

// CollectGitMetrics collects various Git metrics from the repository.
func (g GitMetricsClient) CollectGitMetrics(repoPath, repoName, cutoffDate string) ([]core.Finding, error) {
	var findings []core.Finding

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open git repository: %w", err)
	}

	cutoffTimestamp, err := parseCutoffDate(cutoffDate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cutoff date: %w", err)
	}

	branchFindings, err := getBranchMetrics(repo, repoName, cutoffTimestamp)
	if err != nil {
		return nil, err
	}
	findings = append(findings, branchFindings...)

	tagFindings, err := getTagMetrics(repo, repoName, cutoffTimestamp)
	if err != nil {
		return nil, err
	}
	findings = append(findings, tagFindings...)

	commitFindings, err := getCommitMetrics(repo, repoName, cutoffTimestamp)
	if err != nil {
		return nil, err
	}
	findings = append(findings, commitFindings...)

	return findings, nil
}

// --- Private Helper Functions ---

// parseCutoffDate parses the cutoff date string into a Unix timestamp.
// If the dateStr is empty, it returns -1 to indicate no cutoff.
func parseCutoffDate(dateStr string) (int64, error) {
	if dateStr == "" {
		return -1, nil
	}

	parsedTime, err := dateparser.Parse(nil, dateStr)
	if err != nil {
		return 0, fmt.Errorf("could not parse date string '%s': %w", dateStr, err)
	}

	return parsedTime.Time.Unix(), nil
}

// getBranchMetrics retrieves metrics related to branches in the repository.
func getBranchMetrics(repo *git.Repository, repoName string, cutoffTimestamp int64) ([]core.Finding, error) {
	var findings []core.Finding
	branches, err := repo.References()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve branches: %w", err)
	}

	var branchCount int
	var branchNames []string

	err = branches.ForEach(func(ref *plumbing.Reference) error {
		if ref.Name().IsRemote() || ref.Name().IsBranch() {
			commitIter, err := repo.Log(&git.LogOptions{From: ref.Hash()})
			if err != nil {
				return fmt.Errorf("failed to get commits for branch %s: %w", ref.Name(), err)
			}

			latestCommit, err := commitIter.Next()
			if err != nil {
				return fmt.Errorf("failed to read commit for branch %s: %w", ref.Name(), err)
			}

			commitTime := latestCommit.Author.When.Unix()

			if cutoffTimestamp == -1 || commitTime >= cutoffTimestamp {
				branchNames = append(branchNames, ref.Name().String())
				branchCount++
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	findings = append(findings, core.Finding{
		Name:     "Branch Count",
		Type:     "git_metric",
		Category: "repository_analysis",
		Properties: map[string]interface{}{
			"value":    branchCount,
			"branches": branchNames,
			"cutoff":   cutoffTimestamp,
		},
		RepoName: repoName,
	})

	return findings, nil
}

// getTagMetrics retrieves metrics related to tags in the repository.
func getTagMetrics(repo *git.Repository, repoName string, cutoffTimestamp int64) ([]core.Finding, error) {
	var findings []core.Finding
	var tagDates []time.Time

	if cutoffTimestamp == 0 {
		cutoffTimestamp = -1
	}

	tags, err := repo.Tags()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve tags: %w", err)
	}

	err = tags.ForEach(func(ref *plumbing.Reference) error {
		var tagTime time.Time

		tag, err := repo.TagObject(ref.Hash())
		if err == nil {
			tagTime = tag.Tagger.When
		} else {
			commit, err := repo.CommitObject(ref.Hash())
			if err == nil {
				tagTime = commit.Committer.When
			}
		}

		if cutoffTimestamp == -1 || tagTime.Unix() >= cutoffTimestamp {
			tagDates = append(tagDates, tagTime)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to iterate over tags: %w", err)
	}

	tagCount := len(tagDates)
	findings = append(findings, core.Finding{
		Name:     "Tag Count",
		Type:     "git_metric",
		Category: "repository_analysis",
		Properties: map[string]interface{}{
			"value": tagCount,
		},
		RepoName: repoName,
	})

	if len(tagDates) > 1 {
		sort.Slice(tagDates, func(i, j int) bool {
			return tagDates[i].Before(tagDates[j])
		})

		var totalTime time.Duration
		for i := 1; i < len(tagDates); i++ {
			totalTime += tagDates[i].Sub(tagDates[i-1])
		}

		averageTagTime := totalTime / time.Duration(len(tagDates)-1)
		findings = append(findings, core.Finding{
			Name:     "Average Time Between Tags",
			Type:     "git_metric",
			Category: "repository_analysis",
			Properties: map[string]interface{}{
				"value": fmt.Sprintf("%.2f days", averageTagTime.Hours()/24),
			},
			RepoName: repoName,
		})
	} else {
		findings = append(findings, core.Finding{
			Name:     "Average Time Between Tags",
			Type:     "git_metric",
			Category: "repository_analysis",
			Properties: map[string]interface{}{
				"value": "Not enough tags to calculate",
			},
			RepoName: repoName,
		})
	}

	return findings, nil
}

// getCommitMetrics retrieves metrics related to commits in the repository.
func getCommitMetrics(repo *git.Repository, repoName string, cutoffTimestamp int64) ([]core.Finding, error) {
	var findings []core.Finding

	commitIter, err := repo.Log(&git.LogOptions{
		All: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve commit history: %w", err)
	}

	commitsPerDay := make(map[string]int)
	commitsPerWeek := make(map[string]int)
	commitsPerMonth := make(map[string]int)

	var firstCommitDate, lastCommitDate time.Time
	var totalCommits int
	authorSet := make(map[string]struct{})
	processedCommits := make(map[string]struct{}) // Track processed commits

	err = commitIter.ForEach(func(c *object.Commit) error {
		commitHash := c.Hash.String()
		if _, exists := processedCommits[commitHash]; exists {
			// Already processed; skip duplicate commit.
			return nil
		}
		processedCommits[commitHash] = struct{}{}

		commitDate := c.Committer.When
		if cutoffTimestamp != -1 && commitDate.Unix() < cutoffTimestamp {
			return nil
		}

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
	if err != nil {
		return nil, fmt.Errorf("error processing commits: %w", err)
	}

	maxCommitsPerDay, avgCommitsPerDay := calculateMaxAndAvg(commitsPerDay)
	maxCommitsPerWeek, avgCommitsPerWeek := calculateMaxAndAvg(commitsPerWeek)
	maxCommitsPerMonth, avgCommitsPerMonth := calculateMaxAndAvg(commitsPerMonth)

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

// calculateMaxAndAvg calculates the maximum and average counts from a map.
func calculateMaxAndAvg(counts map[string]int) (int, float64) {
	var max int
	var total int
	for _, count := range counts {
		total += count
		if count > max {
			max = count
		}
	}
	avg := 0.0
	if len(counts) > 0 {
		avg = float64(total) / float64(len(counts))
	}
	return max, avg
}
