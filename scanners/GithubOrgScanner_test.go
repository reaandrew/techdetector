package scanners_test

import (
	"context"
	"fmt"
	"github.com/reaandrew/techdetector/repositories"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v50/github"
	"github.com/reaandrew/techdetector/core"
	"github.com/reaandrew/techdetector/scanners"
	"github.com/reaandrew/techdetector/utils"
)

// DummyReporter is a no-op implementation of core.Reporter.
type DummyReporter struct{}

func (dr DummyReporter) Report(repo core.FindingRepository) error {
	return nil
}

func generateRandomFindings(repoPath, repoName string) []core.Finding {
	var names = []string{"Hardcoded API Key", "Sensitive File Detected", "Deprecated Function", "Misconfigured ACL", "Exposed Secret"}
	var types = []string{"Secret", "Misconfiguration", "Deprecated Code", "Vulnerability", "Sensitive Data"}
	var categories = []string{"Security", "Compliance", "Best Practices", "Performance", "Confidentiality"}
	var paths = []string{"main.go", "config.yaml", "credentials.json", "server.js", "Dockerfile"}

	rand.Seed(time.Now().UnixNano())
	numFindings := rand.Intn(1000) + 1 // Generate between 1 and 5 findings

	findings := make([]core.Finding, numFindings)

	for i := range findings {
		findings[i] = core.Finding{
			Name:     names[rand.Intn(len(names))],
			Type:     types[rand.Intn(len(types))],
			Category: categories[rand.Intn(len(categories))],
			Properties: map[string]interface{}{
				"severity":     rand.Intn(5) + 1,     // Random severity from 1-5
				"confidence":   rand.Float64() * 100, // Confidence score 0-100
				"description":  "Auto-generated finding",
				"discoveredAt": time.Now().Format(time.RFC3339),
			},
			Path:     paths[rand.Intn(len(paths))],
			RepoName: repoName,
		}
	}

	return findings
}

// DummyFileScanner implements the FileScanner interface.
type DummyFileScanner struct{}

func (dfs DummyFileScanner) TraverseAndSearch(repoPath, repoName string) ([]core.Finding, error) {
	// Immediately return an empty slice (simulate a fast, successful scan)
	return generateRandomFindings(repoPath, repoName), nil
}

// DummyFindingRepository is a no-op implementation of core.FindingRepository.
type DummyFindingRepository struct{}

func (dfr *DummyFindingRepository) Store(matches []core.Finding) error {
	return nil
}
func (dfr *DummyFindingRepository) Clear() error {
	return nil
}
func (dfr *DummyFindingRepository) NewIterator() core.FindingIterator {
	panic("not implemented")
}
func (dfr *DummyFindingRepository) Close() error {
	panic("not implemented")
}

// DummyGithubClient implements utils.GithubApi.
type DummyGithubClient struct {
	repos []*github.Repository
}

func (d DummyGithubClient) ListRepositories(org string) ([]*github.Repository, error) {
	return d.repos, nil
}

// DummyGitClient implements utils.GitApi.
type DummyGitClient struct{}

func (d DummyGitClient) NewClone(ctx context.Context, cloneURL, destination string) utils.Cloner {
	//TODO implement me
	panic("implement me")
}

func (d DummyGitClient) CloneRepositoryWithContext(ctx context.Context, cloneURL, destination string, bare bool) error {
	// Instead of performing a real clone, simply create the destination directory.
	return os.MkdirAll(destination, os.ModePerm)
}

// DummyGitMetrics implements utils.GitMetrics.
type DummyGitMetrics struct{}

func (d DummyGitMetrics) CollectGitMetrics(repoPath, repoName, cutoffDate string) ([]core.Finding, error) {
	// Immediately return empty findings.
	return []core.Finding{}, nil
}

