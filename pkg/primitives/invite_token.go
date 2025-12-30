// Package primitives defines the immutable data structures for all canon primitives.
//
// This file defines the InviteToken for intersection creation.
// Reference: docs/QUANTUMLIFE_CANON_V1.md §Intersections
package primitives

import (
	"fmt"
	"time"
)

// InviteToken represents a cryptographically signed invitation to create an intersection.
// The token is issued by one circle and can be accepted by another to form an intersection.
//
// Canon Reference: docs/QUANTUMLIFE_CANON_V1.md §Intersections
type InviteToken struct {
	// TokenID uniquely identifies this invite token.
	TokenID string

	// IssuerCircleID is the circle that issued the invitation.
	IssuerCircleID string

	// TargetCircleID is the intended recipient (optional, "" means any circle can accept).
	TargetCircleID string

	// ProposedName is the human-readable name for the intersection.
	ProposedName string

	// Template contains the proposed intersection terms.
	Template IntersectionTemplate

	// IssuedAt is when the token was created.
	IssuedAt time.Time

	// ExpiresAt is when the token expires and can no longer be accepted.
	ExpiresAt time.Time

	// Signature is the cryptographic signature over the token payload.
	Signature []byte

	// SignatureKeyID identifies the key used to create the signature.
	SignatureKeyID string

	// SignatureAlgorithm identifies the algorithm used (for algorithm agility).
	SignatureAlgorithm string
}

// IntersectionTemplate contains proposed terms for an intersection contract.
type IntersectionTemplate struct {
	// Scopes define the capabilities available within the intersection.
	Scopes []IntersectionScope

	// Ceilings define limits on operations within the intersection.
	Ceilings []IntersectionCeiling

	// Governance defines rules for changing the contract.
	Governance IntersectionGovernance
}

// IntersectionScope represents a capability granted within an intersection.
type IntersectionScope struct {
	// Name is the scope identifier (e.g., "calendar:read", "calendar:write").
	Name string

	// Description explains what this scope allows.
	Description string

	// Permission is the access level: "read", "write", "execute", "delegate".
	Permission string
}

// IntersectionCeiling represents a limit within an intersection.
type IntersectionCeiling struct {
	// Type identifies the ceiling type (e.g., "time_window", "duration", "spend").
	Type string

	// Value is the ceiling value.
	Value string

	// Unit is the unit of measurement (e.g., "hours", "USD", "days").
	Unit string

	// Description explains this ceiling.
	Description string
}

// IntersectionGovernance defines rules for intersection management.
type IntersectionGovernance struct {
	// AmendmentRequires specifies who must agree to changes.
	// Values: "all_parties", "majority", "initiator_only"
	AmendmentRequires string

	// DissolutionPolicy specifies how the intersection can be ended.
	// Values: "any_party", "all_parties", "initiator_only"
	DissolutionPolicy string

	// MinNoticePeriod is the minimum notice before dissolution (optional).
	MinNoticePeriod time.Duration
}

// SigningPayload returns the bytes to be signed for this token.
// Excludes the signature fields themselves.
func (t *InviteToken) SigningPayload() []byte {
	// Create deterministic payload for signing
	payload := fmt.Sprintf(
		"INVITE_TOKEN_V1|%s|%s|%s|%s|%d|%d|%v",
		t.TokenID,
		t.IssuerCircleID,
		t.TargetCircleID,
		t.ProposedName,
		t.IssuedAt.Unix(),
		t.ExpiresAt.Unix(),
		t.Template,
	)
	return []byte(payload)
}

// Validate checks that the token has all required fields.
func (t *InviteToken) Validate() error {
	if t.TokenID == "" {
		return ErrMissingTokenID
	}
	if t.IssuerCircleID == "" {
		return ErrMissingIssuer
	}
	if t.IssuedAt.IsZero() {
		return ErrMissingTimestamp
	}
	if t.ExpiresAt.IsZero() {
		return ErrMissingExpiry
	}
	if len(t.Signature) == 0 {
		return ErrMissingSignature
	}
	if t.SignatureKeyID == "" {
		return ErrMissingKeyID
	}
	if t.SignatureAlgorithm == "" {
		return ErrMissingAlgorithm
	}
	return nil
}

// IsExpired checks if the token has expired.
func (t *InviteToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// CanBeAcceptedBy checks if the given circle can accept this token.
func (t *InviteToken) CanBeAcceptedBy(circleID string) bool {
	// If no target specified, anyone can accept
	if t.TargetCircleID == "" {
		return true
	}
	return t.TargetCircleID == circleID
}

// InviteToken validation errors.
var (
	ErrMissingTokenID       = tokenError("missing token id")
	ErrMissingExpiry        = tokenError("missing expiry")
	ErrMissingSignature     = tokenError("missing signature")
	ErrMissingKeyID         = tokenError("missing key id")
	ErrMissingAlgorithm     = tokenError("missing algorithm")
	ErrTokenExpired         = tokenError("token expired")
	ErrInvalidSignature     = tokenError("invalid signature")
	ErrUnauthorizedAcceptor = tokenError("circle not authorized to accept this token")
)

type tokenError string

func (e tokenError) Error() string { return string(e) }
