package reporters

import (
	"database/sql"
	"github.com/reaandrew/techdetector/utils"
	"os"
	"testing"

	"github.com/reaandrew/techdetector/core"

	"github.com/stretchr/testify/assert"
	"github.com/xuri/excelize/v2"

	_ "github.com/mattn/go-sqlite3"
)

func setupMockRepository() core.FindingRepository {
	return utils.MockMatchRepository{
		Matches: []core.Finding{
			{
				Type:     "Azure Bicep",
				Name:     "Resource1",
				Category: "Category1",
				RepoName: "Repo1",
				Path:     "Path1",
				Properties: map[string]interface{}{
					"resource": "ResourceType1",
				},
			},
			{
				Type:     "Azure Bicep",
				Name:     "Resource1",
				Category: "Category1",
				RepoName: "Repo2",
				Path:     "Path1",
				Properties: map[string]interface{}{
					"resource": "ResourceType1",
				},
			},
		},
	}
}

func TestXlsxSummaryReporter_Report(t *testing.T) {
	reporter := XlsxSummaryReporter{}
	repo := setupMockRepository()

	// Create a temporary database for testing
	defer os.Remove(XlsxSummaryReport)
	defer os.Remove("findings.db")

	err := reporter.Report(repo)
	assert.NoError(t, err, "Expected no error while generating the report")

	// Verify that the XLSX file was created
	_, err = os.Stat(XlsxSummaryReport)
	assert.NoError(t, err, "Expected summary report file to exist")

	// Verify content in the worksheet
	excelFile, err := excelize.OpenFile(XlsxSummaryReport)
	assert.NoError(t, err, "Expected no error while opening the Excel file")

	sheetName := "Redundancy Analysis" // Use one of the sheet names from queries
	rows, err := excelFile.GetRows(sheetName)
	assert.NoError(t, err, "Expected no error while fetching rows from the sheet")

	// Verify headers and data rows
	assert.NotEmpty(t, rows, "Expected rows to be present in the sheet")
	assert.Greater(t, len(rows), 1, "Expected more than one row (headers + data)")
}

func TestXlsxSummaryReporter_createTables(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	assert.NoError(t, err, "Expected no error while opening SQLite database")
	defer db.Close()

	reporter := XlsxSummaryReporter{}
	err = reporter.createTables(db)
	assert.NoError(t, err, "Expected no error while creating tables")

	// Verify that a sample table exists
	_, err = db.Query("SELECT * FROM azure_bicep")
	assert.NoError(t, err, "Expected table azure_bicep to exist")
}

func TestXlsxSummaryReporter_importFindings(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	assert.NoError(t, err, "Expected no error while opening SQLite database")
	defer db.Close()

	reporter := XlsxSummaryReporter{}
	repo := setupMockRepository()

	err = reporter.createTables(db)
	assert.NoError(t, err, "Expected no error while creating tables")

	err = reporter.importFindings(db, repo)
	assert.NoError(t, err, "Expected no error while importing findings")

	// Verify the data was inserted
	row := db.QueryRow("SELECT resource FROM azure_bicep WHERE Name = ?", "Resource1")
	var resource string
	err = row.Scan(&resource)
	assert.NoError(t, err, "Expected no error while querying the imported data")
	assert.Equal(t, "ResourceType1", resource, "Expected resource to match the inserted value")
}

func TestXlsxSummaryReporter_executeAndWriteQuery(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	assert.NoError(t, err, "Expected no error while opening SQLite database")
	defer db.Close()

	reporter := XlsxSummaryReporter{}
	err = reporter.createTables(db)
	assert.NoError(t, err, "Expected no error while creating tables")

	repo := setupMockRepository()
	err = reporter.importFindings(db, repo)
	assert.NoError(t, err, "Expected no error while importing findings")

	excelFile := excelize.NewFile()
	defer os.Remove("test_output.xlsx")

	query := `SELECT resource, COUNT(*) as Count FROM azure_bicep GROUP BY resource`
	err = reporter.executeAndWriteQuery(db, excelFile, query, "TestSheet")
	assert.NoError(t, err, "Expected no error while executing and writing query")

	err = excelFile.SaveAs("test_output.xlsx")
	assert.NoError(t, err, "Expected no error while saving the Excel file")

	// Verify the sheet exists
	sheets := excelFile.GetSheetList()
	assert.Contains(t, sheets, "TestSheet", "Expected sheet to exist in Excel file")

	// Verify content in the worksheet
	rows, err := excelFile.GetRows("TestSheet")
	assert.NoError(t, err, "Expected no error while fetching rows from the sheet")
	assert.NotEmpty(t, rows, "Expected rows to be present in the sheet")
	assert.Greater(t, len(rows), 1, "Expected more than one row (headers + data)")
}
