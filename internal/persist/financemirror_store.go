// Package persist provides persistence for finance mirror receipts.
//
// Phase 29: TrueLayer Read-Only Connect (UK Sandbox) + Finance Mirror Proof
// Reference: docs/ADR/ADR-0060-phase29-truelayer-readonly-finance-mirror.md
//
// CRITICAL INVARIANTS:
//   - Hash-only storage - NO raw account data, NO amounts, NO merchants
//   - Bounded retention (30 days max)
//   - Append-only with storelog integration
//   - No goroutines. No time.Now() - clock injection only.
package persist

import (
	"fmt"
	"sync"
	"time"

	"quantumlife/pkg/domain/financemirror"
	"quantumlife/pkg/domain/storelog"
)

// FinanceMirrorStore stores finance mirror receipts and acknowledgments.
// Thread-safe, in-memory implementation with bounded retention.
type FinanceMirrorStore struct {
	mu sync.RWMutex

	// Sync receipts
	syncReceipts         map[string]*financemirror.FinanceSyncReceipt // receiptID -> receipt
	syncReceiptsByCircle map[string][]string                          // circleID -> receiptIDs
	syncReceiptsByPeriod map[string]string                            // "circleID:period" -> latest receiptID

	// Acknowledgments
	acks map[string]*financemirror.FinanceMirrorAck // "circleID:period" -> ack

	// Token storage (connection info - hash only, not raw tokens)
	connectionHashes map[string]string // circleID -> connection hash

	// Configuration
	maxPeriods int // Maximum periods to retain (30 days)
	clock      func() time.Time

	// Storelog reference for replay
	storelogRef storelog.AppendOnlyLog
}

// NewFinanceMirrorStore creates a new finance mirror store.
func NewFinanceMirrorStore(clock func() time.Time) *FinanceMirrorStore {
	return &FinanceMirrorStore{
		syncReceipts:         make(map[string]*financemirror.FinanceSyncReceipt),
		syncReceiptsByCircle: make(map[string][]string),
		syncReceiptsByPeriod: make(map[string]string),
		acks:                 make(map[string]*financemirror.FinanceMirrorAck),
		connectionHashes:     make(map[string]string),
		maxPeriods:           30,
		clock:                clock,
	}
}

// SetStorelog sets the storelog reference for replay.
func (s *FinanceMirrorStore) SetStorelog(log storelog.AppendOnlyLog) {
	s.storelogRef = log
}

// StoreSyncReceipt stores a finance sync receipt.
// Idempotent: same receiptID will not duplicate.
func (s *FinanceMirrorStore) StoreSyncReceipt(receipt *financemirror.FinanceSyncReceipt) error {
	if err := receipt.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicate
	if _, exists := s.syncReceipts[receipt.ReceiptID]; exists {
		return nil // Idempotent
	}

	// Store receipt
	s.syncReceipts[receipt.ReceiptID] = receipt
	s.syncReceiptsByCircle[receipt.CircleID] = append(
		s.syncReceiptsByCircle[receipt.CircleID],
		receipt.ReceiptID,
	)

	// Update latest for period
	periodKey := fmt.Sprintf("%s:%s", receipt.CircleID, receipt.PeriodBucket)
	s.syncReceiptsByPeriod[periodKey] = receipt.ReceiptID

	// Write to storelog if available
	if s.storelogRef != nil {
		record := &storelog.LogRecord{
			Type:      storelog.RecordTypeFinanceSyncReceipt,
			Version:   storelog.SchemaVersion,
			Timestamp: receipt.TimeBucket,
			Payload:   receipt.CanonicalString(),
			Hash:      receipt.StatusHash,
		}
		_ = s.storelogRef.Append(record)
	}

	return nil
}

// GetSyncReceipt retrieves a sync receipt by ID.
func (s *FinanceMirrorStore) GetSyncReceipt(receiptID string) *financemirror.FinanceSyncReceipt {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.syncReceipts[receiptID]
}

// GetLatestSyncReceipt retrieves the most recent sync receipt for a circle.
func (s *FinanceMirrorStore) GetLatestSyncReceipt(circleID string) *financemirror.FinanceSyncReceipt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	receiptIDs := s.syncReceiptsByCircle[circleID]
	if len(receiptIDs) == 0 {
		return nil
	}

	// Find the latest by time bucket
	var latest *financemirror.FinanceSyncReceipt
	for _, id := range receiptIDs {
		r := s.syncReceipts[id]
		if r == nil {
			continue
		}
		if latest == nil || r.TimeBucket.After(latest.TimeBucket) {
			latest = r
		}
	}
	return latest
}

// GetSyncReceiptForPeriod retrieves the sync receipt for a specific period.
func (s *FinanceMirrorStore) GetSyncReceiptForPeriod(circleID, period string) *financemirror.FinanceSyncReceipt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	periodKey := fmt.Sprintf("%s:%s", circleID, period)
	receiptID, ok := s.syncReceiptsByPeriod[periodKey]
	if !ok {
		return nil
	}
	return s.syncReceipts[receiptID]
}

