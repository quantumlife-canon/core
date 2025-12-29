// Package seal validates capability manifests and gates capability access.
//
// Canon Reference: docs/QUANTUMLIFE_CANON_V1.md §Marketplace
// Technical Split Reference: docs/TECHNICAL_SPLIT_V1.md §3.8 Marketplace & Seal Validation
package seal

import (
	"context"
)

// Validator validates capability manifests.
type Validator interface {
	// Validate validates a capability manifest.
	// Returns nil if valid, error with reason otherwise.
	Validate(ctx context.Context, manifest Manifest) error

	// ValidateSignature verifies the manifest signature.
	ValidateSignature(ctx context.Context, manifest Manifest) error

	// ValidateScopes checks if requested scopes are valid.
	ValidateScopes(ctx context.Context, manifest Manifest) error

	// CheckSealStatus checks the certification status.
	CheckSealStatus(ctx context.Context, capabilityID string, version string) (*SealStatus, error)
}

// Registry provides access to the capability registry.
type Registry interface {
	// Get retrieves a capability manifest by ID and version.
	Get(ctx context.Context, capabilityID string, version string) (*Manifest, error)

	// List lists capabilities matching filter.
	List(ctx context.Context, filter RegistryFilter) ([]ManifestSummary, error)

	// GetVersions lists all versions of a capability.
	GetVersions(ctx context.Context, capabilityID string) ([]string, error)

	// CheckCertified checks if a capability version is certified.
	CheckCertified(ctx context.Context, capabilityID string, version string) (bool, error)
}

// Gatekeeper gates capability usage based on seal status.
type Gatekeeper interface {
	// Gate checks if a capability can be used.
	// Certified: allowed (but circle must still grant authority)
	// Uncertified: requires explicit human approval
	// Revoked: blocked
	Gate(ctx context.Context, req GateRequest) (*GateResult, error)

	// RecordApproval records human approval for uncertified capability.
	RecordApproval(ctx context.Context, approval HumanApproval) error

	// CheckApproval checks if a circle has approved an uncertified capability.
	CheckApproval(ctx context.Context, circleID string, capabilityID string) (bool, error)
}
