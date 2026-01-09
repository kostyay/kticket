package cmd

import (
	"fmt"
	"sort"

	"github.com/kostyay/kticket/internal/store"
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
	// Resolve all ticket IDs first (read-only) to get canonical IDs
	ids := make([]string, 0, len(args))
	for _, id := range args {
		t, err := Store.Resolve(id)
		if err != nil {
			return err
		}
		ids = append(ids, t.ID)
	}

	// Sort IDs to prevent deadlocks when locking multiple tickets
	sort.Strings(ids)

	// Lock all tickets in sorted order
	locked := make([]*store.LockedTicket, 0, len(ids))
	defer func() {
		for _, lt := range locked {
			lt.Release()
		}
	}()

	for _, id := range ids {
		lt, err := Store.GetForUpdate(id)
		if err != nil {
			return err
		}
		locked = append(locked, lt)
	}

	// Add symmetric links between all pairs
	for i, lt1 := range locked {
		for j, lt2 := range locked {
			if i == j {
				continue
			}
			// Add lt2 to lt1's links if not already there
			if !containsString(lt1.Ticket.Links, lt2.Ticket.ID) {
				lt1.Ticket.Links = append(lt1.Ticket.Links, lt2.Ticket.ID)
			}
		}
	}

	// Save all (keep locks until all saves complete)
	tickets := make([]*ticket.Ticket, 0, len(locked))
	for _, lt := range locked {
		if err := lt.SaveAndRelease(); err != nil {
			return err
		}
		tickets = append(tickets, lt.Ticket)
	}
	locked = nil // Already released

	if IsJSON() {
		return PrintJSON(tickets)
	}

	resultIDs := make([]string, len(tickets))
	for i, t := range tickets {
		resultIDs[i] = t.ID
	}
	fmt.Printf("Linked: %v\n", resultIDs)
	return nil
}

func runLinkRm(cmd *cobra.Command, args []string) error {
	// Resolve IDs first (read-only)
	t1, err := Store.Resolve(args[0])
	if err != nil {
		return err
	}
	t2, err := Store.Resolve(args[1])
	if err != nil {
		return err
	}

	// Sort IDs to prevent deadlocks
	ids := []string{t1.ID, t2.ID}
	sort.Strings(ids)

	// Lock both tickets in sorted order
	lt1, err := Store.GetForUpdate(ids[0])
	if err != nil {
		return err
	}
	lt2, err := Store.GetForUpdate(ids[1])
	if err != nil {
		lt1.Release()
		return err
	}

	// Remove from both directions
	lt1.Ticket.Links = removeString(lt1.Ticket.Links, lt2.Ticket.ID)
	lt2.Ticket.Links = removeString(lt2.Ticket.Links, lt1.Ticket.ID)

	if err := lt1.SaveAndRelease(); err != nil {
		lt2.Release()
		return err
	}
	if err := lt2.SaveAndRelease(); err != nil {
		return err
	}

	if IsJSON() {
		return PrintJSON(map[string]any{
			"unlinked": []string{ids[0], ids[1]},
		})
	}

	fmt.Printf("Unlinked %s and %s\n", ids[0], ids[1])
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
