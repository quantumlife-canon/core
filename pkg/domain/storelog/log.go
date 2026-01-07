// Package storelog provides append-only log storage for QuantumLife.
//
// CRITICAL: This package is append-only. Records are NEVER modified or deleted.
// Each record is written as a canonical line: TYPE|VERSION|TS|HASH|PAYLOAD
//
// GUARDRAIL: This package does NOT spawn goroutines. All operations are synchronous.
// No time.Now() calls - clock must be injected.
//
// Reference: docs/ADR/ADR-0027-phase12-persistence-replay.md
package storelog

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"quantumlife/pkg/domain/identity"
)

// Schema version for log records
const SchemaVersion = "v1"

// Record types
const (
	RecordTypeEvent    = "EVENT"
	RecordTypeDraft    = "DRAFT"
	RecordTypeApproval = "APPROVAL"
	RecordTypeFeedback = "FEEDBACK"
	RecordTypeRun      = "RUN"

	// Identity graph record types (Phase 13)
	RecordTypeIdentityEntity = "IDENTITY_ENTITY_UPSERT"
	RecordTypeIdentityEdge   = "IDENTITY_EDGE_UPSERT"

	// Policy and suppression record types (Phase 14)
	RecordTypePolicySet      = "POLICY_SET"
	RecordTypeSuppressionAdd = "SUPPRESSION_ADD"
	RecordTypeSuppressionRem = "SUPPRESSION_REM"

	// Intersection and approval record types (Phase 15)
	RecordTypeIntersectionPolicy  = "INTERSECTION_POLICY"
	RecordTypeApprovalStateCreate = "APPROVAL_STATE_CREATE"
	RecordTypeApprovalStateRecord = "APPROVAL_STATE_RECORD"
	RecordTypeApprovalTokenCreate = "APPROVAL_TOKEN_CREATE"
	RecordTypeApprovalTokenRevoke = "APPROVAL_TOKEN_REVOKE"

	// Notification record types (Phase 16)
	RecordTypeNotificationPlanned    = "NOTIFICATION_PLANNED"
	RecordTypeNotificationDelivered  = "NOTIFICATION_DELIVERED"
	RecordTypeNotificationSuppressed = "NOTIFICATION_SUPPRESSED"
	RecordTypeNotificationExpired    = "NOTIFICATION_EXPIRED"
	RecordTypeNotifyEnvelope         = "NOTIFY_ENVELOPE"
	RecordTypeNotifyBadge            = "NOTIFY_BADGE"
	RecordTypeNotifyDigest           = "NOTIFY_DIGEST"

	// Finance execution record types (Phase 17b)
	RecordTypeFinanceEnvelope       = "FINANCE_ENVELOPE"
	RecordTypeFinanceEnvelopeStatus = "FINANCE_ENVELOPE_STATUS"
	RecordTypeFinanceAttempt        = "FINANCE_ATTEMPT"
	RecordTypeFinanceAttemptStatus  = "FINANCE_ATTEMPT_STATUS"

	// Connection record types (Phase 18.6)
	RecordTypeConnectionIntent = "CONNECTION_INTENT"

	// Shadow LLM record types (Phase 19)
	// CRITICAL: These records contain METADATA ONLY - never content.
	RecordTypeShadowLLMRun    = "SHADOWLLM_RUN"
	RecordTypeShadowLLMSignal = "SHADOWLLM_SIGNAL"

	// Phase 19.2: Shadow Receipt record type
	// CRITICAL: Contains ONLY abstract data (buckets, hashes) - never content.
	RecordTypeShadowLLMReceipt = "SHADOWLLM_RECEIPT"

	// Phase 19.4: Shadow Diff and Calibration record types
	// CRITICAL: Contains ONLY abstract data (buckets, hashes) - never content.
	RecordTypeShadowDiff        = "SHADOW_DIFF"
	RecordTypeShadowCalibration = "SHADOW_CALIBRATION"

	// Phase 25: Undoable Execution record types
	// CRITICAL: Contains ONLY hashes and enums - never identifiers.
	RecordTypeUndoExecRecord = "UNDO_EXEC_RECORD"
	RecordTypeUndoExecAck    = "UNDO_EXEC_ACK"

	// Phase 26A: Guided Journey record types
	// CRITICAL: Contains ONLY hashes and period keys - never identifiers.
	RecordTypeJourneyDismissal = "JOURNEY_DISMISSAL"

	// Phase 26B: First Five Minutes Proof record types
	// CRITICAL: Contains ONLY hashes, abstract signals, and period keys - never identifiers.
	// This is NOT analytics. This is narrative proof.
	RecordTypeFirstMinutesSummary   = "FIRST_MINUTES_SUMMARY"
	RecordTypeFirstMinutesDismissal = "FIRST_MINUTES_DISMISSAL"

	// Phase 26C: Connected Reality Check record types
	// CRITICAL: Contains ONLY hashes and period keys - never identifiers.
	// This is NOT analytics. This is a trust proof page.
	RecordTypeRealityAck = "REALITY_ACK"

	// Phase 27: Real Shadow Receipt (Primary Proof) record types
	// CRITICAL: Contains ONLY hashes - never identifiers.
	// CRITICAL: Vote does NOT change behavior.
	// CRITICAL: Vote feeds Phase 19 calibration only.
	RecordTypeShadowReceiptAck  = "SHADOW_RECEIPT_ACK"
	RecordTypeShadowReceiptVote = "SHADOW_RECEIPT_VOTE"

	// Phase 28: Trust Kept record types
	// CRITICAL: Contains ONLY hashes - never identifiers.
	// CRITICAL: First and only trust-confirming real action.
	// CRITICAL: After execution: silence forever.
	RecordTypeTrustActionReceipt = "TRUST_ACTION_RECEIPT"
	RecordTypeTrustActionUpdate  = "TRUST_ACTION_UPDATE"

	// Phase 29: TrueLayer Read-Only Connect + Finance Mirror Proof record types
	// CRITICAL: Contains ONLY hashes and abstract buckets - never raw data.
	// CRITICAL: No amounts, no merchants, no account numbers, no identifiers.
	RecordTypeTrueLayerConnection = "TRUELAYER_CONNECTION"
	RecordTypeFinanceSyncReceipt  = "FINANCE_SYNC_RECEIPT"
	RecordTypeFinanceMirrorAck    = "FINANCE_MIRROR_ACK"

	// Phase 30A: Identity + Replay record types
	// CRITICAL: Contains ONLY hashes and fingerprints - never raw keys or identifiers.
	// CRITICAL: Bounded: max 5 devices per circle.
	RecordTypeCircleBinding = "CIRCLE_BINDING"

	// Phase 31: Commerce Observers record types
	// CRITICAL: Contains ONLY buckets and hashes - never amounts, merchants, or timestamps.
	// CRITICAL: Default outcome: NOTHING SHOWN. Commerce is observed. Nothing else.
	RecordTypeCommerceObservation = "COMMERCE_OBSERVATION"
)

