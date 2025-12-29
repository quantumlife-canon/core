// Package authority validates authority grants and enforces policy boundaries.
// All operations in this package are deterministic — no LLM/SLM involvement.
//
// Canon Reference: docs/QUANTUMLIFE_CANON_V1.md §Ontology (Authority Grant)
// Technical Split Reference: docs/TECHNICAL_SPLIT_V1.md §3.3 Authority & Policy Engine
package authority

import (
	"context"

	"quantumlife/pkg/primitives"
)

// Validator validates authority for operations.
// All validation is deterministic — no side effects.
type Validator interface {
	// Validate checks if an operation is authorized.
	// Returns nil if authorized, error with reason otherwise.
	Validate(ctx context.Context, req ValidationRequest) error

	// ValidateScopes checks if all required scopes are granted.
	ValidateScopes(ctx context.Context, grantID string, requiredScopes []string) error

	// ValidateCeilings checks if an operation is within ceilings.
	ValidateCeilings(ctx context.Context, grantID string, operation Operation) error
}

// GrantStore provides read access to authority grants.
// Write operations are in the circle package.
type GrantStore interface {
	// Get retrieves an authority grant by ID.
	Get(ctx context.Context, grantID string) (*Grant, error)

	// GetByIntersection retrieves grants for an intersection.
	GetByIntersection(ctx context.Context, intersectionID string) ([]Grant, error)

	// GetActiveGrants retrieves all active (non-expired, non-revoked) grants for a circle.
	GetActiveGrants(ctx context.Context, circleID string) ([]Grant, error)

	// IsRevoked checks if a grant has been revoked.
	IsRevoked(ctx context.Context, grantID string) (bool, error)

	// IsExpired checks if a grant has expired.
	IsExpired(ctx context.Context, grantID string) (bool, error)
}

// ExpiryChecker monitors grant expiration.
type ExpiryChecker interface {
	// CheckExpiry checks and marks expired grants.
	CheckExpiry(ctx context.Context) ([]string, error)

	// GetExpiringGrants returns grants expiring within a duration.
	GetExpiringGrants(ctx context.Context, within string) ([]Grant, error)
}

// LoopAuthorityNegotiator provides loop-aware authority negotiation.
// Used by the orchestrator at step 3 (Authority Negotiation) of the Irreducible Loop.
type LoopAuthorityNegotiator interface {
	// NegotiateForLoop confirms or acquires authority for a loop operation.
	// Returns the grant if authority is confirmed, or details about why it was denied.
	NegotiateForLoop(ctx context.Context, loopCtx LoopContext, req NegotiationRequest) (*NegotiationResult, error)
}

// LoopContext is imported from primitives for loop threading.
type LoopContext = primitives.LoopContext

// NegotiationRequest contains a request for authority negotiation.
type NegotiationRequest struct {
	IntersectionID  string
	RequiredScopes  []string
	ActionType      string
	RequestedBy     string
	Conditions      []string
}

// NegotiationResult contains the result of authority negotiation.
type NegotiationResult struct {
	Granted            bool
	GrantID            string
	GrantedScopes      []string
	DenialReason       string
	RequiresEscalation bool
	Conditions         []string
}
