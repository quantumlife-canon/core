// Package execution provides v9 financial execution primitives.
//
// This file implements the v9.5 Presentation Gate for strengthened approval semantics.
//
// CRITICAL: Bundle MUST be presented to an approver BEFORE their approval can be accepted.
// This ensures:
// 1) All approvers explicitly received the bundle (not just assumed)
// 2) The approval references a specific presented bundle hash
// 3) Presentation has not expired
// 4) Presentation matches envelope/action hash
//
// Subordinate to:
// - docs/QUANTUMLIFE_CANON_V1.md
// - docs/CANON_ADDENDUM_V9_FINANCIAL_EXECUTION.md
// - docs/TECHNICAL_SPLIT_V9_EXECUTION.md
package execution

import (
	"fmt"
	"sync"
	"time"

	"quantumlife/pkg/events"
)

// PresentationRecord tracks when a bundle was presented to an approver.
type PresentationRecord struct {
	// RecordID uniquely identifies this presentation record.
	RecordID string

	// ApproverCircleID is the circle being presented to.
	ApproverCircleID string

	// ApproverID is the individual approver within the circle.
	ApproverID string

	// BundleHash is the ContentHash of the presented bundle.
	BundleHash string

	// EnvelopeID is the envelope this presentation is for.
	EnvelopeID string

	// ActionHash is the action hash for verification.
	ActionHash string

	// TraceID links to the execution trace.
	TraceID string

	// PresentedAt is when the bundle was presented.
	PresentedAt time.Time

	// ExpiresAt is when the presentation expires.
	ExpiresAt time.Time
}

// IsExpired returns true if the presentation has expired.
func (p *PresentationRecord) IsExpired(now time.Time) bool {
	return now.After(p.ExpiresAt)
}

// PresentationStore stores and retrieves presentation records.
type PresentationStore struct {
	mu      sync.RWMutex
	records map[string]*PresentationRecord // key: approverCircleID:bundleHash:envelopeID

	idGenerator  func() string
	auditEmitter func(event events.Event)
}

// NewPresentationStore creates a new presentation store.
func NewPresentationStore(idGen func() string, emitter func(event events.Event)) *PresentationStore {
	return &PresentationStore{
		records:      make(map[string]*PresentationRecord),
		idGenerator:  idGen,
		auditEmitter: emitter,
	}
}

// presentationKey generates a unique key for a presentation.
func presentationKey(approverCircleID, bundleHash, envelopeID string) string {
	return fmt.Sprintf("%s:%s:%s", approverCircleID, bundleHash, envelopeID)
}

// RecordPresentation records that a bundle was presented to an approver.
func (s *PresentationStore) RecordPresentation(
	approverCircleID string,
	approverID string,
	bundle *ApprovalBundle,
	envelope *ExecutionEnvelope,
	traceID string,
	expiryDuration time.Duration,
	now time.Time,
) *PresentationRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	record := &PresentationRecord{
		RecordID:         s.idGenerator(),
		ApproverCircleID: approverCircleID,
		ApproverID:       approverID,
		BundleHash:       bundle.ContentHash,
		EnvelopeID:       envelope.EnvelopeID,
		ActionHash:       envelope.ActionHash,
		TraceID:          traceID,
		PresentedAt:      now,
		ExpiresAt:        now.Add(expiryDuration),
	}

	key := presentationKey(approverCircleID, bundle.ContentHash, envelope.EnvelopeID)
	s.records[key] = record

	// Emit audit event
	if s.auditEmitter != nil {
		s.auditEmitter(events.Event{
			ID:             s.idGenerator(),
			Type:           events.EventV95ApprovalPresentationRecorded,
			Timestamp:      now,
			CircleID:       approverCircleID,
			IntersectionID: envelope.IntersectionID,
			SubjectID:      record.RecordID,
			SubjectType:    "presentation",
			TraceID:        traceID,
			Metadata: map[string]string{
				"approver_id": approverID,
				"bundle_hash": bundle.ContentHash,
				"envelope_id": envelope.EnvelopeID,
				"action_hash": envelope.ActionHash,
				"expires_at":  record.ExpiresAt.Format(time.RFC3339),
			},
		})
	}

	return record
}

// GetPresentation retrieves a presentation record.
func (s *PresentationStore) GetPresentation(approverCircleID, bundleHash, envelopeID string) (*PresentationRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := presentationKey(approverCircleID, bundleHash, envelopeID)
	record, exists := s.records[key]
	return record, exists
}

// PresentationGate verifies that bundle was presented before approval.
type PresentationGate struct {
	store        *PresentationStore
	auditEmitter func(event events.Event)
	idGenerator  func() string
}

// NewPresentationGate creates a new presentation gate.
func NewPresentationGate(store *PresentationStore, idGen func() string, emitter func(event events.Event)) *PresentationGate {
	return &PresentationGate{
		store:        store,
		auditEmitter: emitter,
		idGenerator:  idGen,
	}
}

// PresentationVerifyRequest contains parameters for verifying presentation.
type PresentationVerifyRequest struct {
	// ApproverCircleID is the approver's circle.
	ApproverCircleID string

	// BundleHash is the bundle content hash the approval references.
	BundleHash string

	// EnvelopeID is the envelope ID.
	EnvelopeID string

	// ActionHash is the expected action hash.
	ActionHash string

	// Now is the current time for expiry check.
	Now time.Time
}

// PresentationVerifyResult contains the result of presentation verification.
type PresentationVerifyResult struct {
	// Verified indicates if presentation was verified.
	Verified bool

	// BlockedReason explains why verification failed.
	BlockedReason string

	// Record is the matched presentation record (if found).
	Record *PresentationRecord
}

