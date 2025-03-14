package utils

import (
	"github.com/schollz/progressbar/v3"
	"os"
)

// ProgressReporter defines methods for reporting progress.
type ProgressReporter interface {
	// SetTotal reinitializes the progress bar with the new total count.
	SetTotal(total int)
	// Increment increases the progress by one.
	Increment()
}

// BarProgressReporter is a concrete implementation using progressbar.
type BarProgressReporter struct {
	description string
	bar         *progressbar.ProgressBar
	total       int
}

// NewBarProgressReporter creates a new BarProgressReporter with the given total and description.
func NewBarProgressReporter(total int, description string) *BarProgressReporter {
	bar := progressbar.NewOptions(total,
		progressbar.OptionSetWriter(os.Stdout),
		progressbar.OptionSetDescription(description),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionThrottle(100e6),           // rate-limit updates
		progressbar.OptionSetRenderBlankState(true), // show an initial blank bar
		progressbar.OptionUseANSICodes(true),        // force ANSI codes even if not a TTY
	)
	return &BarProgressReporter{
		description: description,
		bar:         bar,
		total:       total,
	}
}

// SetTotal reinitializes the progress bar with the new total count.
func (p *BarProgressReporter) SetTotal(total int) {
	p.total = total
	p.bar = progressbar.NewOptions(total,
		progressbar.OptionSetWriter(os.Stdout),
		progressbar.OptionSetDescription(p.description),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionThrottle(100e6),
		progressbar.OptionSetRenderBlankState(true),
		progressbar.OptionUseANSICodes(true),
	)
}

// Increment increases the progress bar by one.
func (p *BarProgressReporter) Increment() {
	_ = p.bar.Add(1)
}
