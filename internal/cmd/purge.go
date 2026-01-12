package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kostyay/kticket/internal/ticket"
	"github.com/spf13/cobra"
)

var purgeCmd = &cobra.Command{
	Use:   "purge",
	Short: "Delete all closed tickets",
	Long:  "Permanently delete all closed ticket files. Validates that no open tickets reference them.",
	RunE:  runPurge,
}

func init() {
	rootCmd.AddCommand(purgeCmd)
}

type purgeResult struct {
	Deleted int      `json:"deleted"`
	Errors  []string `json:"errors,omitempty"`
}

func runPurge(cmd *cobra.Command, args []string) error {
	// Get all tickets
	allTickets, err := Store.List()
	if err != nil {
		return fmt.Errorf("list tickets: %w", err)
	}

	// Filter closed tickets
	var closedTickets []*ticket.Ticket
	for _, t := range allTickets {
		if t.Status == ticket.StatusClosed {
			closedTickets = append(closedTickets, t)
		}
	}

	// Early exit if nothing to purge
	if len(closedTickets) == 0 {
		if IsJSON() {
			return PrintJSON(purgeResult{Deleted: 0})
		}
		fmt.Println("No closed tickets to purge")
		return nil
	}

	// Validate references
	if err := validatePurge(allTickets, closedTickets); err != nil {
		return err
	}

	// Interactive confirmation (skip in JSON mode)
	if IsJSON() {
		return fmt.Errorf("refusing to purge in JSON mode (interactive confirmation required)")
	}

	confirmed, err := promptConfirmation(closedTickets)
	if err != nil {
		return fmt.Errorf("prompt: %w", err)
	}

	if !confirmed {
		fmt.Println("Purge cancelled")
		return nil
	}

	// Delete files
	deleted := 0
	for _, t := range closedTickets {
		path := filepath.Join(Store.Dir, t.ID+".md")
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("delete %s: %w", t.ID, err)
		}
		deleted++
	}

	if IsJSON() {
		return PrintJSON(purgeResult{Deleted: deleted})
	}

	fmt.Printf("Purged %d tickets\n", deleted)
	return nil
}

// validatePurge checks if any non-closed tickets reference closed tickets
func validatePurge(allTickets, closedTickets []*ticket.Ticket) error {
	// Build set of closed ticket IDs for fast lookup
	closedSet := make(map[string]bool)
	for _, t := range closedTickets {
		closedSet[t.ID] = true
	}

	// Check all non-closed tickets for references
	for _, t := range allTickets {
		if t.Status == ticket.StatusClosed {
			continue
		}

		// Check parent reference
		if t.Parent != "" && closedSet[t.Parent] {
			return fmt.Errorf("cannot purge %s: ticket %s has it as parent", t.Parent, t.ID)
		}

		// Check dependencies
		for _, dep := range t.Deps {
			if closedSet[dep] {
				return fmt.Errorf("cannot purge %s: ticket %s depends on it", dep, t.ID)
			}
		}

		// Check links
		for _, link := range t.Links {
			if closedSet[link] {
				return fmt.Errorf("cannot purge %s: ticket %s links to it", link, t.ID)
			}
		}
	}

	return nil
}

// promptConfirmation shows tickets and asks for user confirmation
func promptConfirmation(tickets []*ticket.Ticket) (bool, error) {
	fmt.Printf("Found %d closed tickets:\n", len(tickets))
	for _, t := range tickets {
		fmt.Printf("  %s: %s\n", t.ID, t.Title)
	}
	fmt.Printf("\nPurge %d tickets? [y/N] ", len(tickets))

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}

	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes", nil
}
