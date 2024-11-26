package reporters

// FileProcessor is an interface that defines a generic processor.
type FileProcessor interface {
	Supports(filePath string) bool

	Process(path string, repoName string, content string) ([]Finding, error)
}
