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

	"github.com/go-git/go-git/v5"
	"github.com/google/go-github/v50/github"
	"github.com/spf13/cobra"
	"github.com/xuri/excelize/v2"
	"golang.org/x/oauth2"
)

//go:embed data/cloud_service_mappings/*.json
var servicesFS embed.FS

//go:embed data/frameworks/*.json
var frameworksFS embed.FS

type Service struct {
	CloudVendor  string `json:"cloud_vendor"`
	CloudService string `json:"cloud_service"`
	Language     string `json:"language"`
	Reference    string `json:"reference"`
}

type Framework struct {
	Name            string `json:"name,omitempty"`
	Category        string `json:"category,omitempty"`
	PackageFileName string `json:"package_file_name"`
	Pattern         string `json:"pattern"`
}

type Finding struct {
	Service    *Service   `json:"service,omitempty"`
	Framework  *Framework `json:"framework,omitempty"`
	Repository string     `json:"repository"`
	Filepath   string     `json:"filepath"`
}

type ServiceRegex struct {
	Service Service
	Regex   *regexp.Regexp
}

type FrameworkRegex struct {
	Framework Framework
	Regex     *regexp.Regexp
}

const (
	MaxWorkers      = 10
	MaxFileWorkers  = 10
	CloneBaseDir    = "/tmp/techdetector" // You can make this configurable if needed
	DefaultReport   = "cloud_services_report.xlsx"
	ServicesSheet   = "Services"
	FrameworksSheet = "Frameworks"
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

// Global variables
var (
	services                []Service
	frameworks              []Framework
	serviceRegexes          []ServiceRegex
	frameworkRegexes        []FrameworkRegex
	supportedFileExtensions []string
)

func main() {
	var reportFormat string

	rootCmd := &cobra.Command{
		Use:   "techdetector",
		Short: "TechDetector is a tool to scan repositories for technologies.",
	}

	scanCmd := createScanCommand(&reportFormat)

	rootCmd.AddCommand(scanCmd)

	// Load services and frameworks
	services = loadAllCloudServices()
	supportedFileExtensions = getSupportedFileExtensions(services)
	serviceRegexes = compileServicesRegexes(services)
	frameworks = loadAllFrameworks() // Ensure this uses frameworksFS
	frameworkRegexes = compileFrameworkRegexes(frameworks)

	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Error executing command: %v", err)
	}
}

func createScanCommand(reportFormat *string) *cobra.Command {

	scanCmd := &cobra.Command{
		Use:   "scan",
		Short: "Scan repositories or organizations for technologies.",
	}

	// Add the --report flag to the scan command
	scanCmd.PersistentFlags().StringVar(reportFormat, "report", "", "Report format (supported: xlsx)")

	scanRepoCmd := &cobra.Command{
		Use:   "repo <REPO_URL>",
		Short: "Scan a single Git repository for technologies.",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			repoURL := args[0]
			runScanRepo(repoURL, *reportFormat)
		},
	}

	scanOrgCmd := &cobra.Command{
		Use:   "github_org <ORG_NAME>",
		Short: "Scan all repositories within a GitHub organization for technologies.",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			orgName := args[0]
			runScanOrg(orgName, *reportFormat)
		},
	}

	scanCmd.AddCommand(scanRepoCmd)
	scanCmd.AddCommand(scanOrgCmd)
	return scanCmd
}

func getSupportedFileExtensions(services []Service) []string {
	extMap := make(map[string]struct{})
	for _, service := range services {
		if service.Language != "" {
			extMap[service.Language] = struct{}{}
		}
	}
	var supportedExtensions []string
	for ext := range extMap {
		supportedExtensions = append(supportedExtensions, ext)
	}
	return supportedExtensions
}

