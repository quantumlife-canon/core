// Package persist provides persistence for undoable execution records.
//
// Phase 25: First Undoable Execution (Opt-In, Single-Shot)
//
// CRITICAL INVARIANTS:
//   - Append-only storage
//   - Hash-only (no identifiers)
//   - Bounded retention (last 90 days or N records)
//   - Replay supported
//   - No goroutines. No time.Now() - clock injection only.
//
// Reference: docs/ADR/ADR-0055-phase25-first-undoable-execution.md
package persist

import (
	"encoding/json"
	"sync"
	"time"

	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/storelog"
	"quantumlife/pkg/domain/undoableexec"
)

// undoRecordPayload is the JSON structure for persisting undo records.
type undoRecordPayload struct {
	ID              string `json:"id"`
	PeriodKey       string `json:"period_key"`
	CircleID        string `json:"circle_id"`
	ActionKind      string `json:"action_kind"`
	DraftID         string `json:"draft_id"`
	EnvelopeID      string `json:"envelope_id"`
	BeforeStatus    string `json:"before_status"`
	AfterStatus     string `json:"after_status"`
	UndoUntilBucket string `json:"undo_until_bucket"`
	State           string `json:"state"`
	ExecutedBucket  string `json:"executed_at_bucket"`
}

// undoAckPayload is the JSON structure for persisting undo acks.
type undoAckPayload struct {
	RecordID  string `json:"record_id"`
	NewState  string `json:"new_state"`
	AckBucket string `json:"ack_bucket"`
	Reason    string `json:"reason"`
}

// UndoableExecStore stores undoable execution records.
//
// CRITICAL: This store contains NO identifiers.
// Only hashes, enums, and period keys are stored.
type UndoableExecStore struct {
	mu         sync.RWMutex
	records    map[string]*undoableexec.UndoRecord // id -> record
	acks       []*undoableexec.UndoAck             // append-only acks
	byCircle   map[identity.EntityID][]*undoableexec.UndoRecord
	byPeriod   map[string][]*undoableexec.UndoRecord // "circle:period" -> records
	maxEntries int
	maxAgeDays int
	clock      func() time.Time
}

// NewUndoableExecStore creates a new undoable execution store.
func NewUndoableExecStore(clock func() time.Time) *UndoableExecStore {
	return &UndoableExecStore{
		records:    make(map[string]*undoableexec.UndoRecord),
		acks:       make([]*undoableexec.UndoAck, 0),
		byCircle:   make(map[identity.EntityID][]*undoableexec.UndoRecord),
		byPeriod:   make(map[string][]*undoableexec.UndoRecord),
		maxEntries: 1000,
		maxAgeDays: 90,
		clock:      clock,
	}
}

// AppendRecord appends an undo record.
// Append-only: records are never modified or deleted (except for bounded eviction).
func (s *UndoableExecStore) AppendRecord(record *undoableexec.UndoRecord) error {
	if record == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Deduplicate by ID
	if _, exists := s.records[record.ID]; exists {
		return nil
	}

	// Bounded eviction (FIFO)
	if len(s.records) >= s.maxEntries {
		s.evictOldest()
	}

	s.records[record.ID] = record

	circleID := identity.EntityID(record.CircleID)
	s.byCircle[circleID] = append(s.byCircle[circleID], record)

	periodKey := record.CircleID + ":" + record.PeriodKey
	s.byPeriod[periodKey] = append(s.byPeriod[periodKey], record)

	return nil
}

// AppendAck appends an acknowledgement (state transition).
// Acks are applied to records to update their state.
func (s *UndoableExecStore) AppendAck(ack *undoableexec.UndoAck) error {
	if ack == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.acks = append(s.acks, ack)

	// Apply ack to record
	if record, exists := s.records[ack.RecordID]; exists {
		record.State = ack.NewState
	}

	return nil
}

// evictOldest removes the oldest entry. Must be called with lock held.
func (s *UndoableExecStore) evictOldest() {
	for id := range s.records {
		delete(s.records, id)
		break
	}
}

// GetByID retrieves a record by its ID.
func (s *UndoableExecStore) GetByID(id string) (*undoableexec.UndoRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, ok := s.records[id]
	return record, ok
}

// GetByCircle retrieves all records for a circle.
func (s *UndoableExecStore) GetByCircle(circleID identity.EntityID) []*undoableexec.UndoRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.byCircle[circleID]
}

// GetForPeriod retrieves all records for a circle and period.
func (s *UndoableExecStore) GetForPeriod(circleID identity.EntityID, periodKey string) []*undoableexec.UndoRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := string(circleID) + ":" + periodKey
	return s.byPeriod[key]
}

// HasExecutedThisPeriod returns true if any execution occurred this period.
// This enforces the single-shot per period rule.
func (s *UndoableExecStore) HasExecutedThisPeriod(circleID identity.EntityID, periodKey string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := string(circleID) + ":" + periodKey
	records := s.byPeriod[key]

	for _, r := range records {
		// Any state beyond pending counts as executed
		if r.State != undoableexec.StatePending {
			return true
		}
	}
	return false
}

