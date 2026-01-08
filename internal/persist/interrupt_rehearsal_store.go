// Package persist provides Phase 41 Interrupt Rehearsal storage.
//
// CRITICAL INVARIANTS:
//   - Hash-only storage. No raw identifiers.
//   - Append-only with replay via storelog.
//   - Bounded retention: 30 days OR 500 records max, FIFO eviction.
//   - No goroutines. Clock injection required.
//   - Deduplication by composite key (circle_id_hash|period_key|attempt_id_hash).
//
// Reference: docs/ADR/ADR-0078-phase41-live-interrupt-loop-apns.md
package persist

import (
	"sort"
	"sync"
	"time"

	ir "quantumlife/pkg/domain/interruptrehearsal"
	"quantumlife/pkg/domain/storelog"
)

// InterruptRehearsalStore stores rehearsal receipts.
// CRITICAL: Hash-only. No raw identifiers.
type InterruptRehearsalStore struct {
	mu          sync.RWMutex
	receipts    map[string]*ir.RehearsalReceipt // key: composite key
	receiptList []*storedReceipt                // for FIFO eviction
	acks        map[string]*ir.RehearsalAck     // key: ack ID
	storelogRef storelog.AppendOnlyLog
}

// storedReceipt wraps a receipt with metadata for retention.
type storedReceipt struct {
	key        string
	receipt    *ir.RehearsalReceipt
	storedTime time.Time
}

// NewInterruptRehearsalStore creates a new rehearsal store.
func NewInterruptRehearsalStore(storelogRef storelog.AppendOnlyLog) *InterruptRehearsalStore {
	return &InterruptRehearsalStore{
		receipts:    make(map[string]*ir.RehearsalReceipt),
		receiptList: make([]*storedReceipt, 0),
		acks:        make(map[string]*ir.RehearsalAck),
		storelogRef: storelogRef,
	}
}

// makeKey creates a composite key for deduplication.
func makeKey(circleIDHash, periodKey, attemptIDHash string) string {
	return circleIDHash + "|" + periodKey + "|" + attemptIDHash
}

// ═══════════════════════════════════════════════════════════════════════════
// Receipt Storage
// ═══════════════════════════════════════════════════════════════════════════

// AppendReceipt stores a rehearsal receipt.
// CRITICAL: Deduplicates by composite key. Updates existing if key matches.
func (s *InterruptRehearsalStore) AppendReceipt(receipt *ir.RehearsalReceipt, now time.Time) error {
	if receipt == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key := makeKey(receipt.CircleIDHash, receipt.PeriodKey, receipt.AttemptIDHash)

	// Check if key already exists (update case)
	if existing, ok := s.receipts[key]; ok {
		// Update existing receipt
		s.receipts[key] = receipt
		// Update in list
		for _, sr := range s.receiptList {
			if sr.key == key {
				sr.receipt = receipt
				break
			}
		}
		// Write update to storelog
		if s.storelogRef != nil {
			record := storelog.NewRecord(
				storelog.RecordTypeInterruptRehearsalReceipt,
				now,
				"",
				receipt.CanonicalString(),
			)
			_ = s.storelogRef.Append(record)
		}
		_ = existing // silence unused
		return nil
	}

	// New receipt
	s.receipts[key] = receipt
	s.receiptList = append(s.receiptList, &storedReceipt{
		key:        key,
		receipt:    receipt,
		storedTime: now,
	})

	// Write to storelog
	if s.storelogRef != nil {
		record := storelog.NewRecord(
			storelog.RecordTypeInterruptRehearsalReceipt,
			now,
			"",
			receipt.CanonicalString(),
		)
		_ = s.storelogRef.Append(record)
	}

	// Evict if needed
	s.evictIfNeededLocked(now)

	return nil
}

// evictIfNeededLocked evicts old records. Must be called with lock held.
func (s *InterruptRehearsalStore) evictIfNeededLocked(now time.Time) {
	// Evict by time (30 days)
	cutoff := now.AddDate(0, 0, -ir.MaxRetentionDays)
	newList := make([]*storedReceipt, 0, len(s.receiptList))
	for _, sr := range s.receiptList {
		if sr.storedTime.After(cutoff) {
			newList = append(newList, sr)
		} else {
			delete(s.receipts, sr.key)
		}
	}
	s.receiptList = newList

	// Evict by count (FIFO)
	for len(s.receiptList) > ir.MaxRecords {
		oldest := s.receiptList[0]
		s.receiptList = s.receiptList[1:]
		delete(s.receipts, oldest.key)
	}
}