// VerifyPresentation checks that a bundle was presented to an approver.
func (g *PresentationGate) VerifyPresentation(req PresentationVerifyRequest) *PresentationVerifyResult {
	result := &PresentationVerifyResult{Verified: true}

	record, exists := g.store.GetPresentation(req.ApproverCircleID, req.BundleHash, req.EnvelopeID)

	if !exists {
		result.Verified = false
		result.BlockedReason = fmt.Sprintf("no presentation record for approver %s with bundle hash %s",
			req.ApproverCircleID, req.BundleHash[:16]+"...")

		if g.auditEmitter != nil {
			g.auditEmitter(events.Event{
				ID:          g.idGenerator(),
				Type:        events.EventV95ApprovalPresentationMissing,
				Timestamp:   req.Now,
				CircleID:    req.ApproverCircleID,
				SubjectID:   req.EnvelopeID,
				SubjectType: "envelope",
				Metadata: map[string]string{
					"bundle_hash": req.BundleHash,
					"reason":      "no_presentation_record",
				},
			})
		}
		return result
	}

	// Check expiry
	if record.IsExpired(req.Now) {
		result.Verified = false
		result.BlockedReason = fmt.Sprintf("presentation expired at %s", record.ExpiresAt.Format(time.RFC3339))

		if g.auditEmitter != nil {
			g.auditEmitter(events.Event{
				ID:          g.idGenerator(),
				Type:        events.EventV95ApprovalPresentationExpired,
				Timestamp:   req.Now,
				CircleID:    req.ApproverCircleID,
				SubjectID:   record.RecordID,
				SubjectType: "presentation",
				Metadata: map[string]string{
					"expired_at": record.ExpiresAt.Format(time.RFC3339),
					"checked_at": req.Now.Format(time.RFC3339),
				},
			})
		}
		return result
	}

	// Check action hash match
	if record.ActionHash != req.ActionHash {
		result.Verified = false
		result.BlockedReason = fmt.Sprintf("action hash mismatch: presentation has %s, approval references %s",
			record.ActionHash[:16]+"...", req.ActionHash[:16]+"...")

		if g.auditEmitter != nil {
			g.auditEmitter(events.Event{
				ID:          g.idGenerator(),
				Type:        events.EventV95ApprovalPresentationMissing,
				Timestamp:   req.Now,
				CircleID:    req.ApproverCircleID,
				SubjectID:   req.EnvelopeID,
				SubjectType: "envelope",
				Metadata: map[string]string{
					"reason":            "action_hash_mismatch",
					"presentation_hash": record.ActionHash,
					"approval_hash":     req.ActionHash,
				},
			})
		}
		return result
	}

	result.Record = record

	if g.auditEmitter != nil {
		g.auditEmitter(events.Event{
			ID:          g.idGenerator(),
			Type:        events.EventV95ApprovalPresentationVerified,
			Timestamp:   req.Now,
			CircleID:    req.ApproverCircleID,
			SubjectID:   record.RecordID,
			SubjectType: "presentation",
			Metadata: map[string]string{
				"bundle_hash":  req.BundleHash,
				"envelope_id":  req.EnvelopeID,
				"presented_at": record.PresentedAt.Format(time.RFC3339),
			},
		})
	}

	return result
}

// VerifyAllPresentations verifies that all approvals have corresponding presentations.
func (g *PresentationGate) VerifyAllPresentations(
	approvals []MultiPartyApprovalArtifact,
	bundle *ApprovalBundle,
	envelope *ExecutionEnvelope,
	now time.Time,
) (*AllPresentationsResult, error) {
	result := &AllPresentationsResult{
		AllVerified:       true,
		VerifiedApprovers: make([]string, 0),
		MissingApprovers:  make([]string, 0),
	}

	for _, approval := range approvals {
		verifyResult := g.VerifyPresentation(PresentationVerifyRequest{
			ApproverCircleID: approval.ApproverCircleID,
			BundleHash:       approval.BundleContentHash,
			EnvelopeID:       envelope.EnvelopeID,
			ActionHash:       approval.ActionHash,
			Now:              now,
		})

		if verifyResult.Verified {
			result.VerifiedApprovers = append(result.VerifiedApprovers, approval.ApproverCircleID)
		} else {
			result.AllVerified = false
			result.MissingApprovers = append(result.MissingApprovers, approval.ApproverCircleID)
			if result.BlockedReason == "" {
				result.BlockedReason = verifyResult.BlockedReason
			}
		}
	}

	return result, nil
}

// AllPresentationsResult contains the result of verifying all presentations.
type AllPresentationsResult struct {
	// AllVerified indicates if all approvals have matching presentations.
	AllVerified bool

	// BlockedReason explains why verification failed.
	BlockedReason string

	// VerifiedApprovers lists approvers with verified presentations.
	VerifiedApprovers []string

	// MissingApprovers lists approvers missing presentations.
	MissingApprovers []string
}

// Errors for presentation gate.
var (
	// ErrPresentationMissing is returned when no presentation exists for an approval.
	ErrPresentationMissing = presentationError("presentation missing: bundle was not presented to approver")

	// ErrPresentationExpired is returned when the presentation has expired.
	ErrPresentationExpired = presentationError("presentation expired")

	// ErrPresentationHashMismatch is returned when the approval references a different bundle hash.
	ErrPresentationHashMismatch = presentationError("presentation hash mismatch")
)

type presentationError string

func (e presentationError) Error() string { return string(e) }
