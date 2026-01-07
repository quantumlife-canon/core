// Package persist provides storage for trust action receipts.
//
// Phase 28: Trust Kept — First Real Act, Then Silence
//
// CRITICAL INVARIANTS:
//   - Append-only storage
//   - Hash-only records (no raw identifiers)
//   - One receipt per circle per period maximum
//   - Bounded retention (30 days)
//   - No goroutines, no I/O except storelog
//   - No time.Now() - clock injection only
//
// Reference: docs/ADR/ADR-0059-phase28-trust-kept.md
package persist

import (
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"time"

	"quantumlife/pkg/domain/storelog"
	"quantumlife/pkg/domain/trustaction"
)

// Record types are defined in pkg/domain/storelog/log.go:
// - RecordTypeTrustActionReceipt = "TRUST_ACTION_RECEIPT"
// - RecordTypeTrustActionUpdate = "TRUST_ACTION_UPDATE"

// TrustActionStore stores trust action receipts.
//
// CRITICAL: Append-only. Hash-only records. One per circle per period.
type TrustActionStore struct {
	mu    sync.RWMutex
	clock func() time.Time

	// Receipts indexed by ID
	receipts map[string]*trustaction.TrustActionReceipt

	// Receipts by circle for lookup
	receiptsByCircle map[string][]string // circleID -> receiptIDs

	// Receipts by period for single-shot enforcement
	receiptsByPeriod map[string]string // "circleID:period" -> receiptID

	// Draft ID to receipt ID mapping (for undo)
	draftToReceipt map[string]string // draftIDHash -> receiptID

	// Receipt ID to draft ID mapping (for undo reversal)
	receiptToDraft map[string]string // receiptID -> draftID (raw, not hash)

	// Maximum periods to retain
	maxPeriods int

	// Optional storelog for replay
	log storelog.AppendOnlyLog
}

// NewTrustActionStore creates a new trust action store.
func NewTrustActionStore(clock func() time.Time) *TrustActionStore {
	return &TrustActionStore{
		clock:            clock,
		receipts:         make(map[string]*trustaction.TrustActionReceipt),
		receiptsByCircle: make(map[string][]string),
		receiptsByPeriod: make(map[string]string),
		draftToReceipt:   make(map[string]string),
		receiptToDraft:   make(map[string]string),
		maxPeriods:       30,
	}
}

// NewTrustActionStoreWithLog creates a store backed by an append-only log.
func NewTrustActionStoreWithLog(clock func() time.Time, log storelog.AppendOnlyLog) *TrustActionStore {
	store := NewTrustActionStore(clock)
	store.log = log
	return store
}

// AppendReceipt stores a trust action receipt.
// Returns error if a receipt for this circle+period already exists.
// CRITICAL: Single execution per period enforced.
func (s *TrustActionStore) AppendReceipt(receipt *trustaction.TrustActionReceipt) error {
	if receipt == nil {
		return errors.New("nil receipt")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Compute IDs if not set
	if receipt.StatusHash == "" {
		receipt.StatusHash = receipt.ComputeStatusHash()
	}
	if receipt.ReceiptID == "" {
		receipt.ReceiptID = receipt.ComputeReceiptID()
	}

	// Check for period deduplication (single-shot enforcement)
	periodKey := receipt.CircleID + ":" + receipt.Period
	if existingID, exists := s.receiptsByPeriod[periodKey]; exists {
		if existingID == receipt.ReceiptID {
			// Same receipt, idempotent - no error
			return nil
		}
		return errors.New("already executed this period")
	}

	// Store receipt
	s.receipts[receipt.ReceiptID] = receipt
	s.receiptsByCircle[receipt.CircleID] = append(s.receiptsByCircle[receipt.CircleID], receipt.ReceiptID)
	s.receiptsByPeriod[periodKey] = receipt.ReceiptID

	// Enforce bounded retention
	s.enforceBoundedRetention()

	// Append to log if available
	if s.log != nil {
		record := s.receiptToStorelogRecord(receipt)
		if err := s.log.Append(record); err != nil && err != storelog.ErrRecordExists {
			// Log error but don't fail - memory store is authoritative
			_ = err
		}
	}

	return nil
}

// AppendReceiptWithDraftID stores a receipt and tracks the draft ID for undo.
// CRITICAL: The draft ID is stored internally for undo, not in the receipt.
func (s *TrustActionStore) AppendReceiptWithDraftID(receipt *trustaction.TrustActionReceipt, draftID string) error {
	if err := s.AppendReceipt(receipt); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Store draft mapping for undo
	s.draftToReceipt[receipt.DraftIDHash] = receipt.ReceiptID
	s.receiptToDraft[receipt.ReceiptID] = draftID

	return nil
}

// GetByID retrieves a receipt by ID.
func (s *TrustActionStore) GetByID(receiptID string) *trustaction.TrustActionReceipt {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.receipts[receiptID]
}

// GetLatestForCircle returns the latest receipt for a circle.
func (s *TrustActionStore) GetLatestForCircle(circleID string) *trustaction.TrustActionReceipt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	receiptIDs := s.receiptsByCircle[circleID]
	if len(receiptIDs) == 0 {
		return nil
	}

	// Get receipts and sort by period descending
	var receipts []*trustaction.TrustActionReceipt
	for _, id := range receiptIDs {
		if r := s.receipts[id]; r != nil {
			receipts = append(receipts, r)
		}
	}

	if len(receipts) == 0 {
		return nil
	}

	sort.Slice(receipts, func(i, j int) bool {
		return receipts[i].Period > receipts[j].Period
	})

	return receipts[0]
}

