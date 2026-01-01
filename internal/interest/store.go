// Package interest provides email interest capture for Phase 18.1.
// This is a simple append-only store with no automation, no spam, no urgency.
package interest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"time"
)

// Entry represents a single interest registration.
type Entry struct {
	// Email is the registered email address.
	Email string

	// EmailHash is the SHA-256 hash of the email (for deduplication without storing plaintext).
	EmailHash string

	// RegisteredAt is when the interest was registered.
	RegisteredAt time.Time

	// Source identifies where the registration came from.
	Source string
}

// Store provides append-only storage for interest registrations.
type Store struct {
	mu      sync.RWMutex
	entries []Entry
	hashes  map[string]bool // For deduplication

	// File path for append-only persistence (optional).
	filePath string

	// Clock injection for determinism.
	clock func() time.Time
}

// Option configures the Store.
type Option func(*Store)

// WithClock sets the clock function.
func WithClock(clock func() time.Time) Option {
	return func(s *Store) {
		s.clock = clock
	}
}

// WithFile sets the file path for append-only persistence.
func WithFile(path string) Option {
	return func(s *Store) {
		s.filePath = path
	}
}

// NewStore creates a new interest store.
func NewStore(opts ...Option) *Store {
	s := &Store{
		entries: make([]Entry, 0),
		hashes:  make(map[string]bool),
		clock:   time.Now,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Register records an interest registration.
// Returns (isNew, error) - isNew is true if this is a new registration.
func (s *Store) Register(email, source string) (bool, error) {
	if email == "" {
		return false, fmt.Errorf("email required")
	}

	// Hash email for privacy-preserving deduplication
	hash := hashEmail(email)

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicate
	if s.hashes[hash] {
		return false, nil // Already registered, no error
	}

	entry := Entry{
		Email:        email,
		EmailHash:    hash,
		RegisteredAt: s.clock(),
		Source:       source,
	}

	s.entries = append(s.entries, entry)
	s.hashes[hash] = true

	// Append to file if configured
	if s.filePath != "" {
		if err := s.appendToFile(entry); err != nil {
			return true, fmt.Errorf("failed to persist: %w", err)
		}
	}

	return true, nil
}

// Count returns the number of registrations.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

// Entries returns a copy of all entries.
func (s *Store) Entries() []Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Entry, len(s.entries))
	copy(result, s.entries)
	return result
}

// appendToFile appends an entry to the file.
func (s *Store) appendToFile(entry Entry) error {
	f, err := os.OpenFile(s.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	// Simple CSV-like format: timestamp,hash,source
	line := fmt.Sprintf("%s,%s,%s\n",
		entry.RegisteredAt.Format(time.RFC3339),
		entry.EmailHash,
		entry.Source,
	)
	_, err = f.WriteString(line)
	return err
}

// hashEmail returns a SHA-256 hash of the email.
func hashEmail(email string) string {
	h := sha256.Sum256([]byte(email))
	return hex.EncodeToString(h[:])
}
