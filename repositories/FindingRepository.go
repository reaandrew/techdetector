package repositories

import (
	"github.com/reaandrew/techdetector/core"
)

type FindingSet struct {
	Matches []reporters.Finding `json:"matchSet"`
}

type FindingRepository interface {
	Store(matches []reporters.Finding) error
	Clear() error
	NewIterator() FindingIterator
}

type FindingIterator interface {
	HasNext() bool
	Next() (FindingSet, error)
}
