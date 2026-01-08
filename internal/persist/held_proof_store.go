// Package persist provides Phase 43 Held Proof storage.
//
// CRITICAL INVARIANTS:
//   - Hash-only storage. No raw identifiers.
//   - Append-only with replay via storelog.
//   - Bounded retention: 30 days OR max records, FIFO eviction.
//   - Dedup by (dayKey + EvidenceHash) for signals.
//   - No goroutines. Clock injection required.
//
// Reference: docs/ADR/ADR-0080-phase43-held-under-agreement-proof-ledger.md
package persist

import (
	"sort"
	"sync"
	"time"

	hp "quantumlife/pkg/domain/heldproof"
	"quantumlife/pkg/domain/storelog"
)

// ============================================================================
// Signal Store
// ============================================================================

// HeldProofSignalStore stores held proof signals.
// CRITICAL: Hash-only. No raw identifiers.
type HeldProofSignalStore struct {
	mu          sync.RWMutex
	signals     map[string][]storedSignal // key: dayKey
	signalList  []*storedSignal           // for FIFO eviction
	dedupIndex  map[string]bool           // key: dayKey + "|" + evidenceHash
	storelogRef storelog.AppendOnlyLog
}

// storedSignal wraps a signal with metadata for retention.
type storedSignal struct {
	dayKey     string
	signal     hp.HeldProofSignal
	storedTime time.Time
}

// NewHeldProofSignalStore creates a new held proof signal store.
func NewHeldProofSignalStore(storelogRef storelog.AppendOnlyLog) *HeldProofSignalStore {
	return &HeldProofSignalStore{
		signals:     make(map[string][]storedSignal),
		signalList:  make([]*storedSignal, 0),
		dedupIndex:  make(map[string]bool),
		storelogRef: storelogRef,
	}
}

// AppendSignal stores a signal for a day.
// CRITICAL: Dedup by (dayKey + EvidenceHash).
func (s *HeldProofSignalStore) AppendSignal(dayKey string, sig hp.HeldProofSignal, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Dedup key
	dedupKey := dayKey + "|" + sig.EvidenceHash
	if s.dedupIndex[dedupKey] {
		return nil // Already exists
	}

	stored := &storedSignal{
		dayKey:     dayKey,
		signal:     sig,
		storedTime: now,
	}

	s.signals[dayKey] = append(s.signals[dayKey], *stored)
	s.signalList = append(s.signalList, stored)
	s.dedupIndex[dedupKey] = true

	// Write to storelog
	if s.storelogRef != nil {
		record := storelog.NewRecord(
			storelog.RecordTypeHeldProofSignal,
			now,
			"",
			sig.CanonicalString(),
		)
		_ = s.storelogRef.Append(record)
	}

	// Evict if needed
	s.evictIfNeededLocked(now)

	return nil
}

// evictIfNeededLocked evicts old records. Must be called with lock held.
func (s *HeldProofSignalStore) evictIfNeededLocked(now time.Time) {
	// Evict by time (30 days)
	cutoff := now.AddDate(0, 0, -hp.MaxRetentionDays)
	newList := make([]*storedSignal, 0, len(s.signalList))

	for _, stored := range s.signalList {
		if stored.storedTime.After(cutoff) {
			newList = append(newList, stored)
		} else {
			// Remove from dedup index and day map
			dedupKey := stored.dayKey + "|" + stored.signal.EvidenceHash
			delete(s.dedupIndex, dedupKey)
			s.removeFromDayMap(stored.dayKey, stored.signal.EvidenceHash)
		}
	}
	s.signalList = newList

	// Evict by count (FIFO)
	for len(s.signalList) > hp.MaxSignalRecords {
		oldest := s.signalList[0]
		s.signalList = s.signalList[1:]
		dedupKey := oldest.dayKey + "|" + oldest.signal.EvidenceHash
		delete(s.dedupIndex, dedupKey)
		s.removeFromDayMap(oldest.dayKey, oldest.signal.EvidenceHash)
	}
}

// removeFromDayMap removes a signal from the day map.
func (s *HeldProofSignalStore) removeFromDayMap(dayKey, evidenceHash string) {
	signals := s.signals[dayKey]
	var remaining []storedSignal
	for _, sig := range signals {
		if sig.signal.EvidenceHash != evidenceHash {
			remaining = append(remaining, sig)
		}
	}
	if len(remaining) == 0 {
		delete(s.signals, dayKey)
	} else {
		s.signals[dayKey] = remaining
	}
}

// ListSignals returns signals for a day.
func (s *HeldProofSignalStore) ListSignals(dayKey string) []hp.HeldProofSignal {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stored := s.signals[dayKey]
	if len(stored) == 0 {
		return nil
	}

	result := make([]hp.HeldProofSignal, len(stored))
	for i, st := range stored {
		result[i] = st.signal
	}

	// Sort by EvidenceHash for determinism
	sort.Slice(result, func(i, j int) bool {
		return result[i].EvidenceHash < result[j].EvidenceHash
	})

	return result
}

// Count returns the total number of signals.
func (s *HeldProofSignalStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.signalList)
}

// EvictOldRecords evicts records older than retention period.
func (s *HeldProofSignalStore) EvictOldRecords(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictIfNeededLocked(now)
}

