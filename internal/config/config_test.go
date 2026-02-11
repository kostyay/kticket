package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDirEnvOverride(t *testing.T) {
	t.Setenv(EnvDir, "/custom/path")
	assert.Equal(t, "/custom/path", Dir())
}

func TestDirUsesGitRoot(t *testing.T) {
	t.Setenv(EnvDir, "")

	dir := Dir()
	assert.True(t, filepath.IsAbs(dir), "expected absolute path, got %s", dir)
	assert.True(t, strings.HasSuffix(dir, DefaultDir))
}

func TestDirFallbackNoGitRoot(t *testing.T) {
	t.Setenv(EnvDir, "")

	// cd to a temp dir with no .git
	tmp := t.TempDir()
	orig, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmp))
	defer os.Chdir(orig)

	dir := Dir()
	assert.Equal(t, DefaultDir, dir)
}
