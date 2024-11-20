package reporters

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"net/http"

	"github.com/reaandrew/techdetector/repositories"
)

type ReportIdGenerator interface {
	Generate() string
}

type UuidReportGenerator struct {
}

func (u UuidReportGenerator) Generate() string {
	return uuid.New().String()
}

type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type DefaultHttpClient struct {
}

func (d DefaultHttpClient) Do(req *http.Request) (*http.Response, error) {
	return http.DefaultClient.Do(req)
}

func NewDefaultHttpReporter(baseUrl string) HttpReporter {
	return HttpReporter{
		BaseURL:           baseUrl,
		HTTPClient:        DefaultHttpClient{},
		ReportIdGenerator: UuidReportGenerator{},
	}
}

type HttpReporter struct {
	BaseURL           string
	HTTPClient        HttpClient
	ReportIdGenerator ReportIdGenerator
}

func (h HttpReporter) Report(repository repositories.MatchRepository) error {
	iterator := repository.NewIterator()

	reportId := h.ReportIdGenerator.Generate()

	for iterator.HasNext() {
		matchSet, _ := iterator.Next()

		err := h.postMatch(matchSet, reportId)
		if err != nil {
			return fmt.Errorf("failed to report matchSet: %v", err)
		}
	}

	err := h.signalCompletion(reportId)
	if err != nil {
		return fmt.Errorf("failed to signal completion: %v", err)
	}

	return nil
}

func (h HttpReporter) postMatch(match repositories.MatchSet, reportId string) error {
	url := fmt.Sprintf("%s/reports/%s/results", h.BaseURL, reportId)

	payload, err := json.Marshal(match)
	if err != nil {
		return fmt.Errorf("failed to marshal match: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected response status: %d", resp.StatusCode)
	}

	return nil
}

func (h HttpReporter) signalCompletion(reportId string) error {
	url := fmt.Sprintf("%s/reports/%s", h.BaseURL, reportId)

	req, err := http.NewRequest("PATCH", url, bytes.NewReader([]byte(`{
    "status": "completed"
}`)))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	resp, err := h.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected response status: %d", resp.StatusCode)
	}

	return nil
}
