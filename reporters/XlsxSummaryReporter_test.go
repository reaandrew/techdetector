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

func TestDynamicXlsxSummaryReporter_createDynamicTables(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	assert.NoError(t, err)
	defer db.Close()

	xr := DynamicXlsxSummaryReporterForFindingsSqlTable{}
	typeProperties := map[string]map[string]bool{
		"TypeA": {
			"Key1": true,
			"Key2": true,
		},
	}

	err = xr.createDynamicTables(db, typeProperties)
	assert.NoError(t, err)

	// Verify the table exists
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name='typea'")
	assert.NoError(t, err)
	assert.True(t, rows.Next())
	rows.Close()
}

func TestDynamicXlsxSummaryReporter_importFindings(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	assert.NoError(t, err)
	defer db.Close()

	xr := DynamicXlsxSummaryReporterForFindingsSqlTable{}
	typeProperties := map[string]map[string]bool{
		"TypeA": {
			"Key1": true,
			"Key2": true,
		},
	}

	err = xr.createDynamicTables(db, typeProperties)
	assert.NoError(t, err)

	findings := []core.Finding{
		{
			Name:     "Finding1",
			Type:     "TypeA",
			Category: "Category1",
			Properties: map[string]interface{}{
				"Key1": "Value1",
			},
		},
	}
	err = xr.importFindings(db, findings, typeProperties)
	assert.NoError(t, err)

	// Verify the data was inserted
	rows, err := db.Query("SELECT * FROM typea")
	assert.NoError(t, err)
	assert.True(t, rows.Next())
	rows.Close()
}
