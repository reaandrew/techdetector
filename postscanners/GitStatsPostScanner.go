package postscanners

import (
	"github.com/reaandrew/techdetector/core"
	"github.com/reaandrew/techdetector/utils"
)

type GitStatsPostScanner struct {
	GitMetrics utils.GitMetrics
	CutOffDate string
}

func (g GitStatsPostScanner) Scan(path, name string) ([]core.Finding, error) {
	return g.GitMetrics.CollectGitMetrics(path, name, g.CutOffDate)
}
