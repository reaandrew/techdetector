package reporters

import (
	"fmt"
	"os"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/reaandrew/techdetector/core"
	"github.com/reaandrew/techdetector/utils"
	"github.com/stretchr/testify/assert"
)

func TestDynamicXlsxSummaryReporter_Report(t *testing.T) {
	// Create a temporary SQLite database file
	dbFile, err := os.CreateTemp("", "test_findings_*.db")
	assert.NoError(t, err)
	defer os.Remove(dbFile.Name())

	repo := &utils.MockMatchRepository{}
	findings := []core.Finding{
		{Name: "Finding1", Type: "TypeA", Category: "Category1", Path: "/path/to/file1", RepoName: "Repo1"},
		{Name: "Finding2", Type: "TypeB", Category: "Category2", Path: "/path/to/file2", RepoName: "Repo2"},
	}
	repo.Store(findings)

	xr := DynamicXlsxSummaryReporterForFindingsSqlTable{
		SqlQueries: core.SqlQueries{
			Queries: []core.SqlQuery{
				{
					Name:  "Query1",
					Query: "SELECT * FROM typea",
				},
				{
					Name:  "Query2",
					Query: "SELECT * FROM typeb",
				},
			},
		},
		DumpSchema:     true,
		ArtifactPrefix: dbFile.Name(),
	}

	err = xr.Report(repo)
	assert.NoError(t, err)

	// Check if the summary report file was created
	reportFile := fmt.Sprintf("%s_summary_report.xlsx", dbFile.Name())
	_, err = os.Stat(reportFile)
	assert.NoError(t, err)
	defer os.Remove(reportFile)
}