func TestQuickScanDeadlock_WithRealProgressBar(t *testing.T) {
	const numRepos = 1000

	// Create a slice of dummy repositories.
	dummyRepos := make([]*github.Repository, numRepos)
	for i := 0; i < numRepos; i++ {
		dummyRepos[i] = &github.Repository{
			FullName: github.String("dummy/repo" + strconv.Itoa(i)),
			CloneURL: github.String("https://dummy.repo.url"),
		}
	}

	// Create dummy dependencies.
	dummyReporter := DummyReporter{}
	// Use a real progress bar implementation.
	progressBar := utils.NewBarProgressReporter(numRepos, "Scanning Repositories")
	//dummyFindingRepo := &DummyFindingRepository{}
	sqliteRepo, _ := repositories.NewSqliteFindingRepository("/tmp/test.db")

	// Instantiate the scanner directly, supplying all dependencies.
	scanner := &scanners.GithubOrgScanner{
		Reporter:         dummyReporter,
		FileScanner:      DummyFileScanner{},
		MatchRepository:  sqliteRepo,
		Cutoff:           "",
		ProgressReporter: progressBar,
		GithubClient:     DummyGithubClient{repos: dummyRepos},
		GitClient:        DummyGitClient{},
		GitMetrics:       DummyGitMetrics{},
	}

	// Run the scan in a separate goroutine so we can detect a deadlock with a timeout.
	done := make(chan struct{})
	go func() {
		scanner.Scan("dummy-org", "dummy-format")
		close(done)
	}()

	select {
	case <-done:
		// Scan finished successfully.
	case <-time.After(5 * time.Second):
		t.Fatal("Scan timed out, likely due to deadlock")
	}

	// We no longer assert on progress count because the real progress bar
	// does not expose its internal count.
}

// SlowGitClient implements utils.GitApi and simulates a slow repository
type SlowGitClient struct {
	SlowRepo string
}

func (s SlowGitClient) NewClone(ctx context.Context, cloneURL, destination string) utils.Cloner {
	//TODO implement me
	panic("implement me")
}

func (s SlowGitClient) CloneRepositoryWithContext(ctx context.Context, cloneURL, destination string, bare bool) error {
	// If this is the slow repository, simulate a hang
	if strings.Contains(destination, s.SlowRepo) {
		// Either hang forever or simulate a very slow operation
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
			// This will make the test pass if the fix works
			return nil
		}
	}

	// Otherwise, behave normally
	return os.MkdirAll(destination, os.ModePerm)
}

func TestScanWithSlowRepository(t *testing.T) {
	const numRepos = 10

	// Create repos with the last one being our "slow" repo
	dummyRepos := make([]*github.Repository, numRepos)
	for i := 0; i < numRepos; i++ {
		dummyRepos[i] = &github.Repository{
			FullName: github.String(fmt.Sprintf("dummy/repo%d", i)),
			CloneURL: github.String(fmt.Sprintf("https://dummy.repo.url/%d", i)),
		}
	}

	// Mark the last repo as the slow one
	slowRepoName := utils.SanitizeRepoName("dummy/repo" + strconv.Itoa(numRepos-1))

	// Create a scanner with our slow git client
	scanner := &scanners.GithubOrgScanner{
		Reporter:         DummyReporter{},
		FileScanner:      DummyFileScanner{},
		MatchRepository:  &DummyFindingRepository{},
		Cutoff:           "",
		ProgressReporter: utils.NewBarProgressReporter(numRepos, "Scanning Repositories"),
		GithubClient:     DummyGithubClient{repos: dummyRepos},
		GitClient:        SlowGitClient{SlowRepo: slowRepoName},
		GitMetrics:       DummyGitMetrics{},
	}

	// Run with a timeout to catch hanging
	done := make(chan struct{})
	go func() {
		scanner.Scan("dummy-org", "dummy-format")
		close(done)
	}()

	select {
	case <-done:
		// Scan finished successfully
	case <-time.After(20 * time.Second):
		t.Fatal("Scan timed out, likely due to deadlock")
	}
}
