package scanners

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/reaandrew/techdetector/core"
	"github.com/reaandrew/techdetector/utils"
	log "github.com/sirupsen/logrus"
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

const (
	workerBufferSize = 100 // Buffer size for channels
)

type ProjectJob struct {
	Project *gitlab.Project
}

type ProjectResult struct {
	Matches     []core.Finding
	Error       error
	ProjectName string
}

type GitlabEEScanner struct {
	Reporter         core.Reporter
	FileScanner      FileScanner
	MatchRepository  core.FindingRepository
	Cutoff           string
	ProgressReporter utils.ProgressReporter
	GitlabApi        utils.GitlabApi
	GitClient        utils.GitApi
	GitMetrics       utils.GitMetrics
}

func (scanner GitlabEEScanner) Scan() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := os.MkdirAll(CloneBaseDir, os.ModePerm); err != nil {
		log.Fatalf("Failed to create clone base directory '%s': %v", CloneBaseDir, err)
	}

	projects, err := scanner.GitlabApi.ListAllProjects()
	if err != nil || len(projects) == 0 {
		log.Fatalf("Error listing projects or no projects found: %v", err)
	}

	scanner.ProgressReporter.SetTotal(len(projects))

	jobs := make(chan ProjectJob, workerBufferSize)
	results := make(chan ProjectResult, workerBufferSize)

	// Use WaitGroup pool with configurable workers
	var wg sync.WaitGroup
	workerCount := min(MaxWorkers, len(projects))
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go scanner.worker(ctx, i+1, jobs, results, &wg)
	}

	// Feed jobs
	go func() {
		defer close(jobs)
		for _, project := range projects {
			select {
			case jobs <- ProjectJob{Project: project}:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Handle results in separate goroutine
	go func() {
		wg.Wait()
		close(results)
	}()

	for res := range results {
		if res.Error != nil {
			log.Printf("Error processing project '%s': %v", res.ProjectName, res.Error)
			continue
		}
		if err := scanner.MatchRepository.Store(res.Matches); err != nil {
			log.Printf("Error storing matches for '%s': %v", res.ProjectName, err)
			continue
		}
		scanner.ProgressReporter.Increment()
	}

	scanner.ProgressReporter.Finish()
	if err := scanner.Reporter.Report(scanner.MatchRepository); err != nil {
		log.Fatalf("Error generating report: %v", err)
	}
}

func (scanner GitlabEEScanner) worker(
	ctx context.Context,
	id int,
	jobs <-chan ProjectJob,
	results chan<- ProjectResult,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	for {
		select {
		case job, ok := <-jobs:
			if !ok {
				return
			}
			if err := scanner.processProject(ctx, job.Project, id, results); err != nil {
				results <- ProjectResult{
					Error:       err,
					ProjectName: job.Project.PathWithNamespace,
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func (scanner GitlabEEScanner) processProject(
	ctx context.Context,
	project *gitlab.Project,
	workerID int,
	results chan<- ProjectResult,
) error {
	projectName := project.PathWithNamespace
	log.Printf("Worker %d: Processing project %s", workerID, projectName)

	projectPath := filepath.Join(CloneBaseDir, utils.SanitizeRepoName(projectName))
	bareProjectPath := projectPath + "_bare"

	// Cleanup function
	defer func() {
		for _, path := range []string{projectPath, bareProjectPath} {
			if err := os.RemoveAll(path); err != nil {
				log.Printf("Warning: failed to remove %q: %v", path, err)
			}
		}
	}()

	// Regular clone
	if err := scanner.GitClient.NewClone(ctx, project.HTTPURLToRepo, projectPath).
		WithToken(scanner.GitlabApi.Token()).
		Clone(); err != nil {
		return fmt.Errorf("failed to clone project '%s': %w", projectName, err)
	}

	// File scanning
	matches, err := scanner.FileScanner.TraverseAndSearch(projectPath, projectName)
	if err != nil {
		return fmt.Errorf("error searching project '%s': %w", projectName, err)
	}

	// Bare clone for metrics
	if err := scanner.GitClient.NewClone(ctx, project.HTTPURLToRepo, bareProjectPath).
		WithBare(true).
		WithToken(scanner.GitlabApi.Token()).
		Clone(); err != nil {
		return fmt.Errorf("failed to perform bare clone for '%s': %w", projectName, err)
	}

	// Collect git metrics
	gitFindings, err := scanner.GitMetrics.CollectGitMetrics(bareProjectPath, projectName, scanner.Cutoff)
	if err != nil {
		return fmt.Errorf("error collecting Git metrics for '%s': %w", projectName, err)
	}

	matches = append(matches, gitFindings...)
	results <- ProjectResult{
		Matches:     matches,
		ProjectName: projectName,
	}
	return nil
}
