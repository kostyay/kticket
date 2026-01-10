package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/kostyay/kticket/internal/ticket"
	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show <id>...",
	Short: "Display ticket(s)",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runShow,
}

var editCmd = &cobra.Command{
	Use:   "edit <id>",
	Short: "Open ticket in $EDITOR",
	Args:  cobra.ExactArgs(1),
	RunE:  runEdit,
}

var addNoteCmd = &cobra.Command{
	Use:   "add-note <id> [text]",
	Short: "Append timestamped note (or pipe stdin)",
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runAddNote,
}

func init() {
	rootCmd.AddCommand(showCmd)
	rootCmd.AddCommand(editCmd)
	rootCmd.AddCommand(addNoteCmd)
}

func runShow(cmd *cobra.Command, args []string) error {
	tickets := make([]*ticket.Ticket, 0, len(args))

	for _, id := range args {
		t, err := Store.Resolve(id)
		if err != nil {
			Errorf("%s", err)
			continue
		}
		tickets = append(tickets, t)
	}

	if IsJSON() {
		if len(tickets) == 1 {
			return PrintJSON(tickets[0])
		}
		return PrintJSON(tickets)
	}

	for i, t := range tickets {
		if i > 0 {
			fmt.Println()
		}
		printTicket(t)
	}

	return nil
}

func printTicket(t *ticket.Ticket) {
	fmt.Printf("%s [%s] %s\n", t.ID, t.Status, t.Title)
	fmt.Printf("Type: %s  Priority: %d  Assignee: %s\n", t.Type, t.Priority, t.Assignee)
	fmt.Printf("Created: %s\n", t.Created)

	if len(t.Deps) > 0 {
		fmt.Printf("Deps: %s\n", strings.Join(t.Deps, ", "))
	}
	if len(t.Links) > 0 {
		fmt.Printf("Links: %s\n", strings.Join(t.Links, ", "))
	}
	if t.ExternalRef != "" {
		fmt.Printf("External: %s\n", t.ExternalRef)
	}
	if t.Parent != "" {
		fmt.Printf("Parent: %s\n", t.Parent)
	}

	if t.Description != "" {
		fmt.Printf("\n%s\n", t.Description)
	}
	if t.Design != "" {
		fmt.Printf("\n## Design\n%s\n", t.Design)
	}
	if t.AcceptanceCriteria != "" {
		fmt.Printf("\n## Acceptance Criteria\n%s\n", t.AcceptanceCriteria)
	}
	if t.Tests != "" {
		fmt.Printf("\n## Tests\n%s\n", t.Tests)
		if t.TestsPassed {
			fmt.Println("✓ Tests passed")
		} else {
			fmt.Println("✗ Tests not passed")
		}
	}
	if t.Notes != "" {
		fmt.Printf("\n## Notes\n%s\n", t.Notes)
	}
}

func runEdit(cmd *cobra.Command, args []string) error {
	t, err := Store.Resolve(args[0])
	if err != nil {
		return err
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	path := Store.Path(t.ID)
	c := exec.Command(editor, path)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	return c.Run()
}

func runAddNote(cmd *cobra.Command, args []string) error {
	t, err := Store.Resolve(args[0])
	if err != nil {
		return err
	}

	var note string
	if len(args) > 1 {
		note = args[1]
	} else {
		// Read from stdin
		data, err := os.ReadFile(os.Stdin.Name())
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		note = strings.TrimSpace(string(data))
	}

	if note == "" {
		return fmt.Errorf("note text required")
	}

	// Add timestamp and append
	timestamp := time.Now().UTC().Format(time.RFC3339)
	if t.Notes != "" {
		t.Notes += "\n\n"
	}
	t.Notes += fmt.Sprintf("**%s**\n\n%s", timestamp, note)

	if err := Store.Save(t); err != nil {
		return err
	}

	if IsJSON() {
		return PrintJSON(t)
	}

	fmt.Printf("Note added to %s\n", t.ID)
	return nil
}
