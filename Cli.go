package main

import "github.com/spf13/cobra"

// Cli represents the command-line interface
type Cli struct {
	reportFormat string
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

	scanRepoCmd := &cobra.Command{
		Use:   "repo <REPO_URL>",
		Short: "Scan a single Git repository for technologies.",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			scanner := NewRepoScanner(Reporter{}, InitializeProcessors())
			repoURL := args[0]
			scanner.scan(repoURL, cli.reportFormat)
		},
	}

	scanOrgCmd := &cobra.Command{
		Use:   "github_org <ORG_NAME>",
		Short: "Scan all repositories within a GitHub organization for technologies.",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			scanner := NewGithubOrgScanner(Reporter{}, InitializeProcessors())
			orgName := args[0]
			scanner.scan(orgName, cli.reportFormat)
		},
	}

	scanCmd.AddCommand(scanRepoCmd)
	scanCmd.AddCommand(scanOrgCmd)
	return scanCmd
}