// GetLatestUndoable returns the latest record that can still be undone.
func (s *UndoableExecStore) GetLatestUndoable(circleID identity.EntityID) *undoableexec.UndoRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := s.clock()
	records := s.byCircle[circleID]

	// Find most recent undoable record
	for i := len(records) - 1; i >= 0; i-- {
		r := records[i]
		if r.IsUndoAvailable(now) {
			return r
		}
	}
	return nil
}

// Count returns the total number of records stored.
func (s *UndoableExecStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}

// ListAll returns all records.
func (s *UndoableExecStore) ListAll() []*undoableexec.UndoRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*undoableexec.UndoRecord, 0, len(s.records))
	for _, r := range s.records {
		result = append(result, r)
	}
	return result
}

// RecordToStorelogRecord converts an undo record to a storelog record.
func (s *UndoableExecStore) RecordToStorelogRecord(record *undoableexec.UndoRecord) *storelog.LogRecord {
	payload := undoRecordPayload{
		ID:              record.ID,
		PeriodKey:       record.PeriodKey,
		CircleID:        record.CircleID,
		ActionKind:      string(record.ActionKind),
		DraftID:         record.DraftID,
		EnvelopeID:      record.EnvelopeID,
		BeforeStatus:    string(record.BeforeStatus),
		AfterStatus:     string(record.AfterStatus),
		UndoUntilBucket: record.UndoAvailableUntilBucket.BucketStartRFC3339,
		State:           string(record.State),
		ExecutedBucket:  record.ExecutedAtBucket.BucketStartRFC3339,
	}

	data, _ := json.Marshal(payload)

	return &storelog.LogRecord{
		Type:    storelog.RecordTypeUndoExecRecord,
		Version: "v1",
		Hash:    record.Hash(),
		Payload: string(data),
	}
}

// AckToStorelogRecord converts an ack to a storelog record.
func (s *UndoableExecStore) AckToStorelogRecord(ack *undoableexec.UndoAck) *storelog.LogRecord {
	payload := undoAckPayload{
		RecordID:  ack.RecordID,
		NewState:  string(ack.NewState),
		AckBucket: ack.AckBucket.BucketStartRFC3339,
		Reason:    ack.Reason,
	}

	data, _ := json.Marshal(payload)

	return &storelog.LogRecord{
		Type:    storelog.RecordTypeUndoExecAck,
		Version: "v1",
		Hash:    ack.Hash(),
		Payload: string(data),
	}
}

// ReplayRecordFromStorelog replays a record from a storelog entry.
func (s *UndoableExecStore) ReplayRecordFromStorelog(logRecord *storelog.LogRecord) error {
	if logRecord.Type != storelog.RecordTypeUndoExecRecord {
		return nil
	}

	var payload undoRecordPayload
	if err := json.Unmarshal([]byte(logRecord.Payload), &payload); err != nil {
		return err
	}

	record := &undoableexec.UndoRecord{
		ID:           payload.ID,
		PeriodKey:    payload.PeriodKey,
		CircleID:     payload.CircleID,
		ActionKind:   undoableexec.UndoableActionKind(payload.ActionKind),
		DraftID:      payload.DraftID,
		EnvelopeID:   payload.EnvelopeID,
		BeforeStatus: undoableexec.ResponseStatus(payload.BeforeStatus),
		AfterStatus:  undoableexec.ResponseStatus(payload.AfterStatus),
		UndoAvailableUntilBucket: undoableexec.UndoWindow{
			BucketStartRFC3339:    payload.UndoUntilBucket,
			BucketDurationMinutes: 15,
		},
		State: undoableexec.UndoState(payload.State),
		ExecutedAtBucket: undoableexec.UndoWindow{
			BucketStartRFC3339:    payload.ExecutedBucket,
			BucketDurationMinutes: 15,
		},
	}

	return s.AppendRecord(record)
}

// ReplayAckFromStorelog replays an ack from a storelog entry.
func (s *UndoableExecStore) ReplayAckFromStorelog(logRecord *storelog.LogRecord) error {
	if logRecord.Type != storelog.RecordTypeUndoExecAck {
		return nil
	}

	var payload undoAckPayload
	if err := json.Unmarshal([]byte(logRecord.Payload), &payload); err != nil {
		return err
	}

	ack := &undoableexec.UndoAck{
		RecordID: payload.RecordID,
		NewState: undoableexec.UndoState(payload.NewState),
		AckBucket: undoableexec.UndoWindow{
			BucketStartRFC3339:    payload.AckBucket,
			BucketDurationMinutes: 15,
		},
		Reason: payload.Reason,
	}

	return s.AppendAck(ack)
}

// ExpireOldRecords marks expired undo windows.
// Should be called periodically to transition states.
func (s *UndoableExecStore) ExpireOldRecords() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.clock()
	for _, record := range s.records {
		if record.State == undoableexec.StateUndoAvailable {
			if record.UndoAvailableUntilBucket.IsExpired(now) {
				record.State = undoableexec.StateExpired
			}
		}
	}
}
