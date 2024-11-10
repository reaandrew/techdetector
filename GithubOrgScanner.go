package main

import (
	"context"
	"fmt"
	"github.com/google/go-github/v50/github"
	"golang.org/x/oauth2"
	"log"
	"os"
	"path/filepath"
	"sync"
)

type RepoJob struct {
	Repo *github.Repository
}

type RepoResult struct {
	Matches  []Match
	Error    error
	RepoName string
}

type GithubOrgScanner struct {
	reporter    Reporter
	fileScanner FileScanner
}

func NewGithubOrgScanner(reporter Reporter, processors []FileProcessor) *GithubOrgScanner {
	return &GithubOrgScanner{
		reporter:    reporter,
		fileScanner: FileScanner{processors: processors},
	}
}

func (githubOrgScanner GithubOrgScanner) scan(orgName string, reportFormat string) {
	client := initializeGitHubClient()

	// Ensure clone base directory exists
	err := os.MkdirAll(CloneBaseDir, os.ModePerm)
	if err != nil {
		log.Fatalf("Failed to create clone base directory '%s': %v", CloneBaseDir, err)
	}

	fmt.Printf("Fetching repositories for organization: %s\n", orgName)

	repositories, err := listRepositories(client, orgName)
	if err != nil {
		log.Fatalf("Error listing repositories: %v", err)
	}
	if len(repositories) == 0 {
		log.Fatalf("No repositories found in organization '%s'. Exiting.", orgName)
	}

	jobs := make(chan RepoJob, len(repositories))
	results := make(chan RepoResult, len(repositories))

	var wg sync.WaitGroup
	for w := 1; w <= MaxWorkers; w++ {
		wg.Add(1)
		go githubOrgScanner.worker(w, jobs, results, &wg)
	}

	for _, repo := range repositories {
		jobs <- RepoJob{Repo: repo}
	}
	close(jobs)

	wg.Wait()
	close(results)

	var allMatches []Match
	for res := range results {
		if res.Error != nil {
			log.Printf("Error processing repository '%s': %v", res.RepoName, res.Error)
			continue
		}
		allMatches = append(allMatches, res.Matches...)
	}

	fmt.Printf("Total Matches: %d\n", len(allMatches)) // Debug statement

	// Generate report
	err = githubOrgScanner.reporter.GenerateReport(allMatches, reportFormat)
	if err != nil {
		log.Fatalf("Error generating report: %v", err)
	}
}

// worker processes repositories from the jobs channel and sends results to the results channel.
func (githubOrgScanner GithubOrgScanner) worker(id int, jobs <-chan RepoJob, results chan<- RepoResult, wg *sync.WaitGroup) {
	defer wg.Done()
	for job := range jobs {
		repo := job.Repo
		repoName := repo.GetFullName()
		fmt.Printf("Worker: Cloning repository %s\n", repoName)

		repoPath := filepath.Join(CloneBaseDir, SanitizeRepoName(repoName))
		err := CloneRepository(repo.GetCloneURL(), repoPath)
		if err != nil {
			results <- RepoResult{
				Matches:  nil,
				Error:    fmt.Errorf("failed to clone repository '%s': %w", repoName, err),
				RepoName: repoName,
			}
			continue
		}

		Matches, err := githubOrgScanner.fileScanner.TraverseAndSearch(repoPath, repoName)
		if err != nil {
			results <- RepoResult{
				Matches:  nil,
				Error:    fmt.Errorf("error searching repository '%s': %w", repoName, err),
				RepoName: repoName,
			}
			continue
		}

		results <- RepoResult{
			Matches:  Matches,
			Error:    nil,
			RepoName: repoName,
		}
	}
}

// initializeGitHubClient initializes and returns a GitHub client.
func initializeGitHubClient() *github.Client {
	ctx := context.Background()
	var client *github.Client

	token := os.Getenv("GITHUB_TOKEN")
	if token != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
		tc := oauth2.NewClient(ctx, ts)
		client = github.NewClient(tc)
	} else {
		client = github.NewClient(nil)
	}

	return client
}

// listRepositories lists all repositories within a GitHub organization.
func listRepositories(client *github.Client, org string) ([]*github.Repository, error) {
	var allRepos []*github.Repository
	opt := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		repos, resp, err := client.Repositories.ListByOrg(context.Background(), org, opt)
		if err != nil {
			return nil, fmt.Errorf("failed to list repositories: %w", err)
		}

		allRepos = append(allRepos, repos...)

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	fmt.Printf("Number of repos: %v\n", len(allRepos))
	return allRepos, nil
}
