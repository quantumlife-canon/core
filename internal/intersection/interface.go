// Package intersection manages shared contract spaces between circles.
//
// Canon Reference: docs/QUANTUMLIFE_CANON_V1.md §Intersections
// Technical Split Reference: docs/TECHNICAL_SPLIT_V1.md §3.2 Intersection Runtime
package intersection

import (
	"context"

	"quantumlife/pkg/primitives"
)

// Runtime defines the interface for intersection operations.
type Runtime interface {
	// Lifecycle operations

	// Create initializes a new intersection from an accepted proposal.
	Create(ctx context.Context, req CreateRequest) (*Intersection, error)

	// Get retrieves an intersection by ID.
	Get(ctx context.Context, intersectionID string) (*Intersection, error)

	// Amend modifies an intersection (requires all-party consent).
	Amend(ctx context.Context, req AmendRequest) (*Intersection, error)

	// Dissolve ends an intersection (any party can initiate).
	Dissolve(ctx context.Context, intersectionID string, initiatorCircleID string) error

	// Contract operations

	// GetContract retrieves the current contract version.
	GetContract(ctx context.Context, intersectionID string) (*Contract, error)

	// GetContractHistory retrieves all contract versions.
	GetContractHistory(ctx context.Context, intersectionID string) ([]Contract, error)

	// Party operations

	// ListParties returns all circles in this intersection.
	ListParties(ctx context.Context, intersectionID string) ([]Party, error)

	// IsParty checks if a circle is a party to this intersection.
	IsParty(ctx context.Context, intersectionID string, circleID string) (bool, error)
}

// InviteService defines the interface for invite token operations.
type InviteService interface {
	// CreateInvite generates a signed invite token.
	CreateInvite(ctx context.Context, req InviteRequest) (*InviteToken, error)

	// ValidateInvite verifies and parses an invite token.
	ValidateInvite(ctx context.Context, token string) (*InviteToken, error)

	// AcceptInvite accepts an invite and creates an intersection.
	AcceptInvite(ctx context.Context, token string, acceptorCircleID string) (*Intersection, error)

	// RejectInvite rejects an invite.
	RejectInvite(ctx context.Context, token string, rejectorCircleID string) error
}

// MessageChannel defines the interface for intersection-scoped messaging.
// Per Technology Selection: server-mediated, no direct agent-to-agent.
type MessageChannel interface {
	// Send sends a message within an intersection.
	Send(ctx context.Context, intersectionID string, msg Message) error

	// Receive receives messages for a circle within an intersection.
	Receive(ctx context.Context, intersectionID string, circleID string) (<-chan Message, error)
}

// LoopDiscoverer provides loop-aware intersection discovery.
// Used by the orchestrator at step 2 (Intersection Discovery) of the Irreducible Loop.
type LoopDiscoverer interface {
	// DiscoverForLoop finds or creates an intersection for a loop.
	// Returns the intersection ID and whether it was newly created.
	DiscoverForLoop(ctx context.Context, loopCtx LoopContext, criteria DiscoveryCriteria) (*DiscoveryResult, error)
}

// LoopContext is imported from primitives for loop threading.
type LoopContext = primitives.LoopContext

// DiscoveryCriteria contains criteria for intersection discovery.
type DiscoveryCriteria struct {
	IssuerCircleID  string
	TargetCircleID  string
	RequiredScopes  []string
	PreferExisting  bool
	ContractTerms   *ContractTemplate
}

// DiscoveryResult contains the result of intersection discovery.
type DiscoveryResult struct {
	IntersectionID  string
	IsNew           bool
	ContractVersion string
	AvailableScopes []string
}
