package main

import (
	"fmt"
	"github.com/reaandrew/techdetector/core"
	"github.com/reaandrew/techdetector/processors"
	"github.com/reaandrew/techdetector/reporters"
	"github.com/reaandrew/techdetector/repositories"
	"github.com/reaandrew/techdetector/scanners"
	"github.com/reaandrew/techdetector/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"os"
	"strings"
)

const SQLiteDBFilename = "findings.db"

// Cli represents the command-line interface
type Cli struct {
	reportFormat string
	baseUrl      string
	queriesPath  string
	dumpSchema   bool
	prefix       string
	cutoff       string
}

// Execute sets up and runs the root command
func (cli *Cli) Execute() error {
	rootCmd := &cobra.Command{
		Use:   "techdetector",
		Short: "TechDetector is a tool to scan repositories for technologies.",
	}

	scanCmd := cli.createScanCommand()

	rootCmd.AddCommand(scanCmd)

	err := rootCmd.Execute()

	return err
}

func sanitizeForDB(name string) string {
	s := name
	// Example: remove protocols
	s = strings.ReplaceAll(s, "https://", "")
	s = strings.ReplaceAll(s, "http://", "")
	// Replace slashes, colons, etc. with underscores
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, ":", "_")
	// Optionally lower-case everything
	s = strings.ToLower(s)
	return s
}

// createScanCommand creates the 'scan' subcommand with its flags and subcommands
func (cli *Cli) createScanCommand() *cobra.Command {
	scanCmd := &cobra.Command{
		Use:     "scan",
		Short:   "Scan repositories or organizations for technologies.",
		Version: Version,
	}

	scanCmd.PersistentFlags().StringVar(&cli.reportFormat, "report", "xlsx", "Type format (supported: xlsx, json, http)")
	scanCmd.PersistentFlags().StringVar(&cli.baseUrl, "baseurl", "", "Http report base url (used only if --report=http)")
	scanCmd.PersistentFlags().StringVar(&cli.queriesPath, "queries-path", "", "Queries path (YAML file)")
	scanCmd.PersistentFlags().BoolVar(&cli.dumpSchema, "dump-schema", false, "Dump SQLite schema to a text file")
	//scanCmd.PersistentFlags().StringVar(&cli.prefix, "prefix", "techdetector", "A prefix for the output artifacts")
	scanCmd.PersistentFlags().StringVar(&cli.cutoff, "date-cutoff", "", "A date cutoff (e.g. 2021-01-01) to process git repos in")

	if err := scanCmd.MarkPersistentFlagRequired("queries-path"); err != nil {
		log.Printf("Error making queries-path flag required: %v\n", err)
		os.Exit(1)
	}

	//----------------------------------------------------------------------
	// Subcommand: repo <REPO_URL>
	//----------------------------------------------------------------------
	scanRepoCmd := &cobra.Command{
		Use:   "repo <REPO_URL>",
		Short: "Scan a single Git repository for technologies.",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			err, queries := cli.loadQueries()
			if err != nil {
				log.Fatal(err)
			}

			// 1) Sanitize the repo URL for a DB filename
			repoURL := args[0]
			cli.prefix = "td_" + sanitizeForDB(repoURL)

			dbFile := cli.prefix + "_findings.db"
			utils.InitializeSQLiteDB(dbFile)

			reporter, err := cli.createReporter(cli.reportFormat, queries, dbFile)
			if err != nil {
				log.Fatal(err)
			}

			// 2) Create SQLite repository
			repository, err := repositories.NewSqliteFindingRepository(dbFile)
			if err != nil {
				log.Fatalf("Failed to create SQLite repository for %s: %v", repoURL, err)
			}
			defer func() {
				_ = repository.Close()
			}()

			// 3) Create and run the scanner
			scanner := scanners.NewRepoScanner(reporter, processors.InitializeProcessors(), repository, cli.cutoff)
			scanner.Scan(repoURL, cli.reportFormat)
		},
	}

	//----------------------------------------------------------------------
	// Subcommand: github_org <ORG_NAME>
	//----------------------------------------------------------------------
	scanOrgCmd := &cobra.Command{
		Use:   "github_org <ORG_NAME>",
		Short: "Scan all repositories within a GitHub organization for technologies.",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			err, queries := cli.loadQueries()
			if err != nil {
				log.Fatal(err)
			}
			orgName := args[0]
			cli.prefix = "td_" + sanitizeForDB(orgName)

			// 1) Sanitize the GH org name

			dbFile := cli.prefix + "_findings.db"

			reporter, err := cli.createReporter(cli.reportFormat, queries, dbFile)
			if err != nil {
				log.Fatal(err)
			}

			// 2) Create SQLite repository
			repository, err := repositories.NewSqliteFindingRepository(dbFile)
			if err != nil {
				log.Fatalf("Failed to create SQLite repository for org %s: %v", orgName, err)
			}
			defer func() {
				_ = repository.Close()
			}()

			progressReporter := utils.NewBarProgressReporter(0, "Scanning GitHub org repositories")
			// 3) Create and run the scanner
			scanner := scanners.NewGithubOrgScanner(reporter, processors.InitializeProcessors(), repository, cli.cutoff, progressReporter)
			scanner.Scan(orgName, cli.reportFormat)
		},
	}

	//----------------------------------------------------------------------
	// Subcommand: dir [DIRECTORY]
	//----------------------------------------------------------------------
	scanDirCmd := &cobra.Command{
		Use:   "dir [DIRECTORY]",
		Short: "Scan all top-level directories in the specified directory (defaults to CWD) for technologies.",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			var directory string
			if len(args) == 0 {
				cwd, err := os.Getwd()
				if err != nil {
					log.Fatalf("Failed to get current working directory: %v", err)
				}
				directory = cwd
			} else {
				directory = args[0]
			}
			cli.prefix = "td_" + sanitizeForDB(directory)

			info, err := os.Stat(directory)
			if err != nil {
				log.Fatalf("Error accessing directory '%s': %v", directory, err)
			}
			if !info.IsDir() {
				log.Fatalf("Provided path '%s' is not a directory.", directory)
			}

			err, queries := cli.loadQueries()
			if err != nil {
				log.Fatal(err)
			}

			// 1) Sanitize directory name for a DB filename
			dbFile := cli.prefix + "_findings.db"

			reporter, err := cli.createReporter(cli.reportFormat, queries, dbFile)
			if err != nil {
				log.Fatal(err)
			}

			// 2) Create SQLite repository
			repository, err := repositories.NewSqliteFindingRepository(dbFile)
			if err != nil {
				log.Fatalf("Failed to create SQLite repository for directory %s: %v", directory, err)
			}
			defer func() {
				_ = repository.Close()
			}()

			// 3) Create and run the scanner
			directoryScanner := scanners.NewDirectoryScanner(reporter, processors.InitializeProcessors(), repository)
			directoryScanner.Scan(directory, cli.reportFormat)
		},
	}

	scanCmd.AddCommand(scanRepoCmd)
	scanCmd.AddCommand(scanOrgCmd)
	scanCmd.AddCommand(scanDirCmd)
	return scanCmd
}

