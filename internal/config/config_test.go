package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDirDefault(t *testing.T) {
	os.Unsetenv(EnvDir)
	assert.Equal(t, DefaultDir, Dir())
	assert.Equal(t, ".ktickets", Dir())
}

func TestDirEnvOverride(t *testing.T) {
	os.Setenv(EnvDir, "/custom/path")
	defer os.Unsetenv(EnvDir)

	assert.Equal(t, "/custom/path", Dir())
}

func TestDirEnvEmpty(t *testing.T) {
	os.Setenv(EnvDir, "")
	defer os.Unsetenv(EnvDir)

	// Empty env var should fall back to default
	assert.Equal(t, DefaultDir, Dir())
}