func runScanRepo(repoURL string, reportFormat string) {
	if len(services) == 0 {
		log.Fatal("No services found. Exiting.")
	}

	if len(serviceRegexes) == 0 {
		log.Fatal("No valid regex patterns compiled. Exiting.")
	}

	if len(frameworkRegexes) == 0 {
		log.Fatal("No valid framework regex patterns compiled. Exiting.")
	}

	// Ensure clone base directory exists
	err := os.MkdirAll(CloneBaseDir, os.ModePerm)
	if err != nil {
		log.Fatalf("Failed to create clone base directory '%s': %v", CloneBaseDir, err)
	}

	repoName, err := extractRepoName(repoURL)
	if err != nil {
		log.Fatalf("Invalid repository URL '%s': %v", repoURL, err)
	}

	repoPath := filepath.Join(CloneBaseDir, sanitizeRepoName(repoName))
	fmt.Printf("Cloning repository: %s\n", repoName)
	err = cloneRepository(repoURL, repoPath)
	if err != nil {
		log.Fatalf("Failed to clone repository '%s': %v", repoName, err)
	}

	// Initialize processors
	processors := initializeProcessors()

	// Traverse and search with processors
	findings, err := traverseAndSearch(repoPath, repoName, processors)
	if err != nil {
		log.Fatalf("Error searching repository '%s': %v", repoName, err)
	}

	fmt.Printf("Number of findings: %d\n", len(findings)) // Debug statement

	// Generate report
	err = generateReport(findings, reportFormat)
	if err != nil {
		log.Fatalf("Error generating report: %v", err)
	}
}

