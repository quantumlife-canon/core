// Package attempts provides idempotency and replay defense primitives for v9.6 financial execution.
package attempts

import (
	"time"

	"quantumlife/pkg/events"
)

// AttemptLedger provides replay defense and idempotency enforcement.
//
// The ledger enforces:
// - (envelope_id, attempt_id) uniqueness
// - (envelope_id, idempotency_key) uniqueness
// - One in-flight attempt per envelope
// - Terminal attempts cannot be retried
type AttemptLedger interface {
	// StartAttempt creates a new attempt record in "started" status.
	// Returns ErrAttemptAlreadyExists if attempt_id already exists.
	// Returns ErrAttemptInFlight if another attempt for this envelope is in-flight.
	// Returns ErrIdempotencyKeyConflict if the idempotency key is already used.
	StartAttempt(req StartAttemptRequest) (*AttemptRecord, error)

	// GetAttempt retrieves an attempt by ID.
	GetAttempt(attemptID string) (*AttemptRecord, bool)

	// GetAttemptByEnvelopeAndKey retrieves an attempt by envelope and idempotency key.
	GetAttemptByEnvelopeAndKey(envelopeID, idempotencyKey string) (*AttemptRecord, bool)

	// GetInFlightAttempt returns the in-flight attempt for an envelope, if any.
	GetInFlightAttempt(envelopeID string) (*AttemptRecord, bool)

	// UpdateStatus transitions an attempt to a new status.
	// Returns ErrAttemptNotFound if the attempt doesn't exist.
	// Returns ErrAttemptTerminal if the attempt is already terminal.
	// Returns ErrInvalidTransition for invalid status transitions.
	UpdateStatus(attemptID string, status AttemptStatus, now time.Time) error

	// FinalizeAttempt marks an attempt as terminal with additional details.
	// This is called when execution completes (success or failure).
	FinalizeAttempt(req FinalizeAttemptRequest) error

	// CheckReplay determines if an execution request is a replay.
	// Returns (existing record, true) if this is a replay of a terminal attempt.
	// Returns (nil, false) if this is not a replay.
	CheckReplay(envelopeID, attemptID string) (*AttemptRecord, bool)

	// ListAttempts returns all attempts for an envelope.
	ListAttempts(envelopeID string) []*AttemptRecord
}

// StartAttemptRequest contains parameters for starting a new attempt.
type StartAttemptRequest struct {
	// AttemptID uniquely identifies this attempt.
	AttemptID string

	// EnvelopeID identifies the envelope being executed.
	EnvelopeID string

	// ActionHash is the deterministic hash of the action specification.
	ActionHash string

	// IdempotencyKey is the derived key for provider deduplication.
	IdempotencyKey string

	// CircleID is the actor circle initiating the execution.
	CircleID string

	// IntersectionID is the intersection context (if applicable).
	IntersectionID string

	// TraceID links to the execution trace for audit.
	TraceID string

	// Provider identifies which connector will be used.
	Provider string

	// Now is the current time.
	Now time.Time
}

// FinalizeAttemptRequest contains parameters for finalizing an attempt.
type FinalizeAttemptRequest struct {
	// AttemptID identifies the attempt to finalize.
	AttemptID string

	// Status is the terminal status.
	Status AttemptStatus

	// ProviderRef is the provider's reference ID (if any).
	ProviderRef string

	// BlockedReason explains why the attempt was blocked (if applicable).
	BlockedReason string

	// MoneyMoved indicates if real money was transferred.
	MoneyMoved bool

	// Now is the current time.
	Now time.Time
}

// LedgerConfig configures the attempt ledger behavior.
type LedgerConfig struct {
	// MaxAttemptsPerEnvelope limits how many attempts can be created per envelope.
	// Default is 1 (single attempt per envelope).
	MaxAttemptsPerEnvelope int

	// AllowConcurrentAttempts allows multiple in-flight attempts.
	// Default is false (one in-flight attempt at a time).
	AllowConcurrentAttempts bool
}

// DefaultLedgerConfig returns the default ledger configuration.
func DefaultLedgerConfig() LedgerConfig {
	return LedgerConfig{
		MaxAttemptsPerEnvelope:  1,
		AllowConcurrentAttempts: false,
	}
}

// InMemoryLedger implements AttemptLedger with in-memory storage.
type InMemoryLedger struct {
	config       LedgerConfig
	attempts     map[string]*AttemptRecord   // attemptID -> record
	byEnvelope   map[string][]*AttemptRecord // envelopeID -> records
	byKey        map[string]*AttemptRecord   // envelopeID:idempotencyKey -> record
	idGenerator  func() string
	auditEmitter func(event events.Event)
}

// NewInMemoryLedger creates a new in-memory attempt ledger.
func NewInMemoryLedger(config LedgerConfig, idGen func() string, emitter func(events.Event)) *InMemoryLedger {
	return &InMemoryLedger{
		config:       config,
		attempts:     make(map[string]*AttemptRecord),
		byEnvelope:   make(map[string][]*AttemptRecord),
		byKey:        make(map[string]*AttemptRecord),
		idGenerator:  idGen,
		auditEmitter: emitter,
	}
}

