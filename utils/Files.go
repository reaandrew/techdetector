package utils

import (
	"fmt"
	"os"
)

func DeleteDatabaseFileIfExists(path string) error {
	// Check if the file exists
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		// File does not exist; nothing to delete
		return nil
	} else if err != nil {
		// An error occurred while trying to stat the file
		return fmt.Errorf("failed to check if file exists at path %s: %w", path, err)
	}

	// Ensure that the path is a file and not a directory
	if info.IsDir() {
		return fmt.Errorf("path %s is a directory, not a file", path)
	}

	// Attempt to delete the file
	err = os.Remove(path)
	if err != nil {
		return fmt.Errorf("failed to delete database file at path %s: %w", path, err)
	}

	return nil
}
