package surface

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// ActionStore records user actions on surfaced items.
// Stores ONLY hashes, never raw content.
// Append-only with bounded size.
type ActionStore struct {
	mu         sync.RWMutex
	records    []ActionRecord
	maxRecords int
	clock      func() time.Time
}

// StoreOption configures the ActionStore.
type StoreOption func(*ActionStore)

// WithStoreClock sets a custom clock for the store.
func WithStoreClock(clock func() time.Time) StoreOption {
	return func(s *ActionStore) {
		s.clock = clock
	}
}

// WithMaxRecords sets the maximum number of records to retain.
func WithMaxRecords(max int) StoreOption {
	return func(s *ActionStore) {
		s.maxRecords = max
	}
}

// NewActionStore creates a new action store with options.
func NewActionStore(opts ...StoreOption) *ActionStore {
	s := &ActionStore{
		records:    make([]ActionRecord, 0),
		maxRecords: 100, // Default bounded size
		clock:      time.Now,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Record stores an action record (hash-only).
func (s *ActionStore) Record(circleID, itemKeyHash string, action Action) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.clock()

	// Compute record hash (deterministic)
	canonical := fmt.Sprintf(
		"action|circle:%s|item:%s|action:%s|ts:%d",
		circleID,
		itemKeyHash,
		action,
		now.Unix(),
	)
	h := sha256.Sum256([]byte(canonical))
	recordHash := hex.EncodeToString(h[:])

	record := ActionRecord{
		CircleID:    circleID,
		ItemKeyHash: itemKeyHash,
		Action:      action,
		RecordedAt:  now,
		RecordHash:  recordHash,
	}

	s.records = append(s.records, record)

	// Enforce bounded size (drop oldest)
	if len(s.records) > s.maxRecords {
		s.records = s.records[len(s.records)-s.maxRecords:]
	}

	return nil
}

// RecordViewed records a view action.
func (s *ActionStore) RecordViewed(circleID, itemKeyHash string) error {
	return s.Record(circleID, itemKeyHash, ActionViewed)
}

// RecordHeld records a hold action.
func (s *ActionStore) RecordHeld(circleID, itemKeyHash string) error {
	return s.Record(circleID, itemKeyHash, ActionHeld)
}

// RecordWhy records a why (explainability) action.
func (s *ActionStore) RecordWhy(circleID, itemKeyHash string) error {
	return s.Record(circleID, itemKeyHash, ActionWhy)
}

// RecordPreferShowAll records a preference change to show_all.
func (s *ActionStore) RecordPreferShowAll(circleID, itemKeyHash string) error {
	return s.Record(circleID, itemKeyHash, ActionPreferShowAll)
}

// Count returns the number of recorded actions.
func (s *ActionStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}

// Records returns a copy of all records.
func (s *ActionStore) Records() []ActionRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]ActionRecord, len(s.records))
	copy(result, s.records)
	return result
}

// LatestRecord returns the most recent record, if any.
func (s *ActionStore) LatestRecord() (ActionRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.records) == 0 {
		return ActionRecord{}, false
	}
	return s.records[len(s.records)-1], true
}

// CountByAction returns the count of records for a specific action.
func (s *ActionStore) CountByAction(action Action) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, r := range s.records {
		if r.Action == action {
			count++
		}
	}
	return count
}

// VerifyRecordHash checks if a record hash exists in the store.
func (s *ActionStore) VerifyRecordHash(hash string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, r := range s.records {
		if r.RecordHash == hash {
			return true
		}
	}
	return false
}

// Clear removes all records (for testing).
func (s *ActionStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = make([]ActionRecord, 0)
}
