package persist

import (
	"sort"
	"strings"
	"sync"
	"time"

	"quantumlife/internal/interruptions"
	"quantumlife/pkg/domain/storelog"
)

// DedupRecordType is the record type for dedup entries.
const DedupRecordType = "DEDUP"

// DedupStore implements interruptions.DedupStore with file-backed persistence.
type DedupStore struct {
	mu   sync.RWMutex
	log  storelog.AppendOnlyLog
	seen map[string]time.Time // key -> first seen time
}

// NewDedupStore creates a new file-backed dedup store.
func NewDedupStore(log storelog.AppendOnlyLog) (*DedupStore, error) {
	store := &DedupStore{
		log:  log,
		seen: make(map[string]time.Time),
	}

	// Replay existing records
	if err := store.replay(); err != nil {
		return nil, err
	}

	return store, nil
}

// replay loads dedup entries from the log.
func (s *DedupStore) replay() error {
	records, err := s.log.ListByType(DedupRecordType)
	if err != nil {
		return err
	}

	for _, record := range records {
		key := parseDedupPayload(record.Payload)
		if key != "" {
			s.seen[key] = record.Timestamp
		}
	}

	return nil
}

// HasSeen returns true if the key was already seen.
func (s *DedupStore) HasSeen(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.seen[key]
	return exists
}

// MarkSeen marks a key as seen.
// NOTE: This method uses the current time. Prefer MarkSeenAt with an injected clock.
func (s *DedupStore) MarkSeen(key string) {
	// This is intentionally left to satisfy the DedupStore interface.
	// In production code, prefer using MarkSeenAt with an injected clock.
	s.MarkSeenAt(key, s.lastSeenTime())
}

// lastSeenTime returns a pseudo-time for interface compatibility.
// Real code should use MarkSeenAt with injected clock.
func (s *DedupStore) lastSeenTime() time.Time {
	// Return a deterministic timestamp based on existing entries
	// This is a fallback - production code should use MarkSeenAt
	if len(s.seen) == 0 {
		return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	var latest time.Time
	for _, t := range s.seen {
		if t.After(latest) {
			latest = t
		}
	}
	return latest.Add(time.Second)
}

// MarkSeenAt marks a key as seen at a specific time.
// Use this version when you have a controlled clock.
func (s *DedupStore) MarkSeenAt(key string, ts time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.seen[key]; exists {
		return // Already seen
	}

	// Create log record
	payload := formatDedupPayload(key)
	record := storelog.NewRecord(
		DedupRecordType,
		ts,
		"", // No circle for dedup
		payload,
	)

	// Append to log (ignore errors for simple operation)
	s.log.Append(record)
	s.seen[key] = ts
}

// Clear removes all entries from memory (log is preserved).
func (s *DedupStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seen = make(map[string]time.Time)
}

// Count returns number of tracked keys.
func (s *DedupStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.seen)
}

// Keys returns all dedup keys in sorted order.
func (s *DedupStore) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]string, 0, len(s.seen))
	for k := range s.seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Flush ensures all records are persisted.
func (s *DedupStore) Flush() error {
	return s.log.Flush()
}

// formatDedupPayload creates a canonical payload for a dedup entry.
func formatDedupPayload(key string) string {
	return "dedup|key:" + escapePayload(key)
}

// parseDedupPayload parses a canonical payload into a dedup key.
func parseDedupPayload(payload string) string {
	parts := strings.Split(payload, "|")
	for _, part := range parts {
		if strings.HasPrefix(part, "key:") {
			return unescapePayload(part[4:])
		}
	}
	return ""
}

// Verify interface compliance.
var _ interruptions.DedupStore = (*DedupStore)(nil)
