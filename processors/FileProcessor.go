package processors

import (
	"embed"
	"github.com/reaandrew/techdetector/core"
)

//go:embed data/patterns/*.json
var patternsFS embed.FS

// FileProcessor is an interface that defines a generic processor.
type FileProcessor interface {
	Supports(filePath string) bool

	Process(path string, repoName string, content string) ([]reporters.Finding, error)
}

// InitializeProcessors creates and returns a slice of FileProcessor implementations.
func InitializeProcessors() []FileProcessor {
	var processors []FileProcessor

	filePatternsProcessor := NewFilePatternsProcessor(patternsFS)
	processors = append(processors, filePatternsProcessor)

	processors = append(processors, NewLibrariesProcessor())

	processors = append(processors, DockerProcessor{})

	processors = append(processors, NewTerraformProcessor())
	return processors
}
