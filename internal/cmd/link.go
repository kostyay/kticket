package cmd

import (
	"fmt"

	"github.com/kostyay/kticket/internal/ticket"
	"github.com/spf13/cobra"
)

var linkCmd = &cobra.Command{
	Use:   "link",
	Short: "Manage ticket links",
}

var linkAddCmd = &cobra.Command{
	Use:   "add <id> <id> [id...]",
	Short: "Link tickets together (symmetric)",
	Args:  cobra.MinimumNArgs(2),
	RunE:  runLinkAdd,
}

var linkRmCmd = &cobra.Command{
	Use:   "rm <id> <target-id>",
	Short: "Remove link between tickets",
	Args:  cobra.ExactArgs(2),
	RunE:  runLinkRm,
}

func init() {
	linkCmd.AddCommand(linkAddCmd)
	linkCmd.AddCommand(linkRmCmd)
	rootCmd.AddCommand(linkCmd)
}

func runLinkAdd(cmd *cobra.Command, args []string) error {
	// Resolve all tickets first
	tickets := make([]*ticket.Ticket, 0, len(args))
	for _, id := range args {
		t, err := Store.Resolve(id)
		if err != nil {
			return err
		}
		tickets = append(tickets, t)
	}

	// Add symmetric links between all pairs
	for i, t1 := range tickets {
		for j, t2 := range tickets {
			if i == j {
				continue
			}
			// Add t2 to t1's links if not already there
			if !containsString(t1.Links, t2.ID) {
				t1.Links = append(t1.Links, t2.ID)
			}
		}
	}

	// Save all
	for _, t := range tickets {
		if err := Store.Save(t); err != nil {
			return err
		}
	}

	if IsJSON() {
		return PrintJSON(tickets)
	}

	ids := make([]string, len(tickets))
	for i, t := range tickets {
		ids[i] = t.ID
	}
	fmt.Printf("Linked: %v\n", ids)
	return nil
}

func runLinkRm(cmd *cobra.Command, args []string) error {
	t1, err := Store.Resolve(args[0])
	if err != nil {
		return err
	}

	t2, err := Store.Resolve(args[1])
	if err != nil {
		return err
	}

	// Remove from both directions
	t1.Links = removeString(t1.Links, t2.ID)
	t2.Links = removeString(t2.Links, t1.ID)

	if err := Store.Save(t1); err != nil {
		return err
	}
	if err := Store.Save(t2); err != nil {
		return err
	}

	if IsJSON() {
		return PrintJSON(map[string]any{
			"unlinked": []string{t1.ID, t2.ID},
		})
	}

	fmt.Printf("Unlinked %s and %s\n", t1.ID, t2.ID)
	return nil
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) []string {
	result := make([]string, 0, len(slice))
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}

// Ready command - list tickets with all deps resolved
var readyCmd = &cobra.Command{
	Use:   "ready",
	Short: "List open/in_progress with deps resolved",
	RunE:  runReady,
}

// Blocked command - list tickets with unresolved deps
var blockedCmd = &cobra.Command{
	Use:   "blocked",
	Short: "List open/in_progress with unresolved deps",
	RunE:  runBlocked,
}

func init() {
	rootCmd.AddCommand(readyCmd)
	rootCmd.AddCommand(blockedCmd)
}

func runReady(cmd *cobra.Command, args []string) error {
	tickets, err := Store.List()
	if err != nil {
		return err
	}

	ready := make([]*ticket.Ticket, 0)
	for _, t := range tickets {
		if t.Status == ticket.StatusClosed {
			continue
		}
		if allDepsResolved(t) {
			ready = append(ready, t)
		}
	}

	if IsJSON() {
		return PrintJSON(ready)
	}

	for _, t := range ready {
		fmt.Printf("%-12s [%-11s] %s\n", t.ID, t.Status, truncate(t.Title, 50))
	}

	return nil
}

func runBlocked(cmd *cobra.Command, args []string) error {
	tickets, err := Store.List()
	if err != nil {
		return err
	}

	blocked := make([]*ticket.Ticket, 0)
	for _, t := range tickets {
		if t.Status == ticket.StatusClosed {
			continue
		}
		if hasUnresolvedDeps(t) {
			blocked = append(blocked, t)
		}
	}

	if IsJSON() {
		return PrintJSON(blocked)
	}

	for _, t := range blocked {
		fmt.Printf("%-12s [%-11s] %s\n", t.ID, t.Status, truncate(t.Title, 50))
	}

	return nil
}
