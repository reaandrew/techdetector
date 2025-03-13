package reporters

import (
	"fmt"
	"github.com/reaandrew/techdetector/core"
	log "github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
	"sort"
	"strings"
)

const (
	DefaultReport = "cloud_services_report.xlsx"
)

type XlsxRawReporter struct {
	ArtifactPrefix string
}

func (xlsxReporter XlsxRawReporter) Report(repository core.FindingRepository) error {
	fmt.Println("Generating XLSX file")

	// Create a new Excel file
	f := excelize.NewFile()

	// Map to collect matches by normalized type
	matchesByType := make(map[string][]core.Finding)

	// Collect all unique property keys per normalized match type
	propertyKeysByType := make(map[string]map[string]struct{})

	// Standard fields (excluding Properties)
	standardFields := []string{"Name", "Category", "RepoName", "Path"}

	iterator := repository.NewIterator()
	for iterator.HasNext() {
		matchSet, _ := iterator.Next()

		for _, match := range matchSet.Matches {
			// Normalize match type (e.g., trim spaces and convert to lower case)
			matchType := strings.TrimSpace(match.Type)
			matchType = strings.ToLower(matchType)

			// Update the match.Type to normalized value to maintain consistency
			match.Type = matchType

			matchesByType[matchType] = append(matchesByType[matchType], match)

			if propertyKeysByType[matchType] == nil {
				propertyKeysByType[matchType] = make(map[string]struct{})
			}

			for key := range match.Properties {
				propertyKeysByType[matchType][key] = struct{}{}
			}
		}
	}

	// Keep track of sheet names
	sheetNames := make(map[string]struct{})

	// For each match type, create a sheet and write the data
	for matchType, matchesOfType := range matchesByType {
		// Check if the sheet already exists
		if _, exists := sheetNames[matchType]; !exists {
			// Create new sheet
			index, err := f.NewSheet(matchType)
			if err != nil {
				return fmt.Errorf("failed to create sheet '%s': %w", matchType, err)
			}

			// Set the active sheet to the first one created
			if len(sheetNames) == 0 {
				f.SetActiveSheet(index)
			}

			// Add the sheet name to the map
			sheetNames[matchType] = struct{}{}
		}

		// Collect property keys for this match type and sort them
		var propertyKeys []string
		for key := range propertyKeysByType[matchType] {
			propertyKeys = append(propertyKeys, key)
		}
		sort.Strings(propertyKeys)

		// Create headers: standard fields + property keys
		headers := append(standardFields, propertyKeys...)

		// Set headers in row 1
		if err := f.SetSheetRow(matchType, "A1", &headers); err != nil {
			return fmt.Errorf("failed to set headers for sheet '%s': %w", matchType, err)
		}

		// Write matches data
		rowNum := 2 // Start from row 2
		for _, match := range matchesOfType {
			// Prepare row data
			rowData := []interface{}{
				match.Name,
				match.Category,
				match.RepoName,
				match.Path,
			}
			// Append property values in order of propertyKeys
			for _, key := range propertyKeys {
				value, ok := match.Properties[key]
				if !ok {
					rowData = append(rowData, "") // Empty if property not present
				} else {
					rowData = append(rowData, value)
				}
			}

			// Set the row data
			cellAddress, err := excelize.CoordinatesToCellName(1, rowNum)
			if err != nil {
				return fmt.Errorf("failed to get cell address for row %d in sheet '%s': %w", rowNum, matchType, err)
			}
			if err := f.SetSheetRow(matchType, cellAddress, &rowData); err != nil {
				return fmt.Errorf("failed to set data for row %d in sheet '%s': %w", rowNum, matchType, err)
			}

			rowNum++
		}
	}

	// Remove default sheet if not used
	defaultSheetName := f.GetSheetName(0)
	if defaultSheetName == "Sheet1" {
		// Delete default sheet
		f.DeleteSheet(defaultSheetName)
	}

	// Save the Excel file
	outputFile := fmt.Sprintf("%s_%s", xlsxReporter.ArtifactPrefix, DefaultReport)

	if err := f.SaveAs(outputFile); err != nil {
		return fmt.Errorf("failed to save XLSX file '%s': %w", outputFile, err)
	}

	log.Printf("XLSX report generated successfully: %s\n", outputFile)
	return nil
}
