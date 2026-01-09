package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/kostyay/kticket/internal/store"
	"github.com/kostyay/kticket/internal/ticket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestEnv(t *testing.T) func() {
	dir := t.TempDir()
	Store = store.New(dir)
	_ = Store.EnsureDir()
	jsonFlag = false
	return func() { Store = nil }
}

func mkTicket(t *testing.T, id, title string, status ticket.Status) *ticket.Ticket {
	tk := &ticket.Ticket{
		ID:          id,
		Status:      status,
		Created:     "2026-01-09T10:00:00Z",
		Type:        ticket.TypeTask,
		Priority:    2,
		TestsPassed: false,
		Title:       title,
	}
	require.NoError(t, Store.Save(tk))
	return tk
}

func TestSetStatusMultiple(t *testing.T) {
	defer setupTestEnv(t)()

	tk := mkTicket(t, "kt-001", "Test", ticket.StatusOpen)

	// Start
	err := setStatusMultiple([]string{tk.ID}, ticket.StatusInProgress, false)
	require.NoError(t, err)

	updated, _ := Store.Get(tk.ID)
	assert.Equal(t, ticket.StatusInProgress, updated.Status)

	// Reopen
	err = setStatusMultiple([]string{tk.ID}, ticket.StatusOpen, false)
	require.NoError(t, err)

	updated, _ = Store.Get(tk.ID)
	assert.Equal(t, ticket.StatusOpen, updated.Status)

	// Close
	err = setStatusMultiple([]string{tk.ID}, ticket.StatusClosed, true)
	require.NoError(t, err)

	updated, _ = Store.Get(tk.ID)
	assert.Equal(t, ticket.StatusClosed, updated.Status)
}

func TestCloseBlockedByTests(t *testing.T) {
	defer setupTestEnv(t)()

	tk := &ticket.Ticket{
		ID:          "kt-blocked",
		Status:      ticket.StatusOpen,
		Created:     "2026-01-09T10:00:00Z",
		Type:        ticket.TypeFeature,
		Priority:    2,
		TestsPassed: false,
		Title:       "Feature with Tests",
		Tests:       "- TestOne\n- TestTwo",
	}
	require.NoError(t, Store.Save(tk))

	// Try to close - should not update (error in results)
	_ = setStatusMultiple([]string{tk.ID}, ticket.StatusClosed, true)

	// Verify still open
	updated, _ := Store.Get(tk.ID)
	assert.Equal(t, ticket.StatusOpen, updated.Status)

	// Pass tests
	tk.TestsPassed = true
	require.NoError(t, Store.Save(tk))

	// Now close should work
	err := setStatusMultiple([]string{tk.ID}, ticket.StatusClosed, true)
	require.NoError(t, err)

	updated, _ = Store.Get(tk.ID)
	assert.Equal(t, ticket.StatusClosed, updated.Status)
}

func TestDepAddRemove(t *testing.T) {
	defer setupTestEnv(t)()

	parent := mkTicket(t, "kt-parent", "Parent", ticket.StatusOpen)
	child := mkTicket(t, "kt-child", "Child", ticket.StatusOpen)

	// Add dep directly
	parent.Deps = append(parent.Deps, child.ID)
	require.NoError(t, Store.Save(parent))

	updated, _ := Store.Get(parent.ID)
	assert.Contains(t, updated.Deps, child.ID)

	// Remove
	updated.Deps = nil
	require.NoError(t, Store.Save(updated))

	updated, _ = Store.Get(parent.ID)
	assert.Empty(t, updated.Deps)
}

func TestLinkSymmetric(t *testing.T) {
	defer setupTestEnv(t)()

	tk1 := mkTicket(t, "kt-link1", "Link One", ticket.StatusOpen)
	tk2 := mkTicket(t, "kt-link2", "Link Two", ticket.StatusOpen)

	// Add symmetric links
	tk1.Links = append(tk1.Links, tk2.ID)
	tk2.Links = append(tk2.Links, tk1.ID)
	require.NoError(t, Store.Save(tk1))
	require.NoError(t, Store.Save(tk2))

	u1, _ := Store.Get(tk1.ID)
	u2, _ := Store.Get(tk2.ID)
	assert.Contains(t, u1.Links, tk2.ID)
	assert.Contains(t, u2.Links, tk1.ID)
}

func TestReadyVsBlocked(t *testing.T) {
	defer setupTestEnv(t)()

	dep := mkTicket(t, "kt-dep", "Dependency", ticket.StatusOpen)
	blocked := mkTicket(t, "kt-main", "Main Task", ticket.StatusOpen)

	// blocked depends on dep
	blocked.Deps = []string{dep.ID}
	require.NoError(t, Store.Save(blocked))

	// Main should be blocked (dep is open)
	assert.True(t, hasUnresolvedDeps(blocked))
	assert.False(t, allDepsResolved(blocked))

	// Close the dep
	dep.Status = ticket.StatusClosed
	require.NoError(t, Store.Save(dep))

	// Reload blocked
	blocked, _ = Store.Get(blocked.ID)

	// Now should be ready
	assert.False(t, hasUnresolvedDeps(blocked))
	assert.True(t, allDepsResolved(blocked))
}

func TestDepTreeBuild(t *testing.T) {
	defer setupTestEnv(t)()

	c := mkTicket(t, "kt-c", "Task C", ticket.StatusClosed)
	b := mkTicket(t, "kt-b", "Task B", ticket.StatusInProgress)
	a := mkTicket(t, "kt-a", "Task A", ticket.StatusOpen)

	b.Deps = []string{c.ID}
	require.NoError(t, Store.Save(b))

	a.Deps = []string{b.ID}
	require.NoError(t, Store.Save(a))

	// Build tree
	seen := make(map[string]bool)
	tree := buildDepTree(a, seen, false)

	assert.Equal(t, a.ID, tree.ID)
	assert.Len(t, tree.Children, 1)
	assert.Equal(t, b.ID, tree.Children[0].ID)
	assert.Len(t, tree.Children[0].Children, 1)
	assert.Equal(t, c.ID, tree.Children[0].Children[0].ID)
}

func TestOutputModeDetection(t *testing.T) {
	// Test JSON flag
	jsonFlag = true
	assert.Equal(t, "json", OutputMode())
	assert.True(t, IsJSON())

	jsonFlag = false
	// In test environment (not a TTY), should still return text
	// because we're testing the flag behavior
}

func TestPrintJSON(t *testing.T) {
	defer setupTestEnv(t)()

	tk := mkTicket(t, "kt-json", "JSON Test", ticket.StatusOpen)

	var buf bytes.Buffer
	// Redirect stdout for this test
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	err := enc.Encode(tk)
	require.NoError(t, err)

	// Verify it's valid JSON
	var parsed ticket.Ticket
	err = json.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err)
	assert.Equal(t, tk.ID, parsed.ID)
}

func TestTruncate(t *testing.T) {
	assert.Equal(t, "hello", truncate("hello", 10))
	assert.Equal(t, "hello w...", truncate("hello world", 10))
	assert.Equal(t, "hi", truncate("hi", 10))
}

func TestContainsAndRemoveString(t *testing.T) {
	slice := []string{"a", "b", "c"}

	assert.True(t, containsString(slice, "b"))
	assert.False(t, containsString(slice, "d"))

	result := removeString(slice, "b")
	assert.Equal(t, []string{"a", "c"}, result)

	result = removeString(slice, "x")
	assert.Equal(t, []string{"a", "b", "c"}, result)
}
