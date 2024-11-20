package repositories

import (
	"github.com/reaandrew/techdetector/processors"
)

type MatchSet struct {
	Matches []processors.Match `json:"matchSet"`
}

type MatchRepository interface {
	Store(matches []processors.Match) error
	Clear() error
	NewIterator() MatchIterator
}

type MatchIterator interface {
	HasNext() bool
	Next() (MatchSet, error)
}