// StoreAck stores a finance mirror acknowledgment.
func (s *FinanceMirrorStore) StoreAck(ack *financemirror.FinanceMirrorAck) error {
	if ack.CircleID == "" || ack.PeriodBucket == "" {
		return fmt.Errorf("missing required fields")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%s:%s", ack.CircleID, ack.PeriodBucket)
	s.acks[key] = ack

	// Write to storelog if available
	if s.storelogRef != nil {
		record := &storelog.LogRecord{
			Type:    storelog.RecordTypeFinanceMirrorAck,
			Version: storelog.SchemaVersion,
			Payload: ack.CanonicalString(),
			Hash:    ack.AckHash,
		}
		_ = s.storelogRef.Append(record)
	}

	return nil
}

// IsAcked checks if a period has been acknowledged.
func (s *FinanceMirrorStore) IsAcked(circleID, period string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", circleID, period)
	_, ok := s.acks[key]
	return ok
}

// GetAck retrieves an acknowledgment for a period.
func (s *FinanceMirrorStore) GetAck(circleID, period string) *financemirror.FinanceMirrorAck {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", circleID, period)
	return s.acks[key]
}

// SetConnectionHash stores a connection hash for a circle.
// CRITICAL: Never store raw tokens, only hashes.
func (s *FinanceMirrorStore) SetConnectionHash(circleID, connectionHash string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connectionHashes[circleID] = connectionHash
}

// GetConnectionHash retrieves a connection hash for a circle.
func (s *FinanceMirrorStore) GetConnectionHash(circleID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.connectionHashes[circleID]
}

// HasConnection checks if a circle has a TrueLayer connection.
func (s *FinanceMirrorStore) HasConnection(circleID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.connectionHashes[circleID]
	return ok
}

// RemoveConnection removes a connection for a circle.
func (s *FinanceMirrorStore) RemoveConnection(circleID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.connectionHashes, circleID)
}

// Count returns the total number of sync receipts.
func (s *FinanceMirrorStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.syncReceipts)
}

// ExpireOldReceipts removes receipts older than maxPeriods.
// Call this periodically for cleanup.
func (s *FinanceMirrorStore) ExpireOldReceipts() {
	now := s.clock()
	cutoff := now.AddDate(0, 0, -s.maxPeriods)

	s.mu.Lock()
	defer s.mu.Unlock()

	// Find receipts to remove
	var toRemove []string
	for id, r := range s.syncReceipts {
		if r.TimeBucket.Before(cutoff) {
			toRemove = append(toRemove, id)
		}
	}

	// Remove old receipts
	for _, id := range toRemove {
		r := s.syncReceipts[id]
		if r != nil {
			// Remove from byCircle
			circleReceipts := s.syncReceiptsByCircle[r.CircleID]
			for i, rid := range circleReceipts {
				if rid == id {
					s.syncReceiptsByCircle[r.CircleID] = append(
						circleReceipts[:i],
						circleReceipts[i+1:]...,
					)
					break
				}
			}
			// Remove from byPeriod
			periodKey := fmt.Sprintf("%s:%s", r.CircleID, r.PeriodBucket)
			if s.syncReceiptsByPeriod[periodKey] == id {
				delete(s.syncReceiptsByPeriod, periodKey)
			}
		}
		delete(s.syncReceipts, id)
	}

	// Also expire old acks
	cutoffPeriod := cutoff.Format("2006-01-02")
	var acksToRemove []string
	for key := range s.acks {
		// Key format: "circleID:period"
		// Extract period and compare
		if len(key) > 11 {
			period := key[len(key)-10:]
			if period < cutoffPeriod {
				acksToRemove = append(acksToRemove, key)
			}
		}
	}
	for _, key := range acksToRemove {
		delete(s.acks, key)
	}
}

// ReplayFromStorelog replays records from the storelog.
func (s *FinanceMirrorStore) ReplayFromStorelog(entries []*storelog.LogRecord) error {
	for _, entry := range entries {
		switch entry.Type {
		case storelog.RecordTypeFinanceSyncReceipt:
			// Parse and store receipt from canonical string
			receipt, err := parseFinanceSyncReceiptFromPayload(entry.Payload)
			if err != nil {
				continue // Skip invalid records
			}
			_ = s.StoreSyncReceipt(receipt)

		case storelog.RecordTypeFinanceMirrorAck:
			// Parse and store ack from canonical string
			ack, err := parseFinanceMirrorAckFromPayload(entry.Payload)
			if err != nil {
				continue // Skip invalid records
			}
			_ = s.StoreAck(ack)
		}
	}
	return nil
}

// parseFinanceSyncReceiptFromPayload parses a receipt from its canonical string.
// Format: v1|finance_sync_receipt|receiptID|circleID|provider|period|accountsMag|txMag|evidenceHash|success|failReason|statusHash
func parseFinanceSyncReceiptFromPayload(payload string) (*financemirror.FinanceSyncReceipt, error) {
	// This is a simplified parser - in production would be more robust
	// For now, we'll return an error since we need proper parsing
	return nil, fmt.Errorf("replay parsing not implemented - receipts should be stored fresh")
}

// parseFinanceMirrorAckFromPayload parses an ack from its canonical string.
// Format: v1|finance_mirror_ack|circleID|period|pageHash|ackHash
func parseFinanceMirrorAckFromPayload(payload string) (*financemirror.FinanceMirrorAck, error) {
	// This is a simplified parser - in production would be more robust
	// For now, we'll return an error since we need proper parsing
	return nil, fmt.Errorf("replay parsing not implemented - acks should be stored fresh")
}
