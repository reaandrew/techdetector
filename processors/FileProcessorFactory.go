package processors

import (
	"embed"
	"github.com/reaandrew/techdetector/core"
)

//go:embed data/patterns/*.json
var patternsFS embed.FS

// InitializeProcessors creates and returns a slice of FileProcessor implementations.
func InitializeProcessors() []core.FileProcessor {
	var processors []core.FileProcessor

	filePatternsProcessor := NewFilePatternsProcessor(patternsFS)
	processors = append(processors, filePatternsProcessor)

	processors = append(processors, NewLibrariesProcessor())

	processors = append(processors, DockerProcessor{})

	processors = append(processors, NewTerraformProcessor())

	processors = append(processors, DockerComposeProcessor{})
	return processors
}
