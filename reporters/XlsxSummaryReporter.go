package reporters

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"github.com/reaandrew/techdetector/core"
	"github.com/xuri/excelize/v2"
	"log"
	"os"
)

type XlsxSummaryReporter struct{}

const XlsxSummaryReport = "summary_report.xlsx"
const xlsxSQLiteDB = "findings.db"

func (xr XlsxSummaryReporter) Report(repository core.FindingRepository) error {
	fmt.Println("Generating Summary XLSX file")

	db, err := sql.Open("sqlite3", xlsxSQLiteDB)
	if err != nil {
		return fmt.Errorf("failed to create SQLite database: %w", err)
	}
	defer db.Close()

	defer os.Remove(xlsxSQLiteDB)

	if err := xr.createTables(db); err != nil {
		return fmt.Errorf("failed to create SQLite tables: %w", err)
	}

	if err := xr.importFindings(db, repository); err != nil {
		return fmt.Errorf("failed to import findings into SQLite: %w", err)
	}

	queries := map[string]string{
		"Library Versions": `
			SELECT 
				Name,
				COUNT(DISTINCT Version) AS VersionCount,
				GROUP_CONCAT(DISTINCT Version) AS Versions
			FROM 
				library
			GROUP BY 
				Name
			HAVING 
				COUNT(DISTINCT Version) > 1
			ORDER BY 
				Name;
		`,
		"Library Versions By RepoName": `
			SELECT 
				Name,
				RepoName,
				COUNT(DISTINCT Version) AS VersionCount,
				GROUP_CONCAT(DISTINCT Version) AS Versions
			FROM 
				library
			GROUP BY 
				Name, RepoName
			HAVING 
				COUNT(DISTINCT Version) > 1
			ORDER BY 
				Name;
		`,
		"Docker Image Owners": `
			SELECT 
				COALESCE(NULLIF(owner, ''), 'EMPTY') AS owner,
				COUNT(*) AS owner_count
			FROM 
				docker_directive
			GROUP BY 
				owner
			ORDER BY 
				owner_count DESC;
		`,
		"Docker Images By Owner": `
			SELECT 
				COALESCE(NULLIF(owner, ''), 'EMPTY') AS owner,
				COUNT(DISTINCT image) AS image_count,
				GROUP_CONCAT(DISTINCT image) AS images
			FROM 
				docker_directive
			GROUP BY 
				owner
			ORDER BY 
				image_count DESC;
		`,
		"Docker Image Versions by Image": `
			SELECT 
				image,
				COUNT(DISTINCT version) AS VersionCount,
				GROUP_CONCAT(DISTINCT version) AS Versions
			FROM 
				docker_directive
			GROUP BY 
				image
			ORDER BY 
				VersionCount DESC;
		`,
	}

	excelFile := excelize.NewFile()

	// Debug lines to see the default sheet name
	fmt.Printf("DEBUG: Sheets initially: %v\n", excelFile.GetSheetList())
	defaultSheet := excelFile.GetSheetName(0)
	fmt.Printf("DEBUG: defaultSheet is %q\n", defaultSheet)

	// Force-delete that sheet
	if err := excelFile.DeleteSheet(defaultSheet); err != nil {
		return fmt.Errorf("failed to delete default sheet %q: %w", defaultSheet, err)
	}

	fmt.Printf("DEBUG: After deletion, sheets: %v\n", excelFile.GetSheetList())

	for sheetName, query := range queries {
		fmt.Printf("Executing query for: %s\n", sheetName)
		if err := xr.executeAndWriteQuery(db, excelFile, query, sheetName); err != nil {
			return fmt.Errorf("failed to write query result for '%s': %w", sheetName, err)
		}
	}

	if err := excelFile.SaveAs(XlsxSummaryReport); err != nil {
		return fmt.Errorf("failed to save summary report: %w", err)
	}

	fmt.Printf("Summary XLSX report generated successfully: %s\n", XlsxSummaryReport)
	return nil
}

