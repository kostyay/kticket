package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kostyay/kticket/internal/store"
	"github.com/kostyay/kticket/internal/ticket"
	"github.com/spf13/cobra"
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

func TestSlicesContainsAndDelete(t *testing.T) {
	slice := []string{"a", "b", "c"}

	assert.True(t, slices.Contains(slice, "b"))
	assert.False(t, slices.Contains(slice, "d"))

	result := slices.DeleteFunc(slices.Clone(slice), func(s string) bool { return s == "b" })
	assert.Equal(t, []string{"a", "c"}, result)

	result = slices.DeleteFunc(slices.Clone(slice), func(s string) bool { return s == "x" })
	assert.Equal(t, []string{"a", "b", "c"}, result)
}

func TestRunList(t *testing.T) {
	defer setupTestEnv(t)()

	mkTicket(t, "kt-001", "Open Task", ticket.StatusOpen)
	mkTicket(t, "kt-002", "In Progress Task", ticket.StatusInProgress)
	mkTicket(t, "kt-003", "Closed Task", ticket.StatusClosed)

	// Test list all
	listStatus = ""
	err := runList(nil, nil)
	require.NoError(t, err)

	// Test filter by status
	listStatus = "open"
	err = runList(nil, nil)
	require.NoError(t, err)

	listStatus = "closed"
	err = runList(nil, nil)
	require.NoError(t, err)
}

func TestRunListJSON(t *testing.T) {
	defer setupTestEnv(t)()
	jsonFlag = true
	defer func() { jsonFlag = false }()

	mkTicket(t, "kt-001", "Task One", ticket.StatusOpen)

	listStatus = ""
	err := runList(nil, nil)
	require.NoError(t, err)
}

func TestRunStats(t *testing.T) {
	defer setupTestEnv(t)()

	mkTicket(t, "kt-001", "Open", ticket.StatusOpen)
	mkTicket(t, "kt-002", "Open2", ticket.StatusOpen)
	mkTicket(t, "kt-003", "InProgress", ticket.StatusInProgress)
	mkTicket(t, "kt-004", "Closed", ticket.StatusClosed)

	err := runStats(nil, nil)
	require.NoError(t, err)
}

func TestRunStatsJSON(t *testing.T) {
	defer setupTestEnv(t)()
	jsonFlag = true
	defer func() { jsonFlag = false }()

	mkTicket(t, "kt-001", "Task", ticket.StatusOpen)

	err := runStats(nil, nil)
	require.NoError(t, err)
}

func TestRunClosed(t *testing.T) {
	defer setupTestEnv(t)()

	mkTicket(t, "kt-001", "Open", ticket.StatusOpen)
	mkTicket(t, "kt-002", "Closed1", ticket.StatusClosed)
	mkTicket(t, "kt-003", "Closed2", ticket.StatusClosed)

	closedLimit = 20
	err := runClosed(nil, nil)
	require.NoError(t, err)
}

func TestRunClosedJSON(t *testing.T) {
	defer setupTestEnv(t)()
	jsonFlag = true
	defer func() { jsonFlag = false }()

	mkTicket(t, "kt-001", "Closed", ticket.StatusClosed)
	closedLimit = 1
	err := runClosed(nil, nil)
	require.NoError(t, err)
}

func TestRunQuery(t *testing.T) {
	defer setupTestEnv(t)()

	mkTicket(t, "kt-001", "Task", ticket.StatusOpen)

	err := runQuery(nil, nil)
	require.NoError(t, err)
}

func TestRunShow(t *testing.T) {
	defer setupTestEnv(t)()

	tk := mkTicket(t, "kt-001", "Show Test", ticket.StatusOpen)

	err := runShow(nil, []string{tk.ID})
	require.NoError(t, err)

	// Test multiple tickets
	tk2 := mkTicket(t, "kt-002", "Show Test 2", ticket.StatusInProgress)
	err = runShow(nil, []string{tk.ID, tk2.ID})
	require.NoError(t, err)
}

func TestRunShowJSON(t *testing.T) {
	defer setupTestEnv(t)()
	jsonFlag = true
	defer func() { jsonFlag = false }()

	tk := mkTicket(t, "kt-001", "Show JSON", ticket.StatusOpen)

	// Single ticket
	err := runShow(nil, []string{tk.ID})
	require.NoError(t, err)

	// Multiple tickets
	tk2 := mkTicket(t, "kt-002", "Show JSON 2", ticket.StatusOpen)
	err = runShow(nil, []string{tk.ID, tk2.ID})
	require.NoError(t, err)
}

func TestRunShowNotFound(t *testing.T) {
	defer setupTestEnv(t)()

	// Non-existent ticket - should not error but print error
	err := runShow(nil, []string{"kt-nonexistent"})
	require.NoError(t, err)
}

func TestPrintTicket(t *testing.T) {
	defer setupTestEnv(t)()

	// Full ticket with all fields
	tk := &ticket.Ticket{
		ID:                 "kt-full",
		Status:             ticket.StatusInProgress,
		Created:            "2026-01-09T10:00:00Z",
		Type:               ticket.TypeFeature,
		Priority:           1,
		Assignee:           "test-user",
		ExternalRef:        "gh-123",
		Parent:             "kt-parent",
		Deps:               []string{"kt-dep1", "kt-dep2"},
		Links:              []string{"kt-link1"},
		TestsPassed:        true,
		Title:              "Full Feature",
		Description:        "This is a description",
		Design:             "Design notes here",
		AcceptanceCriteria: "- AC1\n- AC2",
		Tests:              "- Test1\n- Test2",
		Notes:              "Some notes",
	}

	// Just run it to ensure no panic
	printTicket(tk)

	// Ticket with tests not passed
	tk.TestsPassed = false
	printTicket(tk)

	// Minimal ticket
	tk2 := &ticket.Ticket{
		ID:      "kt-min",
		Status:  ticket.StatusOpen,
		Created: "2026-01-09T10:00:00Z",
		Type:    ticket.TypeTask,
		Title:   "Minimal",
	}
	printTicket(tk2)
}