// Common errors
var (
	ErrRecordExists   = errors.New("record already exists")
	ErrRecordNotFound = errors.New("record not found")
	ErrInvalidRecord  = errors.New("invalid record format")
	ErrHashMismatch   = errors.New("hash mismatch")
	ErrLogCorrupted   = errors.New("log corrupted")
)

// LogRecord represents a single record in the append-only log.
type LogRecord struct {
	// Type is the record type (EVENT, DRAFT, APPROVAL, etc.)
	Type string

	// Version is the schema version (v1, v2, etc.)
	Version string

	// Timestamp is when the record was created (UTC).
	Timestamp time.Time

	// Hash is the SHA256 hash of the canonical payload.
	Hash string

	// Payload is the canonical string representation of the data.
	Payload string

	// CircleID is the circle this record belongs to (optional).
	CircleID identity.EntityID
}

// ComputeHash computes the SHA256 hash of the payload.
func (r *LogRecord) ComputeHash() string {
	h := sha256.Sum256([]byte(r.Payload))
	return hex.EncodeToString(h[:])
}

// Validate checks if the record is valid.
func (r *LogRecord) Validate() error {
	if r.Type == "" {
		return errors.New("record type is required")
	}
	if r.Version == "" {
		return errors.New("record version is required")
	}
	if r.Payload == "" {
		return errors.New("record payload is required")
	}
	if r.Hash == "" {
		return errors.New("record hash is required")
	}

	// Verify hash matches payload
	computed := r.ComputeHash()
	if computed != r.Hash {
		return ErrHashMismatch
	}

	return nil
}