func (xr XlsxSummaryReporter) createTables(db *sql.DB) error {
	scripts := []string{
		`CREATE TABLE IF NOT EXISTS azure_bicep (
			Name TEXT,
			Category TEXT,
			RepoName TEXT,
			Path TEXT,
			resource TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS cloudformation_resource (
			Name TEXT,
			Category TEXT,
			RepoName TEXT,
			Path TEXT,
			properties TEXT,
			resource_type TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS shell (
			Name TEXT,
			Category TEXT,
			RepoName TEXT,
			Path TEXT,
			creator TEXT,
			description TEXT,
			version_supported TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS library (
			Name TEXT,
			Category TEXT,
			RepoName TEXT,
			Path TEXT,
			Language TEXT,
			Version TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS ms_office (
			Name TEXT,
			Category TEXT,
			RepoName TEXT,
			Path TEXT,
			creator TEXT,
			description TEXT,
			version_supported TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS cloud_service_sdk (
			Name TEXT,
			Category TEXT,
			RepoName TEXT,
			Path TEXT,
			language TEXT,
			vendor TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS framework (
			Name TEXT,
			Category TEXT,
			RepoName TEXT,
			Path TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS application (
			Name TEXT,
			Category TEXT,
			RepoName TEXT,
			Path TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS docker_directive (
			Name TEXT,
			Category TEXT,
			RepoName TEXT,
			Path TEXT,
			arguments TEXT,
			image TEXT,
			owner TEXT,
			version TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS docker_compose_service (
			Name TEXT,
			Category TEXT,
			RepoName TEXT,
			Path TEXT,
			image TEXT
		);`,
	}

	for _, script := range scripts {
		if _, err := db.Exec(script); err != nil {
			return fmt.Errorf("failed to execute create table script: %w", err)
		}
	}
	return nil
}

