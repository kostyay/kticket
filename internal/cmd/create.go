package cmd

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/kostyay/kticket/internal/store"
	"github.com/kostyay/kticket/internal/ticket"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create [title]",
	Short: "Create a new ticket",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runCreate,
}

var (
	createDesc       string
	createDesign     string
	createAcceptance string
	createTests      string
	createType       string
	createPriority   int
	createAssignee   string
	createExtRef     string
	createParent     string
)

func init() {
	createCmd.Flags().StringVarP(&createDesc, "description", "d", "", "Description text")
	createCmd.Flags().StringVar(&createDesign, "design", "", "Design notes")
	createCmd.Flags().StringVar(&createAcceptance, "acceptance", "", "Acceptance criteria")
	createCmd.Flags().StringVar(&createTests, "tests", "", "Test requirements")
	createCmd.Flags().StringVarP(&createType, "type", "t", "task", "Type (bug|feature|task|epic|chore)")
	createCmd.Flags().IntVarP(&createPriority, "priority", "p", 2, "Priority 0-4, 0=highest")
	createCmd.Flags().StringVarP(&createAssignee, "assignee", "a", "", "Assignee (default: git user.name)")
	createCmd.Flags().StringVar(&createExtRef, "external-ref", "", "External reference (e.g., gh-123)")
	createCmd.Flags().StringVar(&createParent, "parent", "", "Parent ticket ID")

	rootCmd.AddCommand(createCmd)
}

func runCreate(cmd *cobra.Command, args []string) error {
	var title string
	if len(args) > 0 {
		title = args[0]
	}
	if title == "" {
		return fmt.Errorf("title is required")
	}

	id, err := store.GenerateID()
	if err != nil {
		return fmt.Errorf("generate ID: %w", err)
	}

	assignee := createAssignee
	if assignee == "" {
		assignee = getGitUser()
	}

	t := &ticket.Ticket{
		ID:                 id,
		Status:             ticket.StatusOpen,
		Created:            time.Now().UTC().Format(time.RFC3339),
		Type:               ticket.Type(createType),
		Priority:           createPriority,
		Assignee:           assignee,
		ExternalRef:        createExtRef,
		Parent:             createParent,
		TestsPassed:        false,
		Title:              title,
		Description:        createDesc,
		Design:             createDesign,
		AcceptanceCriteria: createAcceptance,
		Tests:              createTests,
	}

	if err := Store.Save(t); err != nil {
		return fmt.Errorf("save ticket: %w", err)
	}

	if IsJSON() {
		return PrintJSON(t)
	}

	fmt.Println(id)
	return nil
}

func getGitUser() string {
	out, err := exec.Command("git", "config", "user.name").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
