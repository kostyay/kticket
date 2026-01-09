package filelock

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

// DefaultTimeout is the default time to wait for a lock.
const DefaultTimeout = 5 * time.Second

// Lock represents an acquired file lock.
type Lock struct {
	flock  *flock.Flock
	shared bool
}

// Acquire obtains an exclusive lock on the given path.
// Blocks until lock is acquired or timeout (5s) expires.
func Acquire(path string) (*Lock, error) {
	return AcquireContext(context.Background(), path)
}

// AcquireContext obtains an exclusive lock with context support.
func AcquireContext(ctx context.Context, path string) (*Lock, error) {
	return acquire(ctx, path, false)
}

// AcquireShared obtains a shared (read) lock on the given path.
// Multiple readers can hold shared locks simultaneously.
func AcquireShared(path string) (*Lock, error) {
	return AcquireSharedContext(context.Background(), path)
}

// AcquireSharedContext obtains a shared lock with context support.
func AcquireSharedContext(ctx context.Context, path string) (*Lock, error) {
	return acquire(ctx, path, true)
}

func acquire(ctx context.Context, path string, shared bool) (*Lock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("create lock dir: %w", err)
	}

	fl := flock.New(path)

	// Use timeout context if none set
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, DefaultTimeout)
		defer cancel()
	}

	var locked bool
	var err error
	if shared {
		locked, err = fl.TryRLockContext(ctx, 100*time.Millisecond)
	} else {
		locked, err = fl.TryLockContext(ctx, 100*time.Millisecond)
	}

	if err != nil {
		return nil, fmt.Errorf("acquire lock: %w", err)
	}
	if !locked {
		return nil, fmt.Errorf("lock timeout on %s", path)
	}

	return &Lock{flock: fl, shared: shared}, nil
}

// TryAcquire attempts to obtain an exclusive lock without blocking.
// Returns nil, nil if lock is held by another process.
func TryAcquire(path string) (*Lock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("create lock dir: %w", err)
	}

	fl := flock.New(path)
	locked, err := fl.TryLock()
	if err != nil {
		return nil, fmt.Errorf("try lock: %w", err)
	}
	if !locked {
		return nil, nil
	}
	return &Lock{flock: fl, shared: false}, nil
}

// TryAcquireShared attempts to obtain a shared lock without blocking.
// Returns nil, nil if exclusive lock is held by another process.
func TryAcquireShared(path string) (*Lock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("create lock dir: %w", err)
	}

	fl := flock.New(path)
	locked, err := fl.TryRLock()
	if err != nil {
		return nil, fmt.Errorf("try lock: %w", err)
	}
	if !locked {
		return nil, nil
	}
	return &Lock{flock: fl, shared: true}, nil
}

// Release releases the lock.
func (l *Lock) Release() error {
	if l == nil || l.flock == nil {
		return nil
	}
	return l.flock.Unlock()
}

// Path returns the lock file path.
func (l *Lock) Path() string {
	if l == nil || l.flock == nil {
		return ""
	}
	return l.flock.Path()
}
