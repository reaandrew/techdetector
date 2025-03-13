package scanners

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/go-github/v50/github"
	"github.com/reaandrew/techdetector/core"
	"github.com/reaandrew/techdetector/utils"
	log "github.com/sirupsen/logrus"
	"go.etcd.io/bbolt"
	"golang.org/x/oauth2"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	GithubBucketName = "GithubProjects"
	maxCloneRetries  = 3 // Maximum attempts for bare clone.
)

// sanitizeOrg returns a sanitized version of the organization name.
func sanitizeOrg(org string) string {
	s := strings.ToLower(org)
	s = strings.ReplaceAll(s, " ", "_")
	return s
}

// getGithubCacheFile returns the path to the cache file for the given organization.
func getGithubCacheFile(org string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	cacheDir := filepath.Join(homeDir, CacheDirName)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", err
	}
	// For example: "defra_github_cache.db"
	cacheFileName := fmt.Sprintf("%s_github_cache.db", sanitizeOrg(org))
	return filepath.Join(cacheDir, cacheFileName), nil
}

// saveReposToCache saves the provided repositories list to Bolt DB.
func saveReposToCache(org string, repos []*github.Repository) error {
	cacheFile, err := getGithubCacheFile(org)
	if err != nil {
		return err
	}
	db, err := bbolt.Open(cacheFile, 0666, nil)
	if err != nil {
		return err
	}
	defer db.Close()

	return db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(GithubBucketName))
		if err != nil {
			return fmt.Errorf("create bucket: %w", err)
		}
		// Save the entire list under a key based on the org.
		data, err := json.Marshal(repos)
		if err != nil {
			return err
		}
		return b.Put([]byte(sanitizeOrg(org)), data)
	})
}

// loadReposFromCache attempts to load repositories from the cache.
func loadReposFromCache(org string) ([]*github.Repository, error) {
	cacheFile, err := getGithubCacheFile(org)
	if err != nil {
		return nil, err
	}
	db, err := bbolt.Open(cacheFile, 0666, nil)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var repos []*github.Repository
	err = db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(GithubBucketName))
		if b == nil {
			return fmt.Errorf("bucket not found")
		}
		data := b.Get([]byte(sanitizeOrg(org)))
		if data == nil {
			return fmt.Errorf("no cache found")
		}
		return json.Unmarshal(data, &repos)
	})
	return repos, err
}

// listAllRepositories first attempts to load repositories from cache; if that fails, it uses the GitHub API.
func listAllRepositories(client *github.Client, org string, useCache bool) ([]*github.Repository, error) {
	if useCache {
		if repos, err := loadReposFromCache(org); err == nil {
			log.Infof("Loaded %d repositories from cache for org %s", len(repos), org)
			return repos, nil
		} else {
			log.Warnf("Failed to load from cache for org %s: %v", org, err)
		}
	}
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
	log.Infof("Fetched %d repositories from API for org %s", len(allRepos), org)
	if err := saveReposToCache(org, allRepos); err != nil {
		log.Warnf("Failed to save to cache for org %s: %v", org, err)
	}
	return allRepos, nil
}

//
// --- The rest of the GitHub scanner remains largely unchanged ---
//

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

func NewGithubOrgScanner(
	reporter core.Reporter,
	processors []core.FileProcessor,
	matchRepository core.FindingRepository,
	cutoff string,
	progressReporter utils.ProgressReporter,
) *GithubOrgScanner {
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

	err := os.MkdirAll(CloneBaseDir, os.ModePerm)
	if err != nil {
		log.Fatalf("Failed to create clone base directory '%s': %v", CloneBaseDir, err)
	}

	log.Infof("Fetching repos for organization: %s", orgName)
	repos, err := listAllRepositories(client, orgName, true)
	if err != nil {
		log.Fatalf("Error listing repositories: %v", err)
	}
	if len(repos) == 0 {
		log.Fatalf("No repos found in organization '%s'. Exiting.", orgName)
	}

	if githubOrgScanner.progressReporter != nil {
		githubOrgScanner.progressReporter.SetTotal(len(repos))
	}

	jobs := make(chan RepoJob, len(repos))
	results := make(chan RepoResult, len(repos))
	var wg sync.WaitGroup

	for w := 1; w <= MaxWorkers; w++ {
		wg.Add(1)
		go githubOrgScanner.worker(w, jobs, results, &wg, client)
	}

	for _, repo := range repos {
		jobs <- RepoJob{Repo: repo}
	}
	close(jobs)

	wg.Wait()
	close(results)

	for res := range results {
		if res.Error != nil {
			log.Errorf("Error processing repository '%s': %v", res.RepoName, res.Error)
			continue
		}
		err := githubOrgScanner.matchRepository.Store(res.Matches)
		if err != nil {
			log.Fatalf("Error storing matches in '%s': %v", res.RepoName, err)
		}
	}

	err = githubOrgScanner.reporter.Report(githubOrgScanner.matchRepository)
	if err != nil {
		log.Fatalf("Error generating report: %v", err)
	}
}

