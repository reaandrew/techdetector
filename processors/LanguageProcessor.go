package processors

import (
	"github.com/go-enry/go-enry/v2"
	"github.com/reaandrew/techdetector/core"
	"strings"
)

type LanguageProcessor struct {
}

func (l LanguageProcessor) Supports(filePath string) bool {
	return !strings.Contains(filePath, ".git")
}

func (l LanguageProcessor) Process(path string, repoName string, content string) ([]core.Finding, error) {
	language := enry.GetLanguage(path, []byte(content))

	if language != "" {
		finding := core.Finding{
			Name:     language,
			Type:     "Programming Language",
			Category: "metadata",
			Properties: map[string]interface{}{
				"language": language,
			},
			Path:     path,
			RepoName: repoName,
		}
		return []core.Finding{finding}, nil
	}
	return nil, nil
}
