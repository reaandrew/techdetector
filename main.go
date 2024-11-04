package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"

	"github.com/go-git/go-git/v5"
	"github.com/google/go-github/v50/github"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

//go:embed data/cloud_service_mappings/*.json
var servicesFS embed.FS

type Service struct {
	CloudVendor  string `json:"cloud_vendor"`
	CloudService string `json:"cloud_service"`
	Language     string `json:"language"`
	Reference    string `json:"reference"`
}

type Finding struct {
	Service    Service `json:"service"`
	Repository string  `json:"repository"`
	Filepath   string  `json:"filepath"`
}

type ServiceRegex struct {
	Service Service
	Regex   *regexp.Regexp
}

const (
	MaxWorkers     = 5
	MaxFileWorkers = 10
)

// RepoJob represents a job for processing a repository
type RepoJob struct {
	Repo *github.Repository
}

// RepoResult represents the result of processing a repository
type RepoResult struct {
	Findings []Finding
	Error    error
	RepoName string
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "techdetector",
		Short: "TechDetector is a tool to scan repositories for technologies.",
	}

	scanCmd := &cobra.Command{
		Use:   "scan",
		Short: "Scan repositories or organizations for technologies.",
	}

	scanRepoCmd := &cobra.Command{
		Use:   "repo <REPO_URL>",
		Short: "Scan a single Git repository for technologies.",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			repoURL := args[0]
			runScanRepo(repoURL)
		},
	}

	scanOrgCmd := &cobra.Command{
		Use:   "github_org <ORG_NAME>",
		Short: "Scan all repositories within a GitHub organization for technologies.",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			orgName := args[0]
			runScanOrg(orgName)
		},
	}

	scanCmd.AddCommand(scanRepoCmd)
	scanCmd.AddCommand(scanOrgCmd)

	rootCmd.AddCommand(scanCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Error executing command: %v", err)
	}
}

func getSupportedFileExtensions(services []Service) []string {
	var supportedExtensions []string
	for _, service := range services {
		if service.Language != "" {
			supportedExtensions = append(supportedExtensions, service.Language)
		}
	}
	return supportedExtensions
}

func runScanRepo(repoURL string) {
	allServices := loadAllCloudServices()
	allSupportedFileExtensions := getSupportedFileExtensions(allServices)
	if len(allServices) == 0 {
		log.Fatal("No services found. Exiting.")
	}

	serviceRegexes := compileRegexes(allServices)
	if len(serviceRegexes) == 0 {
		log.Fatal("No valid regex patterns compiled. Exiting.")
	}

	cloneBaseDir := "/tmp/techdetector" // You can make this configurable if needed
	err := os.MkdirAll(cloneBaseDir, os.ModePerm)
	if err != nil {
		log.Fatalf("Failed to create clone base directory '%s': %v", cloneBaseDir, err)
	}

	repoName, err := extractRepoName(repoURL)
	if err != nil {
		log.Fatalf("Invalid repository URL '%s': %v", repoURL, err)
	}

	repoPath := filepath.Join(cloneBaseDir, sanitizeRepoName(repoName))
	fmt.Printf("Cloning repository: %s\n", repoName)
	err = cloneRepository(repoURL, repoPath)
	if err != nil {
		log.Fatalf("Failed to clone repository '%s': %v", repoName, err)
	}

	findings, err := traverseAndSearch(repoPath, serviceRegexes, repoName, allSupportedFileExtensions)
	if err != nil {
		log.Fatalf("Error searching repository '%s': %v", repoName, err)
	}

	findingsJSON, err := json.MarshalIndent(findings, "", "    ")
	if err != nil {
		log.Fatalf("Failed to marshal findings to JSON: %v", err)
	}

	fmt.Println(string(findingsJSON))
}

func runScanOrg(orgName string) {
	allServices := loadAllCloudServices()
	allSupportedFileExtensions := getSupportedFileExtensions(allServices)
	if len(allServices) == 0 {
		log.Fatal("No services found. Exiting.")
	}

	serviceRegexes := compileRegexes(allServices)
	if len(serviceRegexes) == 0 {
		log.Fatal("No valid regex patterns compiled. Exiting.")
	}

	client := initializeGitHubClient()

	cloneBaseDir := "/tmp/techdetector" // You can make this configurable if needed
	err := os.MkdirAll(cloneBaseDir, os.ModePerm)
	if err != nil {
		log.Fatalf("Failed to create clone base directory '%s': %v", cloneBaseDir, err)
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
		go worker(w, jobs, results, cloneBaseDir, serviceRegexes, allSupportedFileExtensions, &wg)
	}

	for _, repo := range repositories {
		jobs <- RepoJob{Repo: repo}
	}
	close(jobs)

	wg.Wait()
	close(results)

	var allFindings []Finding
	for res := range results {
		if res.Error != nil {
			log.Printf("Error processing repository '%s': %v", res.RepoName, res.Error)
			continue
		}
		allFindings = append(allFindings, res.Findings...)
	}

	findingsJSON, err := json.MarshalIndent(allFindings, "", "    ")
	if err != nil {
		log.Fatalf("Failed to marshal findings to JSON: %v", err)
	}

	fmt.Println(string(findingsJSON))
}

func sanitizeRepoName(fullName string) string {
	return strings.ReplaceAll(fullName, "/", "_")
}

