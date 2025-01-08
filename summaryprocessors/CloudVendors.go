package summaryprocessors

import "github.com/reaandrew/techdetector/core"

type CloudVendors struct {
	vendors map[string]int
}

func NewCloudVendorsSummaryProcessor() *CloudVendors {
	return &CloudVendors{vendors: map[string]int{}}
}

func (v *CloudVendors) Process(finding core.Finding) {
	if val, ok := finding.Properties["vendor"]; ok {
		if _, vendorOk := v.vendors[val.(string)]; !vendorOk {
			v.vendors[val.(string)] = 0
		}
		v.vendors[val.(string)]++
	}
}

func (v *CloudVendors) GetFindings() []core.Finding {
	var summaries []core.Finding
	for key, value := range v.vendors {
		summaries = append(summaries, core.Finding{
			Name:     "Cloud Vendors",
			Type:     "Summary",
			Category: key,
			Properties: map[string]interface{}{
				"reference_count": value,
			},
			Path:     "",
			RepoName: "",
		})
	}
	return summaries
}