// ToCanonicalLine converts the record to a canonical line format.
// Format: TYPE|VERSION|TS|HASH|CIRCLE_ID|PAYLOAD
func (r *LogRecord) ToCanonicalLine() string {
	var b strings.Builder
	b.WriteString(r.Type)
	b.WriteString("|")
	b.WriteString(r.Version)
	b.WriteString("|")
	b.WriteString(r.Timestamp.UTC().Format(time.RFC3339Nano))
	b.WriteString("|")
	b.WriteString(r.Hash)
	b.WriteString("|")
	b.WriteString(string(r.CircleID))
	b.WriteString("|")
	b.WriteString(r.Payload)
	return b.String()
}

// ParseCanonicalLine parses a canonical line into a LogRecord.
func ParseCanonicalLine(line string) (*LogRecord, error) {
	// Split by pipe, but only first 5 pipes (payload may contain pipes)
	parts := splitN(line, "|", 6)
	if len(parts) < 6 {
		return nil, ErrInvalidRecord
	}

	ts, err := time.Parse(time.RFC3339Nano, parts[2])
	if err != nil {
		return nil, ErrInvalidRecord
	}

	record := &LogRecord{
		Type:      parts[0],
		Version:   parts[1],
		Timestamp: ts,
		Hash:      parts[3],
		CircleID:  identity.EntityID(parts[4]),
		Payload:   parts[5],
	}

	// Validate hash
	if err := record.Validate(); err != nil {
		return nil, err
	}

	return record, nil
}

// splitN splits a string by separator, returning at most n parts.
// The last part contains the remainder (may include separators).
func splitN(s, sep string, n int) []string {
	if n <= 0 {
		return nil
	}

	result := make([]string, 0, n)
	remaining := s

	for i := 0; i < n-1; i++ {
		idx := strings.Index(remaining, sep)
		if idx < 0 {
			result = append(result, remaining)
			return result
		}
		result = append(result, remaining[:idx])
		remaining = remaining[idx+len(sep):]
	}

	// Last part gets the remainder
	result = append(result, remaining)
	return result
}

// AppendOnlyLog is the interface for append-only log storage.
type AppendOnlyLog interface {
	// Append adds a new record to the log.
	// Returns ErrRecordExists if a record with the same hash already exists.
	Append(record *LogRecord) error

	// Contains checks if a record with the given hash exists.
	Contains(hash string) bool

	// Get retrieves a record by hash.
	Get(hash string) (*LogRecord, error)

	// List returns all records in append order.
	List() ([]*LogRecord, error)

	// ListByType returns all records of a given type.
	ListByType(recordType string) ([]*LogRecord, error)

	// ListByCircle returns all records for a given circle.
	ListByCircle(circleID identity.EntityID) ([]*LogRecord, error)

	// Count returns the total number of records.
	Count() int

	// Verify checks that all records have valid hashes.
	Verify() error

	// Flush ensures all records are persisted to disk.
	Flush() error
}

// NewRecord creates a new LogRecord with computed hash.
func NewRecord(recordType string, ts time.Time, circleID identity.EntityID, payload string) *LogRecord {
	record := &LogRecord{
		Type:      recordType,
		Version:   SchemaVersion,
		Timestamp: ts,
		CircleID:  circleID,
		Payload:   payload,
	}
	record.Hash = record.ComputeHash()
	return record
}