func TestRunDepAdd(t *testing.T) {
	defer setupTestEnv(t)()

	parent := mkTicket(t, "kt-parent", "Parent", ticket.StatusOpen)
	child := mkTicket(t, "kt-child", "Child", ticket.StatusOpen)

	err := runDepAdd(nil, []string{parent.ID, child.ID})
	require.NoError(t, err)

	updated, _ := Store.Get(parent.ID)
	assert.Contains(t, updated.Deps, child.ID)
}

func TestRunDepAddJSON(t *testing.T) {
	defer setupTestEnv(t)()
	jsonFlag = true
	defer func() { jsonFlag = false }()

	parent := mkTicket(t, "kt-parent", "Parent", ticket.StatusOpen)
	child := mkTicket(t, "kt-child", "Child", ticket.StatusOpen)

	err := runDepAdd(nil, []string{parent.ID, child.ID})
	require.NoError(t, err)
}

func TestRunDepAddDuplicate(t *testing.T) {
	defer setupTestEnv(t)()

	parent := mkTicket(t, "kt-parent", "Parent", ticket.StatusOpen)
	child := mkTicket(t, "kt-child", "Child", ticket.StatusOpen)

	// Add first time
	err := runDepAdd(nil, []string{parent.ID, child.ID})
	require.NoError(t, err)

	// Add again - should error
	err = runDepAdd(nil, []string{parent.ID, child.ID})
	require.Error(t, err)
}

func TestRunDepRm(t *testing.T) {
	defer setupTestEnv(t)()

	parent := mkTicket(t, "kt-parent", "Parent", ticket.StatusOpen)
	child := mkTicket(t, "kt-child", "Child", ticket.StatusOpen)

	// Add dep
	parent.Deps = []string{child.ID}
	require.NoError(t, Store.Save(parent))

	// Remove
	err := runDepRm(nil, []string{parent.ID, child.ID})
	require.NoError(t, err)

	updated, _ := Store.Get(parent.ID)
	assert.Empty(t, updated.Deps)
}

func TestRunDepRmJSON(t *testing.T) {
	defer setupTestEnv(t)()
	jsonFlag = true
	defer func() { jsonFlag = false }()

	parent := mkTicket(t, "kt-parent", "Parent", ticket.StatusOpen)
	child := mkTicket(t, "kt-child", "Child", ticket.StatusOpen)

	parent.Deps = []string{child.ID}
	require.NoError(t, Store.Save(parent))

	err := runDepRm(nil, []string{parent.ID, child.ID})
	require.NoError(t, err)
}

func TestRunDepRmNotExist(t *testing.T) {
	defer setupTestEnv(t)()

	parent := mkTicket(t, "kt-parent", "Parent", ticket.StatusOpen)
	child := mkTicket(t, "kt-child", "Child", ticket.StatusOpen)

	// Remove dep that doesn't exist
	err := runDepRm(nil, []string{parent.ID, child.ID})
	require.Error(t, err)
}

func TestRunDepTree(t *testing.T) {
	defer setupTestEnv(t)()

	c := mkTicket(t, "kt-c", "Task C", ticket.StatusClosed)
	b := mkTicket(t, "kt-b", "Task B", ticket.StatusInProgress)
	a := mkTicket(t, "kt-a", "Task A", ticket.StatusOpen)

	b.Deps = []string{c.ID}
	require.NoError(t, Store.Save(b))

	a.Deps = []string{b.ID}
	require.NoError(t, Store.Save(a))

	depTreeFull = false
	err := runDepTree(nil, []string{a.ID})
	require.NoError(t, err)
}

func TestRunDepTreeJSON(t *testing.T) {
	defer setupTestEnv(t)()
	jsonFlag = true
	defer func() { jsonFlag = false }()

	b := mkTicket(t, "kt-b", "Task B", ticket.StatusOpen)
	a := mkTicket(t, "kt-a", "Task A", ticket.StatusOpen)

	a.Deps = []string{b.ID}
	require.NoError(t, Store.Save(a))

	err := runDepTree(nil, []string{a.ID})
	require.NoError(t, err)
}

func TestRunDepTreeMissingDep(t *testing.T) {
	defer setupTestEnv(t)()

	a := mkTicket(t, "kt-a", "Task A", ticket.StatusOpen)
	a.Deps = []string{"kt-missing"}
	require.NoError(t, Store.Save(a))

	err := runDepTree(nil, []string{a.ID})
	require.NoError(t, err) // Should handle missing dep gracefully
}

func TestPrintDepTree(t *testing.T) {
	// Test tree printing with various structures
	root := &depTreeNode{
		ID:     "kt-root",
		Status: ticket.StatusOpen,
		Title:  "Root",
		Children: []*depTreeNode{
			{
				ID:     "kt-child1",
				Status: ticket.StatusInProgress,
				Title:  "Child 1",
				Children: []*depTreeNode{
					{ID: "kt-grandchild", Status: ticket.StatusClosed, Title: "Grandchild"},
				},
			},
			{
				ID:     "kt-child2",
				Status: ticket.StatusClosed,
				Title:  "Child 2",
			},
		},
	}

	// Just run to ensure no panic
	printDepTree(root, "", true)
}

