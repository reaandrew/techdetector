package reporters

import (
	"github.com/reaandrew/techdetector/core"
	"log"
)

type XlsxReporter struct {
	Queries    core.SqlQueries
	DumpSchema bool
}

func (x XlsxReporter) Report(repository core.FindingRepository) error {
	// Generate summary report
	summaryReporter := DynamicXlsxSummaryReporter{x.Queries, x.DumpSchema}
	err := summaryReporter.Report(repository)
	if err != nil {
		log.Fatalf("Error generating summary report: %v", err)
		return err
	}

	rawReporter := XlsxRawReporter{}
	err = rawReporter.Report(repository)
	if err != nil {
		log.Fatalf("Error generating report: %v", err)
		return err
	}
	return nil
}
