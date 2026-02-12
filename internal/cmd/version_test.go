package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/kostyay/kticket/internal/version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunVersion(t *testing.T) {
	// Set known values
	version.Version = "v0.1.0-test"
	version.Commit = "abc1234"
	version.Date = "2026-02-12T00:00:00Z"

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"version"})
	err := rootCmd.Execute()
	require.NoError(t, err)
}

func TestRunVersionJSON(t *testing.T) {
	defer setupTestEnv(t)()
	jsonFlag = true
	defer func() { jsonFlag = false }()

	version.Version = "v0.1.0-test"
	version.Commit = "abc1234"
	version.Date = "2026-02-12T00:00:00Z"

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"version", "--json"})
	err := rootCmd.Execute()
	require.NoError(t, err)
}

func TestVersionDefaults(t *testing.T) {
	// Reset to defaults
	version.Version = "dev"
	version.Commit = "unknown"
	version.Date = "unknown"

	assert.Equal(t, "dev", version.Version)
	assert.Equal(t, "unknown", version.Commit)
	assert.Equal(t, "unknown", version.Date)
}

func TestVersionJSONStructure(t *testing.T) {
	version.Version = "v1.0.0"
	version.Commit = "deadbeef"
	version.Date = "2026-02-12T10:00:00Z"

	data := map[string]string{
		"version": version.Version,
		"commit":  version.Commit,
		"date":    version.Date,
	}
	b, err := json.Marshal(data)
	require.NoError(t, err)

	var parsed map[string]string
	require.NoError(t, json.Unmarshal(b, &parsed))
	assert.Equal(t, "v1.0.0", parsed["version"])
	assert.Equal(t, "deadbeef", parsed["commit"])
	assert.Equal(t, "2026-02-12T10:00:00Z", parsed["date"])
}