func (cli *Cli) loadQueries() (error, core.SqlQueries) {
	// Ensure the queries file exists
	if _, err := os.Stat(cli.queriesPath); os.IsNotExist(err) {
		log.Fatalf("Queries file '%s' does not exist.", cli.queriesPath)
	}

	// Initialize the dynamic reporter and load queries
	queries, err := cli.loadSqlQueries(cli.queriesPath)
	if err != nil {
		log.Fatalf("Failed to load queries file: %v", err)
	}
	return err, queries
}

func (cli *Cli) loadSqlQueries(filename string) (core.SqlQueries, error) {
	var sqlQueries core.SqlQueries

	fileData, err := os.ReadFile(filename)
	if err != nil {
		return sqlQueries, fmt.Errorf("failed to read YAML file '%s': %w", filename, err)
	}

	err = yaml.Unmarshal(fileData, &sqlQueries)
	if err != nil {
		return sqlQueries, fmt.Errorf("failed to unmarshal YAML data: %w", err)
	}

	return sqlQueries, nil
}

func (cli *Cli) createReporter(reportFormat string, queries core.SqlQueries, dbFilename string) (core.Reporter, error) {
	if reportFormat == "xlsx" {
		return reporters.XlsxReporter{
			Queries:          queries,
			DumpSchema:       cli.dumpSchema,
			ArtifactPrefix:   cli.prefix,
			SqliteDBFilename: dbFilename,
		}, nil
	}
	if reportFormat == "json" {
		return reporters.JsonReporter{
			Queries:          queries,
			ArtifactPrefix:   cli.prefix,
			SqliteDBFilename: dbFilename,
		}, nil
	}
	if reportFormat == "http" {
		return reporters.NewDefaultHttpReporter(cli.baseUrl), nil
	}

	return nil, fmt.Errorf("unknown report format: %s", reportFormat)
}
