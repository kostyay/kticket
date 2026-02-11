package store

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kostyay/kticket/internal/config"
)

// GenerateID creates a unique ticket ID based on the git root directory name.
// Falls back to cwd name if not in a git repo.
func GenerateID() (string, error) {
	dir, err := projectDirName()
	if err != nil {
		return "", err
	}

	prefix := extractPrefix(dir)

	// 4-char hash from PID + timestamp
	data := fmt.Sprintf("%d%d", os.Getpid(), time.Now().UnixNano())
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(data)))[:4]

	return fmt.Sprintf("%s-%s", prefix, hash), nil
}

// projectDirName returns the base name of the git root, or cwd as fallback.
func projectDirName() (string, error) {
	gitRoot, err := config.FindGitRoot()
	if err == nil {
		return filepath.Base(gitRoot), nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Base(cwd), nil
}

// extractPrefix derives a short prefix from a directory/project name.
func extractPrefix(name string) string {
	parts := strings.FieldsFunc(name, func(r rune) bool {
		return r == '-' || r == '_'
	})
	if len(parts) == 1 {
		if len(name) > 3 {
			return name[:3]
		}
		return name
	}
	var b strings.Builder
	for _, p := range parts {
		if len(p) > 0 {
			b.WriteByte(p[0])
		}
	}
	return b.String()
}