func TestRunLinkAdd(t *testing.T) {
	defer setupTestEnv(t)()

	tk1 := mkTicket(t, "kt-link1", "Link One", ticket.StatusOpen)
	tk2 := mkTicket(t, "kt-link2", "Link Two", ticket.StatusOpen)

	err := runLinkAdd(nil, []string{tk1.ID, tk2.ID})
	require.NoError(t, err)

	u1, _ := Store.Get(tk1.ID)
	u2, _ := Store.Get(tk2.ID)
	assert.Contains(t, u1.Links, tk2.ID)
	assert.Contains(t, u2.Links, tk1.ID)
}

func TestRunLinkAddJSON(t *testing.T) {
	defer setupTestEnv(t)()
	jsonFlag = true
	defer func() { jsonFlag = false }()

	tk1 := mkTicket(t, "kt-link1", "Link One", ticket.StatusOpen)
	tk2 := mkTicket(t, "kt-link2", "Link Two", ticket.StatusOpen)

	err := runLinkAdd(nil, []string{tk1.ID, tk2.ID})
	require.NoError(t, err)
}

func TestRunLinkAddThreeWay(t *testing.T) {
	defer setupTestEnv(t)()

	tk1 := mkTicket(t, "kt-link1", "Link One", ticket.StatusOpen)
	tk2 := mkTicket(t, "kt-link2", "Link Two", ticket.StatusOpen)
	tk3 := mkTicket(t, "kt-link3", "Link Three", ticket.StatusOpen)

	err := runLinkAdd(nil, []string{tk1.ID, tk2.ID, tk3.ID})
	require.NoError(t, err)

	// All should be linked to each other
	u1, _ := Store.Get(tk1.ID)
	u2, _ := Store.Get(tk2.ID)
	u3, _ := Store.Get(tk3.ID)

	assert.Contains(t, u1.Links, tk2.ID)
	assert.Contains(t, u1.Links, tk3.ID)
	assert.Contains(t, u2.Links, tk1.ID)
	assert.Contains(t, u2.Links, tk3.ID)
	assert.Contains(t, u3.Links, tk1.ID)
	assert.Contains(t, u3.Links, tk2.ID)
}

func TestRunLinkRm(t *testing.T) {
	defer setupTestEnv(t)()

	tk1 := mkTicket(t, "kt-link1", "Link One", ticket.StatusOpen)
	tk2 := mkTicket(t, "kt-link2", "Link Two", ticket.StatusOpen)

	// Add links
	tk1.Links = []string{tk2.ID}
	tk2.Links = []string{tk1.ID}
	require.NoError(t, Store.Save(tk1))
	require.NoError(t, Store.Save(tk2))

	err := runLinkRm(nil, []string{tk1.ID, tk2.ID})
	require.NoError(t, err)

	u1, _ := Store.Get(tk1.ID)
	u2, _ := Store.Get(tk2.ID)
	assert.Empty(t, u1.Links)
	assert.Empty(t, u2.Links)
}

func TestRunLinkRmJSON(t *testing.T) {
	defer setupTestEnv(t)()
	jsonFlag = true
	defer func() { jsonFlag = false }()

	tk1 := mkTicket(t, "kt-link1", "Link One", ticket.StatusOpen)
	tk2 := mkTicket(t, "kt-link2", "Link Two", ticket.StatusOpen)

	tk1.Links = []string{tk2.ID}
	tk2.Links = []string{tk1.ID}
	require.NoError(t, Store.Save(tk1))
	require.NoError(t, Store.Save(tk2))

	err := runLinkRm(nil, []string{tk1.ID, tk2.ID})
	require.NoError(t, err)
}

func TestRunReady(t *testing.T) {
	defer setupTestEnv(t)()

	dep := mkTicket(t, "kt-dep", "Dep", ticket.StatusClosed)
	ready := mkTicket(t, "kt-ready", "Ready", ticket.StatusOpen)
	blocked := mkTicket(t, "kt-blocked", "Blocked", ticket.StatusOpen)

	ready.Deps = []string{dep.ID}
	blocked.Deps = []string{"kt-unresolved"}
	require.NoError(t, Store.Save(ready))
	require.NoError(t, Store.Save(blocked))

	err := runReady(nil, nil)
	require.NoError(t, err)
}

func TestRunReadyJSON(t *testing.T) {
	defer setupTestEnv(t)()
	jsonFlag = true
	defer func() { jsonFlag = false }()

	mkTicket(t, "kt-ready", "Ready", ticket.StatusOpen)

	err := runReady(nil, nil)
	require.NoError(t, err)
}

func TestRunBlocked(t *testing.T) {
	defer setupTestEnv(t)()

	dep := mkTicket(t, "kt-dep", "Dep", ticket.StatusOpen)
	blocked := mkTicket(t, "kt-blocked", "Blocked", ticket.StatusOpen)

	blocked.Deps = []string{dep.ID}
	require.NoError(t, Store.Save(blocked))

	err := runBlocked(nil, nil)
	require.NoError(t, err)
}

