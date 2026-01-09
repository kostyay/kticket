package cmd

import (
	"fmt"

	"github.com/kostyay/kticket/internal/ticket"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start <id>...",
	Short: "Set status to in_progress",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runStart,
}

var closeCmd = &cobra.Command{
	Use:   "close <id>...",
	Short: "Set status to closed (validates tests_passed)",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runClose,
}

var reopenCmd = &cobra.Command{
	Use:   "reopen <id>...",
	Short: "Set status to open",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runReopen,
}

var statusCmd = &cobra.Command{
	Use:   "status <id> <status>",
	Short: "Set arbitrary status",
	Args:  cobra.ExactArgs(2),
	RunE:  runStatus,
}

var passCmd = &cobra.Command{
	Use:   "pass <id>...",
	Short: "Set tests_passed = true",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runPass,
}

func init() {
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(closeCmd)
	rootCmd.AddCommand(reopenCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(passCmd)
}

type statusResult struct {
	Updated []string      `json:"updated,omitempty"`
	Errors  []statusError `json:"errors,omitempty"`
}

type statusError struct {
	ID    string `json:"id"`
	Error string `json:"error"`
}

func runStart(cmd *cobra.Command, args []string) error {
	return setStatusMultiple(args, ticket.StatusInProgress, false)
}

func runClose(cmd *cobra.Command, args []string) error {
	return setStatusMultiple(args, ticket.StatusClosed, true)
}

func runReopen(cmd *cobra.Command, args []string) error {
	return setStatusMultiple(args, ticket.StatusOpen, false)
}

func runStatus(cmd *cobra.Command, args []string) error {
	lt, err := Store.ResolveForUpdate(args[0])
	if err != nil {
		return err
	}

	newStatus := ticket.Status(args[1])
	lt.Ticket.Status = newStatus

	if err := lt.SaveAndRelease(); err != nil {
		return err
	}

	if IsJSON() {
		return PrintJSON(lt.Ticket)
	}

	fmt.Printf("%s → %s\n", lt.Ticket.ID, lt.Ticket.Status)
	return nil
}

func runPass(cmd *cobra.Command, args []string) error {
	result := statusResult{}

	for _, id := range args {
		lt, err := Store.ResolveForUpdate(id)
		if err != nil {
			result.Errors = append(result.Errors, statusError{ID: id, Error: err.Error()})
			continue
		}

		lt.Ticket.TestsPassed = true
		if err := lt.SaveAndRelease(); err != nil {
			result.Errors = append(result.Errors, statusError{ID: lt.Ticket.ID, Error: err.Error()})
			continue
		}

		result.Updated = append(result.Updated, lt.Ticket.ID)
	}

	if IsJSON() {
		return PrintJSON(result)
	}

	for _, id := range result.Updated {
		fmt.Printf("%s tests passed ✓\n", id)
	}
	for _, e := range result.Errors {
		Errorf("%s: %s", e.ID, e.Error)
	}

	return nil
}

func setStatusMultiple(ids []string, status ticket.Status, validateClose bool) error {
	result := statusResult{}

	for _, id := range ids {
		lt, err := Store.ResolveForUpdate(id)
		if err != nil {
			result.Errors = append(result.Errors, statusError{ID: id, Error: err.Error()})
			continue
		}

		if validateClose && status == ticket.StatusClosed {
			if err := lt.Ticket.CanClose(); err != nil {
				lt.Release()
				result.Errors = append(result.Errors, statusError{ID: lt.Ticket.ID, Error: err.Error()})
				continue
			}
		}

		lt.Ticket.Status = status
		if err := lt.SaveAndRelease(); err != nil {
			result.Errors = append(result.Errors, statusError{ID: lt.Ticket.ID, Error: err.Error()})
			continue
		}

		result.Updated = append(result.Updated, lt.Ticket.ID)
	}

	if IsJSON() {
		return PrintJSON(result)
	}

	for _, id := range result.Updated {
		fmt.Printf("%s → %s\n", id, status)
	}
	for _, e := range result.Errors {
		Errorf("%s: %s", e.ID, e.Error)
	}

	return nil
}
