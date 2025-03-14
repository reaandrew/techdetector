package reporters

import (
	"database/sql"
	"fmt"
	"github.com/reaandrew/techdetector/core"
	log "github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
	"strings"
)

const (
	XlsxSummaryReport = "summary_report.xlsx"
)

type XlsxReporter struct {
	Queries          core.SqlQueries
	DumpSchema       bool
	ArtifactPrefix   string
	SqliteDBFilename string
}

func (x XlsxReporter) Report(repository core.FindingRepository) error {
	// Generate Excel report from the SQLite data
	if err := x.generateExcelReport(x.SqliteDBFilename); err != nil {
		return fmt.Errorf("failed to generate Excel report: %w", err)
	}

	return nil
}

func (x XlsxReporter) generateExcelReport(dbPath string) error {
	// Open the SQLite database directly
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open SQLite database: %w", err)
	}
	defer db.Close()

	excelFile := excelize.NewFile()

	defaultSheet := excelFile.GetSheetName(0)
	if err := excelFile.DeleteSheet(defaultSheet); err != nil {
		return fmt.Errorf("failed to delete default sheet %q: %w", defaultSheet, err)
	}

	for _, query := range x.Queries.Queries {
		if err := x.executeAndWriteQuery(db, excelFile, query.Query, query.Name); err != nil {
			return fmt.Errorf("failed to write query result for '%s': %w", query.Name, err)
		}
	}

	if err := excelFile.SaveAs(fmt.Sprintf("%s_%s", x.ArtifactPrefix, XlsxSummaryReport)); err != nil {
		return fmt.Errorf("failed to save summary report: %w", err)
	}

	return nil
}

func (x XlsxReporter) executeAndWriteQuery(db *sql.DB, excelFile *excelize.File, query, sheetName string) error {
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
