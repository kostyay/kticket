package store

import (
	"os"
	"path/filepath"
	"sync"
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

func TestGetForUpdate(t *testing.T) {
	s := setupTestStore(t)
	createTestTicket(s, "kt-update", "Update Test", ticket.StatusOpen)

	// Get for update
	lt, err := s.GetForUpdate("kt-update")
	require.NoError(t, err)
	require.NotNil(t, lt)
	assert.Equal(t, "kt-update", lt.Ticket.ID)

	// Modify and save
	lt.Ticket.Status = ticket.StatusInProgress
	err = lt.SaveAndRelease()
	require.NoError(t, err)

	// Verify saved
	updated, err := s.Get("kt-update")
	require.NoError(t, err)
	assert.Equal(t, ticket.StatusInProgress, updated.Status)
}

func TestGetForUpdateRelease(t *testing.T) {
	s := setupTestStore(t)
	createTestTicket(s, "kt-release", "Release Test", ticket.StatusOpen)

	// Get for update
	lt, err := s.GetForUpdate("kt-release")
	require.NoError(t, err)

	// Modify but release without saving
	lt.Ticket.Status = ticket.StatusClosed
	lt.Release()

	// Original should be unchanged
	original, err := s.Get("kt-release")
	require.NoError(t, err)
	assert.Equal(t, ticket.StatusOpen, original.Status)
}

func TestGetForUpdateNotFound(t *testing.T) {
	s := setupTestStore(t)
	_ = s.EnsureDir()

	_, err := s.GetForUpdate("nonexistent")
	require.Error(t, err)
}

func TestResolveForUpdate(t *testing.T) {
	s := setupTestStore(t)
	createTestTicket(s, "kt-abc123", "Resolve Test", ticket.StatusOpen)

	// Resolve by partial ID
	lt, err := s.ResolveForUpdate("abc12")
	require.NoError(t, err)
	assert.Equal(t, "kt-abc123", lt.Ticket.ID)

	lt.Ticket.Priority = 5
	err = lt.SaveAndRelease()
	require.NoError(t, err)

	// Verify
	updated, err := s.Get("kt-abc123")
	require.NoError(t, err)
	assert.Equal(t, 5, updated.Priority)
}

func TestUpdate(t *testing.T) {
	s := setupTestStore(t)
	createTestTicket(s, "kt-atomic", "Atomic Test", ticket.StatusOpen)

	// Atomic update
	err := s.Update("kt-atomic", func(tk *ticket.Ticket) error {
		tk.Status = ticket.StatusClosed
		tk.TestsPassed = true
		return nil
	})
	require.NoError(t, err)

	// Verify
	updated, err := s.Get("kt-atomic")
	require.NoError(t, err)
	assert.Equal(t, ticket.StatusClosed, updated.Status)
	assert.True(t, updated.TestsPassed)
}

func TestUpdateError(t *testing.T) {
	s := setupTestStore(t)
	createTestTicket(s, "kt-err", "Error Test", ticket.StatusOpen)

	// Update that returns error
	err := s.Update("kt-err", func(tk *ticket.Ticket) error {
		tk.Status = ticket.StatusClosed
		return assert.AnError
	})
	require.Error(t, err)

	// Should not have saved
	unchanged, err := s.Get("kt-err")
	require.NoError(t, err)
	assert.Equal(t, ticket.StatusOpen, unchanged.Status)
}

func TestLockedTicketDoubleRelease(t *testing.T) {
	s := setupTestStore(t)
	createTestTicket(s, "kt-double", "Double Release", ticket.StatusOpen)

	lt, err := s.GetForUpdate("kt-double")
	require.NoError(t, err)

	// First release
	lt.Release()

	// Second release should be safe (no-op)
	lt.Release()

	// SaveAndRelease after Release should error
	err = lt.SaveAndRelease()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already released")
}

func TestConcurrentUpdates(t *testing.T) {
	s := setupTestStore(t)

	// Create ticket with priority 0
	tk := &ticket.Ticket{
		ID:       "kt-concurrent",
		Status:   ticket.StatusOpen,
		Created:  "2026-01-09T10:00:00Z",
		Type:     ticket.TypeTask,
		Priority: 0,
		Title:    "Concurrent Test",
	}
	require.NoError(t, s.Save(tk))

	// 10 goroutines each increment priority by 1
	const goroutines = 10
	var wg sync.WaitGroup

	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()

			err := s.Update("kt-concurrent", func(tk *ticket.Ticket) error {
				tk.Priority++
				return nil
			})
			require.NoError(t, err)
		}()
	}

	wg.Wait()

	// Final priority should be exactly 10 (no lost updates)
	final, err := s.Get("kt-concurrent")
	require.NoError(t, err)
	assert.Equal(t, goroutines, final.Priority)
}

func TestConcurrentGetForUpdate(t *testing.T) {
	s := setupTestStore(t)

	// Create ticket
	tk := &ticket.Ticket{
		ID:       "kt-gfu",
		Status:   ticket.StatusOpen,
		Created:  "2026-01-09T10:00:00Z",
		Type:     ticket.TypeTask,
		Priority: 0,
		Title:    "GetForUpdate Concurrent",
	}
	require.NoError(t, s.Save(tk))

	const goroutines = 5
	var wg sync.WaitGroup
	var mu sync.Mutex
	values := make([]int, 0, goroutines)

	for i := range goroutines {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()

			lt, err := s.GetForUpdate("kt-gfu")
			require.NoError(t, err)

			// Record what we read
			mu.Lock()
			values = append(values, lt.Ticket.Priority)
			mu.Unlock()

			// Increment and save
			lt.Ticket.Priority = val + 1
			err = lt.SaveAndRelease()
			require.NoError(t, err)
		}(i)
	}

	wg.Wait()

	// All should have completed
	assert.Len(t, values, goroutines)
}

func TestConcurrentReadWrite(t *testing.T) {
	s := setupTestStore(t)

	tk := &ticket.Ticket{
		ID:       "kt-rw",
		Status:   ticket.StatusOpen,
		Created:  "2026-01-09T10:00:00Z",
		Type:     ticket.TypeTask,
		Priority: 0,
		Title:    "Read Write Concurrent",
	}
	require.NoError(t, s.Save(tk))

	const readers = 5
	const writers = 3
	var wg sync.WaitGroup

	// Start readers
	for range readers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 10 {
				_, err := s.Get("kt-rw")
				require.NoError(t, err)
			}
		}()
	}

	// Start writers
	for range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 5 {
				err := s.Update("kt-rw", func(tk *ticket.Ticket) error {
					tk.Priority++
					return nil
				})
				require.NoError(t, err)
			}
		}()
	}

	wg.Wait()

	// Verify final state
	final, err := s.Get("kt-rw")
	require.NoError(t, err)
	assert.Equal(t, writers*5, final.Priority)
}
