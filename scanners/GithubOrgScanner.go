package scanners

import (
	"context"
	"fmt"
	"github.com/google/go-github/v50/github"
	"github.com/reaandrew/techdetector/core"
	"github.com/reaandrew/techdetector/utils"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type RepoJob struct {
	Repo *github.Repository
}

type RepoResult struct {
	Error    error
	RepoName string
}

type GithubOrgScanner struct {
	Reporter         core.Reporter
	FileScanner      FileScanner
	MatchRepository  core.FindingRepository
	Cutoff           string
	ProgressReporter utils.ProgressReporter
	GithubClient     utils.GithubApi
	GitClient        utils.GitApi
	GitMetrics       utils.GitMetrics
}

func (g *GithubOrgScanner) Scan(orgName string, reportFormat string) {
	repos, err := g.GithubClient.ListRepositories(orgName)
	if err != nil {
		log.Fatalf("Error listing repositories: %v", err)
	}

	totalRepos := len(repos)
	g.ProgressReporter.SetTotal(totalRepos)

	jobs := make(chan RepoJob, totalRepos)
	results := make(chan RepoResult, totalRepos)

	var wg sync.WaitGroup
	for i := 0; i < MaxWorkers; i++ {
		wg.Add(1)
		go func(workerId int) {
			defer wg.Done()
			for job := range jobs {
				repoName := job.Repo.GetFullName()
				if err := g.processRepository(job.Repo, workerId); err != nil {
					log.Errorf("Worker %d failed processing %s: %v", workerId, repoName, err)
					results <- RepoResult{Error: err, RepoName: repoName}
				} else {
					log.Infof("Worker %d completed %s", workerId, repoName)
					results <- RepoResult{RepoName: repoName}
				}
				g.ProgressReporter.Increment()
			}
		}(i)
	}

	for _, repo := range repos {
		jobs <- RepoJob{Repo: repo}
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	for res := range results {
		if res.Error != nil {
			log.Errorf("Error with repo %s: %v", res.RepoName, res.Error)
		}
	}

	// Explicitly finalize the progress bar to ensure completion visually
	g.ProgressReporter.Finish()

	if err := g.Reporter.Report(g.MatchRepository); err != nil {
		log.Fatalf("Report generation failed: %v", err)
	}
}

func (g *GithubOrgScanner) processRepository(repo *github.Repository, workerId int) error {
	repoName := repo.GetFullName()
	log.Infof("Worker %d STARTED %s", workerId, repoName)
	repoPath := filepath.Join(CloneBaseDir, utils.SanitizeRepoName(repoName))
	bareRepoPath := repoPath + "_bare"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := g.GitClient.CloneRepositoryWithContext(ctx, repo.GetCloneURL(), repoPath, false); err != nil {
		log.Errorf("Worker %d ERROR cloning %s: %v", workerId, repoName, err)

		return fmt.Errorf("clone failed: %w", err)
	}
	defer os.RemoveAll(repoPath)

	matches, err := g.FileScanner.TraverseAndSearch(repoPath, repoName)
	if err != nil {
		return fmt.Errorf("file scan failed: %w", err)
	}

	ctxBare, cancelBare := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancelBare()
	if err = g.GitClient.CloneRepositoryWithContext(ctxBare, repo.GetCloneURL(), bareRepoPath, true); err != nil {
		return fmt.Errorf("bare clone failed: %w", err)
	}
	defer os.RemoveAll(bareRepoPath)

	gitFindings, err := g.GitMetrics.CollectGitMetrics(bareRepoPath, repoName, g.Cutoff)
	if err != nil {
		return fmt.Errorf("git metrics failed: %w", err)
	}

	matches = append(matches, gitFindings...)
	log.Infof("Worker %d COMPLETED %s", workerId, repoName)

	return g.MatchRepository.Store(matches)
}
