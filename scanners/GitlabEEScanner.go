package scanners

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/reaandrew/techdetector/core"
	"github.com/reaandrew/techdetector/utils"
	gitlab "gitlab.com/gitlab-org/api/client-go"
	"go.etcd.io/bbolt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const CacheDirName = ".techdetector_cache"
const BucketName = "Projects"

// sanitizeBaseURL converts a base URL into a filesystem-safe name.
func sanitizeBaseURL(baseURL string) string {
	sanitized := strings.ToLower(baseURL)
	sanitized = strings.ReplaceAll(sanitized, "https://", "")
	sanitized = strings.ReplaceAll(sanitized, "http://", "")
	sanitized = strings.ReplaceAll(sanitized, "/", "_")
	sanitized = strings.ReplaceAll(sanitized, ":", "_")
	return sanitized
}

// ProjectJob represents a job for scanning a GitLab project.
type ProjectJob struct {
	Project *gitlab.Project
}

// ProjectResult represents the result after scanning a project.
type ProjectResult struct {
	Matches     []core.Finding
	Error       error
	ProjectName string
}

// GitlabGroupScanner scans projects within a GitLab instance.
type GitlabGroupScanner struct {
	reporter        core.Reporter
	fileScanner     FileScanner
	matchRepository core.FindingRepository
	Cutoff          string
}

// NewGitlabGroupScanner creates a new GitlabGroupScanner.
func NewGitlabGroupScanner(reporter core.Reporter,
	processors []core.FileProcessor,
	matchRepository core.FindingRepository,
	cutoff string) *GitlabGroupScanner {
	return &GitlabGroupScanner{
		reporter:        reporter,
		fileScanner:     FileScanner{processors: processors},
		matchRepository: matchRepository,
		Cutoff:          cutoff,
	}
}

// Scan fetches every project accessible to the authenticated user,
// clones them using the provided token for authentication, scans for findings,
// and generates a report.
func (scanner GitlabGroupScanner) Scan(reportFormat, gitlabToken, gitlabBaseURL string) {
	client := initializeGitLabClient(gitlabToken, gitlabBaseURL)

	// Ensure clone base directory exists
	err := os.MkdirAll(CloneBaseDir, os.ModePerm)
	if err != nil {
		log.Fatalf("Failed to create clone base directory '%s': %v", CloneBaseDir, err)
	}

	fmt.Println("Fetching all projects")

	projects, err := listAllProjects(client, gitlabBaseURL, true)
	if err != nil {
		log.Fatalf("Error listing projects: %v", err)
	}
	if len(projects) == 0 {
		log.Fatalf("No projects found. Exiting.")
	}

	// Set up job processing with a worker pool.
	jobs := make(chan ProjectJob, len(projects))
	results := make(chan ProjectResult, len(projects))

	var wg sync.WaitGroup
	for w := 1; w <= MaxWorkers; w++ {
		wg.Add(1)
		go scanner.worker(w, jobs, results, &wg, gitlabToken)
	}

	for _, project := range projects {
		jobs <- ProjectJob{Project: project}
	}
	close(jobs)

	wg.Wait()
	close(results)

	// Store results
	for res := range results {
		if res.Error != nil {
			log.Printf("Error processing project '%s': %v", res.ProjectName, res.Error)
			continue
		}
		err := scanner.matchRepository.Store(res.Matches)
		if err != nil {
			log.Fatalf("Error storing matches for '%s': %v", res.ProjectName, err)
		}
	}

	// Generate report
	err = scanner.reporter.Report(scanner.matchRepository)
	if err != nil {
		log.Fatalf("Error generating report: %v", err)
	}
}

// worker processes projects from the jobs channel and sends results to the results channel.
// The token parameter is used to authenticate git clone operations.
func (scanner GitlabGroupScanner) worker(id int, jobs <-chan ProjectJob, results chan<- ProjectResult, wg *sync.WaitGroup, token string) {
	defer wg.Done()
	for job := range jobs {
		project := job.Project
		projectName := project.PathWithNamespace
		fmt.Printf("Worker %d: Cloning project %s\n", id, projectName)

		// Path for the normal clone
		projectPath := filepath.Join(CloneBaseDir, utils.SanitizeRepoName(projectName))
		err := utils.CloneRepositoryWithToken(project.HTTPURLToRepo, projectPath, false, token)
		if err != nil {
			results <- ProjectResult{
				Error:       fmt.Errorf("failed to clone project '%s': %w", projectName, err),
				ProjectName: projectName,
			}
			// Even if cloning failed, remove any partial clone
			_ = os.RemoveAll(projectPath)
			continue
		}

		matches, err := scanner.fileScanner.TraverseAndSearch(projectPath, projectName)
		if err != nil {
			results <- ProjectResult{
				Error:       fmt.Errorf("error searching project '%s': %w", projectName, err),
				ProjectName: projectName,
			}
			// Remove normal clone
			_ = os.RemoveAll(projectPath)
			continue
		}

		// Bare clone
		bareProjectPath := filepath.Join(CloneBaseDir, utils.SanitizeRepoName(projectName)+"_bare")
		err = utils.CloneRepositoryWithToken(project.HTTPURLToRepo, bareProjectPath, true, token)
		if err != nil {
			// Remove normal clone
			_ = os.RemoveAll(projectPath)
			log.Fatalf("Failed to perform bare clone for '%s': %v", projectName, err)
		}

		gitFindings, err := utils.CollectGitMetrics(bareProjectPath, projectName, scanner.Cutoff)
		if err != nil {
			// Clean both clones
			_ = os.RemoveAll(projectPath)
			_ = os.RemoveAll(bareProjectPath)
			log.Fatalf("Error collecting Git metrics for '%s': %v", projectName, err)
		}

		fmt.Printf("Git Metrics for %s: %+v\n", projectName, gitFindings)
		matches = append(matches, gitFindings...)

		// Send back the results for this project
		results <- ProjectResult{
			Matches:     matches,
			Error:       nil,
			ProjectName: projectName,
		}

		// ------------
		// CLEANUP
		// ------------
		// Remove normal clone
		if removeErr := os.RemoveAll(projectPath); removeErr != nil {
			log.Printf("warning: failed to remove %q: %v", projectPath, removeErr)
		}
		// Remove bare clone
		if removeErr := os.RemoveAll(bareProjectPath); removeErr != nil {
			log.Printf("warning: failed to remove %q: %v", bareProjectPath, removeErr)
		}
	}
}

