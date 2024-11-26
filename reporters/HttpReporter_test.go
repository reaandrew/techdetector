package reporters

import (
	"bytes"
	"fmt"
	reporters2 "github.com/reaandrew/techdetector/core"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"testing"
)

type MockHttpClient struct {
	requests []http.Request
}

func (m *MockHttpClient) Do(req *http.Request) (*http.Response, error) {
	m.requests = append(m.requests, *req)

	responseBody := "This is a mock response body."

	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewBufferString(responseBody)),
		Header:     make(http.Header),
	}

	return resp, nil
}

func (m MockHttpClient) GetRequests() []http.Request {
	return m.requests
}

type MockMatchRepository struct {
	matches []reporters2.Finding
}

func (m MockMatchRepository) Store(matches []reporters2.Finding) error {
	//TODO implement me
	panic("implement me")
}

func (m MockMatchRepository) Clear() error {
	//TODO implement me
	panic("implement me")
}

func (m MockMatchRepository) NewIterator() reporters2.FindingIterator {
	return &MockMatchIterator{
		position: 0,
		matches: []reporters2.FindingSet{
			{Matches: m.matches},
		},
	}
}

type MockMatchIterator struct {
	position int
	matches  []reporters2.FindingSet
}

func (m *MockMatchIterator) HasNext() bool {
	return m.position < len(m.matches)
}

func (m *MockMatchIterator) Next() (reporters2.FindingSet, error) {
	returnValue := m.matches[m.position]
	m.position++
	return returnValue, nil
}

type MockReportIdGenerator struct {
	id string
}

func (m MockReportIdGenerator) Generate() string {
	return m.id
}

func TestHttpReporter_Report(t *testing.T) {
	expectedId := "101"
	mockRepository := MockMatchRepository{matches: []reporters2.Finding{
		{
			Name:     "Match1",
			Type:     "Type1",
			Category: "Category1",
			Properties: map[string]interface{}{
				"key1": "value1",
			},
			Path:     "/path/to/resource1",
			RepoName: "Repo1",
		},
	}}
	client := MockHttpClient{}
	report := HttpReporter{
		BaseURL:    "https://somewhere",
		HTTPClient: &client,
		ReportIdGenerator: MockReportIdGenerator{
			id: expectedId,
		},
	}
	err := report.Report(mockRepository)
	assert.Nil(t, err)
	assert.Len(t, client.GetRequests(), 2)

	request1 := client.GetRequests()[0]
	assert.Equal(t, fmt.Sprintf("https://somewhere/reports/%s/results", expectedId), request1.URL.String())
	assert.Equal(t, "POST", request1.Method)

	request2 := client.GetRequests()[1]
	assert.Equal(t, fmt.Sprintf("https://somewhere/report/%s", expectedId), request2.URL.String())
	assert.Equal(t, "PATCH", request2.Method)
}
