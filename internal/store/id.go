package store

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

var separatorRe = regexp.MustCompile(`[-_]`)

// GenerateID creates a unique ticket ID based on the current directory name.
func GenerateID() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := filepath.Base(cwd)

	// Extract prefix from directory name
	prefix := extractPrefix(dir)

	// 4-char hash from PID + timestamp
	data := fmt.Sprintf("%d%d", os.Getpid(), time.Now().UnixNano())
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(data)))[:4]

	return fmt.Sprintf("%s-%s", prefix, hash), nil
}

// extractPrefix derives a short prefix from a directory/project name.
func extractPrefix(name string) string {
	// Split on - or _
	parts := separatorRe.Split(name, -1)
	if len(parts) == 1 {
		// No separators: use first 1-3 chars
		if len(name) > 3 {
			return name[:3]
		}
		return name
	}
	// First letter of each part
	var prefix string
	for _, p := range parts {
		if len(p) > 0 {
			prefix += string(p[0])
		}
	}
	return prefix
}
