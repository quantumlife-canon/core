// Package persist provides file-backed persistent stores for QuantumLife.
//
// This file provides persistence for finance execution attempts (idempotency ledger).
//
// CRITICAL: All stores use append-only logging for durability.
// Changes are written to the log immediately and can be replayed.
//
// GUARDRAIL: No goroutines. All operations are synchronous.
// No time.Now() - clock must be injected.
//
// Phase 17b: Finance Execution Attempt Persistence
package persist

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"quantumlife/internal/finance/execution/attempts"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/storelog"
	"quantumlife/pkg/events"
)

// FinanceAttemptStore implements attempt ledger storage with file-backed persistence.
type FinanceAttemptStore struct {
	mu           sync.RWMutex
	log          storelog.AppendOnlyLog
	attempts     map[string]*attempts.AttemptRecord   // attemptID -> record
	byEnvelope   map[string][]*attempts.AttemptRecord // envelopeID -> records
	byKey        map[string]*attempts.AttemptRecord   // envelopeID:idempotencyKey -> record
	config       attempts.LedgerConfig
	idGenerator  func() string
	auditEmitter func(event events.Event)
}

// NewFinanceAttemptStore creates a new file-backed attempt store.
func NewFinanceAttemptStore(
	log storelog.AppendOnlyLog,
	config attempts.LedgerConfig,
	idGen func() string,
	emitter func(event events.Event),
) (*FinanceAttemptStore, error) {
	store := &FinanceAttemptStore{
		log:          log,
		attempts:     make(map[string]*attempts.AttemptRecord),
		byEnvelope:   make(map[string][]*attempts.AttemptRecord),
		byKey:        make(map[string]*attempts.AttemptRecord),
		config:       config,
		idGenerator:  idGen,
		auditEmitter: emitter,
	}

	// Replay existing records
	if err := store.replay(); err != nil {
		return nil, err
	}

	return store, nil
}

// replay loads attempts from the log.
func (s *FinanceAttemptStore) replay() error {
	records, err := s.log.ListByType(storelog.RecordTypeFinanceAttempt)
	if err != nil {
		return err
	}

	for _, record := range records {
		attempt, err := parseAttemptPayload(record.Payload)
		if err != nil {
			continue // Skip corrupted records
		}
		s.attempts[attempt.AttemptID] = attempt
		s.byEnvelope[attempt.EnvelopeID] = append(s.byEnvelope[attempt.EnvelopeID], attempt)
		keyIndex := attempt.EnvelopeID + ":" + attempt.IdempotencyKey
		s.byKey[keyIndex] = attempt
	}

	// Also replay status updates
	statusRecords, err := s.log.ListByType(storelog.RecordTypeFinanceAttemptStatus)
	if err != nil {
		return err
	}

	for _, record := range statusRecords {
		attemptID, status, err := parseAttemptStatusPayload(record.Payload)
		if err != nil {
			continue
		}
		if attempt, ok := s.attempts[attemptID]; ok {
			attempt.Status = status
			attempt.UpdatedAt = record.Timestamp
		}
	}

	return nil
}

// StartAttempt creates a new attempt record.
func (s *FinanceAttemptStore) StartAttempt(req attempts.StartAttemptRequest) (*attempts.AttemptRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if attempt already exists
	if _, exists := s.attempts[req.AttemptID]; exists {
		return nil, attempts.ErrAttemptAlreadyExists
	}

	// Check for in-flight attempts
	if !s.config.AllowConcurrentAttempts {
		for _, attempt := range s.byEnvelope[req.EnvelopeID] {
			if attempt.Status.IsInFlight() {
				return nil, attempts.ErrAttemptInFlight
			}
		}
	}

	// Check idempotency key uniqueness
	keyIndex := req.EnvelopeID + ":" + req.IdempotencyKey
	if _, exists := s.byKey[keyIndex]; exists {
		return nil, attempts.ErrIdempotencyKeyConflict
	}

	// Create the record
	record := &attempts.AttemptRecord{
		AttemptID:      req.AttemptID,
		EnvelopeID:     req.EnvelopeID,
		ActionHash:     req.ActionHash,
		IdempotencyKey: req.IdempotencyKey,
		CircleID:       req.CircleID,
		IntersectionID: req.IntersectionID,
		TraceID:        req.TraceID,
		Status:         attempts.AttemptStatusStarted,
		Provider:       req.Provider,
		CreatedAt:      req.Now,
		UpdatedAt:      req.Now,
	}

	// Persist to log
	payload := formatAttemptPayload(record)
	logRecord := storelog.NewRecord(
		storelog.RecordTypeFinanceAttempt,
		req.Now,
		identity.EntityID(req.CircleID),
		payload,
	)
	if err := s.log.Append(logRecord); err != nil && err != storelog.ErrRecordExists {
		return nil, err
	}

	// Store in memory
	s.attempts[req.AttemptID] = record
	s.byEnvelope[req.EnvelopeID] = append(s.byEnvelope[req.EnvelopeID], record)
	s.byKey[keyIndex] = record

	return record, nil
}

// GetAttempt retrieves an attempt by ID.
func (s *FinanceAttemptStore) GetAttempt(attemptID string) (*attempts.AttemptRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, exists := s.attempts[attemptID]
	return record, exists
}

// GetAttemptByEnvelopeAndKey retrieves an attempt by envelope and idempotency key.
func (s *FinanceAttemptStore) GetAttemptByEnvelopeAndKey(envelopeID, idempotencyKey string) (*attempts.AttemptRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keyIndex := envelopeID + ":" + idempotencyKey
	record, exists := s.byKey[keyIndex]
	return record, exists
}