func TestRunBlockedJSON(t *testing.T) {
	defer setupTestEnv(t)()
	jsonFlag = true
	defer func() { jsonFlag = false }()

	dep := mkTicket(t, "kt-dep", "Dep", ticket.StatusOpen)
	blocked := mkTicket(t, "kt-blocked", "Blocked", ticket.StatusOpen)

	blocked.Deps = []string{dep.ID}
	require.NoError(t, Store.Save(blocked))

	err := runBlocked(nil, nil)
	require.NoError(t, err)
}

func TestRunStart(t *testing.T) {
	defer setupTestEnv(t)()

	tk := mkTicket(t, "kt-001", "Task", ticket.StatusOpen)

	err := runStart(nil, []string{tk.ID})
	require.NoError(t, err)

	updated, _ := Store.Get(tk.ID)
	assert.Equal(t, ticket.StatusInProgress, updated.Status)
}

func TestRunClose(t *testing.T) {
	defer setupTestEnv(t)()

	tk := mkTicket(t, "kt-001", "Task", ticket.StatusOpen)

	err := runClose(nil, []string{tk.ID})
	require.NoError(t, err)

	updated, _ := Store.Get(tk.ID)
	assert.Equal(t, ticket.StatusClosed, updated.Status)
}

func TestRunReopen(t *testing.T) {
	defer setupTestEnv(t)()

	tk := mkTicket(t, "kt-001", "Task", ticket.StatusClosed)

	err := runReopen(nil, []string{tk.ID})
	require.NoError(t, err)

	updated, _ := Store.Get(tk.ID)
	assert.Equal(t, ticket.StatusOpen, updated.Status)
}

func TestRunStatus(t *testing.T) {
	defer setupTestEnv(t)()

	tk := mkTicket(t, "kt-001", "Task", ticket.StatusOpen)

	err := runStatus(nil, []string{tk.ID, "in_progress"})
	require.NoError(t, err)

	updated, _ := Store.Get(tk.ID)
	assert.Equal(t, ticket.StatusInProgress, updated.Status)
}

func TestRunStatusJSON(t *testing.T) {
	defer setupTestEnv(t)()
	jsonFlag = true
	defer func() { jsonFlag = false }()

	tk := mkTicket(t, "kt-001", "Task", ticket.StatusOpen)

	err := runStatus(nil, []string{tk.ID, "closed"})
	require.NoError(t, err)
}

func TestRunPass(t *testing.T) {
	defer setupTestEnv(t)()

	tk := &ticket.Ticket{
		ID:          "kt-pass",
		Status:      ticket.StatusOpen,
		Created:     "2026-01-09T10:00:00Z",
		Type:        ticket.TypeFeature,
		Priority:    2,
		TestsPassed: false,
		Title:       "Feature with Tests",
		Tests:       "- TestOne",
	}
	require.NoError(t, Store.Save(tk))

	err := runPass(nil, []string{tk.ID})
	require.NoError(t, err)

	updated, _ := Store.Get(tk.ID)
	assert.True(t, updated.TestsPassed)
}

func TestRunPassJSON(t *testing.T) {
	defer setupTestEnv(t)()
	jsonFlag = true
	defer func() { jsonFlag = false }()

	tk := mkTicket(t, "kt-001", "Task", ticket.StatusOpen)

	err := runPass(nil, []string{tk.ID})
	require.NoError(t, err)
}

func TestRunPassMultiple(t *testing.T) {
	defer setupTestEnv(t)()

	tk1 := mkTicket(t, "kt-001", "Task 1", ticket.StatusOpen)
	tk2 := mkTicket(t, "kt-002", "Task 2", ticket.StatusOpen)

	err := runPass(nil, []string{tk1.ID, tk2.ID})
	require.NoError(t, err)

	u1, _ := Store.Get(tk1.ID)
	u2, _ := Store.Get(tk2.ID)
	assert.True(t, u1.TestsPassed)
	assert.True(t, u2.TestsPassed)
}

func TestRunPassNotFound(t *testing.T) {
	defer setupTestEnv(t)()

	// Should not error overall, but track error in result
	err := runPass(nil, []string{"kt-nonexistent"})
	require.NoError(t, err)
}

func TestRunCreate(t *testing.T) {
	defer setupTestEnv(t)()

	// Reset flags
	createDesc = "test description"
	createDesign = "test design"
	createAcceptance = "- AC1"
	createTests = "- Test1"
	createType = "feature"
	createPriority = 1
	createAssignee = "test-user"
	createExtRef = "gh-123"
	createParent = ""

	err := runCreate(nil, []string{"Test Create"})
	require.NoError(t, err)

	// Verify ticket was created
	tickets, _ := Store.List()
	assert.Len(t, tickets, 1)
	assert.Equal(t, "Test Create", tickets[0].Title)
	assert.Equal(t, "test description", tickets[0].Description)
	assert.Equal(t, ticket.TypeFeature, tickets[0].Type)
}

func TestRunCreateJSON(t *testing.T) {
	defer setupTestEnv(t)()
	jsonFlag = true
	defer func() { jsonFlag = false }()

	createDesc = ""
	createDesign = ""
	createAcceptance = ""
	createTests = ""
	createType = "task"
	createPriority = 2
	createAssignee = ""
	createExtRef = ""
	createParent = ""

	err := runCreate(nil, []string{"JSON Create"})
	require.NoError(t, err)
}

func TestRunCreateNoTitle(t *testing.T) {
	defer setupTestEnv(t)()

	err := runCreate(nil, []string{})
	require.Error(t, err)

	err = runCreate(nil, []string{""})
	require.Error(t, err)
}

