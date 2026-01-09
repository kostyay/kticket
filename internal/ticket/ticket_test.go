package ticket

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	input := `---
id: kt-a1b2
status: in_progress
deps: [kt-c3d4]
links: []
created: 2026-01-09T10:30:00Z
type: feature
priority: 1
assignee: kostya
tests_passed: false
---
# Add user authentication

Implement basic auth flow with session tokens.

## Design

Use JWT tokens with 24h expiry.

## Acceptance Criteria

- Users can log in with email/password
- Session persists across browser refresh

## Tests

- TestLoginSuccess
- TestLoginInvalidPassword

## Notes

**2026-01-09T14:00:00Z**

Decided to use bcrypt for password hashing.
`

	ticket, err := Parse([]byte(input))
	require.NoError(t, err)

	assert.Equal(t, "kt-a1b2", ticket.ID)
	assert.Equal(t, StatusInProgress, ticket.Status)
	assert.Equal(t, []string{"kt-c3d4"}, ticket.Deps)
	assert.Equal(t, "2026-01-09T10:30:00Z", ticket.Created)
	assert.Equal(t, TypeFeature, ticket.Type)
	assert.Equal(t, 1, ticket.Priority)
	assert.Equal(t, "kostya", ticket.Assignee)
	assert.False(t, ticket.TestsPassed)

	assert.Equal(t, "Add user authentication", ticket.Title)
	assert.Contains(t, ticket.Description, "Implement basic auth flow")
	assert.Contains(t, ticket.Design, "JWT tokens")
	assert.Contains(t, ticket.AcceptanceCriteria, "log in with email/password")
	assert.Contains(t, ticket.Tests, "TestLoginSuccess")
	assert.Contains(t, ticket.Notes, "bcrypt")
}

func TestParseMinimal(t *testing.T) {
	input := `---
id: kt-1234
status: open
created: 2026-01-09T10:00:00Z
type: task
priority: 2
tests_passed: false
---
# Simple task
`

	ticket, err := Parse([]byte(input))
	require.NoError(t, err)

	assert.Equal(t, "kt-1234", ticket.ID)
	assert.Equal(t, StatusOpen, ticket.Status)
	assert.Equal(t, "Simple task", ticket.Title)
	assert.Empty(t, ticket.Deps)
	assert.Empty(t, ticket.Description)
}

func TestMarshalRoundtrip(t *testing.T) {
	original := &Ticket{
		ID:                 "kt-test",
		Status:             StatusOpen,
		Deps:               []string{"kt-dep1"},
		Links:              []string{"kt-link1"},
		Created:            "2026-01-09T12:00:00Z",
		Type:               TypeFeature,
		Priority:           1,
		Assignee:           "tester",
		TestsPassed:        false,
		Title:              "Test Feature",
		Description:        "A test description.",
		Design:             "Design notes here.",
		AcceptanceCriteria: "- Criterion 1\n- Criterion 2",
		Tests:              "- TestOne\n- TestTwo",
		Notes:              "Some notes.",
	}

	data, err := Marshal(original)
	require.NoError(t, err)

	parsed, err := Parse(data)
	require.NoError(t, err)

	assert.Equal(t, original.ID, parsed.ID)
	assert.Equal(t, original.Status, parsed.Status)
	assert.Equal(t, original.Deps, parsed.Deps)
	assert.Equal(t, original.Type, parsed.Type)
	assert.Equal(t, original.Title, parsed.Title)
	assert.Contains(t, parsed.Description, "test description")
	assert.Contains(t, parsed.Design, "Design notes")
	assert.Contains(t, parsed.Tests, "TestOne")
}

func TestWriteAndParseFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-ticket.md")

	original := &Ticket{
		ID:          "kt-file",
		Status:      StatusClosed,
		Created:     "2026-01-09T00:00:00Z",
		Type:        TypeBug,
		Priority:    0,
		TestsPassed: true,
		Title:       "File Test",
		Description: "Testing file operations.",
	}

	err := WriteFile(path, original)
	require.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(path)
	require.NoError(t, err)

	// Parse it back
	parsed, err := ParseFile(path)
	require.NoError(t, err)

	assert.Equal(t, original.ID, parsed.ID)
	assert.Equal(t, original.Status, parsed.Status)
	assert.Equal(t, original.Title, parsed.Title)
}

func TestCanClose(t *testing.T) {
	tests := []struct {
		name        string
		tests       string
		testsPassed bool
		wantErr     bool
	}{
		{
			name:        "no tests section - can close",
			tests:       "",
			testsPassed: false,
			wantErr:     false,
		},
		{
			name:        "tests section, passed - can close",
			tests:       "- TestOne",
			testsPassed: true,
			wantErr:     false,
		},
		{
			name:        "tests section, not passed - blocked",
			tests:       "- TestOne",
			testsPassed: false,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ticket := &Ticket{
				ID:          "kt-test",
				Tests:       tt.tests,
				TestsPassed: tt.testsPassed,
			}

			err := ticket.CanClose()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "tests not passed")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	t.Run("empty file", func(t *testing.T) {
		_, err := Parse([]byte(""))
		assert.Error(t, err)
	})

	t.Run("missing frontmatter", func(t *testing.T) {
		_, err := Parse([]byte("# Just a title"))
		assert.Error(t, err)
	})

	t.Run("invalid yaml", func(t *testing.T) {
		input := `---
id: [invalid
---
# Title
`
		_, err := Parse([]byte(input))
		assert.Error(t, err)
	})
}
