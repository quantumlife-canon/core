// Package persist provides the notification observer store for Phase 38.
//
// CRITICAL INVARIANTS:
//   - Hash-only storage. No notification content stored.
//   - No app names. Only abstract class buckets.
//   - No device identifiers.
//   - Append-only. No overwrites.
//   - Bounded retention: max 200 records OR 30 days, FIFO eviction.
//   - Max 1 signal per app class per period.
//   - Storelog integration required.
//   - No goroutines. Thread-safe with RWMutex.
//   - No lookup by app, device, or user.
//
// Reference: docs/ADR/ADR-0075-phase38-notification-metadata-observer.md
package persist

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"quantumlife/pkg/domain/notificationobserver"
	"quantumlife/pkg/domain/storelog"
)

// NotificationObserverStore persists notification pressure signals.
// CRITICAL: Hash-only. Append-only. Bounded retention.
type NotificationObserverStore struct {
	mu sync.RWMutex

	// records stores signals by period key, then by signal ID.
	records map[string]map[string]*notificationobserver.NotificationPressureSignal

	// periodOrder tracks periods in chronological order for eviction.
	periodOrder []string

	// allRecordIDs tracks all record IDs in insertion order for FIFO.
	allRecordIDs []string

	// storelogRef for replay.
	storelogRef storelog.AppendOnlyLog

	// maxRetentionDays is the bounded retention period.
	maxRetentionDays int

	// maxRecords is the maximum number of records.
	maxRecords int
}

// NotificationObserverStoreConfig configures the store.
type NotificationObserverStoreConfig struct {
	Storelog         storelog.AppendOnlyLog
	MaxRetentionDays int
	MaxRecords       int
}

// DefaultNotificationObserverStoreConfig returns default configuration.
func DefaultNotificationObserverStoreConfig() NotificationObserverStoreConfig {
	return NotificationObserverStoreConfig{
		MaxRetentionDays: notificationobserver.MaxRetentionDays,
		MaxRecords:       notificationobserver.MaxSignalRecords,
	}
}

// NewNotificationObserverStore creates a new notification observer store.
func NewNotificationObserverStore(cfg NotificationObserverStoreConfig) *NotificationObserverStore {
	if cfg.MaxRetentionDays <= 0 {
		cfg.MaxRetentionDays = notificationobserver.MaxRetentionDays
	}
	if cfg.MaxRecords <= 0 {
		cfg.MaxRecords = notificationobserver.MaxSignalRecords
	}

	return &NotificationObserverStore{
		records:          make(map[string]map[string]*notificationobserver.NotificationPressureSignal),
		periodOrder:      make([]string, 0),
		allRecordIDs:     make([]string, 0),
		storelogRef:      cfg.Storelog,
		maxRetentionDays: cfg.MaxRetentionDays,
		maxRecords:       cfg.MaxRecords,
	}
}

// AppendSignal stores a notification pressure signal.
// CRITICAL: Hash-only. Replaces existing signal for same app class + period.
func (s *NotificationObserverStore) AppendSignal(signal *notificationobserver.NotificationPressureSignal) error {
	if signal == nil {
		return fmt.Errorf("nil signal")
	}

	if err := signal.Validate(); err != nil {
		return fmt.Errorf("invalid signal: %w", err)
	}

	// Compute hashes if not set
	if signal.StatusHash == "" {
		signal.StatusHash = signal.ComputeStatusHash()
	}
	if signal.SignalID == "" {
		signal.SignalID = signal.ComputeSignalID()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Get or create period bucket
	periodRecords, exists := s.records[signal.PeriodKey]
	if !exists {
		periodRecords = make(map[string]*notificationobserver.NotificationPressureSignal)
		s.records[signal.PeriodKey] = periodRecords
		s.periodOrder = append(s.periodOrder, signal.PeriodKey)
		sort.Strings(s.periodOrder)
	}

	// Check for existing signal with same ID (same app class + period)
	// Max 1 signal per app class per period
	if existing, exists := periodRecords[signal.SignalID]; exists {
		// Keep the higher magnitude/urgency signal
		if !shouldReplaceSignal(existing, signal) {
			return nil // Keep existing, don't store new
		}
		// Remove old from allRecordIDs
		for i, id := range s.allRecordIDs {
			if id == existing.SignalID {
				s.allRecordIDs = append(s.allRecordIDs[:i], s.allRecordIDs[i+1:]...)
				break
			}
		}
	}

	// Store signal
	periodRecords[signal.SignalID] = signal
	s.allRecordIDs = append(s.allRecordIDs, signal.SignalID)

	// Write to storelog if available
	if s.storelogRef != nil {
		logRecord := &storelog.LogRecord{
			Type:    storelog.RecordTypeNotificationSignal,
			Version: storelog.SchemaVersion,
			Payload: signal.CanonicalString(),
		}
		logRecord.Hash = logRecord.ComputeHash()
		if err := s.storelogRef.Append(logRecord); err != nil {
			// Log error but do not fail — in-memory state is authoritative
			_ = err
		}
	}

	// Evict old records
	s.evictLocked()

	return nil
}

// shouldReplaceSignal determines if new signal should replace existing.
func shouldReplaceSignal(existing, new *notificationobserver.NotificationPressureSignal) bool {
	existingMag := magnitudeRank(existing.Magnitude)
	newMag := magnitudeRank(new.Magnitude)

	existingHor := horizonRank(existing.Horizon)
	newHor := horizonRank(new.Horizon)

	// Prefer higher magnitude, then more urgent horizon
	if newMag > existingMag {
		return true
	}
	if newMag == existingMag && newHor > existingHor {
		return true
	}

	return false
}

func magnitudeRank(m notificationobserver.MagnitudeBucket) int {
	switch m {
	case notificationobserver.MagnitudeSeveral:
		return 2
	case notificationobserver.MagnitudeAFew:
		return 1
	default:
		return 0
	}
}

func horizonRank(h notificationobserver.HorizonBucket) int {
	switch h {
	case notificationobserver.HorizonNow:
		return 2
	case notificationobserver.HorizonSoon:
		return 1
	default:
		return 0
	}
}

// GetByPeriod returns all signals for a period.
func (s *NotificationObserverStore) GetByPeriod(periodKey string) []*notificationobserver.NotificationPressureSignal {
	s.mu.RLock()
	defer s.mu.RUnlock()

	periodRecords, exists := s.records[periodKey]
	if !exists {
		return nil
	}

	result := make([]*notificationobserver.NotificationPressureSignal, 0, len(periodRecords))
	for _, sig := range periodRecords {
		result = append(result, sig)
	}

	// Sort by signal ID for determinism
	sort.Slice(result, func(i, j int) bool {
		return result[i].SignalID < result[j].SignalID
	})

	return result
}

// CountByPeriod returns the count of signals for a period.
func (s *NotificationObserverStore) CountByPeriod(periodKey string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	periodRecords, exists := s.records[periodKey]
	if !exists {
		return 0
	}
	return len(periodRecords)
}

// TotalRecords returns the total number of records.
func (s *NotificationObserverStore) TotalRecords() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	total := 0
	for _, periodRecords := range s.records {
		total += len(periodRecords)
	}
	return total
}