func TestSetStatusMultipleErrors(t *testing.T) {
	defer setupTestEnv(t)()

	// Non-existent tickets
	err := setStatusMultiple([]string{"kt-none1", "kt-none2"}, ticket.StatusOpen, false)
	require.NoError(t, err) // No error, but errors tracked internally
}

func TestSetStatusMultipleJSON(t *testing.T) {
	defer setupTestEnv(t)()
	jsonFlag = true
	defer func() { jsonFlag = false }()

	tk := mkTicket(t, "kt-001", "Task", ticket.StatusOpen)

	err := setStatusMultiple([]string{tk.ID}, ticket.StatusInProgress, false)
	require.NoError(t, err)
}

func TestErrorf(t *testing.T) {
	// Just call to ensure no panic
	Errorf("test error: %s", "message")
}

func mockCmd() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	return cmd
}

func TestRunAddNote(t *testing.T) {
	defer setupTestEnv(t)()

	tk := mkTicket(t, "kt-001", "Task", ticket.StatusOpen)

	// Test with args (not stdin)
	err := runAddNote(mockCmd(), []string{tk.ID, "This is a note"})
	require.NoError(t, err)

	updated, _ := Store.Get(tk.ID)
	assert.Contains(t, updated.Notes, "This is a note")
}

func TestRunAddNoteJSON(t *testing.T) {
	defer setupTestEnv(t)()
	jsonFlag = true
	defer func() { jsonFlag = false }()

	tk := mkTicket(t, "kt-001", "Task", ticket.StatusOpen)

	err := runAddNote(mockCmd(), []string{tk.ID, "JSON note"})
	require.NoError(t, err)
}

func TestRunAddNoteEmpty(t *testing.T) {
	defer setupTestEnv(t)()

	tk := mkTicket(t, "kt-001", "Task", ticket.StatusOpen)

	err := runAddNote(mockCmd(), []string{tk.ID, ""})
	require.Error(t, err)
}

func TestRunAddNoteAppend(t *testing.T) {
	defer setupTestEnv(t)()

	tk := mkTicket(t, "kt-001", "Task", ticket.StatusOpen)
	tk.Notes = "Existing note"
	require.NoError(t, Store.Save(tk))

	err := runAddNote(mockCmd(), []string{tk.ID, "New note"})
	require.NoError(t, err)

	updated, _ := Store.Get(tk.ID)
	assert.Contains(t, updated.Notes, "Existing note")
	assert.Contains(t, updated.Notes, "New note")
}

func TestRunClosedWithLimit(t *testing.T) {
	defer setupTestEnv(t)()

	// Create more tickets than limit
	mkTicket(t, "kt-001", "Closed1", ticket.StatusClosed)
	mkTicket(t, "kt-002", "Closed2", ticket.StatusClosed)
	mkTicket(t, "kt-003", "Closed3", ticket.StatusClosed)

	closedLimit = 2
	err := runClosed(nil, nil)
	require.NoError(t, err)
}

func TestRunStatsText(t *testing.T) {
	defer setupTestEnv(t)()
	jsonFlag = false

	mkTicket(t, "kt-001", "Open", ticket.StatusOpen)
	mkTicket(t, "kt-002", "InProgress", ticket.StatusInProgress)
	mkTicket(t, "kt-003", "Closed", ticket.StatusClosed)

	err := runStats(nil, nil)
	require.NoError(t, err)
}

func TestRunStatusNotFound(t *testing.T) {
	defer setupTestEnv(t)()

	err := runStatus(nil, []string{"kt-nonexistent", "open"})
	require.Error(t, err)
}

func TestRunDepAddNotFound(t *testing.T) {
	defer setupTestEnv(t)()

	parent := mkTicket(t, "kt-parent", "Parent", ticket.StatusOpen)

	// Dep doesn't exist
	err := runDepAdd(nil, []string{parent.ID, "kt-nonexistent"})
	require.Error(t, err)
}

func TestRunDepRmNotFound(t *testing.T) {
	defer setupTestEnv(t)()

	parent := mkTicket(t, "kt-parent", "Parent", ticket.StatusOpen)

	// Dep doesn't exist
	err := runDepRm(nil, []string{parent.ID, "kt-nonexistent"})
	require.Error(t, err)
}

func TestRunLinkAddNotFound(t *testing.T) {
	defer setupTestEnv(t)()

	tk := mkTicket(t, "kt-001", "Task", ticket.StatusOpen)

	// Link to non-existent
	err := runLinkAdd(nil, []string{tk.ID, "kt-nonexistent"})
	require.Error(t, err)
}

func TestRunLinkRmNotFound(t *testing.T) {
	defer setupTestEnv(t)()

	tk := mkTicket(t, "kt-001", "Task", ticket.StatusOpen)

	// Remove link with non-existent
	err := runLinkRm(nil, []string{tk.ID, "kt-nonexistent"})
	require.Error(t, err)
}

func TestDepTreeNotFound(t *testing.T) {
	defer setupTestEnv(t)()

	err := runDepTree(nil, []string{"kt-nonexistent"})
	require.Error(t, err)
}

