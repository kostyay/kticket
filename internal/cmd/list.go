package cmd

import (
	"fmt"
	"sort"

	"github.com/kostyay/kticket/internal/ticket"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:     "ls",
	Aliases: []string{"list"},
	Short:   "List tickets",
	RunE:    runList,
}

var (
	listStatus string
	listParent string
)

func init() {
	listCmd.Flags().StringVar(&listStatus, "status", "", "Filter by status (open|in_progress|closed)")
	listCmd.Flags().StringVar(&listParent, "parent", "", "Filter by parent ticket ID")
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	tickets, err := Store.List()
	if err != nil {
		return err
	}

	// Filter by parent if specified
	if listParent != "" {
		parent, err := Store.Resolve(listParent)
		if err != nil {
			return err
		}
		filtered := make([]*ticket.Ticket, 0)
		for _, t := range tickets {
			if t.Parent == parent.ID {
				filtered = append(filtered, t)
			}
		}
		tickets = filtered
	}

	// Filter by status if specified
	if listStatus != "" {
		filtered := make([]*ticket.Ticket, 0)
		for _, t := range tickets {
			if string(t.Status) == listStatus {
				filtered = append(filtered, t)
			}
		}
		tickets = filtered
	}

	if IsJSON() {
		return PrintJSON(tickets)
	}

	if IsPlain() {
		for _, t := range tickets {
			fmt.Printf("%s [%s] %s\n", t.ID, t.Status, t.Title)
		}
		return nil
	}

	for _, t := range tickets {
		fmt.Printf("%-12s [%-11s] %s\n", t.ID, t.Status, truncate(t.Title, 50))
	}

	return nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// Stats command
var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show ticket counts by status",
	RunE:  runStats,
}

func init() {
	rootCmd.AddCommand(statsCmd)
}

func runStats(cmd *cobra.Command, args []string) error {
	tickets, err := Store.List()
	if err != nil {
		return err
	}

	counts := map[string]int{
		"open":        0,
		"in_progress": 0,
		"closed":      0,
	}

	for _, t := range tickets {
		counts[string(t.Status)]++
	}

	total := len(tickets)

	if IsJSON() {
		result := map[string]int{
			"open":        counts["open"],
			"in_progress": counts["in_progress"],
			"closed":      counts["closed"],
			"total":       total,
		}
		return PrintJSON(result)
	}

	fmt.Printf("open:         %3d\n", counts["open"])
	fmt.Printf("in_progress:  %3d\n", counts["in_progress"])
	fmt.Printf("closed:       %3d\n", counts["closed"])
	fmt.Println("──────────────")
	fmt.Printf("total:        %3d\n", total)

	return nil
}

// Closed command - list recently closed tickets
var closedCmd = &cobra.Command{
	Use:   "closed",
	Short: "List recently closed tickets",
	RunE:  runClosed,
}

var closedLimit int

func init() {
	closedCmd.Flags().IntVar(&closedLimit, "limit", 20, "Maximum number of tickets to show")
	rootCmd.AddCommand(closedCmd)
}

func runClosed(cmd *cobra.Command, args []string) error {
	tickets, err := Store.List()
	if err != nil {
		return err
	}

	// Filter to closed only
	closed := make([]*ticket.Ticket, 0)
	for _, t := range tickets {
		if t.Status == ticket.StatusClosed {
			closed = append(closed, t)
		}
	}

	// Sort by created (most recent first) - already sorted by List()
	sort.Slice(closed, func(i, j int) bool {
		return closed[i].Created > closed[j].Created
	})

	// Apply limit
	if closedLimit > 0 && len(closed) > closedLimit {
		closed = closed[:closedLimit]
	}

	if IsJSON() {
		return PrintJSON(closed)
	}

	if IsPlain() {
		for _, t := range closed {
			fmt.Printf("%s [%s] %s\n", t.ID, t.Status, t.Title)
		}
		return nil
	}

	for _, t := range closed {
		fmt.Printf("%-12s %s\n", t.ID, truncate(t.Title, 60))
	}

	return nil
}
