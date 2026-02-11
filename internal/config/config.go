package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// DefaultDir is the default directory for storing tickets.
	DefaultDir = ".ktickets"

	// EnvDir is the environment variable to override the directory.
	EnvDir = "KTICKET_DIR"
)

// Dir returns the tickets directory.
// Checks KTICKET_DIR env var first, then resolves relative to git root,
// falls back to DefaultDir in cwd if not in a git repo.
func Dir() string {
	if dir := os.Getenv(EnvDir); dir != "" {
		return dir
	}
	gitRoot, err := FindGitRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v; using ./%s\n", err, DefaultDir)
		return DefaultDir
	}
	return filepath.Join(gitRoot, DefaultDir)
}