func TestBuildDepTreeFull(t *testing.T) {
	defer setupTestEnv(t)()

	// Create a diamond dependency
	d := mkTicket(t, "kt-d", "D", ticket.StatusClosed)
	b := mkTicket(t, "kt-b", "B", ticket.StatusOpen)
	c := mkTicket(t, "kt-c", "C", ticket.StatusOpen)
	a := mkTicket(t, "kt-a", "A", ticket.StatusOpen)

	b.Deps = []string{d.ID}
	c.Deps = []string{d.ID}
	a.Deps = []string{b.ID, c.ID}
	require.NoError(t, Store.Save(b))
	require.NoError(t, Store.Save(c))
	require.NoError(t, Store.Save(a))

	// Test with full=false (dedup)
	seen := make(map[string]bool)
	tree := buildDepTree(a, seen, false)
	assert.NotNil(t, tree)

	// Test with full=true (no dedup)
	seen = make(map[string]bool)
	tree = buildDepTree(a, seen, true)
	assert.NotNil(t, tree)
}

func TestRunShowNotFoundPartial(t *testing.T) {
	defer setupTestEnv(t)()

	tk := mkTicket(t, "kt-001", "Exists", ticket.StatusOpen)

	// Mix of existing and non-existing
	err := runShow(nil, []string{tk.ID, "kt-nonexistent"})
	require.NoError(t, err) // Should not error overall
}

func TestRunReadyExcludesClosed(t *testing.T) {
	defer setupTestEnv(t)()

	mkTicket(t, "kt-closed", "Closed", ticket.StatusClosed)
	mkTicket(t, "kt-open", "Open", ticket.StatusOpen)

	err := runReady(nil, nil)
	require.NoError(t, err)
}

func TestRunBlockedExcludesClosed(t *testing.T) {
	defer setupTestEnv(t)()

	closed := mkTicket(t, "kt-closed", "Closed", ticket.StatusClosed)
	closed.Deps = []string{"kt-dep"}
	require.NoError(t, Store.Save(closed))

	err := runBlocked(nil, nil)
	require.NoError(t, err)
}

func TestHasUnresolvedDepsNotFound(t *testing.T) {
	defer setupTestEnv(t)()

	tk := mkTicket(t, "kt-001", "Task", ticket.StatusOpen)
	tk.Deps = []string{"kt-missing"}
	require.NoError(t, Store.Save(tk))

	assert.True(t, hasUnresolvedDeps(tk))
}

func TestRunLinkAddAlreadyLinked(t *testing.T) {
	defer setupTestEnv(t)()

	tk1 := mkTicket(t, "kt-link1", "Link One", ticket.StatusOpen)
	tk2 := mkTicket(t, "kt-link2", "Link Two", ticket.StatusOpen)

	// Already linked
	tk1.Links = []string{tk2.ID}
	require.NoError(t, Store.Save(tk1))

	// Adding again should still work (idempotent)
	err := runLinkAdd(nil, []string{tk1.ID, tk2.ID})
	require.NoError(t, err)

	u1, _ := Store.Get(tk1.ID)
	// Should not have duplicates
	count := 0
	for _, l := range u1.Links {
		if l == tk2.ID {
			count++
		}
	}
	assert.Equal(t, 1, count)
}

func TestRunListTextOutput(t *testing.T) {
	defer setupTestEnv(t)()
	jsonFlag = false

	mkTicket(t, "kt-001", "Task One", ticket.StatusOpen)
	mkTicket(t, "kt-002", "Task Two", ticket.StatusInProgress)

	listStatus = ""
	err := runList(nil, nil)
	require.NoError(t, err)
}

func TestRunReadyText(t *testing.T) {
	defer setupTestEnv(t)()
	jsonFlag = false

	mkTicket(t, "kt-ready", "Ready Task", ticket.StatusOpen)

	err := runReady(nil, nil)
	require.NoError(t, err)
}

func TestRunBlockedText(t *testing.T) {
	defer setupTestEnv(t)()
	jsonFlag = false

	dep := mkTicket(t, "kt-dep", "Dep", ticket.StatusOpen)
	blocked := mkTicket(t, "kt-blocked", "Blocked", ticket.StatusInProgress)
	blocked.Deps = []string{dep.ID}
	require.NoError(t, Store.Save(blocked))

	err := runBlocked(nil, nil)
	require.NoError(t, err)
}

func TestRunClosedText(t *testing.T) {
	defer setupTestEnv(t)()
	jsonFlag = false

	mkTicket(t, "kt-001", "Closed Task", ticket.StatusClosed)
	closedLimit = 10
	err := runClosed(nil, nil)
	require.NoError(t, err)
}

func TestRunDepTreeText(t *testing.T) {
	defer setupTestEnv(t)()
	jsonFlag = false

	b := mkTicket(t, "kt-b", "Task B", ticket.StatusOpen)
	a := mkTicket(t, "kt-a", "Task A", ticket.StatusOpen)
	a.Deps = []string{b.ID}
	require.NoError(t, Store.Save(a))

	depTreeFull = false
	err := runDepTree(nil, []string{a.ID})
	require.NoError(t, err)
}

func TestRunShowText(t *testing.T) {
	defer setupTestEnv(t)()
	jsonFlag = false

	tk := mkTicket(t, "kt-001", "Show Text", ticket.StatusOpen)

	err := runShow(nil, []string{tk.ID})
	require.NoError(t, err)
}

func TestRunShowMultipleText(t *testing.T) {
	defer setupTestEnv(t)()
	jsonFlag = false

	tk1 := mkTicket(t, "kt-001", "Show 1", ticket.StatusOpen)
	tk2 := mkTicket(t, "kt-002", "Show 2", ticket.StatusOpen)

	err := runShow(nil, []string{tk1.ID, tk2.ID})
	require.NoError(t, err)
}

