package core

var SummaryFinding int = 1
var DetailFinding int = 2

type Finding struct {
	Name       string                 `json:"name,omitempty"`
	Type       string                 `json:"type,omitempty"`
	Category   string                 `json:"category,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Path       string                 `json:"path,omitempty"`
	RepoName   string                 `json:"repo_name,omitempty"`
}
