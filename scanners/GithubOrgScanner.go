package scanners

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/go-github/v50/github"
	"github.com/reaandrew/techdetector/core"
	"github.com/reaandrew/techdetector/utils"
	log "github.com/sirupsen/logrus"
)

// RepoJob represents a repository to process
type RepoJob struct {
	Repo *github.Repository
}

// RepoResult captures the outcome of processing a repository
type RepoResult struct {
	Error    error
	RepoName string
}

// GithubOrgScanner scans GitHub organizations for tech findings
type GithubOrgScanner struct {
	Reporter         core.Reporter
	FileScanner      FileScanner
	MatchRepository  core.FindingRepository
	ProgressReporter utils.ProgressReporter
	GithubClient     utils.GithubApi
	GitClient        utils.GitApi
	PostScanners     []core.PostScanner
	wg               sync.WaitGroup
}

// Scan processes repositories from a GitHub organization
func (g *GithubOrgScanner) Scan(orgName string, reportFormat string) {
	log.Infof("Starting scan for org: %s", orgName)
	repos, err := g.GithubClient.ListRepositories(orgName)
	if err != nil {
		log.Fatalf("Error listing repositories: %v", err)
	}

	totalRepos := len(repos)
	if totalRepos == 0 {
		log.Info("No repositories found")
		g.ProgressReporter.Finish()
		return
	}

	log.Infof("Scanning %d repositories", totalRepos)
	g.ProgressReporter.SetTotal(totalRepos)

	jobs := make(chan RepoJob, min(totalRepos, MaxWorkers*2))
	results := make(chan RepoResult, totalRepos)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for i := 0; i < min(MaxWorkers, totalRepos); i++ {
		g.wg.Add(1)
		go g.worker(ctx, i, jobs, results)
	}

	go func() {
		for _, repo := range repos {
			log.Debugf("Enqueuing %s", repo.GetFullName())
			jobs <- RepoJob{Repo: repo}
		}
		log.Debug("All jobs enqueued")
		close(jobs)
	}()

	go func() {
		g.wg.Wait()
		log.Debug("All workers done")
		close(results)
	}()

	var errors []error
	for res := range results {
		if res.Error != nil {
			log.Errorf("Error with %s: %v", res.RepoName, res.Error)
			errors = append(errors, res.Error)
		}
		g.ProgressReporter.Increment()
	}

	log.Info("All results processed")
	g.ProgressReporter.Finish()

	if len(errors) > 0 {
		log.Warnf("Encountered %d errors", len(errors))
	}

	log.Debug("Generating report")
	if err := g.Reporter.Report(g.MatchRepository); err != nil {
		log.Fatalf("Report generation failed: %v", err)
	}
	log.Info("Scan completed")
}

// worker processes repository jobs
func (g *GithubOrgScanner) worker(ctx context.Context, workerId int, jobs <-chan RepoJob, results chan<- RepoResult) {
	defer g.wg.Done()
	for {
		select {
		case <-ctx.Done():
			log.Warnf("Worker %d cancelled", workerId)
			return
		case job, ok := <-jobs:
			if !ok {
				log.Debugf("Worker %d done", workerId)
				return
			}
			repoName := job.Repo.GetFullName()
			log.Infof("Worker %d started %s", workerId, repoName)
			err := g.processRepository(job.Repo)
			if err != nil {
				log.Errorf("Worker %d failed %s: %v", workerId, repoName, err)
				results <- RepoResult{Error: err, RepoName: repoName}
			} else {
				log.Infof("Worker %d completed %s", workerId, repoName)
				results <- RepoResult{RepoName: repoName}
			}
		}
	}
}

// processRepository handles cloning, scanning, and storing findings for a repo
func (g *GithubOrgScanner) processRepository(repo *github.Repository) error {
	repoName := repo.GetFullName()
	repoPath := filepath.Join(CloneBaseDir, utils.SanitizeRepoName(repoName))
	bareRepoPath := repoPath + "_bare"
	startTime := time.Now()

	log.Debugf("Cloning %s to %s", repoName, repoPath)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := os.MkdirAll(filepath.Dir(repoPath), 0755); err != nil {
		log.Errorf("Failed to create dir %s: %v", repoPath, err)
		return fmt.Errorf("failed to create directory for %s: %w", repoPath, err)
	}
	defer func() {
		if err := os.RemoveAll(repoPath); err != nil {
			log.Warnf("Cleanup failed for %s: %v", repoPath, err)
		} else {
			log.Debugf("Cleaned up %s", repoPath)
		}
	}()

	if err := g.GitClient.CloneRepositoryWithContext(ctx, repo.GetCloneURL(), repoPath, false); err != nil {
		log.Errorf("Clone failed for %s: %v", repoName, err)
		return fmt.Errorf("clone failed for %s: %w", repoName, err)
	}
	log.Debugf("Cloned %s in %v", repoName, time.Since(startTime))

	log.Debugf("Scanning files for %s", repoName)
	scanStart := time.Now()

	matches, err := g.FileScanner.TraverseAndSearch(repoPath, repoName)
	if err != nil {
		log.Errorf("File scan failed for %s: %v", repoName, err)
		return fmt.Errorf("file scan failed for %s: %w", repoName, err)
	}
	log.Infof("Scanned %s, found %d matches in %v", repoName, len(matches), time.Since(scanStart))

	log.Debugf("Bare cloning %s to %s", repoName, bareRepoPath)
	ctxBare, cancelBare := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancelBare()
	if err := os.MkdirAll(filepath.Dir(bareRepoPath), 0755); err != nil {
		log.Errorf("Failed to create dir %s: %v", bareRepoPath, err)
		return fmt.Errorf("failed to create directory for %s: %w", bareRepoPath, err)
	}
	defer func() {
		if err := os.RemoveAll(bareRepoPath); err != nil {
			log.Warnf("Cleanup failed for %s: %v", bareRepoPath, err)
		} else {
			log.Debugf("Cleaned up %s", bareRepoPath)
		}
	}()

	if err := g.GitClient.CloneRepositoryWithContext(ctxBare, repo.GetCloneURL(), bareRepoPath, true); err != nil {
		log.Errorf("Bare clone failed for %s: %v", repoName, err)
		return fmt.Errorf("bare clone failed for %s: %w", repoName, err)
	}
	log.Debugf("Bare cloned %s in %v", repoName, time.Since(startTime))

	log.Debugf("Storing %d findings for %s", len(matches), repoName)
	storeStart := time.Now()

	for _, postScanner := range g.PostScanners {
		postScannerMatches, err := postScanner.Scan(bareRepoPath, repoName)
		if err != nil {
			return fmt.Errorf("post scanner error '%s': %w", repoName, err)
		}
		matches = append(matches, postScannerMatches...)
	}

	if err := g.MatchRepository.Store(matches); err != nil {
		log.Errorf("Store failed for %s: %v", repoName, err)
		return fmt.Errorf("failed to store findings for %s: %w", repoName, err)
	}
	log.Infof("Stored %d findings for %s in %v", len(matches), repoName, time.Since(storeStart))

	return nil
}