func TestRunStatusText(t *testing.T) {
	defer setupTestEnv(t)()
	jsonFlag = false

	tk := mkTicket(t, "kt-001", "Task", ticket.StatusOpen)

	err := runStatus(nil, []string{tk.ID, "in_progress"})
	require.NoError(t, err)
}

func TestRunPassText(t *testing.T) {
	defer setupTestEnv(t)()
	jsonFlag = false

	tk := mkTicket(t, "kt-001", "Task", ticket.StatusOpen)

	err := runPass(nil, []string{tk.ID})
	require.NoError(t, err)
}

func TestSetStatusMultipleText(t *testing.T) {
	defer setupTestEnv(t)()
	jsonFlag = false

	tk1 := mkTicket(t, "kt-001", "Task 1", ticket.StatusOpen)
	tk2 := mkTicket(t, "kt-002", "Task 2", ticket.StatusOpen)

	err := setStatusMultiple([]string{tk1.ID, tk2.ID}, ticket.StatusInProgress, false)
	require.NoError(t, err)
}

func TestRunDepAddText(t *testing.T) {
	defer setupTestEnv(t)()
	jsonFlag = false

	parent := mkTicket(t, "kt-parent", "Parent", ticket.StatusOpen)
	child := mkTicket(t, "kt-child", "Child", ticket.StatusOpen)

	err := runDepAdd(nil, []string{parent.ID, child.ID})
	require.NoError(t, err)
}

func TestRunDepRmText(t *testing.T) {
	defer setupTestEnv(t)()
	jsonFlag = false

	parent := mkTicket(t, "kt-parent", "Parent", ticket.StatusOpen)
	child := mkTicket(t, "kt-child", "Child", ticket.StatusOpen)
	parent.Deps = []string{child.ID}
	require.NoError(t, Store.Save(parent))

	err := runDepRm(nil, []string{parent.ID, child.ID})
	require.NoError(t, err)
}

func TestRunLinkAddText(t *testing.T) {
	defer setupTestEnv(t)()
	jsonFlag = false

	tk1 := mkTicket(t, "kt-link1", "Link One", ticket.StatusOpen)
	tk2 := mkTicket(t, "kt-link2", "Link Two", ticket.StatusOpen)

	err := runLinkAdd(nil, []string{tk1.ID, tk2.ID})
	require.NoError(t, err)
}

func TestRunLinkRmText(t *testing.T) {
	defer setupTestEnv(t)()
	jsonFlag = false

	tk1 := mkTicket(t, "kt-link1", "Link One", ticket.StatusOpen)
	tk2 := mkTicket(t, "kt-link2", "Link Two", ticket.StatusOpen)
	tk1.Links = []string{tk2.ID}
	tk2.Links = []string{tk1.ID}
	require.NoError(t, Store.Save(tk1))
	require.NoError(t, Store.Save(tk2))

	err := runLinkRm(nil, []string{tk1.ID, tk2.ID})
	require.NoError(t, err)
}

func TestRunCreateText(t *testing.T) {
	defer setupTestEnv(t)()
	jsonFlag = false

	createDesc = ""
	createDesign = ""
	createAcceptance = ""
	createTests = ""
	createType = "task"
	createPriority = 2
	createAssignee = ""
	createExtRef = ""
	createParent = ""

	err := runCreate(nil, []string{"Text Create"})
	require.NoError(t, err)
}

func TestRunAddNoteText(t *testing.T) {
	defer setupTestEnv(t)()
	jsonFlag = false

	tk := mkTicket(t, "kt-001", "Task", ticket.StatusOpen)

	err := runAddNote(mockCmd(), []string{tk.ID, "Text note"})
	require.NoError(t, err)
}

func TestRunAddNoteNotFound(t *testing.T) {
	defer setupTestEnv(t)()

	err := runAddNote(mockCmd(), []string{"kt-nonexistent", "note"})
	require.Error(t, err)
}

func TestRegisterKtPermission_FileNotExist(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/nonexistent.json"

	err := registerKtPermissionAt(path, false)
	require.NoError(t, err)

	// File should be created with permission
	result, err := os.ReadFile(path)
	require.NoError(t, err)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(result, &parsed))
	perms := parsed["permissions"].(map[string]any)
	allow := perms["allow"].([]any)
	assert.Contains(t, allow, "Bash(kt:*)")
}

func TestRegisterKtPermission_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/settings.json"
	require.NoError(t, os.WriteFile(path, []byte("not json"), 0644))

	err := registerKtPermissionAt(path, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse settings")
}

func TestRegisterKtPermission_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/.claude/settings.local.json"

	err := registerKtPermissionAt(path, false)
	require.NoError(t, err)

	// Directory and file should be created
	result, err := os.ReadFile(path)
	require.NoError(t, err)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(result, &parsed))
	perms := parsed["permissions"].(map[string]any)
	allow := perms["allow"].([]any)
	assert.Contains(t, allow, "Bash(kt:*)")
}

func TestRegisterKtPermission_NoPermissionsSection(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/settings.json"
	data := `{"other": "value"}`
	require.NoError(t, os.WriteFile(path, []byte(data), 0644))

	err := registerKtPermissionAt(path, false)
	require.NoError(t, err)

	// File should have permissions.allow created
	result, _ := os.ReadFile(path)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(result, &parsed))
	assert.Equal(t, "value", parsed["other"])
	perms := parsed["permissions"].(map[string]any)
	allow := perms["allow"].([]any)
	assert.Contains(t, allow, "Bash(kt:*)")
}

