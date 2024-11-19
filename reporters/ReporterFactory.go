package reporters

import (
	"fmt"
)

func CreateReporter(reportFormat string) (Reporter, error) {
	if reportFormat == "xlsx" {
		return XlsxReporter{}, nil
	}
	if reportFormat == "json" {
		return JsonReporter{}, nil
	}

	return nil, fmt.Errorf("unknown report format: %s", reportFormat)
}
