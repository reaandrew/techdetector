package reporters

import (
	"github.com/reaandrew/techdetector/core"
	log "github.com/sirupsen/logrus"
)

type XlsxReporter struct {
	Queries          core.SqlQueries
	DumpSchema       bool
	ArtifactPrefix   string
	SqliteDBFilename string
}

func (x XlsxReporter) Report(repository core.FindingRepository) error {
	// Generate summary report
	summaryReporter := DynamicXlsxSummaryReporterForFindingsSqlTable{
		x.Queries,
		x.DumpSchema,
		x.ArtifactPrefix,
		x.SqliteDBFilename}
	err := summaryReporter.Report(repository)
	if err != nil {
		log.Fatalf("Error generating summary report: %v", err)
		return err
	}

	rawReporter := XlsxRawReporter{x.ArtifactPrefix}
	err = rawReporter.Report(repository)
	if err != nil {
		log.Fatalf("Error generating report: %v", err)
		return err
	}
	return nil
}
