package utils

import "github.com/reaandrew/techdetector/core"

type MockMatchRepository struct {
	Matches []core.Finding
}

func (m MockMatchRepository) Store(matches []core.Finding) error {
	//TODO implement me
	panic("implement me")
}

func (m MockMatchRepository) Clear() error {
	//TODO implement me
	panic("implement me")
}

func (m MockMatchRepository) NewIterator() core.FindingIterator {
	return &MockMatchIterator{
		position: 0,
		matches: []core.FindingSet{
			{Matches: m.Matches},
		},
	}
}

type MockMatchIterator struct {
	position int
	matches  []core.FindingSet
}

func (m *MockMatchIterator) Reset() error {
	m.position = 0
	return nil
}

func (m *MockMatchIterator) HasNext() bool {
	return m.position < len(m.matches)
}

func (m *MockMatchIterator) Next() (core.FindingSet, error) {
	returnValue := m.matches[m.position]
	m.position++
	return returnValue, nil
}
