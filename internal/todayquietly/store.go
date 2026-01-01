package todayquietly

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// PreferenceStore provides append-only storage for preference records.
type PreferenceStore struct {
	mu      sync.RWMutex
	records []PreferenceRecord
	hashes  map[string]bool // For deduplication

	// filePath for append-only persistence (optional).
	filePath string

	// clock injection for determinism.
	clock func() time.Time
}

// StoreOption configures the PreferenceStore.
type StoreOption func(*PreferenceStore)

// WithStoreClock sets the clock function.
func WithStoreClock(clock func() time.Time) StoreOption {
	return func(s *PreferenceStore) {
		s.clock = clock
	}
}

// WithStoreFile sets the file path for persistence.
func WithStoreFile(path string) StoreOption {
	return func(s *PreferenceStore) {
		s.filePath = path
	}
}

// NewPreferenceStore creates a new preference store.
func NewPreferenceStore(opts ...StoreOption) *PreferenceStore {
	s := &PreferenceStore{
		records: make([]PreferenceRecord, 0),
		hashes:  make(map[string]bool),
		clock:   time.Now,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Record stores a preference.
// Returns (isNew, error) - isNew is true if this is a new unique record.
func (s *PreferenceStore) Record(mode, source string) (bool, error) {
	if mode != "quiet" && mode != "show_all" {
		return false, fmt.Errorf("invalid mode: must be 'quiet' or 'show_all'")
	}

	now := s.clock()

	// Compute canonical hash
	canonical := computeCanonicalString(mode, source, now)
	hash := computeHash(canonical)

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicate (same hash)
	if s.hashes[hash] {
		return false, nil // Already recorded
	}

	record := PreferenceRecord{
		Mode:       mode,
		RecordedAt: now,
		Hash:       hash,
		Source:     source,
	}

	s.records = append(s.records, record)
	s.hashes[hash] = true

	// Persist if file path configured
	if s.filePath != "" {
		if err := s.appendToFile(record); err != nil {
			return true, fmt.Errorf("failed to persist: %w", err)
		}
	}

	return true, nil
}

// Count returns the number of records.
func (s *PreferenceStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}

// Records returns a copy of all records.
func (s *PreferenceStore) Records() []PreferenceRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]PreferenceRecord, len(s.records))
	copy(result, s.records)
	return result
}

// LatestPreference returns the most recent preference mode.
// Returns "quiet" as default if no preferences recorded.
func (s *PreferenceStore) LatestPreference() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.records) == 0 {
		return "quiet" // Default
	}
	return s.records[len(s.records)-1].Mode
}

// appendToFile appends a record to the file.
func (s *PreferenceStore) appendToFile(record PreferenceRecord) error {
	// Ensure directory exists
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	f, err := os.OpenFile(s.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	// Append-only format: timestamp|mode|source|hash
	line := fmt.Sprintf("%s|%s|%s|%s\n",
		record.RecordedAt.Format(time.RFC3339),
		record.Mode,
		record.Source,
		record.Hash,
	)
	_, err = f.WriteString(line)
	return err
}

// computeCanonicalString creates the canonical pipe-delimited string.
func computeCanonicalString(mode, source string, t time.Time) string {
	return fmt.Sprintf("%s|%s|%s", mode, source, t.Format(time.RFC3339))
}

// computeHash computes SHA256 of a string.
func computeHash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// ConfirmationMessage returns the confirmation message for a given mode.
func ConfirmationMessage(mode string) string {
	switch mode {
	case "quiet":
		return "Noted. QuantumLife will stay quiet unless it truly matters."
	case "show_all":
		return "Noted. We'll show you everything — and help you silence it later."
	default:
		return "Preference recorded."
	}
}
