package repositories

import (
	"github.com/reaandrew/techdetector/processors"
)

type MatchSet struct {
	Matches []processors.Finding `json:"matchSet"`
}

type MatchRepository interface {
	Store(matches []processors.Finding) error
	Clear() error
	NewIterator() MatchIterator
}

type MatchIterator interface {
	HasNext() bool
	Next() (MatchSet, error)
}
