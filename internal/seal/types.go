package seal

import (
	"time"
)

// Manifest represents a capability manifest.
// Structure per ADR-0006.
type Manifest struct {
	ID          string
	Version     string
	Name        string
	Description string

	ScopesRequested []string
	RiskClass       string // "standard", "elevated", "high"

	AuditHooks  []AuditHook
	Constraints Constraints

	Signature  Signature
	Provenance Provenance
}

// AuditHook defines an audit event for the capability.
type AuditHook struct {
	Trigger string // "on_action", "on_failure", etc.
	HookID  string
}

// Constraints defines operational constraints.
type Constraints struct {
	MaxFrequency         string
	RequiresIntersection bool
}

// Signature contains the manifest signature.
type Signature struct {
	Algorithm string
	Signer    string
	Timestamp time.Time
	Value     []byte
}

// Provenance contains source information.
type Provenance struct {
	SourceRepo string
	CommitHash string
	BuildID    string
}

// SealStatus represents the certification status.
type SealStatus struct {
	CapabilityID string
	Version      string
	Status       Status
	CertifiedAt  *time.Time
	RevokedAt    *time.Time
	Reason       string
}

// Status represents seal certification status.
type Status string

const (
	StatusCertified   Status = "certified"
	StatusUncertified Status = "uncertified"
	StatusRevoked     Status = "revoked"
	StatusPending     Status = "pending"
)

// ManifestSummary contains summary information about a manifest.
type ManifestSummary struct {
	ID          string
	Version     string
	Name        string
	RiskClass   string
	Status      Status
	CertifiedAt *time.Time
}

// RegistryFilter specifies criteria for listing capabilities.
type RegistryFilter struct {
	RiskClass string
	Status    Status
	Limit     int
	Offset    int
}

// GateRequest contains parameters for gating a capability.
type GateRequest struct {
	CapabilityID    string
	Version         string
	CircleID        string
	RequestedScopes []string
}

// GateResult contains the result of gating check.
type GateResult struct {
	Allowed          bool
	RequiresApproval bool
	Reason           string
	Status           Status
}

// HumanApproval records human approval for uncertified capability.
type HumanApproval struct {
	CircleID     string
	CapabilityID string
	Version      string
	ApprovedAt   time.Time
	ApprovedBy   string
	ExpiresAt    *time.Time
}