// GetInFlightAttempt returns the in-flight attempt for an envelope, if any.
func (s *FinanceAttemptStore) GetInFlightAttempt(envelopeID string) (*attempts.AttemptRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, attempt := range s.byEnvelope[envelopeID] {
		if attempt.Status.IsInFlight() {
			return attempt, true
		}
	}
	return nil, false
}

// UpdateStatus transitions an attempt to a new status.
func (s *FinanceAttemptStore) UpdateStatus(attemptID string, status attempts.AttemptStatus, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, exists := s.attempts[attemptID]
	if !exists {
		return attempts.ErrAttemptNotFound
	}

	if record.Status.IsTerminal() {
		return attempts.ErrAttemptTerminal
	}

	// Persist status update
	payload := formatAttemptStatusPayload(attemptID, status)
	logRecord := storelog.NewRecord(
		storelog.RecordTypeFinanceAttemptStatus,
		now,
		identity.EntityID(record.CircleID),
		payload,
	)
	if err := s.log.Append(logRecord); err != nil && err != storelog.ErrRecordExists {
		return err
	}

	record.Status = status
	record.UpdatedAt = now

	return nil
}

// FinalizeAttempt marks an attempt as terminal with additional details.
func (s *FinanceAttemptStore) FinalizeAttempt(req attempts.FinalizeAttemptRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, exists := s.attempts[req.AttemptID]
	if !exists {
		return attempts.ErrAttemptNotFound
	}

	if record.Status.IsTerminal() {
		return attempts.ErrAttemptTerminal
	}

	if !req.Status.IsTerminal() {
		return attempts.ErrInvalidTransition
	}

	// Persist status update
	payload := formatAttemptStatusPayload(req.AttemptID, req.Status)
	logRecord := storelog.NewRecord(
		storelog.RecordTypeFinanceAttemptStatus,
		req.Now,
		identity.EntityID(record.CircleID),
		payload,
	)
	if err := s.log.Append(logRecord); err != nil && err != storelog.ErrRecordExists {
		return err
	}

	record.Status = req.Status
	record.ProviderRef = req.ProviderRef
	record.BlockedReason = req.BlockedReason
	record.MoneyMoved = req.MoneyMoved
	record.UpdatedAt = req.Now
	record.FinalizedAt = req.Now

	return nil
}

// CheckReplay determines if an execution request is a replay.
func (s *FinanceAttemptStore) CheckReplay(envelopeID, attemptID string) (*attempts.AttemptRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, exists := s.attempts[attemptID]
	if !exists {
		return nil, false
	}

	if record.EnvelopeID != envelopeID {
		return nil, false
	}

	if record.Status.IsTerminal() {
		return record, true
	}

	return nil, false
}

// ListAttempts returns all attempts for an envelope.
func (s *FinanceAttemptStore) ListAttempts(envelopeID string) []*attempts.AttemptRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	records := s.byEnvelope[envelopeID]
	result := make([]*attempts.AttemptRecord, len(records))
	copy(result, records)
	return result
}

// formatAttemptPayload formats an attempt record for storage.
func formatAttemptPayload(r *attempts.AttemptRecord) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s|%s",
		r.AttemptID,
		r.EnvelopeID,
		r.ActionHash,
		r.IdempotencyKey,
		r.CircleID,
		r.IntersectionID,
		r.TraceID,
		r.Provider,
		r.CreatedAt.Format(time.RFC3339Nano),
	)
}

// parseAttemptPayload parses an attempt record from storage.
func parseAttemptPayload(payload string) (*attempts.AttemptRecord, error) {
	parts := strings.Split(payload, "|")
	if len(parts) < 9 {
		return nil, fmt.Errorf("invalid attempt payload: expected 9 parts, got %d", len(parts))
	}

	createdAt, err := time.Parse(time.RFC3339Nano, parts[8])
	if err != nil {
		return nil, fmt.Errorf("invalid createdAt: %v", err)
	}

	return &attempts.AttemptRecord{
		AttemptID:      parts[0],
		EnvelopeID:     parts[1],
		ActionHash:     parts[2],
		IdempotencyKey: parts[3],
		CircleID:       parts[4],
		IntersectionID: parts[5],
		TraceID:        parts[6],
		Provider:       parts[7],
		Status:         attempts.AttemptStatusStarted, // Will be updated by status records
		CreatedAt:      createdAt,
		UpdatedAt:      createdAt,
	}, nil
}

// formatAttemptStatusPayload formats a status update for storage.
func formatAttemptStatusPayload(attemptID string, status attempts.AttemptStatus) string {
	return fmt.Sprintf("%s|%s", attemptID, status)
}

// parseAttemptStatusPayload parses a status update from storage.
func parseAttemptStatusPayload(payload string) (string, attempts.AttemptStatus, error) {
	parts := strings.Split(payload, "|")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid status payload")
	}
	return parts[0], attempts.AttemptStatus(parts[1]), nil
}

// Stats returns store statistics.
func (s *FinanceAttemptStore) Stats() FinanceAttemptStoreStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var terminal, inflight int
	for _, r := range s.attempts {
		if r.Status.IsTerminal() {
			terminal++
		} else if r.Status.IsInFlight() {
			inflight++
		}
	}

	return FinanceAttemptStoreStats{
		TotalAttempts:    len(s.attempts),
		TerminalAttempts: terminal,
		InFlightAttempts: inflight,
	}
}

// FinanceAttemptStoreStats contains store statistics.
type FinanceAttemptStoreStats struct {
	TotalAttempts    int
	TerminalAttempts int
	InFlightAttempts int
}

// Verify interface compliance
var _ attempts.AttemptLedger = (*FinanceAttemptStore)(nil)

// Unused field to silence static analysis
var _ = strconv.Itoa
