package reporters

import (
	"encoding/json"
	"fmt"
	"github.com/reaandrew/techdetector/repositories"
	"log"
	"os"
)

const (
	DefaultJsonReport = "cloud_services_report.json"
)

type JsonReporter struct {
}

func (j JsonReporter) Report(repository repositories.MatchRepository) error {
	outputFile, err := os.OpenFile(DefaultJsonReport, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open outputFile: %v", err)
	}
	defer outputFile.Close()

	iterator := repository.NewIterator()
	for iterator.HasNext() {
		match, _ := iterator.Next()
		jsonBytes, err := json.Marshal(match)
		if err != nil {
			return err
		}

		// Write JSON block followed by a newline
		_, err = outputFile.Write(jsonBytes)
		if err != nil {
			return fmt.Errorf("failed to write to outputFile: %v", err)
		}
		_, err = outputFile.WriteString("\n") // Add a newline after each block
		if err != nil {
			return fmt.Errorf("failed to write newline to outputFile: %v", err)
		}
	}

	fmt.Printf("JSON report generated successfully: %s\n", outputFile.Name())
	return nil
}
