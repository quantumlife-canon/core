// Package circle provides the sovereign execution boundary for a circle.
//
// Canon Reference: docs/QUANTUMLIFE_CANON_V1.md §Ontology (Circle)
// Technical Split Reference: docs/TECHNICAL_SPLIT_V1.md §3.1 Circle Runtime
package circle

import (
	"context"
)

// Runtime defines the interface for circle runtime operations.
// This is the primary boundary for circle sovereignty.
type Runtime interface {
	// Lifecycle operations

	// Create initializes a new circle with the given identity.
	Create(ctx context.Context, req CreateRequest) (*Circle, error)

	// Get retrieves a circle by ID.
	Get(ctx context.Context, circleID string) (*Circle, error)

	// Suspend pauses a circle's operations.
	Suspend(ctx context.Context, circleID string) error

	// Resume restarts a suspended circle.
	Resume(ctx context.Context, circleID string) error

	// Terminate permanently ends a circle.
	Terminate(ctx context.Context, circleID string) error

	// Policy operations

	// GetPolicy retrieves the circle's current policy.
	GetPolicy(ctx context.Context, circleID string) (*Policy, error)

	// UpdatePolicy modifies the circle's policy.
	UpdatePolicy(ctx context.Context, circleID string, policy *Policy) error

	// Authority operations

	// GrantAuthority creates a new authority grant.
	GrantAuthority(ctx context.Context, req GrantRequest) (*AuthorityGrant, error)

	// RevokeAuthority revokes an existing authority grant.
	// Per Canon: revocation MUST halt any in-progress actions.
	RevokeAuthority(ctx context.Context, grantID string) error

	// ListGrants returns all authority grants for this circle.
	ListGrants(ctx context.Context, circleID string) ([]AuthorityGrant, error)
}

// PolicyEngine defines the interface for policy evaluation.
// Policy evaluation is deterministic — no LLM/SLM involvement.
type PolicyEngine interface {
	// Evaluate checks if an operation is allowed by policy.
	Evaluate(ctx context.Context, circleID string, operation Operation) (Decision, error)

	// GetBoundaries returns the declared boundaries for a circle.
	GetBoundaries(ctx context.Context, circleID string) ([]Boundary, error)
}

// IdentityProvider defines the interface for circle identity operations.
type IdentityProvider interface {
	// Claim claims an identity for a new circle.
	Claim(ctx context.Context, req ClaimRequest) (*Identity, error)

	// Verify verifies a circle's identity.
	Verify(ctx context.Context, circleID string) (*Identity, error)

	// GetPublicKey returns the circle's public key for verification.
	GetPublicKey(ctx context.Context, circleID string) ([]byte, error)
}
