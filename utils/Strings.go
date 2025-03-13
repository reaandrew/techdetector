package utils

import "strings"

func SanitizeForDB(name string) string {
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