// StartAttempt creates a new attempt record.
func (l *InMemoryLedger) StartAttempt(req StartAttemptRequest) (*AttemptRecord, error) {
	// Check if attempt already exists
	if _, exists := l.attempts[req.AttemptID]; exists {
		l.emitEvent(events.Event{
			ID:             l.idGenerator(),
			Type:           events.EventV96LedgerDuplicateFound,
			Timestamp:      req.Now,
			CircleID:       req.CircleID,
			IntersectionID: req.IntersectionID,
			SubjectID:      req.AttemptID,
			SubjectType:    "attempt",
			TraceID:        req.TraceID,
			Metadata: map[string]string{
				"envelope_id": req.EnvelopeID,
				"reason":      "attempt_id_exists",
			},
		})
		return nil, ErrAttemptAlreadyExists
	}

	// Check for in-flight attempts
	if !l.config.AllowConcurrentAttempts {
		for _, attempt := range l.byEnvelope[req.EnvelopeID] {
			if attempt.Status.IsInFlight() {
				l.emitEvent(events.Event{
					ID:             l.idGenerator(),
					Type:           events.EventV96AttemptInflightBlocked,
					Timestamp:      req.Now,
					CircleID:       req.CircleID,
					IntersectionID: req.IntersectionID,
					SubjectID:      req.AttemptID,
					SubjectType:    "attempt",
					TraceID:        req.TraceID,
					Metadata: map[string]string{
						"envelope_id":      req.EnvelopeID,
						"inflight_attempt": attempt.AttemptID,
						"inflight_status":  string(attempt.Status),
						"reason":           "inflight_policy",
					},
				})
				return nil, ErrAttemptInFlight
			}
		}
	}

	// Check for replay of terminal attempts
	for _, attempt := range l.byEnvelope[req.EnvelopeID] {
		if attempt.Status.IsTerminal() {
			// Check if this is trying to reuse the same attempt ID
			if attempt.AttemptID == req.AttemptID {
				l.emitEvent(events.Event{
					ID:             l.idGenerator(),
					Type:           events.EventV96AttemptReplayBlocked,
					Timestamp:      req.Now,
					CircleID:       req.CircleID,
					IntersectionID: req.IntersectionID,
					SubjectID:      req.AttemptID,
					SubjectType:    "attempt",
					TraceID:        req.TraceID,
					Metadata: map[string]string{
						"envelope_id":     req.EnvelopeID,
						"terminal_status": string(attempt.Status),
						"reason":          "replay_same_attempt_id",
					},
				})
				return nil, ErrAttemptReplay
			}
		}
	}

	// Check idempotency key uniqueness
	keyIndex := req.EnvelopeID + ":" + req.IdempotencyKey
	if existing, exists := l.byKey[keyIndex]; exists {
		l.emitEvent(events.Event{
			ID:             l.idGenerator(),
			Type:           events.EventV96LedgerDuplicateFound,
			Timestamp:      req.Now,
			CircleID:       req.CircleID,
			IntersectionID: req.IntersectionID,
			SubjectID:      req.AttemptID,
			SubjectType:    "attempt",
			TraceID:        req.TraceID,
			Metadata: map[string]string{
				"envelope_id":        req.EnvelopeID,
				"existing_attempt":   existing.AttemptID,
				"idempotency_prefix": IdempotencyKeyPrefix(req.IdempotencyKey),
				"reason":             "idempotency_key_conflict",
			},
		})
		return nil, ErrIdempotencyKeyConflict
	}

	// Check max attempts per envelope
	if len(l.byEnvelope[req.EnvelopeID]) >= l.config.MaxAttemptsPerEnvelope {
		// Allow if all previous attempts are terminal (user explicitly requested new attempt)
		allTerminal := true
		for _, attempt := range l.byEnvelope[req.EnvelopeID] {
			if !attempt.Status.IsTerminal() {
				allTerminal = false
				break
			}
		}
		if !allTerminal {
			return nil, ErrAttemptInFlight
		}
	}

	// Create the record
	record := &AttemptRecord{
		AttemptID:      req.AttemptID,
		EnvelopeID:     req.EnvelopeID,
		ActionHash:     req.ActionHash,
		IdempotencyKey: req.IdempotencyKey,
		CircleID:       req.CircleID,
		IntersectionID: req.IntersectionID,
		TraceID:        req.TraceID,
		Status:         AttemptStatusStarted,
		Provider:       req.Provider,
		CreatedAt:      req.Now,
		UpdatedAt:      req.Now,
	}

	// Store the record
	l.attempts[req.AttemptID] = record
	l.byEnvelope[req.EnvelopeID] = append(l.byEnvelope[req.EnvelopeID], record)
	l.byKey[keyIndex] = record

	// Emit event
	l.emitEvent(events.Event{
		ID:             l.idGenerator(),
		Type:           events.EventV96LedgerEntryCreated,
		Timestamp:      req.Now,
		CircleID:       req.CircleID,
		IntersectionID: req.IntersectionID,
		SubjectID:      req.AttemptID,
		SubjectType:    "attempt",
		TraceID:        req.TraceID,
		Provider:       req.Provider,
		Metadata: map[string]string{
			"envelope_id":        req.EnvelopeID,
			"action_hash":        safePrefix(req.ActionHash, 16) + "...",
			"idempotency_prefix": IdempotencyKeyPrefix(req.IdempotencyKey),
			"status":             string(AttemptStatusStarted),
		},
	})

	return record, nil
}

