package core

type FindingSet struct {
	Matches []Finding `json:"matchSet"`
}

type FindingRepository interface {
	Store(matches []Finding) error
	Clear() error
	NewIterator() FindingIterator
	Close() error // add this method
}

type FindingIterator interface {
	HasNext() bool
	Next() (FindingSet, error)
	Reset() error // New method to reset the iterator
}
