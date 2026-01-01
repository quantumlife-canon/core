package held

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SummaryStore provides append-only storage for summary records.
// CRITICAL: Only stores hashes, never raw data.
type SummaryStore struct {
	mu      sync.RWMutex
	records []SummaryRecord

	// filePath for append-only persistence (optional).
	filePath string

	// clock injection for determinism.
	clock func() time.Time

	// maxRecords limits growth (oldest are dropped when exceeded).
	maxRecords int
}

// StoreOption configures the SummaryStore.
type StoreOption func(*SummaryStore)

// WithStoreClock sets the clock function.
func WithStoreClock(clock func() time.Time) StoreOption {
	return func(s *SummaryStore) {
		s.clock = clock
	}
}

// WithStoreFile sets the file path for persistence.
func WithStoreFile(path string) StoreOption {
	return func(s *SummaryStore) {
		s.filePath = path
	}
}

// WithMaxRecords sets the maximum number of records to retain.
func WithMaxRecords(max int) StoreOption {
	return func(s *SummaryStore) {
		s.maxRecords = max
	}
}

// NewSummaryStore creates a new summary store.
func NewSummaryStore(opts ...StoreOption) *SummaryStore {
	s := &SummaryStore{
		records:    make([]SummaryRecord, 0),
		clock:      time.Now,
		maxRecords: 100, // Default limit to prevent unbounded growth
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Record stores a summary hash.
// CRITICAL: Only the hash is stored, never the summary content.
func (s *SummaryStore) Record(summary HeldSummary) error {
	now := s.clock()

	record := SummaryRecord{
		Hash:       summary.Hash,
		CircleID:   "", // Set by caller if needed
		RecordedAt: now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Append record
	s.records = append(s.records, record)

	// Enforce max records (drop oldest)
	if len(s.records) > s.maxRecords {
		s.records = s.records[len(s.records)-s.maxRecords:]
	}

	// Persist if file path configured
	if s.filePath != "" {
		if err := s.appendToFile(record); err != nil {
			return fmt.Errorf("failed to persist: %w", err)
		}
	}

	return nil
}

// RecordWithCircle stores a summary hash with circle context.
func (s *SummaryStore) RecordWithCircle(summary HeldSummary, circleID string) error {
	now := s.clock()

	record := SummaryRecord{
		Hash:       summary.Hash,
		CircleID:   circleID,
		RecordedAt: now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.records = append(s.records, record)

	// Enforce max records
	if len(s.records) > s.maxRecords {
		s.records = s.records[len(s.records)-s.maxRecords:]
	}

	if s.filePath != "" {
		if err := s.appendToFile(record); err != nil {
			return fmt.Errorf("failed to persist: %w", err)
		}
	}

	return nil
}

// Count returns the number of records.
func (s *SummaryStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}

// Records returns a copy of all records.
func (s *SummaryStore) Records() []SummaryRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]SummaryRecord, len(s.records))
	copy(result, s.records)
	return result
}

// LatestHash returns the most recent summary hash.
// Returns empty string if no records exist.
func (s *SummaryStore) LatestHash() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.records) == 0 {
		return ""
	}
	return s.records[len(s.records)-1].Hash
}

// appendToFile appends a record to the file.
func (s *SummaryStore) appendToFile(record SummaryRecord) error {
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

	// Append-only format: timestamp|circle_id|hash
	line := fmt.Sprintf("%s|%s|%s\n",
		record.RecordedAt.Format(time.RFC3339),
		record.CircleID,
		record.Hash,
	)
	_, err = f.WriteString(line)
	return err
}

// VerifyReplay checks if a summary hash matches a previous record.
// Used for replay verification.
func (s *SummaryStore) VerifyReplay(hash string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, record := range s.records {
		if record.Hash == hash {
			return true
		}
	}
	return false
}
