package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kostyay/kticket/internal/ticket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractPrefix(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"my-project", "mp"},
		{"kticket", "kti"},
		{"kt", "kt"},
		{"a", "a"},
		{"foo-bar-baz", "fbb"},
		{"some_thing", "st"},
		{"mix-of_both", "mob"},
		{"verylongname", "ver"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractPrefix(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateID(t *testing.T) {
	id1, err := GenerateID()
	require.NoError(t, err)
	assert.NotEmpty(t, id1)
	assert.Contains(t, id1, "-")

	// Generate another - should be different
	id2, err := GenerateID()
	require.NoError(t, err)
	assert.NotEqual(t, id1, id2)
}

func setupTestStore(t *testing.T) *Store {
	dir := t.TempDir()
	ticketsDir := filepath.Join(dir, ".tickets")
	return New(ticketsDir)
}

func createTestTicket(s *Store, id, title string, status ticket.Status) *ticket.Ticket {
	t := &ticket.Ticket{
		ID:          id,
		Status:      status,
		Created:     "2026-01-09T10:00:00Z",
		Type:        ticket.TypeTask,
		Priority:    2,
		TestsPassed: false,
		Title:       title,
	}
	_ = s.Save(t)
	return t
}

func TestStoreEnsureDir(t *testing.T) {
	dir := t.TempDir()
	ticketsDir := filepath.Join(dir, "nested", ".tickets")
	s := New(ticketsDir)

	err := s.EnsureDir()
	require.NoError(t, err)

	info, err := os.Stat(ticketsDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestStoreSaveAndGet(t *testing.T) {
	s := setupTestStore(t)

	original := &ticket.Ticket{
		ID:          "kt-save",
		Status:      ticket.StatusOpen,
		Created:     "2026-01-09T10:00:00Z",
		Type:        ticket.TypeFeature,
		Priority:    1,
		TestsPassed: false,
		Title:       "Save Test",
		Description: "Testing save.",
	}

	err := s.Save(original)
	require.NoError(t, err)

	// Verify file exists
	path := s.Path(original.ID)
	_, err = os.Stat(path)
	require.NoError(t, err)

	// Get it back
	retrieved, err := s.Get("kt-save")
	require.NoError(t, err)
	assert.Equal(t, original.ID, retrieved.ID)
	assert.Equal(t, original.Title, retrieved.Title)
}

func TestStoreList(t *testing.T) {
	s := setupTestStore(t)

	createTestTicket(s, "kt-001", "First", ticket.StatusOpen)
	createTestTicket(s, "kt-002", "Second", ticket.StatusClosed)
	createTestTicket(s, "kt-003", "Third", ticket.StatusInProgress)

	tickets, err := s.List()
	require.NoError(t, err)
	assert.Len(t, tickets, 3)
}

func TestStoreListEmpty(t *testing.T) {
	s := setupTestStore(t)
	_ = s.EnsureDir()

	tickets, err := s.List()
	require.NoError(t, err)
	assert.Empty(t, tickets)
}

func TestStoreResolveExact(t *testing.T) {
	s := setupTestStore(t)
	createTestTicket(s, "kt-exact", "Exact Match", ticket.StatusOpen)

	resolved, err := s.Resolve("kt-exact")
	require.NoError(t, err)
	assert.Equal(t, "kt-exact", resolved.ID)
}

func TestStoreResolvePartial(t *testing.T) {
	s := setupTestStore(t)
	createTestTicket(s, "kt-a1b2c3", "Partial Match", ticket.StatusOpen)

	// Should match partial
	resolved, err := s.Resolve("a1b2")
	require.NoError(t, err)
	assert.Equal(t, "kt-a1b2c3", resolved.ID)
}

func TestStoreResolveAmbiguous(t *testing.T) {
	s := setupTestStore(t)
	createTestTicket(s, "kt-abc1", "First", ticket.StatusOpen)
	createTestTicket(s, "kt-abc2", "Second", ticket.StatusOpen)

	_, err := s.Resolve("abc")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ambiguous")
}

func TestStoreResolveNotFound(t *testing.T) {
	s := setupTestStore(t)
	_ = s.EnsureDir()

	_, err := s.Resolve("nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestStoreDelete(t *testing.T) {
	s := setupTestStore(t)
	createTestTicket(s, "kt-delete", "To Delete", ticket.StatusOpen)

	// Verify exists
	_, err := s.Get("kt-delete")
	require.NoError(t, err)

	// Delete
	err = s.Delete("kt-delete")
	require.NoError(t, err)

	// Verify gone
	_, err = s.Get("kt-delete")
	require.Error(t, err)
}

func TestStoreGetNotFound(t *testing.T) {
	s := setupTestStore(t)
	_ = s.EnsureDir()

	_, err := s.Get("nonexistent")
	require.Error(t, err)
}
