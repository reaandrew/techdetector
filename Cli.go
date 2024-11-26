package main

import (
	"fmt"
	"github.com/reaandrew/techdetector/core"
	"github.com/reaandrew/techdetector/processors"
	"github.com/reaandrew/techdetector/reporters"
	"github.com/reaandrew/techdetector/repositories"
	"github.com/reaandrew/techdetector/scanners"
	"github.com/spf13/cobra"
	"log"
	"os"
)

// Cli represents the command-line interface
type Cli struct {
	reportFormat string
	baseUrl      string
}

// Execute sets up and runs the root command
func (cli *Cli) Execute() error {
	rootCmd := &cobra.Command{
		Use:   "techdetector",
		Short: "TechDetector is a tool to scan repositories for technologies.",
	}

	scanCmd := cli.createScanCommand()

	rootCmd.AddCommand(scanCmd)

	return rootCmd.Execute()
}

// createScanCommand creates the 'scan' subcommand with its flags and subcommands
func (cli *Cli) createScanCommand() *cobra.Command {

	scanCmd := &cobra.Command{
		Use:     "scan",
		Short:   "Scan repositories or organizations for technologies.",
		Version: Version,
	}

	// Add the --report flag to the scan command
	scanCmd.PersistentFlags().StringVar(&cli.reportFormat, "report", "xlsx", "Report format (supported: xlsx)")
	scanCmd.PersistentFlags().StringVar(&cli.baseUrl, "baseurl", "xlsx", "Http report base url")

	scanRepoCmd := &cobra.Command{
		Use:   "repo <REPO_URL>",
		Short: "Scan a single Git repository for technologies.",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			reporter, err := cli.createReporter(cli.reportFormat)
			if err != nil {
				log.Fatal(err)
			}
			scanner := scanners.NewRepoScanner(
				reporter,
				processors.InitializeProcessors(),
				repositories.NewFileBasedMatchRepository())
			repoURL := args[0]
			scanner.Scan(repoURL, cli.reportFormat)
		},
	}

	scanOrgCmd := &cobra.Command{
		Use:   "github_org <ORG_NAME>",
		Short: "Scan all repositories within a GitHub organization for technologies.",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			reporter, err := cli.createReporter(cli.reportFormat)
			if err != nil {
				log.Fatal(err)
			}
			scanner := scanners.NewGithubOrgScanner(
				reporter,
				processors.InitializeProcessors(),
				repositories.NewFileBasedMatchRepository())
			orgName := args[0]
			scanner.Scan(orgName, cli.reportFormat)
		},
	}

	scanDirCmd := &cobra.Command{
		Use:   "dir [DIRECTORY]",
		Short: "Scan all top-level directories in the specified directory (defaults to CWD) for technologies.",
		Args:  cobra.MaximumNArgs(1), // Accepts zero or one argument
		Run: func(cmd *cobra.Command, args []string) {
			var directory string
			if len(args) == 0 {
				// No directory provided; use current working directory
				cwd, err := os.Getwd()
				if err != nil {
					log.Fatalf("Failed to get current working directory: %v", err)
				}
				directory = cwd
			} else {
				// Directory provided as argument
				directory = args[0]
			}

			// Check if the provided directory exists and is a directory
			info, err := os.Stat(directory)
			if err != nil {
				log.Fatalf("Error accessing directory '%s': %v", directory, err)
			}
			if !info.IsDir() {
				log.Fatalf("Provided path '%s' is not a directory.", directory)
			}

			reporter, err := cli.createReporter(cli.reportFormat)
			if err != nil {
				log.Fatal(err)
			}
			// Initialize DirectoryScanner
			directoryScanner := scanners.NewDirectoryScanner(
				reporter,
				processors.InitializeProcessors(),
				repositories.NewFileBasedMatchRepository())

			// Execute the scan
			directoryScanner.Scan(directory, cli.reportFormat)
		},
	}

	scanCmd.AddCommand(scanRepoCmd)
	scanCmd.AddCommand(scanOrgCmd)
	scanCmd.AddCommand(scanDirCmd)
	return scanCmd
}

func (cli *Cli) createReporter(reportFormat string) (core.Reporter, error) {
	if reportFormat == "xlsx" {
		return reporters.XlsxReporter{}, nil
	}
	if reportFormat == "json" {
		return reporters.JsonReporter{}, nil
	}
	if reportFormat == "http" {
		return reporters.NewDefaultHttpReporter(cli.baseUrl), nil
	}

	return nil, fmt.Errorf("unknown report format: %s", reportFormat)
}
