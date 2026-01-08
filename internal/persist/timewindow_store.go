// Package persist provides the time window store for Phase 40.
//
// CRITICAL INVARIANTS:
//   - Hash-only storage. No raw content.
//   - Append-only for signals.
//   - Deduplicate by (circle_id_hash + period_key + result_hash).
//   - Bounded retention: 30 days, max 500 records.
//   - FIFO eviction when limits exceeded.
//   - Storelog integration required.
//   - No goroutines. Thread-safe with RWMutex.
//
// Reference: docs/ADR/ADR-0077-phase40-time-window-pressure-sources.md
package persist

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"quantumlife/pkg/domain/storelog"
	tw "quantumlife/pkg/domain/timewindow"
)

// TimeWindowStore persists time window build results.
// CRITICAL: Hash-only. Append-only. Bounded retention.
type TimeWindowStore struct {
	mu sync.RWMutex

	// results stores build results by composite key.
	// Key: circleIDHash|periodKey|resultHash
	results map[string]*tw.TimeWindowBuildResult

	// resultOrder tracks results in chronological order for FIFO eviction.
	resultOrder []string

	// periodOrder tracks periods for retention eviction.
	periodOrder []string

	// storelogRef for replay.
	storelogRef storelog.AppendOnlyLog

	// configuration
	maxRetentionDays int
	maxRecords       int
}

// TimeWindowStoreConfig configures the store.
type TimeWindowStoreConfig struct {
	Storelog         storelog.AppendOnlyLog
	MaxRetentionDays int
	MaxRecords       int
}

// DefaultTimeWindowStoreConfig returns default configuration.
func DefaultTimeWindowStoreConfig() TimeWindowStoreConfig {
	return TimeWindowStoreConfig{
		MaxRetentionDays: tw.MaxRetentionDays,
		MaxRecords:       tw.MaxRecords,
	}
}

// NewTimeWindowStore creates a new time window store.
func NewTimeWindowStore(cfg TimeWindowStoreConfig) *TimeWindowStore {
	if cfg.MaxRetentionDays <= 0 {
		cfg.MaxRetentionDays = tw.MaxRetentionDays
	}
	if cfg.MaxRecords <= 0 {
		cfg.MaxRecords = tw.MaxRecords
	}

	return &TimeWindowStore{
		results:          make(map[string]*tw.TimeWindowBuildResult),
		resultOrder:      make([]string, 0),
		periodOrder:      make([]string, 0),
		storelogRef:      cfg.Storelog,
		maxRetentionDays: cfg.MaxRetentionDays,
		maxRecords:       cfg.MaxRecords,
	}
}

// makeKey creates the composite key for deduplication.
func (s *TimeWindowStore) makeKey(circleIDHash, periodKey, resultHash string) string {
	return fmt.Sprintf("%s|%s|%s", circleIDHash, periodKey, resultHash)
}

// PersistResult stores a time window build result.
// CRITICAL: Deduplicates by (circle_id_hash + period_key + result_hash).
// CRITICAL: Bounded retention with FIFO eviction.
func (s *TimeWindowStore) PersistResult(result *tw.TimeWindowBuildResult) error {
	if result == nil {
		return fmt.Errorf("nil result")
	}

	if err := result.Validate(); err != nil {
		return fmt.Errorf("invalid result: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key := s.makeKey(result.CircleIDHash, result.PeriodKey, result.ResultHash)

	// Check for duplicate
	if _, exists := s.results[key]; exists {
		return nil // Idempotent - already stored
	}

	// Store result
	s.results[key] = result
	s.resultOrder = append(s.resultOrder, key)

	// Track period for retention
	periodExists := false
	for _, p := range s.periodOrder {
		if p == result.PeriodKey {
			periodExists = true
			break
		}
	}
	if !periodExists {
		s.periodOrder = append(s.periodOrder, result.PeriodKey)
		sort.Strings(s.periodOrder)
	}

	// Write to storelog if available
	if s.storelogRef != nil {
		logRecord := storelog.NewRecord(
			storelog.RecordTypeTimeWindowResult,
			time.Now(),
			"",
			result.CanonicalString(),
		)
		if err := s.storelogRef.Append(logRecord); err != nil {
			_ = err // Log error but do not fail
		}

		// Also write individual signals
		for _, signal := range result.Signals {
			signalRecord := storelog.NewRecord(
				storelog.RecordTypeTimeWindowSignal,
				time.Now(),
				"",
				signal.CanonicalString(),
			)
			if err := s.storelogRef.Append(signalRecord); err != nil {
				_ = err
			}
		}
	}

	// FIFO eviction if over max records
	s.evictExcessRecordsLocked()

	return nil
}

// GetResult retrieves a result by composite key.
func (s *TimeWindowStore) GetResult(circleIDHash, periodKey, resultHash string) *tw.TimeWindowBuildResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := s.makeKey(circleIDHash, periodKey, resultHash)
	return s.results[key]
}

// GetLatestResultForCircle returns the most recent result for a circle.
func (s *TimeWindowStore) GetLatestResultForCircle(circleIDHash string) *tw.TimeWindowBuildResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var latest *tw.TimeWindowBuildResult
	var latestPeriod string

	for _, result := range s.results {
		if result.CircleIDHash == circleIDHash {
			if latest == nil || result.PeriodKey > latestPeriod {
				latest = result
				latestPeriod = result.PeriodKey
			}
		}
	}

	return latest
}