// GetAttempt retrieves an attempt by ID.
func (l *InMemoryLedger) GetAttempt(attemptID string) (*AttemptRecord, bool) {
	record, exists := l.attempts[attemptID]
	return record, exists
}

// GetAttemptByEnvelopeAndKey retrieves an attempt by envelope and idempotency key.
func (l *InMemoryLedger) GetAttemptByEnvelopeAndKey(envelopeID, idempotencyKey string) (*AttemptRecord, bool) {
	keyIndex := envelopeID + ":" + idempotencyKey
	record, exists := l.byKey[keyIndex]
	return record, exists
}

// GetInFlightAttempt returns the in-flight attempt for an envelope, if any.
func (l *InMemoryLedger) GetInFlightAttempt(envelopeID string) (*AttemptRecord, bool) {
	for _, attempt := range l.byEnvelope[envelopeID] {
		if attempt.Status.IsInFlight() {
			return attempt, true
		}
	}
	return nil, false
}

// UpdateStatus transitions an attempt to a new status.
func (l *InMemoryLedger) UpdateStatus(attemptID string, status AttemptStatus, now time.Time) error {
	record, exists := l.attempts[attemptID]
	if !exists {
		return ErrAttemptNotFound
	}

	if record.Status.IsTerminal() {
		return ErrAttemptTerminal
	}

	// Validate transition
	if !isValidTransition(record.Status, status) {
		return ErrInvalidTransition
	}

	record.Status = status
	record.UpdatedAt = now

	l.emitEvent(events.Event{
		ID:             l.idGenerator(),
		Type:           events.EventV96LedgerEntryUpdated,
		Timestamp:      now,
		CircleID:       record.CircleID,
		IntersectionID: record.IntersectionID,
		SubjectID:      attemptID,
		SubjectType:    "attempt",
		TraceID:        record.TraceID,
		Provider:       record.Provider,
		Metadata: map[string]string{
			"envelope_id": record.EnvelopeID,
			"status":      string(status),
		},
	})

	return nil
}

// FinalizeAttempt marks an attempt as terminal with additional details.
func (l *InMemoryLedger) FinalizeAttempt(req FinalizeAttemptRequest) error {
	record, exists := l.attempts[req.AttemptID]
	if !exists {
		return ErrAttemptNotFound
	}

	if record.Status.IsTerminal() {
		return ErrAttemptTerminal
	}

	if !req.Status.IsTerminal() {
		return ErrInvalidTransition
	}

	record.Status = req.Status
	record.ProviderRef = req.ProviderRef
	record.BlockedReason = req.BlockedReason
	record.MoneyMoved = req.MoneyMoved
	record.UpdatedAt = req.Now
	record.FinalizedAt = req.Now

	l.emitEvent(events.Event{
		ID:             l.idGenerator(),
		Type:           events.EventV96AttemptFinalized,
		Timestamp:      req.Now,
		CircleID:       record.CircleID,
		IntersectionID: record.IntersectionID,
		SubjectID:      req.AttemptID,
		SubjectType:    "attempt",
		TraceID:        record.TraceID,
		Provider:       record.Provider,
		Metadata: map[string]string{
			"envelope_id":    record.EnvelopeID,
			"status":         string(req.Status),
			"provider_ref":   req.ProviderRef,
			"money_moved":    boolToString(req.MoneyMoved),
			"blocked_reason": req.BlockedReason,
		},
	})

	return nil
}

// CheckReplay determines if an execution request is a replay.
func (l *InMemoryLedger) CheckReplay(envelopeID, attemptID string) (*AttemptRecord, bool) {
	record, exists := l.attempts[attemptID]
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
func (l *InMemoryLedger) ListAttempts(envelopeID string) []*AttemptRecord {
	attempts := l.byEnvelope[envelopeID]
	result := make([]*AttemptRecord, len(attempts))
	copy(result, attempts)
	return result
}

// emitEvent emits an audit event if emitter is configured.
func (l *InMemoryLedger) emitEvent(event events.Event) {
	if l.auditEmitter != nil {
		l.auditEmitter(event)
	}
}

// isValidTransition checks if a status transition is allowed.
func isValidTransition(from, to AttemptStatus) bool {
	switch from {
	case AttemptStatusStarted:
		return to == AttemptStatusPrepared || to.IsTerminal()
	case AttemptStatusPrepared:
		return to == AttemptStatusInvoked || to.IsTerminal()
	case AttemptStatusInvoked:
		return to.IsTerminal()
	default:
		return false
	}
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// safePrefix returns at most n characters from s, handling short strings.
func safePrefix(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// Verify interface compliance.
var _ AttemptLedger = (*InMemoryLedger)(nil)
