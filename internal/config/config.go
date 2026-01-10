package config

import "os"

const (
	// DefaultDir is the default directory for storing tickets.
	DefaultDir = ".ktickets"

	// EnvDir is the environment variable to override the directory.
	EnvDir = "KTICKET_DIR"
)

// Dir returns the tickets directory.
// Checks KTICKET_DIR env var first, falls back to DefaultDir.
func Dir() string {
	if dir := os.Getenv(EnvDir); dir != "" {
		return dir
	}
	return DefaultDir
}
