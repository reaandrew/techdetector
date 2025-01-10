// File: ./reporters/DynamicXlsxSummaryReporter_test.go
package reporters

import (
	"github.com/reaandrew/techdetector/core"
	"github.com/reaandrew/techdetector/utils"
)

func setupMockRepository() core.FindingRepository {
	return &utils.MockMatchRepository{
		Matches: []core.Finding{
			{
				Type:     "Azure Bicep",
				Name:     "Resource1",
				Category: "Category1",
				RepoName: "Repo1",
				Path:     "Path1",
				Properties: map[string]interface{}{
					"resource": "ResourceType1",
				},
			},
			{
				Type:     "Azure Bicep",
				Name:     "Resource2",
				Category: "Category2",
				RepoName: "Repo2",
				Path:     "Path2",
				Properties: map[string]interface{}{
					"resource": "ResourceType1",
				},
			},
			{
				Type:     "Library",
				Name:     "Lib1",
				Category: "LibCat1",
				RepoName: "Repo1",
				Path:     "Path3",
				Properties: map[string]interface{}{
					"Language": "Go",
					"Version":  "1.15",
				},
			},
		},
	}
}