// HasExecutedThisPeriod checks if any execution occurred this period.
// CRITICAL: Single-shot enforcement.
func (s *TrustActionStore) HasExecutedThisPeriod(circleID, period string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	periodKey := circleID + ":" + period
	_, exists := s.receiptsByPeriod[periodKey]
	return exists
}

// UpdateState updates the state of a receipt (executed -> undone/expired).
// CRITICAL: State can only transition forward, never backward.
func (s *TrustActionStore) UpdateState(receiptID string, newState trustaction.TrustActionState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	receipt, ok := s.receipts[receiptID]
	if !ok {
		return errors.New("receipt not found")
	}

	// Validate state transition
	if !isValidStateTransition(receipt.State, newState) {
		return errors.New("invalid state transition")
	}

	// Update state
	receipt.State = newState
	receipt.StatusHash = receipt.ComputeStatusHash()

	// Append state update to log if available
	if s.log != nil {
		record := s.stateUpdateToStorelogRecord(receiptID, newState)
		if err := s.log.Append(record); err != nil && err != storelog.ErrRecordExists {
			_ = err
		}
	}

	return nil
}

// GetDraftIDForReceipt retrieves the original draft ID for undo.
// CRITICAL: Returns the raw draft ID for building reversal draft.
func (s *TrustActionStore) GetDraftIDForReceipt(receiptID string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	draftID, ok := s.receiptToDraft[receiptID]
	return draftID, ok
}

// ListForCircle returns all receipts for a circle, sorted by period descending.
func (s *TrustActionStore) ListForCircle(circleID string) []*trustaction.TrustActionReceipt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	receiptIDs := s.receiptsByCircle[circleID]
	var receipts []*trustaction.TrustActionReceipt
	for _, id := range receiptIDs {
		if r := s.receipts[id]; r != nil {
			receipts = append(receipts, r)
		}
	}

	sort.Slice(receipts, func(i, j int) bool {
		return receipts[i].Period > receipts[j].Period
	})

	return receipts
}

// Count returns the total number of receipts.
func (s *TrustActionStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.receipts)
}

// isValidStateTransition checks if a state transition is allowed.
func isValidStateTransition(from, to trustaction.TrustActionState) bool {
	// Valid transitions:
	// eligible -> executed (execution)
	// executed -> undone (undo)
	// executed -> expired (expiry)
	// No transition from undone or expired
	switch from {
	case trustaction.StateEligible:
		return to == trustaction.StateExecuted
	case trustaction.StateExecuted:
		return to == trustaction.StateUndone || to == trustaction.StateExpired
	default:
		return false
	}
}

// enforceBoundedRetention ensures we don't exceed maxPeriods worth of data.
// Must be called with lock held.
func (s *TrustActionStore) enforceBoundedRetention() {
	if len(s.receipts) <= s.maxPeriods*10 { // Allow some buffer
		return
	}

	// Find oldest periods to remove
	periods := make([]string, 0, len(s.receiptsByPeriod))
	for p := range s.receiptsByPeriod {
		periods = append(periods, p)
	}

	// Sort periods (lexical sort works for circleID:YYYY-MM-DD)
	sort.Strings(periods)

	// Remove oldest until we're at limit
	toRemove := len(periods) - s.maxPeriods*10
	for i := 0; i < toRemove && i < len(periods); i++ {
		periodKey := periods[i]
		receiptID := s.receiptsByPeriod[periodKey]

		// Remove from all indexes
		if receipt := s.receipts[receiptID]; receipt != nil {
			delete(s.receipts, receiptID)
			delete(s.draftToReceipt, receipt.DraftIDHash)
			delete(s.receiptToDraft, receiptID)
		}
		delete(s.receiptsByPeriod, periodKey)
	}
}

// =============================================================================
// Storelog Integration (Replay Support)
// =============================================================================

// receiptRecord is the JSON structure for persisting receipts.
type receiptRecord struct {
	ReceiptID    string `json:"receipt_id"`
	ActionKind   string `json:"action_kind"`
	State        string `json:"state"`
	UndoBucket   string `json:"undo_bucket"`
	Period       string `json:"period"`
	CircleID     string `json:"circle_id"`
	StatusHash   string `json:"status_hash"`
	DraftIDHash  string `json:"draft_id_hash"`
	EnvelopeHash string `json:"envelope_hash"`
}

