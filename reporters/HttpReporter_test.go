package reporters

import (
	"bytes"
	"fmt"
	"github.com/reaandrew/techdetector/core"
	"github.com/reaandrew/techdetector/utils"
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

type MockReportIdGenerator struct {
	id string
}

func (m MockReportIdGenerator) Generate() string {
	return m.id
}

func TestHttpReporter_Report(t *testing.T) {
	expectedId := "101"
	mockRepository := utils.MockMatchRepository{Matches: []core.Finding{
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
