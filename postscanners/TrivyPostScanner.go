package postscanners

import (
	"github.com/reaandrew/techdetector/core"
	"github.com/reaandrew/techdetector/tools"
)

type TrivyPostScanner struct {
}

func (t TrivyPostScanner) Scan(path, name string) ([]core.Finding, error) {
	return tools.TrivyScan(path, name)
}
