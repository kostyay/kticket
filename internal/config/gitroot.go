package config

import (
	"errors"
	"os"
	"path/filepath"
)

// FindGitRoot walks from cwd upward looking for a .git directory.
func FindGitRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return findGitRootFrom(cwd)
}

func findGitRootFrom(dir string) (string, error) {
	for {
		if info, err := os.Stat(filepath.Join(dir, ".git")); err == nil && info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("not a git repository (or any parent up to /)")
		}
		dir = parent
	}
}