func (xr XlsxSummaryReporter) importFindings(db *sql.DB, repo core.FindingRepository) error {
	insertStmts := map[string]string{
		"Azure Bicep": `
			INSERT INTO azure_bicep (Name, Category, RepoName, Path, resource)
			VALUES (?, ?, ?, ?, ?);
		`,
		"CloudFormation Resource": `
			INSERT INTO cloudformation_resource (Name, Category, RepoName, Path, properties, resource_type)
			VALUES (?, ?, ?, ?, ?, ?);
		`,
		"Shell": `
			INSERT INTO shell (Name, Category, RepoName, Path, creator, description, version_supported)
			VALUES (?, ?, ?, ?, ?, ?, ?);
		`,
		"Library": `
			INSERT INTO library (Name, Category, RepoName, Path, Language, Version)
			VALUES (?, ?, ?, ?, ?, ?);
		`,
		"MS Office": `
			INSERT INTO ms_office (Name, Category, RepoName, Path, creator, description, version_supported)
			VALUES (?, ?, ?, ?, ?, ?, ?);
		`,
		"Cloud Service SDK": `
			INSERT INTO cloud_service_sdk (Name, Category, RepoName, Path, language, vendor)
			VALUES (?, ?, ?, ?, ?, ?);
		`,
		"Framework": `
			INSERT INTO framework (Name, Category, RepoName, Path)
			VALUES (?, ?, ?, ?);
		`,
		"Application": `
			INSERT INTO application (Name, Category, RepoName, Path)
			VALUES (?, ?, ?, ?);
		`,
		"Docker Directive": `
			INSERT INTO docker_directive (Name, Category, RepoName, Path, arguments, image, owner, version)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?);
		`,
		"Docker Compose Service": `
			INSERT INTO docker_compose_service (Name, Category, RepoName, Path, image)
			VALUES (?, ?, ?, ?, ?);
		`,
	}

	iterator := repo.NewIterator()
	for iterator.HasNext() {
		set, _ := iterator.Next()
		for _, finding := range set.Matches {
			stmt, ok := insertStmts[finding.Type]
			if !ok {
				continue
			}

			var params []interface{}
			switch finding.Type {
			case "Azure Bicep":
				params = []interface{}{
					finding.Name,
					finding.Category,
					finding.RepoName,
					finding.Path,
					finding.Properties["resource"],
				}
			case "CloudFormation Resource":
				params = []interface{}{
					finding.Name,
					finding.Category,
					finding.RepoName,
					finding.Path,
					finding.Properties["properties"],
					finding.Properties["resource_type"],
				}
			case "Shell":
				params = []interface{}{
					finding.Name,
					finding.Category,
					finding.RepoName,
					finding.Path,
					finding.Properties["creator"],
					finding.Properties["description"],
					finding.Properties["version_supported"],
				}
			case "Library":
				params = []interface{}{
					finding.Name,
					finding.Category,
					finding.RepoName,
					finding.Path,
					finding.Properties["Language"],
					finding.Properties["Version"],
				}
			case "MS Office":
				params = []interface{}{
					finding.Name,
					finding.Category,
					finding.RepoName,
					finding.Path,
					finding.Properties["creator"],
					finding.Properties["description"],
					finding.Properties["version_supported"],
				}
			case "Cloud Service SDK":
				params = []interface{}{
					finding.Name,
					finding.Category,
					finding.RepoName,
					finding.Path,
					finding.Properties["language"],
					finding.Properties["vendor"],
				}
			case "Framework":
				params = []interface{}{
					finding.Name,
					finding.Category,
					finding.RepoName,
					finding.Path,
				}
			case "Application":
				params = []interface{}{
					finding.Name,
					finding.Category,
					finding.RepoName,
					finding.Path,
				}
			case "Docker Directive":
				params = []interface{}{
					finding.Name,
					finding.Category,
					finding.RepoName,
					finding.Path,
					finding.Properties["arguments"],
					finding.Properties["image"],
					finding.Properties["owner"],
					finding.Properties["version"],
				}
			case "Docker Compose Service":
				params = []interface{}{
					finding.Name,
					finding.Category,
					finding.RepoName,
					finding.Path,
					finding.Properties["image"],
				}
			}

			if _, err := db.Exec(stmt, params...); err != nil {
				log.Printf("Error inserting finding type '%s': %v", finding.Type, err)
			}
		}
	}
	return nil
}

func (xr XlsxSummaryReporter) executeAndWriteQuery(db *sql.DB, excelFile *excelize.File, query, sheetName string) error {
	// 1) Query
	rows, err := db.Query(query)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	// 2) Read the rows here before any other queries
	colNames, err := rows.Columns()
	if err != nil {
		rows.Close()
		return fmt.Errorf("failed to get columns: %w", err)
	}

	// Create the sheet, write headers, etc.
	_, err = excelFile.NewSheet(sheetName)
	if err != nil {
		rows.Close()
		return fmt.Errorf("failed to create sheet '%s': %w", sheetName, err)
	}
	if err := excelFile.SetSheetRow(sheetName, "A1", &colNames); err != nil {
		rows.Close()
		return fmt.Errorf("failed to write headers: %w", err)
	}

	rowIndex := 2
	for rows.Next() {
		cols := make([]interface{}, len(colNames))
		colPtrs := make([]interface{}, len(cols))
		for i := range cols {
			colPtrs[i] = &cols[i]
		}

		if err := rows.Scan(colPtrs...); err != nil {
			rows.Close()
			return fmt.Errorf("failed to scan row: %w", err)
		}

		cellAddr := fmt.Sprintf("A%d", rowIndex)
		if err := excelFile.SetSheetRow(sheetName, cellAddr, &cols); err != nil {
			rows.Close()
			return fmt.Errorf("failed to write data row: %w", err)
		}
		rowIndex++
	}
	rows.Close()

	return nil
}
