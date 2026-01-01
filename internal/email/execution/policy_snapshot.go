package execution

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"quantumlife/pkg/domain/identity"
)

// PolicySnapshot captures the policy state at a point in time.
//
// CRITICAL: Must be verified before execution.
// CRITICAL: Mismatch blocks execution (policy drift).
type PolicySnapshot struct {
	// PolicyHash is the deterministic hash of this snapshot.
	PolicyHash string

	// CircleID identifies the circle this policy applies to.
	CircleID identity.EntityID

	// IntersectionID is optional for shared contexts.
	IntersectionID identity.EntityID

	// CapturedAt is when this snapshot was taken.
	CapturedAt time.Time

	// EmailWriteEnabled indicates if email writes are allowed.
	EmailWriteEnabled bool

	// AllowedProviders lists providers that can be used.
	AllowedProviders []string

	// MaxSendsPerDay is the maximum sends allowed per day per circle.
	// 0 means unlimited.
	MaxSendsPerDay int

	// DryRunMode if true prevents actual sends.
	DryRunMode bool
}

// PolicySnapshotParams contains parameters for creating a policy snapshot.
type PolicySnapshotParams struct {
	CircleID          identity.EntityID
	IntersectionID    identity.EntityID
	EmailWriteEnabled bool
	AllowedProviders  []string
	MaxSendsPerDay    int
	DryRunMode        bool
}

// NewPolicySnapshot creates a new policy snapshot with computed hash.
func NewPolicySnapshot(params PolicySnapshotParams, now time.Time) PolicySnapshot {
	snapshot := PolicySnapshot{
		CircleID:          params.CircleID,
		IntersectionID:    params.IntersectionID,
		CapturedAt:        now,
		EmailWriteEnabled: params.EmailWriteEnabled,
		AllowedProviders:  params.AllowedProviders,
		MaxSendsPerDay:    params.MaxSendsPerDay,
		DryRunMode:        params.DryRunMode,
	}

	snapshot.PolicyHash = snapshot.ComputeHash()
	return snapshot
}

// ComputeHash computes a deterministic hash of the policy.
//
// CRITICAL: Uses canonical string, not JSON, for determinism.
func (p *PolicySnapshot) ComputeHash() string {
	// Sort providers for determinism
	providers := make([]string, len(p.AllowedProviders))
	copy(providers, p.AllowedProviders)
	sort.Strings(providers)

	canonical := fmt.Sprintf("email-policy|circle:%s|intersection:%s|enabled:%t|providers:%s|max_sends:%d|dry_run:%t",
		p.CircleID,
		p.IntersectionID,
		p.EmailWriteEnabled,
		strings.Join(providers, ","),
		p.MaxSendsPerDay,
		p.DryRunMode,
	)

	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:16])
}

// PolicyVerifier verifies policy snapshots.
type PolicyVerifier struct {
	// currentPolicyProvider provides the current policy.
	currentPolicyProvider func(circleID, intersectionID identity.EntityID) PolicySnapshot
}

// NewPolicyVerifier creates a new policy verifier.
func NewPolicyVerifier(provider func(circleID, intersectionID identity.EntityID) PolicySnapshot) *PolicyVerifier {
	return &PolicyVerifier{
		currentPolicyProvider: provider,
	}
}

// Verify verifies that the snapshot matches current policy.
//
// CRITICAL: Returns error if policy has drifted.
func (v *PolicyVerifier) Verify(snapshot PolicySnapshot) error {
	if v.currentPolicyProvider == nil {
		// No provider = skip verification
		return nil
	}

	current := v.currentPolicyProvider(snapshot.CircleID, snapshot.IntersectionID)

	if current.PolicyHash != snapshot.PolicyHash {
		return fmt.Errorf("policy drift detected: snapshot=%s current=%s",
			snapshot.PolicyHash, current.PolicyHash)
	}

	if !current.EmailWriteEnabled {
		return fmt.Errorf("email write is disabled for this circle")
	}

	return nil
}

// DefaultEmailPolicy returns a default permissive policy.
func DefaultEmailPolicy(circleID identity.EntityID) PolicySnapshot {
	return PolicySnapshot{
		CircleID:          circleID,
		EmailWriteEnabled: true,
		AllowedProviders:  []string{"google", "mock"},
		MaxSendsPerDay:    100,
		DryRunMode:        false,
	}
}
