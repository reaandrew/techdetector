package scanners

import (
	"context"
	"fmt"
	"github.com/google/go-github/v50/github"
	"github.com/reaandrew/techdetector/core"
	"github.com/reaandrew/techdetector/utils"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type RepoJob struct {
	Repo *github.Repository
}

type RepoResult struct {
	Matches  []core.Finding
	Error    error
	RepoName string
}

type GithubOrgScanner struct {
	reporter         core.Reporter
	fileScanner      FileScanner
	matchRepository  core.FindingRepository
	Cutoff           string
	progressReporter utils.ProgressReporter
}

func NewGithubOrgScanner(reporter core.Reporter,
	processors []core.FileProcessor,
	matchRepository core.FindingRepository,
	cutoff string,
	progressReporter utils.ProgressReporter) *GithubOrgScanner {
	return &GithubOrgScanner{
		reporter:         reporter,
		fileScanner:      FileScanner{processors: processors},
		matchRepository:  matchRepository,
		Cutoff:           cutoff,
		progressReporter: progressReporter,
	}
}

func (githubOrgScanner GithubOrgScanner) Scan(orgName string, reportFormat string) {
	client := initializeGitHubClient()

	if err := os.MkdirAll(CloneBaseDir, os.ModePerm); err != nil {
		log.Fatalf("Failed to create clone base directory '%s': %v", CloneBaseDir, err)
	}

	repos, err := listRepositories(client, orgName)
	if err != nil || len(repos) == 0 {
		log.Fatalf("Error listing repos or no repos found: %v", err)
	}

	totalRepos := len(repos)
	if githubOrgScanner.progressReporter != nil {
		githubOrgScanner.progressReporter.SetTotal(totalRepos)
	}

	jobs := make(chan RepoJob, totalRepos)
	results := make(chan RepoResult, MaxWorkers) // Small buffer

	var wg sync.WaitGroup
	for w := 1; w <= MaxWorkers; w++ {
		wg.Add(1)
		go githubOrgScanner.worker(w, jobs, results, &wg)
	}

	go func() {
		for _, repo := range repos {
			jobs <- RepoJob{Repo: repo}
		}
		close(jobs)
	}()

	// Important fix: concurrent result consumption and progress increment
	go func() {
		wg.Wait()
		close(results)
	}()

	processed := 0
	for res := range results {
		if res.Error != nil {
			log.Printf("Error processing repository '%s': %v", res.RepoName, res.Error)
		}
		if githubOrgScanner.progressReporter != nil {
			githubOrgScanner.progressReporter.Increment()
		}
		processed++
	}

	log.Printf("Finished scanning %d repositories.", processed)
	log.Println("Generating report...")
	if err := githubOrgScanner.reporter.Report(githubOrgScanner.matchRepository); err != nil {
		log.Fatalf("Error generating report: %v", err)
	}
}

func (githubOrgScanner GithubOrgScanner) worker(id int, jobs <-chan RepoJob, results chan<- RepoResult, wg *sync.WaitGroup) {
	defer wg.Done()
	for job := range jobs {
		repo := job.Repo
		repoName := repo.GetFullName()
		repoPath := filepath.Join(CloneBaseDir, utils.SanitizeRepoName(repoName))

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		err := utils.CloneRepositoryWithContext(ctx, repo.GetCloneURL(), repoPath, false)
		cancel()

		if err != nil {
			results <- RepoResult{Error: fmt.Errorf("failed to clone '%s': %w", repoName, err), RepoName: repoName}
			if githubOrgScanner.progressReporter != nil {
				githubOrgScanner.progressReporter.Increment()
			}
			continue
		}

		matches, err := githubOrgScanner.fileScanner.TraverseAndSearch(repoPath, repoName)
		if err != nil {
			results <- RepoResult{Error: fmt.Errorf("error scanning '%s': %w", repoName, err), RepoName: repoName}
			if githubOrgScanner.progressReporter != nil {
				githubOrgScanner.progressReporter.Increment()
			}
			continue
		}

		bareRepoPath := filepath.Join(CloneBaseDir, utils.SanitizeRepoName(repoName)+"_bare")
		ctxBare, cancelBare := context.WithTimeout(context.Background(), 1*time.Minute)
		err = utils.CloneRepositoryWithContext(ctxBare, repo.GetCloneURL(), bareRepoPath, true)
		cancelBare()

		if err != nil {
			results <- RepoResult{Error: fmt.Errorf("failed bare clone '%s': %w", repoName, err), RepoName: repoName}
			if githubOrgScanner.progressReporter != nil {
				githubOrgScanner.progressReporter.Increment()
			}
			continue
		}

		gitFindings, err := utils.CollectGitMetrics(bareRepoPath, repoName, githubOrgScanner.Cutoff)
		if err != nil {
			results <- RepoResult{Error: fmt.Errorf("git metrics '%s': %w", repoName, err), RepoName: repoName}
			if githubOrgScanner.progressReporter != nil {
				githubOrgScanner.progressReporter.Increment()
			}
			continue
		}

		matches = append(matches, gitFindings...)

		err = githubOrgScanner.matchRepository.Store(matches)
		if err != nil {
			results <- RepoResult{Error: fmt.Errorf("storing matches '%s': %w", repoName, err), RepoName: repoName}
			continue
		}

		results <- RepoResult{Matches: matches, RepoName: repoName}
		_ = os.RemoveAll(repoPath)
		_ = os.RemoveAll(bareRepoPath)

		if githubOrgScanner.progressReporter != nil {
			githubOrgScanner.progressReporter.Increment()
		}
	}
}

func initializeGitHubClient() *github.Client {
	ctx := context.Background()
	token := os.Getenv("GITHUB_TOKEN")
	if token != "" {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
		tc := oauth2.NewClient(ctx, ts)
		return github.NewClient(tc)
	}
	return github.NewClient(nil)
}

func listRepositories(client *github.Client, org string) ([]*github.Repository, error) {
	var allRepos []*github.Repository
	opt := &github.RepositoryListByOrgOptions{ListOptions: github.ListOptions{PerPage: 100}}
	for {
		repos, resp, err := client.Repositories.ListByOrg(context.Background(), org, opt)
		if err != nil {
			return nil, err
		}
		allRepos = append(allRepos, repos...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return allRepos, nil
}
