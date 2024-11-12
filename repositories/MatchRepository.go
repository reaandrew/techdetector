package repositories

import (
	"github.com/reaandrew/techdetector/processors"
)

type MatchRepository interface {
	Store(matches []processors.Match) error
	Clear() error
	NewIterator() *MatchIterator
}
