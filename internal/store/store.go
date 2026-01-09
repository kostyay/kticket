package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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

// EnsureDir creates the tickets directory if it doesn't exist.
func (s *Store) EnsureDir() error {
	return os.MkdirAll(s.Dir, 0755)
}

// List returns all tickets in the store.
func (s *Store) List() ([]*ticket.Ticket, error) {
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
func (s *Store) Get(id string) (*ticket.Ticket, error) {
	path := filepath.Join(s.Dir, id+".md")
	return ticket.ParseFile(path)
}

// Resolve finds a ticket by partial ID match.
func (s *Store) Resolve(partial string) (*ticket.Ticket, error) {
	// Try exact match first
	if t, err := s.Get(partial); err == nil {
		return t, nil
	}

	// Find substring matches
	pattern := filepath.Join(s.Dir, "*"+partial+"*.md")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("ticket %q not found", partial)
	case 1:
		return ticket.ParseFile(matches[0])
	default:
		ids := make([]string, len(matches))
		for i, m := range matches {
			ids[i] = strings.TrimSuffix(filepath.Base(m), ".md")
		}
		return nil, fmt.Errorf("ambiguous ID %q matches multiple tickets: %v", partial, ids)
	}
}

// Save writes a ticket to disk.
func (s *Store) Save(t *ticket.Ticket) error {
	if err := s.EnsureDir(); err != nil {
		return err
	}
	path := filepath.Join(s.Dir, t.ID+".md")
	return ticket.WriteFile(path, t)
}

// Delete removes a ticket from disk.
func (s *Store) Delete(id string) error {
	path := filepath.Join(s.Dir, id+".md")
	return os.Remove(path)
}

// Path returns the file path for a ticket ID.
func (s *Store) Path(id string) string {
	return filepath.Join(s.Dir, id+".md")
}