// ============================================================================
// Ack Store
// ============================================================================

// HeldProofAckStore stores held proof acknowledgments.
// CRITICAL: Hash-only. No raw identifiers.
type HeldProofAckStore struct {
	mu          sync.RWMutex
	viewed      map[string]*storedAck // key: dayKey + "|" + statusHash
	dismissed   map[string]*storedAck // key: dayKey + "|" + statusHash
	ackList     []*storedAck          // for FIFO eviction
	storelogRef storelog.AppendOnlyLog
}

// storedAck wraps an ack with metadata for retention.
type storedAck struct {
	dayKey     string
	statusHash string
	ackKind    hp.HeldProofAckKind
	storedTime time.Time
}

// NewHeldProofAckStore creates a new held proof ack store.
func NewHeldProofAckStore(storelogRef storelog.AppendOnlyLog) *HeldProofAckStore {
	return &HeldProofAckStore{
		viewed:      make(map[string]*storedAck),
		dismissed:   make(map[string]*storedAck),
		ackList:     make([]*storedAck, 0),
		storelogRef: storelogRef,
	}
}

// ackKey returns the key for the ack maps.
func ackKey(dayKey, statusHash string) string {
	return dayKey + "|" + statusHash
}

// RecordViewed records that the proof page was viewed.
func (s *HeldProofAckStore) RecordViewed(dayKey, statusHash string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := ackKey(dayKey, statusHash)
	if s.viewed[key] != nil {
		return nil // Already recorded
	}

	ack := &storedAck{
		dayKey:     dayKey,
		statusHash: statusHash,
		ackKind:    hp.AckViewed,
		storedTime: now,
	}

	s.viewed[key] = ack
	s.ackList = append(s.ackList, ack)

	// Write to storelog
	if s.storelogRef != nil {
		hpAck := hp.HeldProofAck{
			Period:     hp.HeldProofPeriod{DayKey: dayKey},
			AckKind:    hp.AckViewed,
			StatusHash: statusHash,
		}
		hpAck.AckHash = hpAck.ComputeHash()

		record := storelog.NewRecord(
			storelog.RecordTypeHeldProofAck,
			now,
			"",
			hpAck.CanonicalString(),
		)
		_ = s.storelogRef.Append(record)
	}

	// Evict if needed
	s.evictIfNeededLocked(now)

	return nil
}

// RecordDismissed records that the proof page was dismissed.
func (s *HeldProofAckStore) RecordDismissed(dayKey, statusHash string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := ackKey(dayKey, statusHash)
	if s.dismissed[key] != nil {
		return nil // Already recorded
	}

	ack := &storedAck{
		dayKey:     dayKey,
		statusHash: statusHash,
		ackKind:    hp.AckDismissed,
		storedTime: now,
	}

	s.dismissed[key] = ack
	s.ackList = append(s.ackList, ack)

	// Write to storelog
	if s.storelogRef != nil {
		hpAck := hp.HeldProofAck{
			Period:     hp.HeldProofPeriod{DayKey: dayKey},
			AckKind:    hp.AckDismissed,
			StatusHash: statusHash,
		}
		hpAck.AckHash = hpAck.ComputeHash()

		record := storelog.NewRecord(
			storelog.RecordTypeHeldProofAck,
			now,
			"",
			hpAck.CanonicalString(),
		)
		_ = s.storelogRef.Append(record)
	}

	// Evict if needed
	s.evictIfNeededLocked(now)

	return nil
}

// evictIfNeededLocked evicts old records. Must be called with lock held.
func (s *HeldProofAckStore) evictIfNeededLocked(now time.Time) {
	// Evict by time (30 days)
	cutoff := now.AddDate(0, 0, -hp.MaxRetentionDays)
	newList := make([]*storedAck, 0, len(s.ackList))

	for _, ack := range s.ackList {
		if ack.storedTime.After(cutoff) {
			newList = append(newList, ack)
		} else {
			// Remove from maps
			key := ackKey(ack.dayKey, ack.statusHash)
			if ack.ackKind == hp.AckViewed {
				delete(s.viewed, key)
			} else {
				delete(s.dismissed, key)
			}
		}
	}
	s.ackList = newList

	// Evict by count (FIFO)
	for len(s.ackList) > hp.MaxAckRecords {
		oldest := s.ackList[0]
		s.ackList = s.ackList[1:]
		key := ackKey(oldest.dayKey, oldest.statusHash)
		if oldest.ackKind == hp.AckViewed {
			delete(s.viewed, key)
		} else {
			delete(s.dismissed, key)
		}
	}
}

// IsDismissed checks if the proof page was dismissed.
func (s *HeldProofAckStore) IsDismissed(dayKey, statusHash string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dismissed[ackKey(dayKey, statusHash)] != nil
}

// HasViewed checks if the proof page was viewed.
func (s *HeldProofAckStore) HasViewed(dayKey, statusHash string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.viewed[ackKey(dayKey, statusHash)] != nil
}

// Count returns the total number of acks.
func (s *HeldProofAckStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.ackList)
}

// EvictOldRecords evicts records older than retention period.
func (s *HeldProofAckStore) EvictOldRecords(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictIfNeededLocked(now)
}
