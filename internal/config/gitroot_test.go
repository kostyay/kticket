package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindGitRootFromRepoRoot(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))

	got, err := findGitRootFrom(root)
	require.NoError(t, err)
	assert.Equal(t, root, got)
}

func TestFindGitRootFromSubdir(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))

	sub := filepath.Join(root, "a", "b", "c")
	require.NoError(t, os.MkdirAll(sub, 0o755))

	got, err := findGitRootFrom(sub)
	require.NoError(t, err)
	assert.Equal(t, root, got)
}

func TestFindGitRootNotFound(t *testing.T) {
	dir := t.TempDir()

	_, err := findGitRootFrom(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a git repository")
}