// GetLatestByCircleAndPeriod returns the latest receipt for a circle and period.
func (s *InterruptRehearsalStore) GetLatestByCircleAndPeriod(circleIDHash, periodKey string) *ir.RehearsalReceipt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var latest *ir.RehearsalReceipt
	var latestTime time.Time

	for _, sr := range s.receiptList {
		if sr.receipt.CircleIDHash == circleIDHash && sr.receipt.PeriodKey == periodKey {
			if latest == nil || sr.storedTime.After(latestTime) {
				latest = sr.receipt
				latestTime = sr.storedTime
			}
		}
	}

	return latest
}

// CountDeliveriesByCircleAndPeriod returns the delivery count for a circle and period.
// Returns abstract bucket, not raw count.
func (s *InterruptRehearsalStore) CountDeliveriesByCircleAndPeriod(circleIDHash, periodKey string) ir.DeliveryBucket {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, sr := range s.receiptList {
		if sr.receipt.CircleIDHash == circleIDHash &&
			sr.receipt.PeriodKey == periodKey &&
			sr.receipt.Status == ir.StatusDelivered {
			count++
		}
	}

	if count == 0 {
		return ir.DeliveryNone
	}
	return ir.DeliveryOne
}

// GetDeliveryCountRaw returns the raw delivery count (for rate limiting).
func (s *InterruptRehearsalStore) GetDeliveryCountRaw(circleIDHash, periodKey string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, sr := range s.receiptList {
		if sr.receipt.CircleIDHash == circleIDHash &&
			sr.receipt.PeriodKey == periodKey &&
			sr.receipt.Status == ir.StatusDelivered {
			count++
		}
	}

	return count
}

// ListByCircleAndPeriod returns all receipts for a circle and period.
func (s *InterruptRehearsalStore) ListByCircleAndPeriod(circleIDHash, periodKey string) []*ir.RehearsalReceipt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*ir.RehearsalReceipt
	for _, sr := range s.receiptList {
		if sr.receipt.CircleIDHash == circleIDHash && sr.receipt.PeriodKey == periodKey {
			results = append(results, sr.receipt)
		}
	}

	// Sort by stored time (most recent first)
	sort.SliceStable(results, func(i, j int) bool {
		// Use StatusHash as tie-breaker for determinism
		return results[i].StatusHash > results[j].StatusHash
	})

	return results
}

// EvictOldPeriods evicts records older than retention period.
func (s *InterruptRehearsalStore) EvictOldPeriods(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictIfNeededLocked(now)
}

// Count returns the total number of receipts.
func (s *InterruptRehearsalStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.receiptList)
}

// ═══════════════════════════════════════════════════════════════════════════
// Acknowledgment Storage
// ═══════════════════════════════════════════════════════════════════════════

// AppendAck stores a rehearsal acknowledgment.
func (s *InterruptRehearsalStore) AppendAck(ack *ir.RehearsalAck, now time.Time) error {
	if ack == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	ack.AckID = ack.ComputeAckID()
	s.acks[ack.AckID] = ack

	// Write to storelog
	if s.storelogRef != nil {
		record := storelog.NewRecord(
			storelog.RecordTypeInterruptRehearsalAck,
			now,
			"",
			ack.CanonicalString(),
		)
		_ = s.storelogRef.Append(record)
	}

	return nil
}

// HasAck checks if an ack exists for the circle and period.
func (s *InterruptRehearsalStore) HasAck(circleIDHash, periodKey string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, ack := range s.acks {
		if ack.CircleIDHash == circleIDHash && ack.PeriodKey == periodKey {
			return true
		}
	}
	return false
}

// ═══════════════════════════════════════════════════════════════════════════
// Rate Limit Source Implementation
// ═══════════════════════════════════════════════════════════════════════════

// CanDeliver implements RateLimitSource for the engine.
func (s *InterruptRehearsalStore) CanDeliver(circleIDHash string, periodKey string) (bool, ir.RehearsalRejectReason) {
	count := s.GetDeliveryCountRaw(circleIDHash, periodKey)
	if count >= ir.MaxDeliveriesPerDay {
		return false, ir.RejectRateLimited
	}
	return true, ir.RejectNone
}

// GetDailyDeliveryCount implements RateLimitSource for the engine.
func (s *InterruptRehearsalStore) GetDailyDeliveryCount(circleIDHash string, periodKey string) int {
	return s.GetDeliveryCountRaw(circleIDHash, periodKey)
}
