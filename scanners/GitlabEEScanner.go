package scanners

import (
	"fmt"
	"github.com/reaandrew/techdetector/core"
	"github.com/reaandrew/techdetector/utils"
	"github.com/xanzy/go-gitlab"
	"log"
	"os"
	"path/filepath"
	"sync"
)

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

	projects, err := listAllProjects(client)
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

		projectPath := filepath.Join(CloneBaseDir, utils.SanitizeRepoName(projectName))
		err := utils.CloneRepositoryWithToken(project.HTTPURLToRepo, projectPath, false, token)
		if err != nil {
			results <- ProjectResult{
				Matches:     nil,
				Error:       fmt.Errorf("failed to clone project '%s': %w", projectName, err),
				ProjectName: projectName,
			}
			continue
		}

		matches, err := scanner.fileScanner.TraverseAndSearch(projectPath, projectName)
		if err != nil {
			results <- ProjectResult{
				Matches:     nil,
				Error:       fmt.Errorf("error searching project '%s': %w", projectName, err),
				ProjectName: projectName,
			}
			continue
		}

		// Perform a bare clone to extract metadata
		bareProjectPath := filepath.Join(CloneBaseDir, utils.SanitizeRepoName(projectName)+"_bare")
		err = utils.CloneRepositoryWithToken(project.HTTPURLToRepo, bareProjectPath, true, token)
		if err != nil {
			log.Fatalf("Failed to perform bare clone for '%s': %v", projectName, err)
		}

		// Collect Git metrics
		gitFindings, err := utils.CollectGitMetrics(bareProjectPath, projectName, scanner.Cutoff)
		if err != nil {
			log.Fatalf("Error collecting Git metrics for '%s': %v", projectName, err)
		}

		fmt.Printf("Git Metrics for %s: %+v\n", projectName, gitFindings)
		matches = append(matches, gitFindings...)

		results <- ProjectResult{
			Matches:     matches,
			Error:       nil,
			ProjectName: projectName,
		}
	}
}

// initializeGitLabClient initializes and returns a GitLab client using the provided token and base URL.
func initializeGitLabClient(token, baseURL string) *gitlab.Client {
	if token == "" {
		log.Fatal("GitLab token is required (provide via --gitlab-token flag)")
	}
	var client *gitlab.Client
	var err error
	if baseURL != "" {
		client, err = gitlab.NewClient(token, gitlab.WithBaseURL(baseURL))
	} else {
		client, err = gitlab.NewClient(token)
	}
	if err != nil {
		log.Fatalf("Failed to create GitLab client: %v", err)
	}
	return client
}

// listAllProjects lists every project accessible to the authenticated user.
func listAllProjects(client *gitlab.Client) ([]*gitlab.Project, error) {
	var allProjects []*gitlab.Project
	opts := &gitlab.ListProjectsOptions{
		// Setting Membership to false will return all projects the admin can access.
		Membership: gitlab.Bool(false),
		ListOptions: gitlab.ListOptions{
			Page:    1,   // start at page 1
			PerPage: 100, // adjust as needed
		},
	}

	for {
		projects, resp, err := client.Projects.ListProjects(opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list projects: %w", err)
		}
		allProjects = append(allProjects, projects...)
		if resp.CurrentPage >= resp.TotalPages {
			break
		}
		opts.Page = resp.NextPage
	}

	fmt.Printf("Number of projects found: %v\n", len(allProjects))
	return allProjects, nil
}
