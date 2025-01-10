// File: ./utils/Utils.go
package utils

import (
	"fmt"
	"github.com/reaandrew/techdetector/core"
)

// MockMatchRepository is a mock implementation of core.FindingRepository
type MockMatchRepository struct {
	Matches []core.Finding
}

// Store appends the provided findings to the repository's Matches slice.
func (m *MockMatchRepository) Store(matches []core.Finding) error {
	m.Matches = append(m.Matches, matches...)
	return nil
}

// Clear removes all findings from the repository.
func (m *MockMatchRepository) Clear() error {
	m.Matches = nil
	return nil
}

// NewIterator returns a new MockMatchIterator for iterating over the findings.
func (m *MockMatchRepository) NewIterator() core.FindingIterator {
	// Create copies of the findings to prevent mutation during iteration
	copiedFindings := make([]core.Finding, len(m.Matches))
	copy(copiedFindings, m.Matches)

	return &MockMatchIterator{
		position: 0,
		matches:  []core.FindingSet{{Matches: copiedFindings}},
	}
}

// MockMatchIterator is a mock implementation of core.FindingIterator
type MockMatchIterator struct {
	position int
	matches  []core.FindingSet
}

// Reset resets the iterator to the beginning.
func (m *MockMatchIterator) Reset() error {
	m.position = 0
	return nil
}

// HasNext checks if there are more findings to iterate over.
func (m *MockMatchIterator) HasNext() bool {
	return m.position < len(m.matches)
}

// Next retrieves the next set of findings.
func (m *MockMatchIterator) Next() (core.FindingSet, error) {
	if !m.HasNext() {
		return core.FindingSet{}, fmt.Errorf("no more findings")
	}
	findingSet := m.matches[m.position]
	m.position++
	return findingSet, nil
}
