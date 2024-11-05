package main

import (
	"encoding/json"
	"fmt"
	"github.com/xuri/excelize/v2"
)

const (
	DefaultReport   = "cloud_services_report.xlsx"
	ServicesSheet   = "Services"
	FrameworksSheet = "Frameworks"
)

type XlsxReporter struct {
}

// Report generates an XLSX report from the findings.
// It creates two worksheets: "Services" and "Frameworks".
func (xlsxReporter XlsxReporter) Report(findings []Finding) error {
	fmt.Println("Generating XLSX file")

	// Create a new Excel file
	f := excelize.NewFile()

	// Rename the default sheet to "Services"
	defaultSheet := f.GetSheetName(0)
	if defaultSheet != ServicesSheet {
		if err := f.SetSheetName(defaultSheet, ServicesSheet); err != nil {
			return fmt.Errorf("failed to rename sheet '%s' to '%s': %w", defaultSheet, ServicesSheet, err)
		}
		fmt.Printf("Renamed default sheet '%s' to '%s'\n", defaultSheet, ServicesSheet)
	}

	// Create the "Frameworks" sheet
	frameworksIndex, err := f.NewSheet(FrameworksSheet)
	if err != nil {
		return fmt.Errorf("failed to create sheet '%s': %w", FrameworksSheet, err)
	}
	fmt.Printf("Created sheet '%s' with index %d\n", FrameworksSheet, frameworksIndex)

	// Set headers for Services sheet
	servicesHeaders := []string{
		"Cloud Vendor",
		"Cloud CloudService",
		"Language",
		"Reference",
		"Repository",
		"Filepath",
	}
	if err := f.SetSheetRow(ServicesSheet, "A1", &servicesHeaders); err != nil {
		return fmt.Errorf("failed to set headers for sheet '%s': %w", ServicesSheet, err)
	}
	fmt.Printf("Set headers for sheet '%s'\n", ServicesSheet)

	// Set headers for Frameworks sheet
	frameworksHeaders := []string{
		"Name",
		"Category",
		"Package File Name",
		"Pattern",
		"Repository",
		"Filepath",
	}
	if err := f.SetSheetRow(FrameworksSheet, "A1", &frameworksHeaders); err != nil {
		return fmt.Errorf("failed to set headers for sheet '%s': %w", FrameworksSheet, err)
	}
	fmt.Printf("Set headers for sheet '%s'\n", FrameworksSheet)

	// Initialize row counters for each sheet
	servicesRow := 2   // Starting from row 2 (row 1 is for headers)
	frameworksRow := 2 // Starting from row 2 (row 1 is for headers)

	// Iterate over findings and populate respective sheets
	for _, finding := range findings {
		if finding.Service != nil {
			// Prepare data for Services sheet
			rowData := []interface{}{
				finding.Service.CloudVendor,
				finding.Service.CloudService,
				finding.Service.Language,
				finding.Service.Reference,
				finding.Repository,
				finding.Filepath,
			}

			// Convert row number to cell address (e.g., A2)
			cellAddress, err := excelize.CoordinatesToCellName(1, servicesRow)
			if err != nil {
				return fmt.Errorf("failed to get cell address for row %d in sheet '%s': %w", servicesRow, ServicesSheet, err)
			}

			// Set the row data starting from column A
			if err := f.SetSheetRow(ServicesSheet, cellAddress, &rowData); err != nil {
				return fmt.Errorf("failed to set data for row %d in sheet '%s': %w", servicesRow, ServicesSheet, err)
			}

			servicesRow++ // Move to the next row for Services
		}

		if finding.Framework != nil {
			// Prepare data for Frameworks sheet
			rowData := []interface{}{
				finding.Framework.Name,
				finding.Framework.Category,
				finding.Framework.PackageFileName,
				finding.Framework.Pattern,
				finding.Repository,
				finding.Filepath,
			}

			// Convert row number to cell address (e.g., A2)
			cellAddress, err := excelize.CoordinatesToCellName(1, frameworksRow)
			if err != nil {
				return fmt.Errorf("failed to get cell address for row %d in sheet '%s': %w", frameworksRow, FrameworksSheet, err)
			}

			// Set the row data starting from column A
			if err := f.SetSheetRow(FrameworksSheet, cellAddress, &rowData); err != nil {
				return fmt.Errorf("failed to set data for row %d in sheet '%s': %w", frameworksRow, FrameworksSheet, err)
			}

			frameworksRow++ // Move to the next row for Frameworks
		}
	}

	index, _ := f.GetSheetIndex(ServicesSheet)
	// Optionally, set the active sheet to Services
	f.SetActiveSheet(index)

	// Determine the output file name
	outputFile := DefaultReport
	//if len(findings) > 0 {
	//	if findings[0].Service != nil || findings[0].Framework != nil {
	//		outputFile = fmt.Sprintf("report_%s.xlsx", strings.ReplaceAll(findings[0].Repository, "/", "_"))
	//	}
	//}

	// Save the Excel file
	if err := f.SaveAs(outputFile); err != nil {
		return fmt.Errorf("failed to save XLSX file '%s': %w", outputFile, err)
	}

	fmt.Printf("XLSX report generated successfully: %s\n", outputFile)
	return nil
}

type Reporter struct {
	xlsxReporter XlsxReporter
}

// GenerateReport decides which report to generate based on the report format.
func (reporter Reporter) GenerateReport(findings []Finding, reportFormat string) error {
	if reportFormat == "xlsx" {
		return reporter.xlsxReporter.Report(findings)
	}

	// Default to JSON output
	findingsJSON, err := json.MarshalIndent(findings, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal findings to JSON: %w", err)
	}

	fmt.Println(string(findingsJSON))
	return nil
}
