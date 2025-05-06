package reportstorage

import (
	"fmt"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

type FileReportStorage struct {
	ArtifactPrefix string
	OutputDir      string
	Extension      string
}

const defaultJsonSummaryReport = "cloud_services_summary.json"

func CreateFileReportStorage(artifactPrefix, outputDir, extension string) (FileReportStorage, error) {
	return FileReportStorage{
		ArtifactPrefix: artifactPrefix,
		OutputDir:      outputDir,
		Extension:      extension,
	}, nil
}

func (s *FileReportStorage) setDefaultOutputDir() {
	if s.OutputDir == "" {
		s.OutputDir = "."
	}
}

func (s FileReportStorage) Store(data []byte) error {
	s.setDefaultOutputDir()

	outputFilePath := fmt.Sprintf("%s/%s_%s", s.OutputDir, s.ArtifactPrefix, defaultJsonSummaryReport)
	outputFile, err := os.Create(outputFilePath)
	if err != nil {
		return fmt.Errorf("failed to create summary JSON output file: %w", err)
	}
	defer outputFile.Close()

	_, err = outputFile.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write to summary output file: %v", err)
	}

	return nil
}
