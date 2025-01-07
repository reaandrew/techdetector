package scanners

import (
	"context"
	"fmt"
	"github.com/google/go-github/v50/github"
	"github.com/reaandrew/techdetector/core"
	"github.com/reaandrew/techdetector/utils"
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
	Matches  []core.Finding
	Error    error
	RepoName string
}

type GithubOrgScanner struct {
	reporter        core.Reporter
	fileScanner     FileScanner
	matchRepository core.FindingRepository
}

func NewGithubOrgScanner(reporter core.Reporter,
	processors []core.FileProcessor,
	matchRepository core.FindingRepository) *GithubOrgScanner {
	return &GithubOrgScanner{
		reporter:        reporter,
		fileScanner:     FileScanner{processors: processors},
		matchRepository: matchRepository,
	}
}

func (githubOrgScanner GithubOrgScanner) Scan(orgName string, reportFormat string) {
	client := initializeGitHubClient()

	// Ensure clone base directory exists
	err := os.MkdirAll(CloneBaseDir, os.ModePerm)
	if err != nil {
		log.Fatalf("Failed to create clone base directory '%s': %v", CloneBaseDir, err)
	}

	fmt.Printf("Fetching repos for organization: %s\n", orgName)

	repos, err := listRepositories(client, orgName)
	if err != nil {
		log.Fatalf("Error listing repos: %v", err)
	}
	if len(repos) == 0 {
		log.Fatalf("No repos found in organization '%s'. Exiting.", orgName)
	}

	jobs := make(chan RepoJob, len(repos))
	results := make(chan RepoResult, len(repos))

	var wg sync.WaitGroup
	for w := 1; w <= MaxWorkers; w++ {
		wg.Add(1)
		go githubOrgScanner.worker(w, jobs, results, &wg)
	}

	for _, repo := range repos {
		jobs <- RepoJob{Repo: repo}
	}
	close(jobs)

	wg.Wait()
	close(results)

	for res := range results {
		if res.Error != nil {
			log.Printf("Error processing repository '%s': %v", res.RepoName, res.Error)
			continue
		}
		err := githubOrgScanner.matchRepository.Store(res.Matches)
		if err != nil {
			log.Fatalf("Error storing matches in '%s': %v", res.RepoName, err)
		}
	}

	// Generate summaries

	cloudVendors := map[string]int{}
	var summaries []core.Finding
	iterator := githubOrgScanner.matchRepository.NewIterator()
	for iterator.HasNext() {
		matchSet, _ := iterator.Next()

		for _, match := range matchSet.Matches {
			if val, ok := match.Properties["vendor"]; ok {
				if _, vendorOk := cloudVendors[val.(string)]; !vendorOk {
					cloudVendors[val.(string)] = 0
				}
				cloudVendors[val.(string)]++
			}
		}

	}

	for key, value := range cloudVendors {
		summaries = append(summaries, core.Finding{
			Name:     "Cloud Vendors",
			Report:   "Summary",
			Category: key,
			Properties: map[string]interface{}{
				"count": value,
			},
			Path:     "",
			RepoName: "",
		})
	}

	githubOrgScanner.matchRepository.Store(summaries)

	// Generate report
	err = githubOrgScanner.reporter.Report(githubOrgScanner.matchRepository)
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

		repoPath := filepath.Join(CloneBaseDir, utils.SanitizeRepoName(repoName))
		err := utils.CloneRepository(repo.GetCloneURL(), repoPath)
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
