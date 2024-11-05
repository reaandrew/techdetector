package main

import "embed"

type Finding struct {
	Service    *CloudService `json:"service,omitempty"`
	Framework  *Framework    `json:"framework,omitempty"`
	Repository string        `json:"repository"`
	Filepath   string        `json:"filepath"`
}

//go:embed data/cloud_service_mappings/*.json
var servicesFS embed.FS

// Processor is an interface that defines a generic processor.
type Processor interface {
	Supports(filePath string) bool

	Process(path string, repoName string, content string) []Finding
}

// InitializeProcessors creates and returns a slice of Processor implementations.
func InitializeProcessors() []Processor {
	var processors []Processor

	// Initialize CloudServiceProcessor
	serviceProcessor := NewServiceProcessor(NewFileCloudServicesLoader(servicesFS))
	processors = append(processors, serviceProcessor)

	// Initialize FrameworkProcessor
	frameworkProcessor := NewFrameworkProcessor()
	processors = append(processors, frameworkProcessor)

	return processors
}
