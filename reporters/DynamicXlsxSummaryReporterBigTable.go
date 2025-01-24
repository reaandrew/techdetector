package reporters

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/reaandrew/techdetector/core"
	"github.com/reaandrew/techdetector/utils"
	"github.com/xuri/excelize/v2"
)

const (
	XlsxSummaryReport = "summary_report.xlsx"
)

type DynamicXlsxSummaryReporterForFindingsSqlTable struct {
	SqlQueries       core.SqlQueries
	DumpSchema       bool
	ArtifactPrefix   string
	SqliteDBFilename string
}

func (xr DynamicXlsxSummaryReporterForFindingsSqlTable) Report(repository core.FindingRepository) error {
	dbPath := fmt.Sprintf("%s_%s", xr.ArtifactPrefix, xr.SqliteDBFilename)

	// Initialize SQLite database
	db, err := utils.InitializeSQLiteDB(dbPath)
	if err != nil {
		return fmt.Errorf("failed to initialize SQLite database: %w", err)
	}
	defer db.Close()

	// Process findings incrementally and store them in SQLite
	err = utils.ProcessFindingsIncrementally(db, repository)
	if err != nil {
		return fmt.Errorf("failed to process findings: %w", err)
	}

	// Generate Excel report from the SQLite data
	if err := xr.generateExcelReport(dbPath); err != nil {
		return fmt.Errorf("failed to generate Excel report: %w", err)
	}

	// Optionally dump the schema
	if xr.DumpSchema {
		utils.DumpSQLiteSchemaForFindings(dbPath)
	}

	return nil
}

func (xr DynamicXlsxSummaryReporterForFindingsSqlTable) generateExcelReport(dbPath string) error {
	db, err := utils.InitializeSQLiteDB(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open SQLite database: %w", err)
	}
	defer db.Close()

	excelFile := excelize.NewFile()

	defaultSheet := excelFile.GetSheetName(0)
	if err := excelFile.DeleteSheet(defaultSheet); err != nil {
		return fmt.Errorf("failed to delete default sheet %q: %w", defaultSheet, err)
	}

	for _, query := range xr.SqlQueries.Queries {
		if err := xr.executeAndWriteQuery(db, excelFile, query.Query, query.Name); err != nil {
			return fmt.Errorf("failed to write query result for '%s': %w", query.Name, err)
		}
	}

	if err := excelFile.SaveAs(fmt.Sprintf("%s_%s", xr.ArtifactPrefix, XlsxSummaryReport)); err != nil {
		return fmt.Errorf("failed to save summary report: %w", err)
	}

	return nil
}

func (xr DynamicXlsxSummaryReporterForFindingsSqlTable) executeAndWriteQuery(db *sql.DB, excelFile *excelize.File, query, sheetName string) error {
	rows, err := db.Query(query)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			log.Printf("Skipping query for sheet '%s': %v", sheetName, err)
			return nil
		}
		return fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	colNames, _ := rows.Columns()
	_, _ = excelFile.NewSheet(sheetName)
	_ = excelFile.SetSheetRow(sheetName, "A1", &colNames)

	rowIndex := 2
	for rows.Next() {
		cols := make([]interface{}, len(colNames))
		colPtrs := make([]interface{}, len(cols))
		for i := range cols {
			colPtrs[i] = &cols[i]
		}
		rows.Scan(colPtrs...)
		_ = excelFile.SetSheetRow(sheetName, fmt.Sprintf("A%d", rowIndex), &cols)
		rowIndex++
	}

	return nil
}