func extractRepoName(repoURL string) (string, error) {
	var repoName string
	if strings.HasPrefix(repoURL, "git@") {
		parts := strings.Split(repoURL, ":")
		if len(parts) != 2 {
			return "", fmt.Errorf("unexpected repository URL format")
		}
		repoName = strings.TrimSuffix(parts[1], ".git")
	} else if strings.HasPrefix(repoURL, "https://") || strings.HasPrefix(repoURL, "http://") {
		parts := strings.Split(repoURL, "/")
		if len(parts) < 2 {
			return "", fmt.Errorf("unexpected repository URL format")
		}
		repoName = strings.TrimSuffix(parts[len(parts)-1], ".git")
	} else {
		return "", fmt.Errorf("unsupported repository URL format")
	}
	return repoName, nil
}

func compileRegexes(allServices []Service) []ServiceRegex {
	serviceRegexes := []ServiceRegex{}
	for _, service := range allServices {
		pattern := service.Reference
		re, err := regexp.Compile(pattern)
		if err != nil {
			log.Printf("Failed to compile regex pattern '%s' from service '%s': %v", pattern, service.CloudService, err)
			continue
		}
		serviceRegexes = append(serviceRegexes, ServiceRegex{
			Service: service,
			Regex:   re,
		})
	}
	return serviceRegexes
}

func loadAllCloudServices() []Service {
	var allServices []Service

	entries, err := servicesFS.ReadDir("data/cloud_service_mappings")
	if err != nil {
		log.Fatalf("Failed to read embedded directory: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		content, err := servicesFS.ReadFile(fmt.Sprintf("data/cloud_service_mappings/%s", entry.Name()))
		if err != nil {
			log.Printf("Failed to read file %s: %v", entry.Name(), err)
			continue
		}

		var services []Service
		err = json.Unmarshal(content, &services)
		if err != nil {
			log.Printf("Failed to unmarshal JSON from file %s: %v", entry.Name(), err)
			continue
		}

		allServices = append(allServices, services...)
	}
	return allServices
}

func traverseAndSearch(targetDir string, serviceRegexes []ServiceRegex, repoName string, supportedFileExtensions []string) ([]Finding, error) {
	var findings []Finding

	info, err := os.Stat(targetDir)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("target directory '%s' does not exist", targetDir)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("'%s' is not a directory", targetDir)
	}

	supportedExtMap := make(map[string]struct{})
	for _, ext := range supportedFileExtensions {
		supportedExtMap[ext] = struct{}{}
	}

	files := make(chan string, 100)
	fileFindings := make(chan Finding, 100)

	var wg sync.WaitGroup

	for i := 0; i < MaxFileWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range files {
				ext := strings.TrimLeft(filepath.Ext(path), ".")
				if _, ok := supportedExtMap[ext]; !ok {
					continue
				}

				content, err := os.ReadFile(path)
				if err != nil {
					log.Printf("Failed to read file '%s': %v", path, err)
					continue
				}

				text := string(content)

				for _, sre := range serviceRegexes {
					if sre.Service.Language != "" && sre.Service.Language != ext {
						continue
					}

					matches := sre.Regex.FindAllString(text, -1)
					if len(matches) > 0 {
						for range matches {
							fileFindings <- Finding{
								Service:    sre.Service,
								Repository: repoName,
								Filepath:   path,
							}
						}
					}
				}
			}
		}()
	}

	go func() {
		filepath.WalkDir(targetDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				log.Printf("Error accessing path '%s': %v", path, err)
				return nil // Continue walking.
			}

			if d.IsDir() {
				return nil
			}

			files <- path
			return nil
		})
		close(files)
	}()

	// Collect findings in a separate goroutine
	go func() {
		wg.Wait()
		close(fileFindings)
	}()

	for finding := range fileFindings {
		findings = append(findings, finding)
	}

	return findings, nil
}

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

	fmt.Printf("Number of repos %v \n", len(allRepos))
	syscall.Exit(1)
	return allRepos, nil
}

func cloneRepository(cloneURL, destination string) error {
	if _, err := os.Stat(destination); err == nil {
		log.Printf("Repository already cloned at '%s'. Skipping clone.", destination)
		return nil
	}

	_, err := git.PlainClone(destination, false, &git.CloneOptions{
		URL:      cloneURL,
		Progress: os.Stdout,
	})
	if err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	return nil
}

func worker(id int, jobs <-chan RepoJob, results chan<- RepoResult, cloneBaseDir string, serviceRegexes []ServiceRegex, supportedFileExtensions []string, wg *sync.WaitGroup) {
	defer wg.Done()
	for job := range jobs {
		repo := job.Repo
		repoName := repo.GetFullName()
		fmt.Printf("Worker %d: Cloning repository %s\n", id, repoName)

		repoPath := filepath.Join(cloneBaseDir, sanitizeRepoName(repoName))
		err := cloneRepository(repo.GetCloneURL(), repoPath)
		if err != nil {
			results <- RepoResult{
				Findings: nil,
				Error:    fmt.Errorf("failed to clone repository '%s': %w", repoName, err),
				RepoName: repoName,
			}
			continue
		}

		findings, err := traverseAndSearch(repoPath, serviceRegexes, repoName, supportedFileExtensions)
		if err != nil {
			results <- RepoResult{
				Findings: nil,
				Error:    fmt.Errorf("error searching repository '%s': %w", repoName, err),
				RepoName: repoName,
			}
			continue
		}

		results <- RepoResult{
			Findings: findings,
			Error:    nil,
			RepoName: repoName,
		}
	}
}
