package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/wesjorgensen/EthAppList/backend/scripts/pkg/utils"
)

func main() {
	// Get the project root directory
	dir, err := utils.FindProjectRoot()
	if err != nil {
		log.Fatalf("Failed to find project root: %v", err)
	}

	// Build and run the setup command with Docker flag
	setupPath := filepath.Join(dir, "scripts", "cmd", "setup", "main.go")
	cmd := exec.Command("go", "run", setupPath, "-docker")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Println("Running Docker database setup...")
	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to run Docker setup: %v", err)
	}
}

// findProjectRoot attempts to find the root directory of the project
func findProjectRoot() (string, error) {
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
