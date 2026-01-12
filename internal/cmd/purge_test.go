package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kostyay/kticket/internal/ticket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPurgeBasic(t *testing.T) {
	defer setupTestEnv(t)()

	// Create 3 tickets, close 2
	open := mkTicket(t, "kt-001", "Open Task", ticket.StatusOpen)
	closed1 := mkTicket(t, "kt-002", "Closed Task 1", ticket.StatusClosed)
	closed2 := mkTicket(t, "kt-003", "Closed Task 2", ticket.StatusClosed)

	// Mock stdin with "y"
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	r, w, _ := os.Pipe()
	os.Stdin = r
	go func() {
		defer w.Close()
		w.Write([]byte("y\n"))
	}()

	err := runPurge(nil, nil)
	require.NoError(t, err)

	// Verify only 1 file remains (open ticket)
	files, _ := filepath.Glob(filepath.Join(Store.Dir, "*.md"))
	assert.Len(t, files, 1)

	// Verify open ticket still exists
	_, err = Store.Get(open.ID)
	assert.NoError(t, err)

	// Verify closed tickets are deleted
	_, err = Store.Get(closed1.ID)
	assert.Error(t, err)
	_, err = Store.Get(closed2.ID)
	assert.Error(t, err)
}

func TestPurgeNoClosedTickets(t *testing.T) {
	defer setupTestEnv(t)()

	mkTicket(t, "kt-001", "Open Task 1", ticket.StatusOpen)
	mkTicket(t, "kt-002", "Open Task 2", ticket.StatusOpen)

	err := runPurge(nil, nil)
	require.NoError(t, err)

	// Verify all tickets still exist
	files, _ := filepath.Glob(filepath.Join(Store.Dir, "*.md"))
	assert.Len(t, files, 2)
}

func TestPurgeBlockedByParent(t *testing.T) {
	defer setupTestEnv(t)()

	parent := mkTicket(t, "kt-parent", "Parent Epic", ticket.StatusClosed)
	child := mkTicket(t, "kt-child", "Child Task", ticket.StatusOpen)

	// Set parent reference
	child.Parent = parent.ID
	require.NoError(t, Store.Save(child))

	// Try to purge - should be blocked
	err := runPurge(nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot purge")
	assert.Contains(t, err.Error(), parent.ID)
	assert.Contains(t, err.Error(), "parent")

	// Verify parent still exists
	_, err = Store.Get(parent.ID)
	assert.NoError(t, err)
}

func TestPurgeBlockedByDep(t *testing.T) {
	defer setupTestEnv(t)()

	dep := mkTicket(t, "kt-dep", "Dependency", ticket.StatusClosed)
	task := mkTicket(t, "kt-task", "Task", ticket.StatusOpen)

	// Set dependency
	task.Deps = []string{dep.ID}
	require.NoError(t, Store.Save(task))

	// Try to purge - should be blocked
	err := runPurge(nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot purge")
	assert.Contains(t, err.Error(), dep.ID)
	assert.Contains(t, err.Error(), "depends")

	// Verify dep still exists
	_, err = Store.Get(dep.ID)
	assert.NoError(t, err)
}

func TestPurgeBlockedByLink(t *testing.T) {
	defer setupTestEnv(t)()

	linked := mkTicket(t, "kt-linked", "Linked", ticket.StatusClosed)
	task := mkTicket(t, "kt-task", "Task", ticket.StatusOpen)

	// Set link
	task.Links = []string{linked.ID}
	require.NoError(t, Store.Save(task))

	// Try to purge - should be blocked
	err := runPurge(nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot purge")
	assert.Contains(t, err.Error(), linked.ID)
	assert.Contains(t, err.Error(), "links")

	// Verify linked ticket still exists
	_, err = Store.Get(linked.ID)
	assert.NoError(t, err)
}

func TestPurgeUserCancels(t *testing.T) {
	defer setupTestEnv(t)()

	closed := mkTicket(t, "kt-001", "Closed Task", ticket.StatusClosed)

	// Mock stdin with "n"
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	r, w, _ := os.Pipe()
	os.Stdin = r
	go func() {
		defer w.Close()
		w.Write([]byte("n\n"))
	}()

	err := runPurge(nil, nil)
	require.NoError(t, err)

	// Verify ticket still exists
	_, err = Store.Get(closed.ID)
	assert.NoError(t, err)

	files, _ := filepath.Glob(filepath.Join(Store.Dir, "*.md"))
	assert.Len(t, files, 1)
}

func TestPurgeJSONMode(t *testing.T) {
	defer setupTestEnv(t)()
	jsonFlag = true
	defer func() { jsonFlag = false }()

	mkTicket(t, "kt-001", "Closed Task", ticket.StatusClosed)

	// JSON mode should error (requires interactive confirmation)
	err := runPurge(nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "refusing to purge in JSON mode")

	// Verify ticket still exists
	files, _ := filepath.Glob(filepath.Join(Store.Dir, "*.md"))
	assert.Len(t, files, 1)
}

func TestPurgeJSONModeNoClosed(t *testing.T) {
	defer setupTestEnv(t)()
	jsonFlag = true
	defer func() { jsonFlag = false }()

	mkTicket(t, "kt-001", "Open Task", ticket.StatusOpen)

	// Should succeed with no closed tickets
	err := runPurge(nil, nil)
	require.NoError(t, err)
}

func TestValidatePurge(t *testing.T) {
	defer setupTestEnv(t)()

	// Create tickets
	closed1 := mkTicket(t, "kt-closed1", "Closed 1", ticket.StatusClosed)
	closed2 := mkTicket(t, "kt-closed2", "Closed 2", ticket.StatusClosed)
	open1 := mkTicket(t, "kt-open1", "Open 1", ticket.StatusOpen)
	open2 := mkTicket(t, "kt-open2", "Open 2", ticket.StatusOpen)

	allTickets := []*ticket.Ticket{closed1, closed2, open1, open2}
	closedTickets := []*ticket.Ticket{closed1, closed2}

	// Should pass - no references
	err := validatePurge(allTickets, closedTickets)
	assert.NoError(t, err)

	// Add parent reference - should fail
	open1.Parent = closed1.ID
	err = validatePurge(allTickets, closedTickets)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), closed1.ID)
	assert.Contains(t, err.Error(), "parent")

	// Remove parent, add dep - should fail
	open1.Parent = ""
	open1.Deps = []string{closed2.ID}
	err = validatePurge(allTickets, closedTickets)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), closed2.ID)
	assert.Contains(t, err.Error(), "depends")

	// Remove dep, add link - should fail
	open1.Deps = nil
	open2.Links = []string{closed1.ID}
	err = validatePurge(allTickets, closedTickets)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), closed1.ID)
	assert.Contains(t, err.Error(), "links")
}

