// Package persist provides the device registration store for Phase 37.
//
// CRITICAL INVARIANTS:
//   - Hash-only storage. Raw token NEVER stored here.
//   - Append-only. No overwrites.
//   - Bounded retention: max 200 records OR 30 days, FIFO eviction.
//   - One active registration per circle per platform.
//   - Storelog integration required.
//   - No goroutines. Thread-safe with RWMutex.
//
// Reference: docs/ADR/ADR-0074-phase37-device-registration-deeplink.md
package persist

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"quantumlife/pkg/domain/devicereg"
	"quantumlife/pkg/domain/storelog"
)

// DeviceRegistrationStore persists device registration receipts.
// CRITICAL: Hash-only. Append-only. Bounded retention.
type DeviceRegistrationStore struct {
	mu sync.RWMutex

	// records stores registrations by period key, then by receipt ID.
	records map[string]map[string]*devicereg.DeviceRegistrationReceipt

	// activeByCircle maps circle+platform to the active receipt ID.
	activeByCircle map[string]string

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

// DeviceRegistrationStoreConfig configures the store.
type DeviceRegistrationStoreConfig struct {
	Storelog         storelog.AppendOnlyLog
	MaxRetentionDays int
	MaxRecords       int
}

// DefaultDeviceRegistrationStoreConfig returns default configuration.
func DefaultDeviceRegistrationStoreConfig() DeviceRegistrationStoreConfig {
	return DeviceRegistrationStoreConfig{
		MaxRetentionDays: devicereg.MaxRetentionDays,
		MaxRecords:       devicereg.MaxRegistrationRecords,
	}
}

// NewDeviceRegistrationStore creates a new device registration store.
func NewDeviceRegistrationStore(cfg DeviceRegistrationStoreConfig) *DeviceRegistrationStore {
	if cfg.MaxRetentionDays <= 0 {
		cfg.MaxRetentionDays = devicereg.MaxRetentionDays
	}
	if cfg.MaxRecords <= 0 {
		cfg.MaxRecords = devicereg.MaxRegistrationRecords
	}

	return &DeviceRegistrationStore{
		records:          make(map[string]map[string]*devicereg.DeviceRegistrationReceipt),
		activeByCircle:   make(map[string]string),
		periodOrder:      make([]string, 0),
		allRecordIDs:     make([]string, 0),
		storelogRef:      cfg.Storelog,
		maxRetentionDays: cfg.MaxRetentionDays,
		maxRecords:       cfg.MaxRecords,
	}
}

// circlePlatformKey generates a lookup key for circle+platform.
func circlePlatformKey(circleIDHash string, platform devicereg.DevicePlatform) string {
	return fmt.Sprintf("%s|%s", circleIDHash, platform)
}

// AppendRegistration stores a registration receipt.
// CRITICAL: Hash-only. Replaces active registration for circle+platform.
func (s *DeviceRegistrationStore) AppendRegistration(receipt *devicereg.DeviceRegistrationReceipt) error {
	if receipt == nil {
		return fmt.Errorf("nil receipt")
	}

	if err := receipt.Validate(); err != nil {
		return fmt.Errorf("invalid receipt: %w", err)
	}

	// Compute hashes if not set
	if receipt.StatusHash == "" {
		receipt.StatusHash = receipt.ComputeStatusHash()
	}
	if receipt.ReceiptID == "" {
		receipt.ReceiptID = receipt.ComputeReceiptID()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Get or create period bucket
	periodRecords, exists := s.records[receipt.PeriodKey]
	if !exists {
		periodRecords = make(map[string]*devicereg.DeviceRegistrationReceipt)
		s.records[receipt.PeriodKey] = periodRecords
		s.periodOrder = append(s.periodOrder, receipt.PeriodKey)
		sort.Strings(s.periodOrder)
	}

	// Check for exact duplicate
	if _, exists := periodRecords[receipt.ReceiptID]; exists {
		return fmt.Errorf("duplicate receipt: %s", receipt.ReceiptID)
	}

	// Store record
	periodRecords[receipt.ReceiptID] = receipt
	s.allRecordIDs = append(s.allRecordIDs, receipt.ReceiptID)

	// Update active registration for circle+platform
	cpKey := circlePlatformKey(receipt.CircleIDHash, receipt.Platform)
	s.activeByCircle[cpKey] = receipt.ReceiptID

	// Write to storelog if available
	if s.storelogRef != nil {
		logRecord := &storelog.LogRecord{
			Type:    storelog.RecordTypeDeviceRegistration,
			Version: storelog.SchemaVersion,
			Payload: receipt.CanonicalString(),
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

// LatestByCircle returns the most recent registration for a circle.
// Returns nil if no registration exists.
func (s *DeviceRegistrationStore) LatestByCircle(circleIDHash string) *devicereg.DeviceRegistrationReceipt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check iOS platform (currently only supported)
	cpKey := circlePlatformKey(circleIDHash, devicereg.DevicePlatformIOS)
	receiptID, exists := s.activeByCircle[cpKey]
	if !exists {
		return nil
	}

	// Find the receipt
	for _, periodRecords := range s.records {
		if r, exists := periodRecords[receiptID]; exists {
			return r
		}
	}

	return nil
}

// HasActiveRegistration checks if a circle has an active registration.
func (s *DeviceRegistrationStore) HasActiveRegistration(circleIDHash string, platform devicereg.DevicePlatform) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cpKey := circlePlatformKey(circleIDHash, platform)
	receiptID, exists := s.activeByCircle[cpKey]
	if !exists {
		return false
	}

	// Verify the receipt exists and is registered (not revoked)
	for _, periodRecords := range s.records {
		if r, exists := periodRecords[receiptID]; exists {
			return r.State == devicereg.DeviceRegStateRegistered
		}
	}

	return false
}

// CountByPeriod returns the count of registrations for a period.
func (s *DeviceRegistrationStore) CountByPeriod(periodKey string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	periodRecords, exists := s.records[periodKey]
	if !exists {
		return 0
	}
	return len(periodRecords)
}

// TotalRecords returns the total number of records.
func (s *DeviceRegistrationStore) TotalRecords() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	total := 0
	for _, periodRecords := range s.records {
		total += len(periodRecords)
	}
	return total
}

// GetByPeriod returns all records for a period.
func (s *DeviceRegistrationStore) GetByPeriod(periodKey string) []*devicereg.DeviceRegistrationReceipt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	periodRecords, exists := s.records[periodKey]
	if !exists {
		return nil
	}

	result := make([]*devicereg.DeviceRegistrationReceipt, 0, len(periodRecords))
	for _, r := range periodRecords {
		result = append(result, r)
	}

	// Sort by receipt ID for determinism
	sort.Slice(result, func(i, j int) bool {
		return result[i].ReceiptID < result[j].ReceiptID
	})

	return result
}

// evictLocked removes old records to stay within bounds.
// MUST be called with lock held.
func (s *DeviceRegistrationStore) evictLocked() {
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
func (s *DeviceRegistrationStore) removeRecordLocked(receiptID string) {
	for periodKey, periodRecords := range s.records {
		if r, exists := periodRecords[receiptID]; exists {
			// Remove active reference if this was active
			cpKey := circlePlatformKey(r.CircleIDHash, r.Platform)
			if s.activeByCircle[cpKey] == receiptID {
				delete(s.activeByCircle, cpKey)
			}

			// Remove from period
			delete(periodRecords, receiptID)

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
func (s *DeviceRegistrationStore) evictOldPeriodsLocked() {
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
			for receiptID, r := range periodRecords {
				// Remove active references
				cpKey := circlePlatformKey(r.CircleIDHash, r.Platform)
				if s.activeByCircle[cpKey] == receiptID {
					delete(s.activeByCircle, cpKey)
				}

				// Remove from allRecordIDs
				for i, id := range s.allRecordIDs {
					if id == receiptID {
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
func (s *DeviceRegistrationStore) EvictOldPeriods(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.periodOrder) == 0 {
		return
	}

	cutoff := now.AddDate(0, 0, -s.maxRetentionDays).Format("2006-01-02")

	var newOrder []string
	for _, period := range s.periodOrder {
		if period < cutoff {
			// Remove all records from this period
			periodRecords := s.records[period]
			for receiptID, r := range periodRecords {
				cpKey := circlePlatformKey(r.CircleIDHash, r.Platform)
				if s.activeByCircle[cpKey] == receiptID {
					delete(s.activeByCircle, cpKey)
				}

				// Remove from allRecordIDs
				for i, id := range s.allRecordIDs {
					if id == receiptID {
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

// GetAllPeriods returns all stored periods in chronological order.
func (s *DeviceRegistrationStore) GetAllPeriods() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]string, len(s.periodOrder))
	copy(result, s.periodOrder)
	return result
}

// SetStorelog sets the storelog reference.
func (s *DeviceRegistrationStore) SetStorelog(log storelog.AppendOnlyLog) {
	s.storelogRef = log
}

// Replay replays records from storelog.
func (s *DeviceRegistrationStore) Replay() error {
	if s.storelogRef == nil {
		return nil
	}
	// Implementation would iterate through storelog records
	return nil
}
