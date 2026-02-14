package cmd

import (
	"context"
	"testing"
	"time"

	"github.com/kostyay/kticket/internal/ticket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fastTicker(d time.Duration) *time.Ticker {
	return time.NewTicker(1 * time.Millisecond)
}

func TestRunWait_AlreadyClosed(t *testing.T) {
	defer setupTestEnv(t)()

	tk := mkTicket(t, "kt-done", "Done", ticket.StatusClosed)

	err := runWaitWithClock(
		context.Background(), tk.ID,
		fastTicker, fastTicker,
	)
	require.NoError(t, err)
}

func TestRunWait_AlreadyClosedJSON(t *testing.T) {
	defer setupTestEnv(t)()
	jsonFlag = true
	defer func() { jsonFlag = false }()

	tk := mkTicket(t, "kt-done", "Done", ticket.StatusClosed)

	err := runWaitWithClock(
		context.Background(), tk.ID,
		fastTicker, fastTicker,
	)
	require.NoError(t, err)
}

func TestRunWait_BecomesClosedDuringPoll(t *testing.T) {
	defer setupTestEnv(t)()

	tk := mkTicket(t, "kt-wait", "Waiting", ticket.StatusInProgress)

	go func() {
		time.Sleep(50 * time.Millisecond)
		lt, err := Store.GetForUpdate(tk.ID)
		if err != nil {
			return
		}
		lt.Ticket.Status = ticket.StatusClosed
		_ = lt.SaveAndRelease()
	}()

	err := runWaitWithClock(
		context.Background(), tk.ID,
		fastTicker, fastTicker,
	)
	require.NoError(t, err)

	updated, _ := Store.Get(tk.ID)
	assert.Equal(t, ticket.StatusClosed, updated.Status)
}

func TestRunWait_NotFound(t *testing.T) {
	defer setupTestEnv(t)()

	err := runWaitWithClock(
		context.Background(), "kt-nonexistent",
		fastTicker, fastTicker,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRunWait_ContextCancelled(t *testing.T) {
	defer setupTestEnv(t)()

	tk := mkTicket(t, "kt-wait", "Waiting", ticket.StatusOpen)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := runWaitWithClock(
		ctx, tk.ID,
		fastTicker, fastTicker,
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestRunWait_TicketDeletedDuringPoll(t *testing.T) {
	defer setupTestEnv(t)()

	tk := mkTicket(t, "kt-del", "Deleted", ticket.StatusOpen)

	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = Store.Delete(tk.ID)
	}()

	err := runWaitWithClock(
		context.Background(), tk.ID,
		fastTicker, fastTicker,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read ticket")
}