func TestPromptConfirmation(t *testing.T) {
	closed1 := &ticket.Ticket{
		ID:     "kt-001",
		Status: ticket.StatusClosed,
		Title:  "First Closed",
	}
	closed2 := &ticket.Ticket{
		ID:     "kt-002",
		Status: ticket.StatusClosed,
		Title:  "Second Closed",
	}

	tickets := []*ticket.Ticket{closed1, closed2}

	// Test "y" input
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	r, w, _ := os.Pipe()
	os.Stdin = r
	go func() {
		defer w.Close()
		w.Write([]byte("y\n"))
	}()

	confirmed, err := promptConfirmation(tickets)
	require.NoError(t, err)
	assert.True(t, confirmed)

	// Test "yes" input
	r, w, _ = os.Pipe()
	os.Stdin = r
	go func() {
		defer w.Close()
		w.Write([]byte("yes\n"))
	}()

	confirmed, err = promptConfirmation(tickets)
	require.NoError(t, err)
	assert.True(t, confirmed)

	// Test "n" input
	r, w, _ = os.Pipe()
	os.Stdin = r
	go func() {
		defer w.Close()
		w.Write([]byte("n\n"))
	}()

	confirmed, err = promptConfirmation(tickets)
	require.NoError(t, err)
	assert.False(t, confirmed)

	// Test "no" input
	r, w, _ = os.Pipe()
	os.Stdin = r
	go func() {
		defer w.Close()
		w.Write([]byte("no\n"))
	}()

	confirmed, err = promptConfirmation(tickets)
	require.NoError(t, err)
	assert.False(t, confirmed)

	// Test empty input (default to no)
	r, w, _ = os.Pipe()
	os.Stdin = r
	go func() {
		defer w.Close()
		w.Write([]byte("\n"))
	}()

	confirmed, err = promptConfirmation(tickets)
	require.NoError(t, err)
	assert.False(t, confirmed)
}

func TestPurgeMultipleReferences(t *testing.T) {
	defer setupTestEnv(t)()

	// Create closed ticket referenced by multiple open tickets
	closed := mkTicket(t, "kt-closed", "Closed", ticket.StatusClosed)
	open1 := mkTicket(t, "kt-open1", "Open 1", ticket.StatusOpen)
	open2 := mkTicket(t, "kt-open2", "Open 2", ticket.StatusOpen)

	// Multiple references
	open1.Deps = []string{closed.ID}
	open2.Links = []string{closed.ID}
	require.NoError(t, Store.Save(open1))
	require.NoError(t, Store.Save(open2))

	// Should be blocked
	err := runPurge(nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot purge")
}

func TestPurgeOnlyClosedReferences(t *testing.T) {
	defer setupTestEnv(t)()

	// Closed tickets can reference each other
	closed1 := mkTicket(t, "kt-closed1", "Closed 1", ticket.StatusClosed)
	closed2 := mkTicket(t, "kt-closed2", "Closed 2", ticket.StatusClosed)

	closed1.Deps = []string{closed2.ID}
	closed2.Links = []string{closed1.ID}
	require.NoError(t, Store.Save(closed1))
	require.NoError(t, Store.Save(closed2))

	// Mock stdin with "y"
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	r, w, _ := os.Pipe()
	os.Stdin = r
	go func() {
		defer w.Close()
		w.Write([]byte("y\n"))
	}()

	// Should succeed - closed tickets can reference each other
	err := runPurge(nil, nil)
	require.NoError(t, err)

	// Both should be deleted
	files, _ := filepath.Glob(filepath.Join(Store.Dir, "*.md"))
	assert.Len(t, files, 0)
}

func TestPurgeInProgressReferences(t *testing.T) {
	defer setupTestEnv(t)()

	// In-progress ticket references closed ticket
	closed := mkTicket(t, "kt-closed", "Closed", ticket.StatusClosed)
	inProgress := mkTicket(t, "kt-progress", "In Progress", ticket.StatusInProgress)

	inProgress.Deps = []string{closed.ID}
	require.NoError(t, Store.Save(inProgress))

	// Should be blocked
	err := runPurge(nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot purge")
}