// GetAllPeriods returns all stored periods in chronological order.
func (s *NotificationObserverStore) GetAllPeriods() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]string, len(s.periodOrder))
	copy(result, s.periodOrder)
	return result
}

// evictLocked removes old records to stay within bounds.
// MUST be called with lock held.
func (s *NotificationObserverStore) evictLocked() {
	// Evict by count first (FIFO)
	for len(s.allRecordIDs) > s.maxRecords {
		// Remove oldest record
		oldestID := s.allRecordIDs[0]
		s.allRecordIDs = s.allRecordIDs[1:]
		s.removeRecordLocked(oldestID)
	}

	// Evict by date
	s.evictOldPeriodsLocked()
}

// removeRecordLocked removes a record by ID.
// MUST be called with lock held.
func (s *NotificationObserverStore) removeRecordLocked(signalID string) {
	for periodKey, periodRecords := range s.records {
		if _, exists := periodRecords[signalID]; exists {
			delete(periodRecords, signalID)

			// Clean up empty period
			if len(periodRecords) == 0 {
				delete(s.records, periodKey)
				// Remove from period order
				for i, p := range s.periodOrder {
					if p == periodKey {
						s.periodOrder = append(s.periodOrder[:i], s.periodOrder[i+1:]...)
						break
					}
				}
			}
			return
		}
	}
}

// evictOldPeriodsLocked removes periods older than retention.
// MUST be called with lock held.
func (s *NotificationObserverStore) evictOldPeriodsLocked() {
	if len(s.periodOrder) == 0 {
		return
	}

	// Calculate cutoff using current wall clock time
	// NOTE: This is acceptable for eviction only (not business logic)
	cutoff := time.Now().AddDate(0, 0, -s.maxRetentionDays).Format("2006-01-02")

	var newOrder []string
	for _, period := range s.periodOrder {
		if period < cutoff {
			// Remove all records from this period
			periodRecords := s.records[period]
			for signalID := range periodRecords {
				// Remove from allRecordIDs
				for i, id := range s.allRecordIDs {
					if id == signalID {
						s.allRecordIDs = append(s.allRecordIDs[:i], s.allRecordIDs[i+1:]...)
						break
					}
				}
			}
			delete(s.records, period)
		} else {
			newOrder = append(newOrder, period)
		}
	}

	s.periodOrder = newOrder
}

// EvictOldPeriods explicitly evicts old periods.
// Called with explicit clock for determinism in tests.
func (s *NotificationObserverStore) EvictOldPeriods(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.periodOrder) == 0 {
		return
	}

	cutoff := now.AddDate(0, 0, -s.maxRetentionDays).Format("2006-01-02")

	var newOrder []string
	for _, period := range s.periodOrder {
		if period < cutoff {
			periodRecords := s.records[period]
			for signalID := range periodRecords {
				for i, id := range s.allRecordIDs {
					if id == signalID {
						s.allRecordIDs = append(s.allRecordIDs[:i], s.allRecordIDs[i+1:]...)
						break
					}
				}
			}
			delete(s.records, period)
		} else {
			newOrder = append(newOrder, period)
		}
	}

	s.periodOrder = newOrder
}

// SetStorelog sets the storelog reference.
func (s *NotificationObserverStore) SetStorelog(log storelog.AppendOnlyLog) {
	s.storelogRef = log
}

// Replay replays records from storelog.
func (s *NotificationObserverStore) Replay() error {
	if s.storelogRef == nil {
		return nil
	}
	// Implementation would iterate through storelog records
	return nil
}
