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
// CRITICAL: Execution MUST fail if current policy differs from snapshot.
type PolicySnapshot struct {
	// SnapshotID is the deterministic ID for this snapshot.
	SnapshotID string

	// CircleID is the circle this policy applies to.
	CircleID identity.EntityID

	// IntersectionID is the intersection this policy applies to.
	IntersectionID identity.EntityID

	// CapturedAt is when this snapshot was taken.
	CapturedAt time.Time

	// PolicyHash is the hash of the policy content.
	PolicyHash string

	// CalendarWriteEnabled indicates calendar writes are enabled.
	CalendarWriteEnabled bool

	// AllowedCalendarIDs lists calendars where writes are allowed.
	// Empty means all calendars are allowed.
	AllowedCalendarIDs []string

	// AllowedProviders lists providers where writes are allowed.
	AllowedProviders []string

	// RequireExplicitApproval indicates each write needs approval.
	RequireExplicitApproval bool

	// MaxStalenessMinutes is the max view staleness allowed.
	MaxStalenessMinutes int

	// DryRunMode indicates writes should be simulated, not executed.
	DryRunMode bool
}

// PolicySnapshotParams contains parameters for creating a policy snapshot.
type PolicySnapshotParams struct {
	CircleID                identity.EntityID
	IntersectionID          identity.EntityID
	CalendarWriteEnabled    bool
	AllowedCalendarIDs      []string
	AllowedProviders        []string
	RequireExplicitApproval bool
	MaxStalenessMinutes     int
	DryRunMode              bool
}

// NewPolicySnapshot creates a new policy snapshot.
func NewPolicySnapshot(params PolicySnapshotParams, now time.Time) PolicySnapshot {
	// Sort for determinism
	sortedCalendarIDs := make([]string, len(params.AllowedCalendarIDs))
	copy(sortedCalendarIDs, params.AllowedCalendarIDs)
	sort.Strings(sortedCalendarIDs)

	sortedProviders := make([]string, len(params.AllowedProviders))
	copy(sortedProviders, params.AllowedProviders)
	sort.Strings(sortedProviders)

	// Compute policy hash
	policyHash := computePolicyHash(
		params.CalendarWriteEnabled,
		sortedCalendarIDs,
		sortedProviders,
		params.RequireExplicitApproval,
		params.MaxStalenessMinutes,
		params.DryRunMode,
	)

	// Compute snapshot ID
	snapshotID := computeSnapshotID(params.CircleID, params.IntersectionID, policyHash, now)

	return PolicySnapshot{
		SnapshotID:              snapshotID,
		CircleID:                params.CircleID,
		IntersectionID:          params.IntersectionID,
		CapturedAt:              now,
		PolicyHash:              policyHash,
		CalendarWriteEnabled:    params.CalendarWriteEnabled,
		AllowedCalendarIDs:      sortedCalendarIDs,
		AllowedProviders:        sortedProviders,
		RequireExplicitApproval: params.RequireExplicitApproval,
		MaxStalenessMinutes:     params.MaxStalenessMinutes,
		DryRunMode:              params.DryRunMode,
	}
}

// computePolicyHash computes a deterministic hash of policy content.
func computePolicyHash(
	calendarWriteEnabled bool,
	allowedCalendarIDs []string,
	allowedProviders []string,
	requireExplicitApproval bool,
	maxStalenessMinutes int,
	dryRunMode bool,
) string {
	canonical := fmt.Sprintf("policy|%t|%s|%s|%t|%d|%t",
		calendarWriteEnabled,
		strings.Join(allowedCalendarIDs, ","),
		strings.Join(allowedProviders, ","),
		requireExplicitApproval,
		maxStalenessMinutes,
		dryRunMode,
	)
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:])
}

// computeSnapshotID computes a deterministic snapshot ID.
func computeSnapshotID(
	circleID identity.EntityID,
	intersectionID identity.EntityID,
	policyHash string,
	capturedAt time.Time,
) string {
	canonical := fmt.Sprintf("snapshot|%s|%s|%s|%s",
		circleID,
		intersectionID,
		policyHash,
		capturedAt.UTC().Format(time.RFC3339),
	)
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:])
}

// IsCalendarAllowed checks if a calendar ID is allowed by this policy.
func (p PolicySnapshot) IsCalendarAllowed(calendarID string) bool {
	if !p.CalendarWriteEnabled {
		return false
	}
	// Empty list means all allowed
	if len(p.AllowedCalendarIDs) == 0 {
		return true
	}
	for _, allowed := range p.AllowedCalendarIDs {
		if allowed == calendarID {
			return true
		}
	}
	return false
}

// IsProviderAllowed checks if a provider is allowed by this policy.
func (p PolicySnapshot) IsProviderAllowed(provider string) bool {
	if !p.CalendarWriteEnabled {
		return false
	}
	// Empty list means all allowed
	if len(p.AllowedProviders) == 0 {
		return true
	}
	for _, allowed := range p.AllowedProviders {
		if allowed == provider {
			return true
		}
	}
	return false
}

// MaxStaleness returns the max staleness as a duration.
func (p PolicySnapshot) MaxStaleness() time.Duration {
	return time.Duration(p.MaxStalenessMinutes) * time.Minute
}

// PolicyVerifier verifies policy snapshots against current policy.
type PolicyVerifier struct {
	// getCurrentPolicy returns the current policy hash.
	getCurrentPolicy func(circleID, intersectionID identity.EntityID) (string, error)
}

// NewPolicyVerifier creates a new policy verifier.
func NewPolicyVerifier(getPolicyFn func(circleID, intersectionID identity.EntityID) (string, error)) *PolicyVerifier {
	return &PolicyVerifier{
		getCurrentPolicy: getPolicyFn,
	}
}

// Verify checks if a policy snapshot matches current policy.
// CRITICAL: Returns error if policy has changed since snapshot.
func (v *PolicyVerifier) Verify(snapshot PolicySnapshot) error {
	currentHash, err := v.getCurrentPolicy(snapshot.CircleID, snapshot.IntersectionID)
	if err != nil {
		return fmt.Errorf("failed to get current policy: %w", err)
	}

	if currentHash != snapshot.PolicyHash {
		return ErrPolicyMismatch
	}

	return nil
}

// Policy verification errors.
var (
	ErrPolicyMismatch = policyError("policy has changed since snapshot")
	ErrPolicyDisabled = policyError("calendar writes are disabled")
)

type policyError string

func (e policyError) Error() string { return string(e) }