func (githubOrgScanner GithubOrgScanner) worker(id int, jobs <-chan RepoJob, results chan<- RepoResult, wg *sync.WaitGroup, client *github.Client) {
	defer wg.Done()
	for job := range jobs {
		repo := job.Repo
		repoName := repo.GetFullName()
		log.Infof("Worker %d: Cloning repository %s", id, repoName)

		repoPath := filepath.Join(CloneBaseDir, utils.SanitizeRepoName(repoName))
		err := utils.CloneRepository(repo.GetCloneURL(), repoPath, false)
		if err != nil {
			results <- RepoResult{
				Matches:  nil,
				Error:    fmt.Errorf("failed to clone repository '%s': %w", repoName, err),
				RepoName: repoName,
			}
			if githubOrgScanner.progressReporter != nil {
				githubOrgScanner.progressReporter.Increment()
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
			if githubOrgScanner.progressReporter != nil {
				githubOrgScanner.progressReporter.Increment()
			}
			continue
		}

		bareRepoPath := filepath.Join(CloneBaseDir, utils.SanitizeRepoName(repoName)+"_bare")
		// Add retry logic for bare clone
		var bareErr error
		for attempt := 1; attempt <= maxCloneRetries; attempt++ {
			bareErr = utils.CloneRepository(repo.GetCloneURL(), bareRepoPath, true)
			if bareErr == nil {
				break
			}
			log.Warnf("Bare clone failed for '%s' on attempt %d: %v", repoName, attempt, bareErr)
		}
		if bareErr != nil {
			results <- RepoResult{
				Matches:  nil,
				Error:    fmt.Errorf("failed to perform bare clone for '%s' after %d attempts: %w", repoName, maxCloneRetries, bareErr),
				RepoName: repoName,
			}
			if githubOrgScanner.progressReporter != nil {
				githubOrgScanner.progressReporter.Increment()
			}
			_ = os.RemoveAll(repoPath)
			continue
		}

		gitFindings, err := utils.CollectGitMetrics(bareRepoPath, repoName, githubOrgScanner.Cutoff)
		if err != nil {
			_ = os.RemoveAll(repoPath)
			_ = os.RemoveAll(bareRepoPath)
			results <- RepoResult{
				Matches:  nil,
				Error:    fmt.Errorf("error collecting Git metrics for '%s': %w", repoName, err),
				RepoName: repoName,
			}
			if githubOrgScanner.progressReporter != nil {
				githubOrgScanner.progressReporter.Increment()
			}
			continue
		}
		log.Infof("Git Metrics for %s: %+v", repoName, gitFindings)
		Matches = append(Matches, gitFindings...)

		results <- RepoResult{
			Matches:  Matches,
			Error:    nil,
			RepoName: repoName,
		}

		if err := os.RemoveAll(repoPath); err != nil {
			log.Errorf("warning: failed to remove %q: %v", repoPath, err)
		}
		if err := os.RemoveAll(bareRepoPath); err != nil {
			log.Errorf("warning: failed to remove %q: %v", bareRepoPath, err)
		}

		if githubOrgScanner.progressReporter != nil {
			githubOrgScanner.progressReporter.Increment()
		}
	}
}

func initializeGitHubClient() *github.Client {
	ctx := context.Background()
	var client *github.Client
	token := os.Getenv("GITHUB_TOKEN")
	if token != "" {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
		tc := oauth2.NewClient(ctx, ts)
		client = github.NewClient(tc)
	} else {
		client = github.NewClient(nil)
	}
	return client
}
