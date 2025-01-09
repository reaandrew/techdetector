package reporters

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"github.com/reaandrew/techdetector/core"
	"github.com/xuri/excelize/v2"
)

// DynamicXlsxSummaryReporter is responsible for generating the XLSX summary report.
type DynamicXlsxSummaryReporter struct {
	SqlQueries core.SqlQueries
}

// Report generates the summary XLSX file based on the findings repository.
func (xr DynamicXlsxSummaryReporter) Report(repository core.FindingRepository) error {
	fmt.Println("Generating Summary XLSX file")

	// Open SQLite database.
	db, err := sql.Open("sqlite3", xlsxSQLiteDB)
	if err != nil {
		return fmt.Errorf("failed to create SQLite database: %w", err)
	}
	defer db.Close()
	defer os.Remove(xlsxSQLiteDB)

	// Collect all findings and gather unique properties per type.
	typeProperties, findings, err := xr.collectPropertiesAndFindings(repository)
	if err != nil {
		return fmt.Errorf("failed to collect properties and findings: %w", err)
	}

	// Dynamically create tables based on collected properties.
	if err := xr.createDynamicTables(db, typeProperties); err != nil {
		return fmt.Errorf("failed to create SQLite tables: %w", err)
	}

	// Import findings into the dynamic tables.
	if err := xr.importFindings(db, findings); err != nil {
		return fmt.Errorf("failed to import findings into SQLite: %w", err)
	}

	// Initialize Excel file.
	excelFile := excelize.NewFile()

	// Remove default sheet.
	defaultSheet := excelFile.GetSheetName(0)
	if err := excelFile.DeleteSheet(defaultSheet); err != nil {
		return fmt.Errorf("failed to delete default sheet %q: %w", defaultSheet, err)
	}

	fmt.Printf("Queries: %v \n", xr.SqlQueries.Queries)
	// Execute each query and write to Excel sheets.
	for _, query := range xr.SqlQueries.Queries {
		fmt.Printf("Executing query for: %s\n", query.Name)
		if err := xr.executeAndWriteQuery(db, excelFile, query.Query, query.Name); err != nil {
			return fmt.Errorf("failed to write query result for '%s': %w", query.Name, err)
		}
	}

	// Save the Excel report.
	if err := excelFile.SaveAs(XlsxSummaryReport); err != nil {
		return fmt.Errorf("failed to save summary report: %w", err)
	}

	fmt.Printf("Summary XLSX report generated successfully: %s\n", XlsxSummaryReport)
	return nil
}

// collectPropertiesAndFindings iterates through the repository and collects unique properties per type.
func (xr DynamicXlsxSummaryReporter) collectPropertiesAndFindings(repo core.FindingRepository) (map[string]map[string]bool, []core.Finding, error) {
	typeProperties := make(map[string]map[string]bool)
	var allFindings []core.Finding

	iterator := repo.NewIterator()
	for iterator.HasNext() {
		set, _ := iterator.Next()
		for _, finding := range set.Matches {
			allFindings = append(allFindings, finding)
			if _, exists := typeProperties[finding.Type]; !exists {
				typeProperties[finding.Type] = make(map[string]bool)
			}
			for prop := range finding.Properties {
				typeProperties[finding.Type][prop] = true
			}
		}
	}

	return typeProperties, allFindings, nil
}

// createDynamicTables creates tables dynamically based on the collected properties.
func (xr DynamicXlsxSummaryReporter) createDynamicTables(db *sql.DB, typeProperties map[string]map[string]bool) error {
	for typeName, properties := range typeProperties {
		// Base fields from the Finding struct.
		baseFields := map[string]string{
			"Name":     "TEXT",
			"Type":     "TEXT",
			"Category": "TEXT",
			"Path":     "TEXT",
			"RepoName": "TEXT",
		}

		// Add dynamic property fields.
		for prop := range properties {
			// Sanitize property names to be SQL-friendly.
			sanitizedProp := sanitizeIdentifier(prop)
			baseFields[sanitizedProp] = "TEXT" // You might want to infer type based on value
		}

		// Build the CREATE TABLE statement.
		fields := []string{}
		for field, dtype := range baseFields {
			fields = append(fields, fmt.Sprintf("%s %s", field, dtype))
		}

		createStmt := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s);", sanitizeIdentifier(typeName), strings.Join(fields, ", "))
		fmt.Printf("Creating table for type '%s' with columns: %s\n", typeName, strings.Join(fields, ", "))
		if _, err := db.Exec(createStmt); err != nil {
			return fmt.Errorf("failed to execute CREATE TABLE for type '%s': %w", typeName, err)
		}
	}

	return nil
}

