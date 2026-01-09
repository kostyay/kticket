package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kostyay/kticket/internal/filelock"
	"github.com/kostyay/kticket/internal/ticket"
)

const DefaultDir = ".tickets"

type Store struct {
	Dir string
}

// New creates a new Store with the given directory.
func New(dir string) *Store {
	if dir == "" {
		dir = DefaultDir
	}
	return &Store{Dir: dir}
}

// lockPath returns the lock file path for a ticket ID.
func (s *Store) lockPath(id string) string {
	return filepath.Join(s.Dir, ".locks", id+".lock")
}

// storeLockPath returns the store-wide lock file path.
func (s *Store) storeLockPath() string {
	return filepath.Join(s.Dir, ".locks", "store.lock")
}

// EnsureDir creates the tickets directory if it doesn't exist.
func (s *Store) EnsureDir() error {
	return os.MkdirAll(s.Dir, 0755)
}

// List returns all tickets in the store.
// Uses shared store lock to allow concurrent reads.
func (s *Store) List() ([]*ticket.Ticket, error) {
	lock, err := filelock.AcquireShared(s.storeLockPath())
	if err != nil {
		return nil, fmt.Errorf("acquire store lock: %w", err)
	}
	defer func() { _ = lock.Release() }()

	pattern := filepath.Join(s.Dir, "*.md")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	tickets := make([]*ticket.Ticket, 0, len(matches))
	for _, path := range matches {
		t, err := ticket.ParseFile(path)
		if err != nil {
			continue // skip invalid files
		}
		tickets = append(tickets, t)
	}

	// Sort by created date (newest first)
	sort.Slice(tickets, func(i, j int) bool {
		return tickets[i].Created > tickets[j].Created
	})

	return tickets, nil
}

// Get retrieves a ticket by exact ID.
// Uses shared lock to allow concurrent reads.
func (s *Store) Get(id string) (*ticket.Ticket, error) {
	lock, err := filelock.AcquireShared(s.lockPath(id))
	if err != nil {
		return nil, fmt.Errorf("acquire lock: %w", err)
	}
	defer func() { _ = lock.Release() }()

	path := filepath.Join(s.Dir, id+".md")
	return ticket.ParseFile(path)
}

// Resolve finds a ticket by partial ID match.
// Uses appropriate locking for safe concurrent access.
func (s *Store) Resolve(partial string) (*ticket.Ticket, error) {
	// Try exact match first (Get handles its own locking)
	if t, err := s.Get(partial); err == nil {
		return t, nil
	}

	// Use store lock for glob search
	storeLock, err := filelock.AcquireShared(s.storeLockPath())
	if err != nil {
		return nil, fmt.Errorf("acquire store lock: %w", err)
	}

	pattern := filepath.Join(s.Dir, "*"+partial+"*.md")
	matches, err := filepath.Glob(pattern)
	_ = storeLock.Release() // Release early, we have the matches
	if err != nil {
		return nil, err
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("ticket %q not found", partial)
	case 1:
		id := strings.TrimSuffix(filepath.Base(matches[0]), ".md")
		return s.Get(id) // Use Get for proper locking
	default:
		ids := make([]string, len(matches))
		for i, m := range matches {
			ids[i] = strings.TrimSuffix(filepath.Base(m), ".md")
		}
		return nil, fmt.Errorf("ambiguous ID %q matches multiple tickets: %v", partial, ids)
	}
}

// Save writes a ticket to disk.
// Uses exclusive lock to prevent concurrent modifications.
func (s *Store) Save(t *ticket.Ticket) error {
	if err := s.EnsureDir(); err != nil {
		return err
	}

	lock, err := filelock.Acquire(s.lockPath(t.ID))
	if err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	defer func() { _ = lock.Release() }()

	path := filepath.Join(s.Dir, t.ID+".md")
	return ticket.WriteFile(path, t)
}

// Delete removes a ticket from disk.
// Uses exclusive lock to prevent concurrent access.
func (s *Store) Delete(id string) error {
	lock, err := filelock.Acquire(s.lockPath(id))
	if err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	defer func() { _ = lock.Release() }()

	path := filepath.Join(s.Dir, id+".md")
	return os.Remove(path)
}

// Path returns the file path for a ticket ID.
func (s *Store) Path(id string) string {
	return filepath.Join(s.Dir, id+".md")
}

// LockedTicket holds a ticket with an exclusive lock.
// Must call Release() or SaveAndRelease() when done.
type LockedTicket struct {
	Ticket *ticket.Ticket
	store  *Store
	lock   *filelock.Lock
}

// Release releases the lock without saving changes.
func (lt *LockedTicket) Release() {
	if lt.lock != nil {
		_ = lt.lock.Release()
		lt.lock = nil
	}
}

// SaveAndRelease saves changes and releases the lock.
func (lt *LockedTicket) SaveAndRelease() error {
	if lt.lock == nil {
		return fmt.Errorf("lock already released")
	}
	defer lt.Release()

	path := lt.store.Path(lt.Ticket.ID)
	return ticket.WriteFile(path, lt.Ticket)
}

// GetForUpdate retrieves a ticket with an exclusive lock for modification.
// Caller must call Release() or SaveAndRelease() on the returned LockedTicket.
func (s *Store) GetForUpdate(id string) (*LockedTicket, error) {
	lock, err := filelock.Acquire(s.lockPath(id))
	if err != nil {
		return nil, fmt.Errorf("acquire lock: %w", err)
	}

	path := filepath.Join(s.Dir, id+".md")
	t, err := ticket.ParseFile(path)
	if err != nil {
		_ = lock.Release()
		return nil, err
	}

	return &LockedTicket{Ticket: t, store: s, lock: lock}, nil
}

// ResolveForUpdate finds and locks a ticket by partial ID for modification.
// Caller must call Release() or SaveAndRelease() on the returned LockedTicket.
func (s *Store) ResolveForUpdate(partial string) (*LockedTicket, error) {
	// First resolve without lock to find the ID
	t, err := s.Resolve(partial)
	if err != nil {
		return nil, err
	}

	// Now get with exclusive lock (re-read to ensure consistency)
	return s.GetForUpdate(t.ID)
}

// Update atomically modifies a ticket using the provided function.
// The function receives the ticket and can modify it; changes are saved automatically.
func (s *Store) Update(id string, fn func(*ticket.Ticket) error) error {
	lt, err := s.GetForUpdate(id)
	if err != nil {
		return err
	}
	defer lt.Release()

	if err := fn(lt.Ticket); err != nil {
		return err
	}

	path := s.Path(lt.Ticket.ID)
	return ticket.WriteFile(path, lt.Ticket)
}