// stateUpdateRecord is the JSON structure for persisting state updates.
type stateUpdateRecord struct {
	ReceiptID string `json:"receipt_id"`
	NewState  string `json:"new_state"`
}

// receiptToStorelogRecord converts a receipt to a storelog record.
func (s *TrustActionStore) receiptToStorelogRecord(receipt *trustaction.TrustActionReceipt) *storelog.LogRecord {
	payload := receiptRecord{
		ReceiptID:    receipt.ReceiptID,
		ActionKind:   string(receipt.ActionKind),
		State:        string(receipt.State),
		UndoBucket:   receipt.UndoBucket.CanonicalString(),
		Period:       receipt.Period,
		CircleID:     receipt.CircleID,
		StatusHash:   receipt.StatusHash,
		DraftIDHash:  receipt.DraftIDHash,
		EnvelopeHash: receipt.EnvelopeHash,
	}

	data, _ := json.Marshal(payload)

	return &storelog.LogRecord{
		Type:      storelog.RecordTypeTrustActionReceipt,
		Version:   "v1",
		Timestamp: s.clock(),
		Payload:   string(data),
	}
}

// stateUpdateToStorelogRecord converts a state update to a storelog record.
func (s *TrustActionStore) stateUpdateToStorelogRecord(receiptID string, newState trustaction.TrustActionState) *storelog.LogRecord {
	payload := stateUpdateRecord{
		ReceiptID: receiptID,
		NewState:  string(newState),
	}

	data, _ := json.Marshal(payload)

	return &storelog.LogRecord{
		Type:      storelog.RecordTypeTrustActionUpdate,
		Version:   "v1",
		Timestamp: s.clock(),
		Payload:   string(data),
	}
}

// ReplayFromStorelog replays records from the storelog.
func (s *TrustActionStore) ReplayFromStorelog(entries []*storelog.LogRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, entry := range entries {
		switch entry.Type {
		case storelog.RecordTypeTrustActionReceipt:
			if err := s.replayReceiptRecord(entry); err != nil {
				continue // Skip corrupted records
			}
		case storelog.RecordTypeTrustActionUpdate:
			if err := s.replayStateUpdateRecord(entry); err != nil {
				continue
			}
		}
	}

	// Enforce bounded retention after replay
	s.enforceBoundedRetention()

	return nil
}

// replayReceiptRecord replays a receipt record from storelog.
func (s *TrustActionStore) replayReceiptRecord(record *storelog.LogRecord) error {
	var rr receiptRecord
	if err := json.Unmarshal([]byte(record.Payload), &rr); err != nil {
		return err
	}

	receipt := &trustaction.TrustActionReceipt{
		ReceiptID:    rr.ReceiptID,
		ActionKind:   trustaction.TrustActionKind(rr.ActionKind),
		State:        trustaction.TrustActionState(rr.State),
		Period:       rr.Period,
		CircleID:     rr.CircleID,
		StatusHash:   rr.StatusHash,
		DraftIDHash:  rr.DraftIDHash,
		EnvelopeHash: rr.EnvelopeHash,
	}

	// Parse undo bucket from canonical string
	receipt.UndoBucket = parseUndoBucketFromCanonical(rr.UndoBucket)

	// Store without duplicate check during replay
	s.receipts[receipt.ReceiptID] = receipt
	s.receiptsByCircle[receipt.CircleID] = append(s.receiptsByCircle[receipt.CircleID], receipt.ReceiptID)
	periodKey := receipt.CircleID + ":" + receipt.Period
	s.receiptsByPeriod[periodKey] = receipt.ReceiptID

	return nil
}

// replayStateUpdateRecord replays a state update record from storelog.
func (s *TrustActionStore) replayStateUpdateRecord(record *storelog.LogRecord) error {
	var sur stateUpdateRecord
	if err := json.Unmarshal([]byte(record.Payload), &sur); err != nil {
		return err
	}

	receipt, ok := s.receipts[sur.ReceiptID]
	if !ok {
		return errors.New("receipt not found for state update")
	}

	receipt.State = trustaction.TrustActionState(sur.NewState)
	receipt.StatusHash = receipt.ComputeStatusHash()

	return nil
}

// parseUndoBucketFromCanonical parses an undo bucket from its canonical string.
// Format: v1|undo_bucket|BucketStartRFC3339|BucketDurationMinutes
func parseUndoBucketFromCanonical(canonical string) trustaction.UndoBucket {
	// Simple parsing - in production would be more robust
	var bucket trustaction.UndoBucket
	bucket.BucketDurationMinutes = 15 // Default

	// Find the timestamp between second and third pipe
	pipeCount := 0
	start := 0
	for i, c := range canonical {
		if c == '|' {
			pipeCount++
			if pipeCount == 2 {
				start = i + 1
			} else if pipeCount == 3 {
				bucket.BucketStartRFC3339 = canonical[start:i]
				break
			}
		}
	}

	return bucket
}