// importFindings inserts findings into their respective dynamic tables.
func (xr DynamicXlsxSummaryReporter) importFindings(db *sql.DB, findings []core.Finding) error {
	for _, finding := range findings {
		tableName := sanitizeIdentifier(finding.Type)

		// Get column names for the table.
		columns, err := xr.getTableColumns(db, tableName)
		if err != nil {
			log.Printf("Skipping insertion for finding '%s' due to error getting columns: %v", finding.Name, err)
			continue
		}

		fields := []string{}
		placeholders := []string{}
		args := []interface{}{}

		for _, col := range columns {
			switch col {
			case "Name", "Type", "Category", "Path", "RepoName":
				var value interface{}
				switch col {
				case "Name":
					value = finding.Name
				case "Type":
					value = finding.Type
				case "Category":
					value = finding.Category
				case "Path":
					value = finding.Path
				case "RepoName":
					value = finding.RepoName
				}
				fields = append(fields, col)
				placeholders = append(placeholders, "?")
				args = append(args, value)
			default:
				// Dynamic properties
				value, exists := finding.Properties[col]
				if exists {
					fields = append(fields, col)
					placeholders = append(placeholders, "?")
					args = append(args, value)
				} else {
					// Property missing in this finding
					fields = append(fields, col)
					placeholders = append(placeholders, "NULL")
					args = append(args, nil)
				}
			}
		}

		// Build the INSERT statement.
		insertStmt := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s);",
			tableName,
			strings.Join(fields, ", "),
			strings.Join(placeholders, ", "),
		)

		// Execute the INSERT statement.
		if _, err := db.Exec(insertStmt, args...); err != nil {
			log.Printf("Error inserting finding '%s' into table '%s': %v", finding.Name, tableName, err)
		}
	}

	return nil
}

// getTableColumns retrieves the column names for a given table.
func (xr DynamicXlsxSummaryReporter) getTableColumns(db *sql.DB, tableName string) ([]string, error) {
	query := fmt.Sprintf("PRAGMA table_info(%s);", tableName)
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get table info for '%s': %w", tableName, err)
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var cid int
		var name string
		var ctype string
		var notnull int
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			return nil, fmt.Errorf("failed to scan table info: %w", err)
		}
		columns = append(columns, name)
	}

	return columns, nil
}

// executeAndWriteQuery executes a SQL query and writes the result to an Excel sheet.
func (xr DynamicXlsxSummaryReporter) executeAndWriteQuery(db *sql.DB, excelFile *excelize.File, query, sheetName string) error {
	fmt.Printf("sheetName: %s", sheetName)

	// Execute the query.
	rows, err := db.Query(query)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	// Get column names.
	colNames, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("failed to get columns: %w", err)
	}

	// Create a new sheet.
	_, err = excelFile.NewSheet(sheetName)
	if err != nil {
		return fmt.Errorf("failed to create sheet '%s': %w", sheetName, err)
	}

	// Write headers.
	if err := excelFile.SetSheetRow(sheetName, "A1", &colNames); err != nil {
		return fmt.Errorf("failed to write headers: %w", err)
	}

	// Write rows.
	rowIndex := 2
	for rows.Next() {
		cols := make([]interface{}, len(colNames))
		colPtrs := make([]interface{}, len(cols))
		for i := range cols {
			colPtrs[i] = &cols[i]
		}

		if err := rows.Scan(colPtrs...); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		// Convert each column to string for Excel.
		strCols := make([]interface{}, len(cols))
		for i, col := range cols {
			if col == nil {
				strCols[i] = ""
				continue
			}
			// If the column is Properties (JSON), format it.
			if strings.EqualFold(colNames[i], "Properties") {
				var prettyJSON bytes.Buffer
				// json.Indent writes to bytes.Buffer
				err := json.Indent(&prettyJSON, []byte(fmt.Sprintf("%v", col)), "", "  ")
				if err != nil {
					strCols[i] = fmt.Sprintf("%v", col)
				} else {
					strCols[i] = prettyJSON.String()
				}
			} else {
				strCols[i] = fmt.Sprintf("%v", col)
			}
		}

		cellAddr := fmt.Sprintf("A%d", rowIndex)
		if err := excelFile.SetSheetRow(sheetName, cellAddr, &strCols); err != nil {
			return fmt.Errorf("failed to write data row: %w", err)
		}
		rowIndex++
	}

	return nil
}

// sanitizeIdentifier ensures the table name and column names are safe for SQL.
func sanitizeIdentifier(name string) string {
	// Replace spaces and other unsafe characters with underscores.
	return strings.ReplaceAll(name, " ", "_")
}