// initializeGitLabClient initializes and returns a GitLab client using the provided token and base URL.
func initializeGitLabClient(token, baseURL string) *gitlab.Client {
	if token == "" {
		log.Fatal("GitLab token is required (provide via --gitlab-token flag)")
	}
	// Create the client with context support.
	client, err := gitlab.NewClient(token, gitlab.WithBaseURL(baseURL))
	if err != nil {
		log.Fatalf("Failed to create GitLab client: %v", err)
	}
	return client
}

// getCacheFile returns the full path of the cache file for the GitLab base URL.
func getCacheFile(baseURL string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	cacheDir := filepath.Join(homeDir, CacheDirName)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", err
	}

	cacheFileName := fmt.Sprintf("%s_projects_cache.db", sanitizeBaseURL(baseURL))
	return filepath.Join(cacheDir, cacheFileName), nil
}

// saveProjectsToCache saves the list of projects to a BoltDB file in the cache directory.
func saveProjectsToCache(baseURL string, projects []*gitlab.Project) error {
	cacheFile, err := getCacheFile(baseURL)
	if err != nil {
		return err
	}

	db, err := bbolt.Open(cacheFile, 0666, nil)
	if err != nil {
		return err
	}
	defer db.Close()

	return db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(BucketName))
		if err != nil {
			return fmt.Errorf("create bucket: %w", err)
		}

		for _, project := range projects {
			data, _ := json.Marshal(project)
			if err := b.Put([]byte(project.PathWithNamespace), data); err != nil {
				return err
			}
		}
		return nil
	})
}

// loadProjectsFromCache loads the projects from the cache file in the cache directory.
func loadProjectsFromCache(baseURL string) ([]*gitlab.Project, error) {
	cacheFile, err := getCacheFile(baseURL)
	if err != nil {
		return nil, err
	}

	db, err := bbolt.Open(cacheFile, 0666, nil)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var projects []*gitlab.Project
	err = db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(BucketName))
		if b == nil {
			return fmt.Errorf("bucket not found")
		}

		return b.ForEach(func(k, v []byte) error {
			var project gitlab.Project
			if err := json.Unmarshal(v, &project); err != nil {
				return err
			}
			projects = append(projects, &project)
			return nil
		})
	})
	return projects, err
}

// listAllProjects fetches projects from GitLab or uses cache if available.
func listAllProjects(client *gitlab.Client, baseURL string, useCache bool) ([]*gitlab.Project, error) {
	if useCache {
		projects, err := loadProjectsFromCache(baseURL)
		if err == nil {
			fmt.Printf("Loaded %d projects from cache.\n", len(projects))
			return projects, nil
		}
		log.Printf("Failed to load from cache, proceeding with API fetch: %v", err)
	}

	var allProjects []*gitlab.Project
	ctx := context.Background()
	opts := &gitlab.ListProjectsOptions{
		ListOptions: gitlab.ListOptions{
			Page:    1,
			PerPage: 100,
		},
	}

	for {
		projects, resp, err := client.Projects.ListProjects(opts, gitlab.WithContext(ctx))
		if err != nil {
			return nil, fmt.Errorf("failed to list projects: %w", err)
		}

		allProjects = append(allProjects, projects...)
		if err := saveProjectsToCache(baseURL, allProjects); err != nil {
			log.Printf("Failed to save to cache: %v", err)
		}

		if resp.NextPage == 0 {
			break
		}

		fmt.Fprintf(os.Stderr, "Loaded %d projects \n", len(allProjects))
		opts.Page = resp.NextPage
		fmt.Printf("Fetched %d projects, total so far: %d\n", len(projects), len(allProjects))
	}

	fmt.Printf("Number of projects found: %v\n", len(allProjects))
	return allProjects, nil
}