func TestRegisterKtPermission_NoAllowArray(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/settings.json"
	data := `{"permissions": {"deny": ["something"]}}`
	require.NoError(t, os.WriteFile(path, []byte(data), 0644))

	err := registerKtPermissionAt(path, false)
	require.NoError(t, err)

	// File should have allow array created
	result, _ := os.ReadFile(path)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(result, &parsed))
	perms := parsed["permissions"].(map[string]any)
	allow := perms["allow"].([]any)
	deny := perms["deny"].([]any)
	assert.Contains(t, allow, "Bash(kt:*)")
	assert.Contains(t, deny, "something")
}

func TestRegisterKtPermission_AlreadyExists(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/settings.json"
	data := `{"permissions": {"allow": ["Bash(kt:*)", "Other"]}}`
	require.NoError(t, os.WriteFile(path, []byte(data), 0644))

	err := registerKtPermissionAt(path, false)
	require.NoError(t, err) // Should skip if already exists

	// File should be unchanged (except formatting)
	result, _ := os.ReadFile(path)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(result, &parsed))
	perms := parsed["permissions"].(map[string]any)
	allow := perms["allow"].([]any)
	assert.Len(t, allow, 2)
}

func TestRegisterKtPermission_AddsPermission(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/settings.json"
	data := `{"permissions": {"allow": ["Other"]}}`
	require.NoError(t, os.WriteFile(path, []byte(data), 0644))

	err := registerKtPermissionAt(path, false)
	require.NoError(t, err)

	// File should have new permission
	result, _ := os.ReadFile(path)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(result, &parsed))
	perms := parsed["permissions"].(map[string]any)
	allow := perms["allow"].([]any)
	assert.Len(t, allow, 2)
	assert.Contains(t, allow, "Bash(kt:*)")
	assert.Contains(t, allow, "Other")
}

func TestRegisterKtPermission_EmptyAllowArray(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/settings.json"
	data := `{"permissions": {"allow": []}}`
	require.NoError(t, os.WriteFile(path, []byte(data), 0644))

	err := registerKtPermissionAt(path, false)
	require.NoError(t, err)

	// File should have new permission
	result, _ := os.ReadFile(path)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(result, &parsed))
	perms := parsed["permissions"].(map[string]any)
	allow := perms["allow"].([]any)
	assert.Len(t, allow, 1)
	assert.Equal(t, "Bash(kt:*)", allow[0])
}

func TestRegisterKtPermission_PreservesOtherSettings(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/settings.json"
	data := `{"mcpServers": {"test": {}}, "permissions": {"allow": [], "deny": ["Bad"]}, "other": 123}`
	require.NoError(t, os.WriteFile(path, []byte(data), 0644))

	err := registerKtPermissionAt(path, false)
	require.NoError(t, err)

	// Check all settings preserved
	result, _ := os.ReadFile(path)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(result, &parsed))

	assert.Contains(t, parsed, "mcpServers")
	assert.Contains(t, parsed, "other")
	assert.Equal(t, float64(123), parsed["other"])

	perms := parsed["permissions"].(map[string]any)
	deny := perms["deny"].([]any)
	assert.Contains(t, deny, "Bad")
}

func TestGetClaudeConfigDir_Default(t *testing.T) {
	// Unset env var
	os.Unsetenv("CLAUDE_CONFIG_DIR")

	dir := getClaudeConfigDir()
	home, _ := os.UserHomeDir()
	assert.Equal(t, filepath.Join(home, ".claude"), dir)
}

func TestGetClaudeConfigDir_EnvVar(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "/custom/path")

	dir := getClaudeConfigDir()
	assert.Equal(t, "/custom/path", dir)
}

func TestInstallSlashCommands_Project(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	err := installSlashCommands(false)
	require.NoError(t, err)

	// Check files created
	_, err = os.Stat(filepath.Join(dir, ".claude/commands/kt-create.md"))
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(dir, ".claude/commands/kt-run.md"))
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(dir, ".claude/commands/kt-run-all.md"))
	assert.NoError(t, err)

	// Check content
	content, _ := os.ReadFile(filepath.Join(dir, ".claude/commands/kt-create.md"))
	assert.Contains(t, string(content), "epic")
	assert.Contains(t, string(content), "kt create")
}

func TestInstallSlashCommands_Global(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", dir)

	err := installSlashCommands(true)
	require.NoError(t, err)

	// Check files created in custom config dir
	_, err = os.Stat(filepath.Join(dir, "commands/kt-create.md"))
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(dir, "commands/kt-run.md"))
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(dir, "commands/kt-run-all.md"))
	assert.NoError(t, err)
}

func TestWriteKtMd(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kt.md")

	err := writeKtMd(path)
	require.NoError(t, err)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(content), "kt - ticket tracker")
	assert.Contains(t, string(content), "kt create")
}

func TestPromptChoice_ValidInput(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("2\n"))
	choice := promptChoice(reader, "Pick one", []string{"A", "B", "C"})
	assert.Equal(t, 2, choice)
}

func TestPromptChoice_InvalidInput(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("invalid\n"))
	choice := promptChoice(reader, "Pick one", []string{"A", "B", "C"})
	assert.Equal(t, 3, choice) // Defaults to last (Skip)
}

func TestPromptChoice_OutOfRange(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("5\n"))
	choice := promptChoice(reader, "Pick one", []string{"A", "B", "C"})
	assert.Equal(t, 3, choice) // Defaults to last
}
