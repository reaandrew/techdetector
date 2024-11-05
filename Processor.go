package main

// Processor is an interface that defines a generic processor.
type Processor interface {
	Process(path string, repoName string, content string) []Finding
}
