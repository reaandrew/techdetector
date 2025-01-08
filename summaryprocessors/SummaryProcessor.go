package summaryprocessors

import "github.com/reaandrew/techdetector/core"

type SummaryProcessor interface {
	Process(finding core.Finding)
	GetFindings() []core.Finding
}
