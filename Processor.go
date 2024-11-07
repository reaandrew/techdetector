package main

import "embed"

//go:embed data/patterns/*.json
var patternsFS embed.FS

// Processor is an interface that defines a generic processor.
type Processor interface {
	Supports(filePath string) bool

	Process(path string, repoName string, content string) ([]Match, error)
}

// InitializeProcessors creates and returns a slice of Processor implementations.
func InitializeProcessors() []Processor {
	var processors []Processor

	filePatternsProcessor := NewFilePatternsProcessor(patternsFS)
	processors = append(processors, filePatternsProcessor)

	processors = append(processors, NewLibrariesProcessor())
	return processors
}
