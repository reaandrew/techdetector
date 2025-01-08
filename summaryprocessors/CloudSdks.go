package summaryprocessors

import "github.com/reaandrew/techdetector/core"

type CloudSdks struct {
	// Nested map: vendor -> language -> SDK name -> count
	sdkSummary map[string]map[string]map[string]int
}

func NewCloudSdkSummaryProcessor() *CloudSdks {
	return &CloudSdks{
		sdkSummary: make(map[string]map[string]map[string]int),
	}
}

// Process adds the finding data to the summary
func (p *CloudSdks) Process(finding core.Finding) {
	if finding.Type != "Cloud Service SDK" {
		return
	}

	vendor, vendorOk := finding.Properties["vendor"].(string)
	language, languageOk := finding.Properties["language"].(string)
	name := finding.Name

	if vendorOk && languageOk {
		if _, vendorExists := p.sdkSummary[vendor]; !vendorExists {
			p.sdkSummary[vendor] = make(map[string]map[string]int)
		}
		if _, languageExists := p.sdkSummary[vendor][language]; !languageExists {
			p.sdkSummary[vendor][language] = make(map[string]int)
		}
		p.sdkSummary[vendor][language][name]++
	}
}

// GetFindings converts the aggregated data into summary findings
func (p *CloudSdks) GetFindings() []core.Finding {
	var summaries []core.Finding
	for vendor, languages := range p.sdkSummary {
		for language, sdks := range languages {
			for sdkName, count := range sdks {
				summaries = append(summaries, core.Finding{
					Name:     "Cloud SDK Summary",
					Type:     "Summary",
					Category: vendor,
					Properties: map[string]interface{}{
						"language":        language,
						"sdk_name":        sdkName,
						"reference_count": count,
					},
				})
			}
		}
	}
	return summaries
}
