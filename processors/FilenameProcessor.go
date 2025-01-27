package processors

import (
	"github.com/reaandrew/techdetector/core"
	"strings"
)

type FilenameProcessor struct {
}

func (f FilenameProcessor) Supports(filePath string) bool {
	return !strings.Contains(filePath, ".git")
}

func (f FilenameProcessor) Process(path string, repoName string, content string) ([]core.Finding, error) {
	return []core.Finding{
		{
			Name:       path,
			Type:       "File",
			Category:   "",
			Properties: nil,
			Path:       path,
			RepoName:   repoName,
		},
	}, nil
}
