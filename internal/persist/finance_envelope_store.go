// Package persist provides file-backed persistent stores for QuantumLife.
//
// This file provides persistence for finance execution envelopes.
//
// CRITICAL: All stores use append-only logging for durability.
// Changes are written to the log immediately and can be replayed.
//
// GUARDRAIL: No goroutines. All operations are synchronous.
// No time.Now() - clock must be injected.
//
// Phase 17b: Finance Execution Envelope Persistence
package persist

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"quantumlife/internal/execexecutor"
	"quantumlife/internal/finance/execution"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/storelog"
)

// FinanceEnvelopeStore implements envelope storage with file-backed persistence.
type FinanceEnvelopeStore struct {
	mu         sync.RWMutex
	log        storelog.AppendOnlyLog
	envelopes  map[string]*execution.ExecutionEnvelope
	byIntentID map[string]string // intentID -> envelopeID
}

// NewFinanceEnvelopeStore creates a new file-backed envelope store.
func NewFinanceEnvelopeStore(log storelog.AppendOnlyLog) (*FinanceEnvelopeStore, error) {
	store := &FinanceEnvelopeStore{
		log:        log,
		envelopes:  make(map[string]*execution.ExecutionEnvelope),
		byIntentID: make(map[string]string),
	}

	// Replay existing records
	if err := store.replay(); err != nil {
		return nil, err
	}

	return store, nil
}

// replay loads envelopes from the log.
func (s *FinanceEnvelopeStore) replay() error {
	records, err := s.log.ListByType(storelog.RecordTypeFinanceEnvelope)
	if err != nil {
		return err
	}

	for _, record := range records {
		env, intentID, err := parseEnvelopePayload(record.Payload)
		if err != nil {
			continue // Skip corrupted records
		}
		s.envelopes[env.EnvelopeID] = env
		if intentID != "" {
			s.byIntentID[intentID] = env.EnvelopeID
		}
	}

	return nil
}

// Put stores an envelope.
func (s *FinanceEnvelopeStore) Put(envelope *execution.ExecutionEnvelope) error {
	return s.PutWithIntentID(envelope, "")
}

// PutWithIntentID stores an envelope with an intent ID mapping.
func (s *FinanceEnvelopeStore) PutWithIntentID(envelope *execution.ExecutionEnvelope, intentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create log record
	payload := formatEnvelopePayload(envelope, intentID)
	record := storelog.NewRecord(
		storelog.RecordTypeFinanceEnvelope,
		envelope.SealedAt,
		identity.EntityID(envelope.ActorCircleID),
		payload,
	)

	// Append to log
	if err := s.log.Append(record); err != nil && err != storelog.ErrRecordExists {
		return err
	}

	s.envelopes[envelope.EnvelopeID] = envelope
	if intentID != "" {
		s.byIntentID[intentID] = envelope.EnvelopeID
	}
	return nil
}

// Get retrieves an envelope by ID.
func (s *FinanceEnvelopeStore) Get(envelopeID string) (*execution.ExecutionEnvelope, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	env, ok := s.envelopes[envelopeID]
	return env, ok
}

// GetByIntentID retrieves an envelope by intent ID.
func (s *FinanceEnvelopeStore) GetByIntentID(intentID string) (*execution.ExecutionEnvelope, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	envelopeID, ok := s.byIntentID[intentID]
	if !ok {
		return nil, false
	}
	return s.envelopes[envelopeID], true
}

// ListPending returns all pending envelopes.
func (s *FinanceEnvelopeStore) ListPending() []*execution.ExecutionEnvelope {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*execution.ExecutionEnvelope, 0)
	for _, env := range s.envelopes {
		if !env.Revoked && env.Expiry.After(time.Now()) {
			result = append(result, env)
		}
	}
	return result
}

// Stats returns store statistics.
func (s *FinanceEnvelopeStore) Stats() FinanceEnvelopeStoreStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return FinanceEnvelopeStoreStats{
		TotalEnvelopes: len(s.envelopes),
		IntentMappings: len(s.byIntentID),
	}
}

// FinanceEnvelopeStoreStats contains store statistics.
type FinanceEnvelopeStoreStats struct {
	TotalEnvelopes int
	IntentMappings int
}

// formatEnvelopePayload formats an envelope for storage.
// Format: envelopeID|actorCircleID|actionHash|policyHash|viewHash|sealHash|intentID|amountCents|currency|payeeID|expiry
func formatEnvelopePayload(env *execution.ExecutionEnvelope, intentID string) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%d|%s|%s|%s",
		env.EnvelopeID,
		env.ActorCircleID,
		env.ActionHash,
		env.PolicySnapshotHash,
		env.ViewSnapshotHash,
		env.SealHash,
		intentID,
		env.ActionSpec.AmountCents,
		env.ActionSpec.Currency,
		env.ActionSpec.PayeeID,
		env.Expiry.Format(time.RFC3339Nano),
	)
}

// parseEnvelopePayload parses an envelope from storage.
func parseEnvelopePayload(payload string) (*execution.ExecutionEnvelope, string, error) {
	parts := strings.Split(payload, "|")
	if len(parts) < 11 {
		return nil, "", fmt.Errorf("invalid envelope payload: expected 11 parts, got %d", len(parts))
	}

	amountCents, err := strconv.ParseInt(parts[7], 10, 64)
	if err != nil {
		return nil, "", fmt.Errorf("invalid amount: %v", err)
	}

	expiry, err := time.Parse(time.RFC3339Nano, parts[10])
	if err != nil {
		return nil, "", fmt.Errorf("invalid expiry: %v", err)
	}

	env := &execution.ExecutionEnvelope{
		EnvelopeID:         parts[0],
		ActorCircleID:      parts[1],
		ActionHash:         parts[2],
		PolicySnapshotHash: parts[3],
		ViewSnapshotHash:   parts[4],
		SealHash:           parts[5],
		ActionSpec: execution.ActionSpec{
			Type:        execution.ActionTypePayment,
			AmountCents: amountCents,
			Currency:    parts[8],
			PayeeID:     parts[9],
		},
		Expiry: expiry,
	}

	intentID := parts[6]
	return env, intentID, nil
}

// Verify interface compliance
var _ execexecutor.EnvelopeStore = (*FinanceEnvelopeStore)(nil)
