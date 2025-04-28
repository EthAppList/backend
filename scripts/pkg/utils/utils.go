package utils

import (
	"os"
	"path/filepath"
)

// FindProjectRoot attempts to find the root directory of the project
func FindProjectRoot() (string, error) {
	// Start from the current working directory
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Walk up the directory tree looking for go.mod which indicates the project root
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		// Move up one directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// We've reached the filesystem root without finding go.mod
			return "", os.ErrNotExist
		}
		dir = parent
	}
}
