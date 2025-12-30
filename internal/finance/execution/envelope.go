package execution

import (
	"fmt"
	"time"
)

// EnvelopeBuilder constructs ExecutionEnvelopes.
// Per Technical Split v9 §4, envelopes are immutable once sealed.
type EnvelopeBuilder struct {
	idGenerator func() string
}

// NewEnvelopeBuilder creates a new envelope builder.
func NewEnvelopeBuilder(idGen func() string) *EnvelopeBuilder {
	return &EnvelopeBuilder{
		idGenerator: idGen,
	}
}

// BuildRequest contains parameters for building an envelope.
type BuildRequest struct {
	// Intent is the execution intent.
	Intent ExecutionIntent

	// AmountCap is the maximum amount allowed.
	AmountCap int64

	// FrequencyCap is the maximum frequency (1 for single execution).
	FrequencyCap int

	// DurationCap is the maximum duration of authority.
	DurationCap time.Duration

	// Expiry is when the envelope expires.
	Expiry time.Time

	// ApprovalThreshold is the required approval count.
	ApprovalThreshold int

	// RevocationWindowDuration is how long the revocation window lasts.
	RevocationWindowDuration time.Duration

	// RevocationWaived is true only if explicitly waived.
	RevocationWaived bool

	// TraceID is the correlation ID.
	TraceID string
}

// Build creates a sealed ExecutionEnvelope.
// The envelope is immutable after this call.
func (b *EnvelopeBuilder) Build(req BuildRequest, now time.Time) (*ExecutionEnvelope, error) {
	if err := b.validateRequest(req); err != nil {
		return nil, err
	}

	actionHash := ComputeActionHash(req.Intent)

	env := &ExecutionEnvelope{
		EnvelopeID:     b.idGenerator(),
		ActorCircleID:  req.Intent.CircleID,
		IntersectionID: req.Intent.IntersectionID,
		ViewHash:       req.Intent.ViewHash,
		ActionHash:     actionHash,
		ActionSpec: ActionSpec{
			Type:        req.Intent.ActionType,
			AmountCents: req.Intent.AmountCents,
			Currency:    req.Intent.Currency,
			Recipient:   req.Intent.Recipient,
			Description: req.Intent.Description,
		},
		AmountCap:         req.AmountCap,
		FrequencyCap:      req.FrequencyCap,
		DurationCap:       req.DurationCap,
		Expiry:            req.Expiry,
		Approvals:         []ApprovalArtifact{},
		ApprovalThreshold: req.ApprovalThreshold,
		RevocationWaived:  req.RevocationWaived,
		TraceID:           req.TraceID,
		SealedAt:          now,
	}

	// Set revocation window
	if !req.RevocationWaived {
		env.RevocationWindowStart = now
		env.RevocationWindowEnd = now.Add(req.RevocationWindowDuration)
	}

	// Compute seal hash (makes envelope immutable)
	env.SealHash = ComputeSealHash(env)

	return env, nil
}

// validateRequest validates the build request.
func (b *EnvelopeBuilder) validateRequest(req BuildRequest) error {
	if req.Intent.IntentID == "" {
		return fmt.Errorf("intent ID required")
	}
	if req.Intent.CircleID == "" {
		return fmt.Errorf("circle ID required")
	}
	if req.Intent.ViewHash == "" {
		return fmt.Errorf("view hash required (must reference v8 view)")
	}
	if req.AmountCap <= 0 {
		return fmt.Errorf("amount cap must be positive")
	}
	if req.Intent.AmountCents > req.AmountCap {
		return fmt.Errorf("amount %d exceeds cap %d", req.Intent.AmountCents, req.AmountCap)
	}
	if req.FrequencyCap <= 0 {
		return fmt.Errorf("frequency cap must be positive")
	}
	if req.ApprovalThreshold <= 0 {
		return fmt.Errorf("approval threshold must be positive")
	}
	if req.TraceID == "" {
		return fmt.Errorf("trace ID required for audit")
	}
	if !req.RevocationWaived && req.RevocationWindowDuration <= 0 {
		return fmt.Errorf("revocation window duration required unless waived")
	}
	return nil
}

// IsExpired returns true if the envelope has expired.
func (env *ExecutionEnvelope) IsExpired(now time.Time) bool {
	return now.After(env.Expiry)
}

// IsRevoked returns true if the envelope has been revoked.
func (env *ExecutionEnvelope) IsRevoked() bool {
	return env.Revoked
}

// HasSufficientApprovals returns true if approval threshold is met.
func (env *ExecutionEnvelope) HasSufficientApprovals() bool {
	validCount := 0
	for _, a := range env.Approvals {
		if a.ActionHash == env.ActionHash {
			validCount++
		}
	}
	return validCount >= env.ApprovalThreshold
}

// IsInRevocationWindow returns true if currently in revocation window.
func (env *ExecutionEnvelope) IsInRevocationWindow(now time.Time) bool {
	if env.RevocationWaived {
		return false
	}
	return now.After(env.RevocationWindowStart) && now.Before(env.RevocationWindowEnd)
}

// RevocationWindowClosed returns true if revocation window has closed.
func (env *ExecutionEnvelope) RevocationWindowClosed(now time.Time) bool {
	if env.RevocationWaived {
		return true
	}
	return now.After(env.RevocationWindowEnd)
}
