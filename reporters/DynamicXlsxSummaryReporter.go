package reporters

//
//import (
//	"bytes"
//	"database/sql"
//	"encoding/json"
//	"fmt"
//	"github.com/reaandrew/techdetector/core"
//	"github.com/reaandrew/techdetector/utils"
//	log "github.com/sirupsen/logrus"
//
//	//"os"
//	"strings"
//
//	_ "github.com/mattn/go-sqlite3"
//	"github.com/xuri/excelize/v2"
//)
//
//const (
//	XlsxSummaryReport = "summary_report.xlsx"
//	XlsxSQLiteDB      = "findings.db"
//)
//
//type DynamicXlsxSummaryReporterForFindingsSqlTable struct {
//	Queries     core.Queries
//	DumpSchema     bool
//	ArtifactPrefix string
//}
//
//var PredefinedFieldsSlice = []string{"Name", "Type", "Category", "Path", "RepoName"}
//
//func (xr DynamicXlsxSummaryReporterForFindingsSqlTable) Report(repository core.FindingRepository) error {
//	//fmt.Println("Generating Summary XLSX file")
//
//	db, err := sql.Open("sqlite3", fmt.Sprintf("%s_%s", xr.ArtifactPrefix, XlsxSQLiteDB))
//	if err != nil {
//		return fmt.Errorf("failed to create SQLite database: %w", err)
//	}
//	defer db.Close()
//	//defer os.Remove(XlsxSQLiteDB)
//
//	typeProperties, findings, err := xr.collectPropertiesAndFindings(repository)
//	if err != nil {
//		return fmt.Errorf("failed to collect properties and findings: %w", err)
//	}
//
//	if err := xr.createDynamicTables(db, typeProperties); err != nil {
//		return fmt.Errorf("failed to create SQLite tables: %w", err)
//	}
//
//	if err := xr.importFindings(db, findings, typeProperties); err != nil {
//		return fmt.Errorf("failed to import findings into SQLite: %w", err)
//	}
//
//	excelFile := excelize.NewFile()
//
//	defaultSheet := excelFile.GetSheetName(0)
//	if err := excelFile.DeleteSheet(defaultSheet); err != nil {
//		return fmt.Errorf("failed to delete default sheet %q: %w", defaultSheet, err)
//	}
//
//	for _, query := range xr.Queries.Queries {
//		//log.Printf("Executing query for: %s\n", query.Name)
//		if err := xr.executeAndWriteQuery(db, excelFile, query.Query, query.Name); err != nil {
//			return fmt.Errorf("failed to write query result for '%s': %w", query.Name, err)
//		}
//	}
//
//	if err := excelFile.SaveAs(fmt.Sprintf("%s_%s", xr.ArtifactPrefix, XlsxSummaryReport)); err != nil {
//		return fmt.Errorf("failed to save summary report: %w", err)
//	}
//
//	if xr.DumpSchema {
//		utils.DumpSQLiteSchema(XlsxSQLiteDB)
//	}
//
//	//log.Printf("Summary XLSX report generated successfully: %s\n", XlsxSummaryReport)
//	return nil
//}
//
//func (xr DynamicXlsxSummaryReporterForFindingsSqlTable) collectPropertiesAndFindings(repo core.FindingRepository) (map[string]map[string]bool, []core.Finding, error) {
//	typeProperties := make(map[string]map[string]bool)
//	var allFindings []core.Finding
//
//	iterator := repo.NewIterator()
//	for iterator.HasNext() {
//		set, _ := iterator.Next()
//		for _, finding := range set.Matches {
//			allFindings = append(allFindings, finding)
//			if _, exists := typeProperties[finding.Type]; !exists {
//				typeProperties[finding.Type] = make(map[string]bool)
//			}
//			// Flatten properties
//			flattenedProps := flattenProperties(finding.Properties)
//			for prop := range flattenedProps {
//				typeProperties[finding.Type][prop] = true
//			}
//			// Replace original properties with flattened ones for later insertion
//			finding.Properties = flattenedProps
//		}
//	}
//
//	return typeProperties, allFindings, nil
//}
//
//func flattenProperties(properties map[string]interface{}) map[string]interface{} {
//	flattened := make(map[string]interface{})
//	for key, value := range properties {
//		if isPredefinedField(key) {
//			continue
//		}
//
//		switch v := value.(type) {
//		case map[string]interface{}:
//			for subKey, subValue := range v {
//				if isPredefinedField(subKey) {
//					continue
//				}
//				if subMap, ok := subValue.(map[string]interface{}); ok {
//					jsonBytes, err := json.Marshal(subMap)
//					if err != nil {
//						log.Printf("Failed to marshal nested map for key '%s': %v", subKey, err)
//						flattened[subKey] = nil
//					} else {
//						flattened[subKey] = string(jsonBytes)
//					}
//				} else {
//					flattened[subKey] = subValue
//				}
//			}
//		default:
//			flattened[key] = value
//		}
//	}
//	return flattened
//}
//
//func isPredefinedField(key string) bool {
//	for _, field := range PredefinedFieldsSlice {
//		if strings.EqualFold(key, field) {
//			return true
//		}
//	}
//	return false
//}
//
//func (xr DynamicXlsxSummaryReporterForFindingsSqlTable) createDynamicTables(db *sql.DB, typeProperties map[string]map[string]bool) error {
//	for typeName, properties := range typeProperties {
//		sanitizedTypeName := sanitizeIdentifier(typeName)
//
//		// Drop the table if it exists
//		dropStmt := fmt.Sprintf("DROP TABLE IF EXISTS %s;", sanitizedTypeName)
//		if _, err := db.Exec(dropStmt); err != nil {
//			return fmt.Errorf("failed to drop existing table '%s': %w", sanitizedTypeName, err)
//		}
//
//		baseFields := map[string]string{
//			"Name":     "TEXT",
//			"Type":     "TEXT",
//			"Category": "TEXT",
//			"Path":     "TEXT",
//			"RepoName": "TEXT",
//		}
//
//		for prop := range properties {
//			sanitizedProp := sanitizeIdentifier(prop)
//			baseFields[sanitizedProp] = "TEXT" // All dynamic fields are TEXT to accommodate JSON strings
//		}
//
//		fields := []string{}
//		for field, dtype := range baseFields {
//			fields = append(fields, fmt.Sprintf("%s %s", field, dtype))
//		}
//
//		createStmt := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s);", sanitizedTypeName, strings.Join(fields, ", "))
//		//fmt.Println(createStmt)
//		//log.Printf("Creating table for type '%s' with columns: %s\n", typeName, strings.Join(fields, ", "))
//		if _, err := db.Exec(createStmt); err != nil {
//			return fmt.Errorf("failed to execute CREATE TABLE for type '%s': %w", typeName, err)
//		}
//	}
//
//	return nil
//}
//
//func (xr DynamicXlsxSummaryReporterForFindingsSqlTable) importFindings(db *sql.DB, findings []core.Finding, typeProperties map[string]map[string]bool) error {
//	for _, finding := range findings {
//		// Determine the table name dynamically
//		tableName := sanitizeIdentifier(finding.Type)
//
//		// Get dynamic properties for the type from typeProperties
//		dynamicProperties, ok := typeProperties[finding.Type]
//		//fmt.Println(dynamicProperties)
//		if !ok {
//			log.Printf("No dynamic properties found for type '%s'. Skipping.", finding.Type)
//			continue
//		}
//
//		// Prepare fields, placeholders, and arguments
//		fields := []string{}
//		placeholders := []string{}
//		args := []interface{}{}
//
//		// Add predefined fields
//		predefinedFields := map[string]interface{}{
//			"Name":     finding.Name,
//			"Type":     finding.Type,
//			"Category": finding.Category,
//			"Path":     finding.Path,
//			"RepoName": finding.RepoName,
//		}
//
//		for col, val := range predefinedFields {
//			fields = append(fields, col)
//			placeholders = append(placeholders, "?")
//			args = append(args, val)
//		}
//
//		// Add dynamic properties
//		for prop, exists := range dynamicProperties {
//			if exists {
//				if value, ok := finding.Properties[prop]; ok {
//					fields = append(fields, prop)
//					placeholders = append(placeholders, "?")
//
//					// Handle value types
//					switch v := value.(type) {
//					case string, int, int64, float64, bool:
//						args = append(args, v)
//					default:
//						jsonBytes, err := json.Marshal(v)
//						if err != nil {
//							log.Printf("Failed to marshal property '%s': %v", prop, err)
//							args = append(args, nil)
//						} else {
//							args = append(args, string(jsonBytes))
//						}
//					}
//				}
//			}
//		}
//
//		// Construct and execute the INSERT statement
//		insertStmt := fmt.Sprintf(
//			"INSERT INTO %s (%s) VALUES (%s);",
//			tableName,
//			strings.Join(fields, ", "),
//			strings.Join(placeholders, ", "),
//		)
//
//		//fmt.Println(insertStmt)
//
//		if _, err := db.Exec(insertStmt, args...); err != nil {
//			log.Printf("Error inserting finding '%s' into table '%s': %v", finding.Name, tableName, err)
//		}
//	}
//
//	return nil
//}
//
//func (xr DynamicXlsxSummaryReporterForFindingsSqlTable) getTableColumns(db *sql.DB, tableName string) ([]string, error) {
//	query := fmt.Sprintf("PRAGMA table_info(%s);", tableName)
//	rows, err := db.Query(query)
//	if err != nil {
//		return nil, fmt.Errorf("failed to get table info for '%s': %w", tableName, err)
//	}
//	defer rows.Close()
//
//	var columns []string
//	for rows.Next() {
//		var cid int
//		var name string
//		var ctype string
//		var notnull int
//		var dfltValue sql.NullString
//		var pk int
//		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
//			return nil, fmt.Errorf("failed to scan table info: %w", err)
//		}
//		columns = append(columns, name)
//	}
//
//	return columns, nil
//}
//
//func (xr DynamicXlsxSummaryReporterForFindingsSqlTable) executeAndWriteQuery(db *sql.DB, excelFile *excelize.File, query, sheetName string) error {
//	//log.Printf("sheetName: %s\n", sheetName)
//
//	// Try executing the query
//	rows, err := db.Query(query)
//	if err != nil {
//		// Check if the error is related to a missing table
//		if strings.Contains(err.Error(), "no such table") {
//			log.Printf("Skipping query for sheet '%s': %v", sheetName, err)
//			return nil // Safely skip this query
//		}
//		return fmt.Errorf("failed to execute query: %w", err)
//	}
//	defer rows.Close()
//
//	colNames, err := rows.Columns()
//	if err != nil {
//		return fmt.Errorf("failed to get columns: %w", err)
//	}
//
//	_, err = excelFile.NewSheet(sheetName)
//	if err != nil {
//		return fmt.Errorf("failed to create sheet '%s': %w", sheetName, err)
//	}
//
//	if err := excelFile.SetSheetRow(sheetName, "A1", &colNames); err != nil {
//		return fmt.Errorf("failed to write headers: %w", err)
//	}
//
//	rowIndex := 2 // Start from row 2
//	for rows.Next() {
//		cols := make([]interface{}, len(colNames))
//		colPtrs := make([]interface{}, len(cols))
//		for i := range cols {
//			colPtrs[i] = &cols[i]
//		}
//
//		if err := rows.Scan(colPtrs...); err != nil {
//			return fmt.Errorf("failed to scan row: %w", err)
//		}
//
//		strCols := make([]interface{}, len(cols))
//		for i, col := range cols {
//			if col == nil {
//				strCols[i] = ""
//				continue
//			}
//			if isJSONColumn(colNames[i]) {
//				var prettyJSON bytes.Buffer
//				// Attempt to parse as JSON
//				err := json.Indent(&prettyJSON, []byte(fmt.Sprintf("%v", col)), "", "  ")
//				if err != nil {
//					// Not valid JSON, write as is
//					strCols[i] = fmt.Sprintf("%v", col)
//				} else {
//					strCols[i] = prettyJSON.String()
//				}
//			} else {
//				strCols[i] = fmt.Sprintf("%v", col)
//			}
//		}
//
//		cellAddr := fmt.Sprintf("A%d", rowIndex)
//		if err := excelFile.SetSheetRow(sheetName, cellAddr, &strCols); err != nil {
//			return fmt.Errorf("failed to write data row: %w", err)
//		}
//		rowIndex++
//	}
//
//	defaultSheetName := excelFile.GetSheetName(0)
//	if defaultSheetName == "Sheet1" {
//		// Delete default sheet
//		excelFile.DeleteSheet(defaultSheetName)
//	}
//
//	return nil
//}
//
//func isJSONColumn(columnName string) bool {
//	jsonColumns := map[string]bool{
//		"attributes": true,
//	}
//
//	columnName = strings.ToLower(columnName)
//	return jsonColumns[columnName]
//}
//
//func sanitizeIdentifier(name string) string {
//	// Replace spaces and other unsafe characters with underscores and convert to lowercase.
//	return strings.ToLower(strings.ReplaceAll(name, " ", "_"))
//}
//
//func isJSON(s string) bool {
//	var js json.RawMessage
//	return json.Unmarshal([]byte(s), &js) == nil
//}
