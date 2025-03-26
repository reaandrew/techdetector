package utils

import (
	"fmt"
	"github.com/google/uuid"
	"os"
	"strings"
)

func Contains[T comparable](slice []T, element T) bool {
	for _, v := range slice {
		if v == element {
			return true
		}
	}
	return false
}

func GenerateRandomFilename(extension string) string {
	id := uuid.New()
	return fmt.Sprintf("%s.%s", id.String(), extension)
}

func CountFiles(dirPath string) (int, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, entry := range entries {
		if entry.Type().IsRegular() {
			count++
		}
	}
	return count, nil
}

func Sanitize(name string) string {
	s := name
	// Example: remove protocols
	s = strings.ReplaceAll(s, "https://", "")
	s = strings.ReplaceAll(s, "http://", "")
	// Replace slashes, colons, etc. with underscores
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, ":", "_")
	// Optionally lower-case everything
	s = strings.ToLower(s)

	return s
}
