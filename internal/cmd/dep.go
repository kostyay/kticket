package cmd

import (
	"fmt"

	"github.com/kostyay/kticket/internal/ticket"
	"github.com/spf13/cobra"
)

var depCmd = &cobra.Command{
	Use:   "dep",
	Short: "Manage ticket dependencies",
}

var depAddCmd = &cobra.Command{
	Use:   "add <id> <dep-id>",
	Short: "Add dependency (id depends on dep-id)",
	Args:  cobra.ExactArgs(2),
	RunE:  runDepAdd,
}

var depRmCmd = &cobra.Command{
	Use:   "rm <id> <dep-id>",
	Short: "Remove dependency",
	Args:  cobra.ExactArgs(2),
	RunE:  runDepRm,
}

var depTreeCmd = &cobra.Command{
	Use:   "tree <id>",
	Short: "Show dependency tree",
	Args:  cobra.ExactArgs(1),
	RunE:  runDepTree,
}

var depTreeFull bool

func init() {
	depTreeCmd.Flags().BoolVar(&depTreeFull, "full", false, "Disable deduplication")

	depCmd.AddCommand(depAddCmd)
	depCmd.AddCommand(depRmCmd)
	depCmd.AddCommand(depTreeCmd)
	rootCmd.AddCommand(depCmd)
}

func runDepAdd(cmd *cobra.Command, args []string) error {
	t, err := Store.Resolve(args[0])
	if err != nil {
		return err
	}

	depTicket, err := Store.Resolve(args[1])
	if err != nil {
		return err
	}

	// Check if already exists
	for _, d := range t.Deps {
		if d == depTicket.ID {
			return fmt.Errorf("%s already depends on %s", t.ID, depTicket.ID)
		}
	}

	t.Deps = append(t.Deps, depTicket.ID)
	if err := Store.Save(t); err != nil {
		return err
	}

	if IsJSON() {
		return PrintJSON(t)
	}

	fmt.Printf("%s now depends on %s\n", t.ID, depTicket.ID)
	return nil
}

func runDepRm(cmd *cobra.Command, args []string) error {
	t, err := Store.Resolve(args[0])
	if err != nil {
		return err
	}

	depTicket, err := Store.Resolve(args[1])
	if err != nil {
		return err
	}

	// Find and remove
	found := false
	newDeps := make([]string, 0, len(t.Deps))
	for _, d := range t.Deps {
		if d == depTicket.ID {
			found = true
			continue
		}
		newDeps = append(newDeps, d)
	}

	if !found {
		return fmt.Errorf("%s does not depend on %s", t.ID, depTicket.ID)
	}

	t.Deps = newDeps
	if err := Store.Save(t); err != nil {
		return err
	}

	if IsJSON() {
		return PrintJSON(t)
	}

	fmt.Printf("%s no longer depends on %s\n", t.ID, depTicket.ID)
	return nil
}

type depTreeNode struct {
	ID       string         `json:"id"`
	Status   ticket.Status  `json:"status"`
	Title    string         `json:"title"`
	Children []*depTreeNode `json:"children,omitempty"`
}

func runDepTree(cmd *cobra.Command, args []string) error {
	t, err := Store.Resolve(args[0])
	if err != nil {
		return err
	}

	seen := make(map[string]bool)
	tree := buildDepTree(t, seen, depTreeFull)

	if IsJSON() {
		return PrintJSON(tree)
	}

	printDepTree(tree, "", true)
	return nil
}

func buildDepTree(t *ticket.Ticket, seen map[string]bool, full bool) *depTreeNode {
	node := &depTreeNode{
		ID:     t.ID,
		Status: t.Status,
		Title:  t.Title,
	}

	if !full && seen[t.ID] {
		return node
	}
	seen[t.ID] = true

	for _, depID := range t.Deps {
		dep, err := Store.Get(depID)
		if err != nil {
			// Dependency not found, add placeholder
			node.Children = append(node.Children, &depTreeNode{
				ID:     depID,
				Status: "unknown",
				Title:  "(not found)",
			})
			continue
		}
		node.Children = append(node.Children, buildDepTree(dep, seen, full))
	}

	return node
}

func printDepTree(node *depTreeNode, prefix string, isLast bool) {
	// Print current node
	connector := "├── "
	if isLast {
		connector = "└── "
	}
	if prefix == "" {
		// Root node
		fmt.Printf("%s [%s] %s\n", node.ID, node.Status, node.Title)
	} else {
		fmt.Printf("%s%s%s [%s] %s\n", prefix, connector, node.ID, node.Status, node.Title)
	}

	// Print children
	childPrefix := prefix
	if prefix != "" {
		if isLast {
			childPrefix += "    "
		} else {
			childPrefix += "│   "
		}
	}

	for i, child := range node.Children {
		isLastChild := i == len(node.Children)-1
		printDepTree(child, childPrefix, isLastChild)
	}
}

// Helper to check if a ticket has unresolved deps
func hasUnresolvedDeps(t *ticket.Ticket) bool {
	for _, depID := range t.Deps {
		dep, err := Store.Get(depID)
		if err != nil {
			return true // Can't find dep, consider unresolved
		}
		if dep.Status != ticket.StatusClosed {
			return true
		}
	}
	return false
}

// Helper to check if any dependencies exist and are all resolved
func allDepsResolved(t *ticket.Ticket) bool {
	if len(t.Deps) == 0 {
		return true
	}
	return !hasUnresolvedDeps(t)
}