// GetResultsForPeriod returns all results for a given period.
func (s *TimeWindowStore) GetResultsForPeriod(periodKey string) []*tw.TimeWindowBuildResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*tw.TimeWindowBuildResult
	for _, result := range s.results {
		if result.PeriodKey == periodKey {
			results = append(results, result)
		}
	}

	// Sort by circle ID hash for determinism
	sort.Slice(results, func(i, j int) bool {
		return results[i].CircleIDHash < results[j].CircleIDHash
	})

	return results
}

// GetResultsForCircle returns all results for a circle.
func (s *TimeWindowStore) GetResultsForCircle(circleIDHash string) []*tw.TimeWindowBuildResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*tw.TimeWindowBuildResult
	for _, result := range s.results {
		if result.CircleIDHash == circleIDHash {
			results = append(results, result)
		}
	}

	// Sort by period for determinism
	sort.Slice(results, func(i, j int) bool {
		return results[i].PeriodKey < results[j].PeriodKey
	})

	return results
}

// HasResultForPeriod checks if a result exists for the given circle and period.
func (s *TimeWindowStore) HasResultForPeriod(circleIDHash, periodKey string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, result := range s.results {
		if result.CircleIDHash == circleIDHash && result.PeriodKey == periodKey {
			return true
		}
	}
	return false
}

// evictExcessRecordsLocked removes oldest records when over max.
// MUST be called with lock held.
func (s *TimeWindowStore) evictExcessRecordsLocked() {
	for len(s.resultOrder) > s.maxRecords {
		// Remove oldest (first in order)
		oldestKey := s.resultOrder[0]
		s.resultOrder = s.resultOrder[1:]
		delete(s.results, oldestKey)
	}
}

// EvictOldPeriods explicitly evicts old periods.
// Called with explicit clock for determinism in tests.
func (s *TimeWindowStore) EvictOldPeriods(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.periodOrder) == 0 {
		return
	}

	cutoffDay := now.AddDate(0, 0, -s.maxRetentionDays).Format("2006-01-02")

	// Find periods to evict and remove associated results
	var newOrder []string
	for _, period := range s.periodOrder {
		// Extract day from period key (format: YYYY-MM-DDTHH:MM or YYYY-MM-DD)
		periodDay := period
		if len(period) >= 10 {
			periodDay = period[:10]
		}

		if periodDay < cutoffDay {
			// Remove all results for this period
			for key, result := range s.results {
				if result.PeriodKey == period {
					delete(s.results, key)
				}
			}
		} else {
			newOrder = append(newOrder, period)
		}
	}

	s.periodOrder = newOrder

	// Rebuild result order
	var newResultOrder []string
	for _, key := range s.resultOrder {
		if _, exists := s.results[key]; exists {
			newResultOrder = append(newResultOrder, key)
		}
	}
	s.resultOrder = newResultOrder
}

// TotalResults returns the total number of stored results.
func (s *TimeWindowStore) TotalResults() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.results)
}

// SetStorelog sets the storelog reference.
func (s *TimeWindowStore) SetStorelog(log storelog.AppendOnlyLog) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.storelogRef = log
}

// Clear removes all data from the store.
// Used for testing only.
func (s *TimeWindowStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.results = make(map[string]*tw.TimeWindowBuildResult)
	s.resultOrder = make([]string, 0)
	s.periodOrder = make([]string, 0)
}

// GetOverallMagnitude returns the highest magnitude across all recent results for a circle.
func (s *TimeWindowStore) GetOverallMagnitude(circleIDHash string, days int, now time.Time) tw.WindowMagnitudeBucket {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cutoff := now.AddDate(0, 0, -days).Format("2006-01-02")
	maxMag := tw.MagnitudeNothing

	for _, result := range s.results {
		if result.CircleIDHash != circleIDHash {
			continue
		}

		// Extract day from period key
		periodDay := result.PeriodKey
		if len(result.PeriodKey) >= 10 {
			periodDay = result.PeriodKey[:10]
		}

		if periodDay < cutoff {
			continue
		}

		mag := result.GetOverallMagnitude()
		if mag == tw.MagnitudeSeveral {
			return tw.MagnitudeSeveral
		}
		if mag == tw.MagnitudeAFew {
			maxMag = tw.MagnitudeAFew
		}
	}

	return maxMag
}
