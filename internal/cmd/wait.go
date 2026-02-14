package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/kostyay/kticket/internal/ticket"
	"github.com/spf13/cobra"
)

const (
	waitPollInterval      = 2 * time.Second
	waitHeartbeatInterval = 30 * time.Second
)

var waitCmd = &cobra.Command{
	Use:   "wait <id>",
	Short: "Block until a ticket is closed",
	Args:  cobra.ExactArgs(1),
	RunE:  runWait,
}

func init() {
	rootCmd.AddCommand(waitCmd)
}

func runWait(cmd *cobra.Command, args []string) error {
	return runWaitWithClock(cmd.Context(), args[0], time.NewTicker, time.NewTicker)
}

type tickerFactory func(d time.Duration) *time.Ticker

func runWaitWithClock(
	ctx context.Context,
	id string,
	pollFactory tickerFactory,
	heartbeatFactory tickerFactory,
) error {
	t, err := Store.Resolve(id)
	if err != nil {
		return err
	}

	resolvedID := t.ID

	if t.Status == ticket.StatusClosed {
		return printWaitResult(t)
	}

	poll := pollFactory(waitPollInterval)
	defer poll.Stop()
	heartbeat := heartbeatFactory(waitHeartbeatInterval)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-heartbeat.C:
			if !IsJSON() {
				fmt.Fprintln(os.Stderr, "waiting...")
			}
		case <-poll.C:
			t, err = Store.Get(resolvedID)
			if err != nil {
				return fmt.Errorf("read ticket %s: %w", resolvedID, err)
			}
			if t.Status == ticket.StatusClosed {
				return printWaitResult(t)
			}
		}
	}
}

func printWaitResult(t *ticket.Ticket) error {
	if IsJSON() {
		return PrintJSON(t)
	}
	fmt.Printf("%s → %s\n", t.ID, t.Status)
	return nil
}
