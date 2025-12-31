package caps

import (
	"fmt"
	"sync"

	"quantumlife/pkg/clock"
)

// Store tracks daily execution attempts and spend totals.
// All operations are concurrency-safe using sync.Mutex.
//
// CRITICAL: No goroutines or background operations.
// All state updates are synchronous and deterministic.
type Store struct {
	mu sync.Mutex

	// counters maps (dayKey, scopeType, scopeID, currency) -> Counters
	counters map[string]*Counters

	// attemptsSeen tracks (dayKey, scopeType, scopeID, attemptID) to ensure
	// idempotent attempt counting.
	attemptsSeen map[string]bool
}

// Counters tracks attempts and money moved for a scope.
type Counters struct {
	// Attempts is the number of execution attempts started.
	// Increments on OnAttemptStarted, regardless of outcome.
	Attempts int

	// MoneyMovedCents is the total money actually transferred.
	// Increments on OnAttemptFinalized when MoneyMoved=true.
	MoneyMovedCents int64
}

// NewStore creates a new in-memory caps store.
func NewStore() *Store {
	return &Store{
		counters:     make(map[string]*Counters),
		attemptsSeen: make(map[string]bool),
	}
}

// counterKey creates the key for counters map.
func counterKey(dayKey string, scopeType ScopeType, scopeID, currency string) string {
	return fmt.Sprintf("%s|%s|%s|%s", dayKey, scopeType, scopeID, currency)
}

// attemptKey creates the key for attempt deduplication.
func attemptKey(dayKey string, scopeType ScopeType, scopeID, attemptID string) string {
	return fmt.Sprintf("%s|%s|%s|%s", dayKey, scopeType, scopeID, attemptID)
}

// DayKey derives the UTC day key from a clock.
// Format: YYYY-MM-DD
func DayKey(c clock.Clock) string {
	t := c.Now().UTC()
	return t.Format("2006-01-02")
}

// GetCounters returns the current counters for a scope.
// Returns zero counters if none exist.
func (s *Store) GetCounters(dayKey string, scopeType ScopeType, scopeID, currency string) Counters {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := counterKey(dayKey, scopeType, scopeID, currency)
	if c, ok := s.counters[key]; ok {
		return *c
	}
	return Counters{}
}

// GetAttemptCount returns the current attempt count for a scope (across all currencies).
func (s *Store) GetAttemptCount(dayKey string, scopeType ScopeType, scopeID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Sum attempts across all currencies for this scope
	total := 0
	prefix := fmt.Sprintf("%s|%s|%s|", dayKey, scopeType, scopeID)
	for k, c := range s.counters {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			total += c.Attempts
		}
	}
	return total
}

// IncrementAttempt increments the attempt counter for a scope.
// Returns true if this was a new attempt (not seen before).
// Returns false if this attempt was already counted (idempotent).
func (s *Store) IncrementAttempt(dayKey string, scopeType ScopeType, scopeID, currency, attemptID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if this attempt was already counted
	aKey := attemptKey(dayKey, scopeType, scopeID, attemptID)
	if s.attemptsSeen[aKey] {
		return false // Already counted
	}

	// Mark as seen
	s.attemptsSeen[aKey] = true

	// Increment counter
	cKey := counterKey(dayKey, scopeType, scopeID, currency)
	if s.counters[cKey] == nil {
		s.counters[cKey] = &Counters{}
	}
	s.counters[cKey].Attempts++

	return true
}

// IncrementSpend adds to the money moved total for a scope.
func (s *Store) IncrementSpend(dayKey string, scopeType ScopeType, scopeID, currency string, amountCents int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := counterKey(dayKey, scopeType, scopeID, currency)
	if s.counters[key] == nil {
		s.counters[key] = &Counters{}
	}
	s.counters[key].MoneyMovedCents += amountCents
}

// Reset clears all counters and attempt tracking.
// Used for testing only.
func (s *Store) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.counters = make(map[string]*Counters)
	s.attemptsSeen = make(map[string]bool)
}

// PurgeDaysBefore removes all data for days before the given day key.
// This can be called periodically to prevent unbounded memory growth.
// Not required for demo but useful for production.
func (s *Store) PurgeDaysBefore(currentDayKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove old counter entries
	for k := range s.counters {
		// Extract day key (first component before |)
		dayKey := ""
		for i := 0; i < len(k); i++ {
			if k[i] == '|' {
				dayKey = k[:i]
				break
			}
		}
		if dayKey < currentDayKey {
			delete(s.counters, k)
		}
	}

	// Remove old attempt entries
	for k := range s.attemptsSeen {
		dayKey := ""
		for i := 0; i < len(k); i++ {
			if k[i] == '|' {
				dayKey = k[:i]
				break
			}
		}
		if dayKey < currentDayKey {
			delete(s.attemptsSeen, k)
		}
	}
}