func runScanOrg(orgName string, reportFormat string) {
	if len(services) == 0 {
		log.Fatal("No services found. Exiting.")
	}

	if len(serviceRegexes) == 0 {
		log.Fatal("No valid regex patterns compiled. Exiting.")
	}

	if len(frameworkRegexes) == 0 {
		log.Fatal("No valid framework regex patterns compiled. Exiting.")
	}

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
		go worker(w, jobs, results, &wg)
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

	fmt.Printf("Total findings: %d\n", len(allFindings)) // Debug statement

	// Generate report
	err = generateReport(allFindings, reportFormat)
	if err != nil {
		log.Fatalf("Error generating report: %v", err)
	}
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

func compileServicesRegexes(allServices []Service) []ServiceRegex {
	var serviceRegexes []ServiceRegex
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

func compileFrameworkRegexes(allFrameworks []Framework) []FrameworkRegex {
	var frameworkRegexes []FrameworkRegex
	for _, framework := range allFrameworks {
		pattern := framework.Pattern
		re, err := regexp.Compile(pattern)
		if err != nil {
			log.Printf("Failed to compile regex pattern '%s' from framework '%s': %v", pattern, framework.Name, err)
			continue
		}
		frameworkRegexes = append(frameworkRegexes, FrameworkRegex{
			Framework: framework,
			Regex:     re,
		})
	}
	return frameworkRegexes
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

func loadAllFrameworks() []Framework {
	var allFrameworks []Framework

	entries, err := frameworksFS.ReadDir("data/frameworks") // Corrected from servicesFS to frameworksFS
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

		content, err := frameworksFS.ReadFile(fmt.Sprintf("data/frameworks/%s", entry.Name())) // Corrected path
		if err != nil {
			log.Printf("Failed to read file %s: %v", entry.Name(), err)
			continue
		}

		var frameworks []Framework
		err = json.Unmarshal(content, &frameworks)
		if err != nil {
			log.Printf("Failed to unmarshal JSON from file %s: %v", entry.Name(), err)
			continue
		}

		allFrameworks = append(allFrameworks, frameworks...)
	}
	return allFrameworks
}

// initializeProcessors creates and returns a slice of Processor implementations.
func initializeProcessors() []Processor {
	var processors []Processor

	// Initialize CloudServiceProcessor
	serviceProcessor := NewServiceProcessor(serviceRegexes)
	processors = append(processors, serviceProcessor)

	// Initialize FrameworkProcessor
	frameworkProcessor := NewFrameworkProcessor(frameworkRegexes)
	processors = append(processors, frameworkProcessor)

	return processors
}

// traverseAndSearch traverses the target directory and applies all processors to each file.
func traverseAndSearch(targetDir string, repoName string, processors []Processor) ([]Finding, error) {
	var findings []Finding

	info, err := os.Stat(targetDir)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("target directory '%s' does not exist", targetDir)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("'%s' is not a directory", targetDir)
	}

	// Collect supported file extensions and specific file names from all processors
	supportedExtMap := make(map[string]struct{})
	supportedFileNames := make(map[string]struct{})
	for _, processor := range processors {
		switch p := processor.(type) {
		case *CloudServiceProcessor:
			for _, sre := range p.serviceRegexes {
				if sre.Service.Language != "" {
					supportedExtMap[sre.Service.Language] = struct{}{}
				}
			}
		case *FrameworkProcessor:
			for _, fre := range p.frameworkRegexes {
				if fre.Framework.PackageFileName != "" {
					supportedFileNames[fre.Framework.PackageFileName] = struct{}{}
				}
			}
		}
	}

	files := make(chan string, 100)
	fileFindings := make(chan Finding, 100)

	var wg sync.WaitGroup

	// Start file workers
	for i := 0; i < MaxFileWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range files {
				ext := strings.TrimLeft(filepath.Ext(path), ".")
				base := filepath.Base(path)

				// Check if the file extension or name is supported
				if _, ok := supportedExtMap[ext]; !ok {
					if _, nameOk := supportedFileNames[base]; !nameOk {
						continue
					}
				}

				content, err := os.ReadFile(path)
				if err != nil {
					log.Printf("Failed to read file '%s': %v", path, err)
					continue
				}

				text := string(content)

				// Apply all processors
				for _, processor := range processors {
					results := processor.Process(path, repoName, text)
					for _, finding := range results {
						fileFindings <- finding
					}
				}
			}
		}()
	}

	// Walk the directory and send file paths to the workers
	go func() {
		err := filepath.WalkDir(targetDir, func(path string, d fs.DirEntry, err error) error {
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
		if err != nil {
			log.Printf("Error walking the directory: %v", err)
		}
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

// cloneRepository clones a Git repository to the specified destination.
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

// worker processes repositories from the jobs channel and sends results to the results channel.
func worker(id int, jobs <-chan RepoJob, results chan<- RepoResult, wg *sync.WaitGroup) {
	defer wg.Done()
	for job := range jobs {
		repo := job.Repo
		repoName := repo.GetFullName()
		fmt.Printf("Worker: Cloning repository %s\n", repoName)

		repoPath := filepath.Join(CloneBaseDir, sanitizeRepoName(repoName))
		err := cloneRepository(repo.GetCloneURL(), repoPath)
		if err != nil {
			results <- RepoResult{
				Findings: nil,
				Error:    fmt.Errorf("failed to clone repository '%s': %w", repoName, err),
				RepoName: repoName,
			}
			continue
		}

		// Initialize processors
		processors := initializeProcessors()

		findings, err := traverseAndSearch(repoPath, repoName, processors)
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

// reportXlsx generates an XLSX report from the findings.
// It creates two worksheets: "Services" and "Frameworks".
func reportXlsx(findings []Finding) error {
	fmt.Println("Generating XLSX file")

	// Create a new Excel file
	f := excelize.NewFile()

	// Rename the default sheet to "Services"
	defaultSheet := f.GetSheetName(0)
	if defaultSheet != ServicesSheet {
		if err := f.SetSheetName(defaultSheet, ServicesSheet); err != nil {
			return fmt.Errorf("failed to rename sheet '%s' to '%s': %w", defaultSheet, ServicesSheet, err)
		}
		fmt.Printf("Renamed default sheet '%s' to '%s'\n", defaultSheet, ServicesSheet)
	}

	// Create the "Frameworks" sheet
	frameworksIndex, err := f.NewSheet(FrameworksSheet)
	if err != nil {
		return fmt.Errorf("failed to create sheet '%s': %w", FrameworksSheet, err)
	}
	fmt.Printf("Created sheet '%s' with index %d\n", FrameworksSheet, frameworksIndex)

	// Set headers for Services sheet
	servicesHeaders := []string{
		"Cloud Vendor",
		"Cloud Service",
		"Language",
		"Reference",
		"Repository",
		"Filepath",
	}
	if err := f.SetSheetRow(ServicesSheet, "A1", &servicesHeaders); err != nil {
		return fmt.Errorf("failed to set headers for sheet '%s': %w", ServicesSheet, err)
	}
	fmt.Printf("Set headers for sheet '%s'\n", ServicesSheet)

	// Set headers for Frameworks sheet
	frameworksHeaders := []string{
		"Name",
		"Category",
		"Package File Name",
		"Pattern",
		"Repository",
		"Filepath",
	}
	if err := f.SetSheetRow(FrameworksSheet, "A1", &frameworksHeaders); err != nil {
		return fmt.Errorf("failed to set headers for sheet '%s': %w", FrameworksSheet, err)
	}
	fmt.Printf("Set headers for sheet '%s'\n", FrameworksSheet)

	// Initialize row counters for each sheet
	servicesRow := 2   // Starting from row 2 (row 1 is for headers)
	frameworksRow := 2 // Starting from row 2 (row 1 is for headers)

	// Iterate over findings and populate respective sheets
	for _, finding := range findings {
		if finding.Service != nil {
			// Prepare data for Services sheet
			rowData := []interface{}{
				finding.Service.CloudVendor,
				finding.Service.CloudService,
				finding.Service.Language,
				finding.Service.Reference,
				finding.Repository,
				finding.Filepath,
			}

			// Convert row number to cell address (e.g., A2)
			cellAddress, err := excelize.CoordinatesToCellName(1, servicesRow)
			if err != nil {
				return fmt.Errorf("failed to get cell address for row %d in sheet '%s': %w", servicesRow, ServicesSheet, err)
			}

			// Set the row data starting from column A
			if err := f.SetSheetRow(ServicesSheet, cellAddress, &rowData); err != nil {
				return fmt.Errorf("failed to set data for row %d in sheet '%s': %w", servicesRow, ServicesSheet, err)
			}

			servicesRow++ // Move to the next row for Services
		}

		if finding.Framework != nil {
			// Prepare data for Frameworks sheet
			rowData := []interface{}{
				finding.Framework.Name,
				finding.Framework.Category,
				finding.Framework.PackageFileName,
				finding.Framework.Pattern,
				finding.Repository,
				finding.Filepath,
			}

			// Convert row number to cell address (e.g., A2)
			cellAddress, err := excelize.CoordinatesToCellName(1, frameworksRow)
			if err != nil {
				return fmt.Errorf("failed to get cell address for row %d in sheet '%s': %w", frameworksRow, FrameworksSheet, err)
			}

			// Set the row data starting from column A
			if err := f.SetSheetRow(FrameworksSheet, cellAddress, &rowData); err != nil {
				return fmt.Errorf("failed to set data for row %d in sheet '%s': %w", frameworksRow, FrameworksSheet, err)
			}

			frameworksRow++ // Move to the next row for Frameworks
		}
	}

	index, _ := f.GetSheetIndex(ServicesSheet)
	// Optionally, set the active sheet to Services
	f.SetActiveSheet(index)

	// Determine the output file name
	outputFile := DefaultReport
	if len(findings) > 0 {
		if findings[0].Service != nil || findings[0].Framework != nil {
			outputFile = fmt.Sprintf("report_%s.xlsx", strings.ReplaceAll(findings[0].Repository, "/", "_"))
		}
	}

	// Save the Excel file
	if err := f.SaveAs(outputFile); err != nil {
		return fmt.Errorf("failed to save XLSX file '%s': %w", outputFile, err)
	}

	fmt.Printf("XLSX report generated successfully: %s\n", outputFile)
	return nil
}

// generateReport decides which report to generate based on the report format.
func generateReport(findings []Finding, reportFormat string) error {
	if reportFormat == "xlsx" {
		return reportXlsx(findings)
	}

	// Default to JSON output
	findingsJSON, err := json.MarshalIndent(findings, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal findings to JSON: %w", err)
	}

	fmt.Println(string(findingsJSON))
	return nil
}
