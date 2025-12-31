// Package execution provides v9 financial execution primitives.
//
// This file implements v9.12 Policy Snapshot Hash Binding.
//
// CRITICAL: This prevents policy drift between approval and execution by
// binding execution to an explicit, deterministic PolicySnapshotHash.
//
// The snapshot captures ALL runtime gates that can change execution legality:
// 1. Provider allowlist (v9.9 provider registry)
// 2. Payee allowlist (v9.10 payee registry)
// 3. Caps + rate-limit policy (v9.11 caps gate)
//
// If ANY policy changes between approval and execution, the hash mismatches
// and execution is BLOCKED. This ensures immutability of execution rules.
//
// Subordinate to:
// - docs/QUANTUMLIFE_CANON_V1.md
// - docs/CANON_ADDENDUM_V9_FINANCIAL_EXECUTION.md
// - docs/TECHNICAL_SPLIT_V9_EXECUTION.md
package execution

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// PolicySnapshotVersion is the current version of the policy snapshot format.
// Increment when adding new fields to allow evolution.
// v9.12: Initial version with provider, payee, caps policies
// v9.13: Added MaxStalenessSeconds for view freshness policy
const PolicySnapshotVersion = "v9.13"

// PolicySnapshot captures the state of all execution-time policy gates.
// This is a snapshot of RULES/POLICIES, not a snapshot of balances/transactions.
//
// CRITICAL: All fields must be deterministic and stable across processes.
// The same policy inputs must always produce the same hash.
type PolicySnapshot struct {
	// Version identifies the snapshot format version.
	Version string

	// ProviderPolicy captures v9.9 provider registry state.
	ProviderPolicy ProviderPolicySnapshot

	// PayeePolicy captures v9.10 payee registry state.
	PayeePolicy PayeePolicySnapshot

	// CapsPolicy captures v9.11 caps and rate-limit configuration.
	CapsPolicy CapsPolicySnapshot

	// ViewPolicy captures v9.13 view freshness configuration.
	ViewPolicy ViewPolicySnapshot
}

// ViewPolicySnapshot captures the view freshness policy configuration.
type ViewPolicySnapshot struct {
	// MaxStalenessSeconds is the maximum age of a view snapshot before execution.
	// If view.CapturedAt is older than now - MaxStalenessSeconds, execution is blocked.
	MaxStalenessSeconds int
}

// ProviderPolicySnapshot captures the provider registry state.
type ProviderPolicySnapshot struct {
	// AllowedProviderIDs is a sorted list of allowed provider IDs.
	AllowedProviderIDs []string

	// BlockedProviderIDs is a sorted list of registered but blocked provider IDs.
	BlockedProviderIDs []string
}

// PayeePolicySnapshot captures the payee registry state.
type PayeePolicySnapshot struct {
	// AllowedPayeeIDs is a sorted list of allowed payee IDs.
	AllowedPayeeIDs []string

	// BlockedPayeeIDs is a sorted list of registered but blocked payee IDs.
	BlockedPayeeIDs []string
}

// CapsPolicySnapshot captures the caps gate configuration.
type CapsPolicySnapshot struct {
	// Enabled indicates if caps enforcement is active.
	Enabled bool

	// PerCircleDailyCapCents is a sorted representation of circle caps.
	// Format: "currency:cents" pairs, sorted alphabetically.
	PerCircleDailyCapCents []string

	// PerIntersectionDailyCapCents is a sorted representation of intersection caps.
	PerIntersectionDailyCapCents []string

	// PerPayeeDailyCapCents is a sorted representation of payee caps.
	PerPayeeDailyCapCents []string

	// MaxAttemptsPerDayCircle is the circle attempt limit.
	MaxAttemptsPerDayCircle int

	// MaxAttemptsPerDayIntersection is the intersection attempt limit.
	MaxAttemptsPerDayIntersection int
}

// PolicySnapshotHash is the deterministic hash of a PolicySnapshot.
type PolicySnapshotHash string

// ComputePolicySnapshotHash computes a deterministic SHA-256 hash of the policy snapshot.
// The hash is computed from a canonical string representation with explicit field ordering.
//
// CRITICAL: This function MUST be deterministic. Same inputs -> same hash.
// We use explicit string building with sorted field ordering, NOT JSON marshal.
func ComputePolicySnapshotHash(snapshot PolicySnapshot) PolicySnapshotHash {
	h := sha256.New()

	// Write canonical representation with explicit ordering
	canonical := buildCanonicalPolicyString(snapshot)
	h.Write([]byte(canonical))

	return PolicySnapshotHash(hex.EncodeToString(h.Sum(nil)))
}

// buildCanonicalPolicyString builds a deterministic string representation.
func buildCanonicalPolicyString(s PolicySnapshot) string {
	var b strings.Builder

	// Version (always first)
	b.WriteString("version:")
	b.WriteString(s.Version)
	b.WriteString("|")

	// Provider policy
	b.WriteString("providers.allowed:")
	b.WriteString(strings.Join(s.ProviderPolicy.AllowedProviderIDs, ","))
	b.WriteString("|providers.blocked:")
	b.WriteString(strings.Join(s.ProviderPolicy.BlockedProviderIDs, ","))
	b.WriteString("|")

	// Payee policy
	b.WriteString("payees.allowed:")
	b.WriteString(strings.Join(s.PayeePolicy.AllowedPayeeIDs, ","))
	b.WriteString("|payees.blocked:")
	b.WriteString(strings.Join(s.PayeePolicy.BlockedPayeeIDs, ","))
	b.WriteString("|")

	// Caps policy
	b.WriteString("caps.enabled:")
	b.WriteString(fmt.Sprintf("%t", s.CapsPolicy.Enabled))
	b.WriteString("|caps.circle:")
	b.WriteString(strings.Join(s.CapsPolicy.PerCircleDailyCapCents, ","))
	b.WriteString("|caps.intersection:")
	b.WriteString(strings.Join(s.CapsPolicy.PerIntersectionDailyCapCents, ","))
	b.WriteString("|caps.payee:")
	b.WriteString(strings.Join(s.CapsPolicy.PerPayeeDailyCapCents, ","))
	b.WriteString("|caps.maxattempts.circle:")
	b.WriteString(fmt.Sprintf("%d", s.CapsPolicy.MaxAttemptsPerDayCircle))
	b.WriteString("|caps.maxattempts.intersection:")
	b.WriteString(fmt.Sprintf("%d", s.CapsPolicy.MaxAttemptsPerDayIntersection))
	b.WriteString("|")

	// View policy (v9.13)
	b.WriteString("view.maxstaleness:")
	b.WriteString(fmt.Sprintf("%d", s.ViewPolicy.MaxStalenessSeconds))

	return b.String()
}

// PolicySnapshotInput contains the sources for computing a policy snapshot.
type PolicySnapshotInput struct {
	// ProviderDescriptor provides provider registry state.
	ProviderDescriptor ProviderPolicyDescriptor

	// PayeeDescriptor provides payee registry state.
	PayeeDescriptor PayeePolicyDescriptor

	// CapsDescriptor provides caps gate configuration.
	CapsDescriptor CapsPolicyDescriptor

	// ViewDescriptor provides view policy configuration.
	ViewDescriptor ViewPolicyDescriptor
}

// ViewPolicyDescriptor provides read-only access to view policy configuration.
type ViewPolicyDescriptor interface {
	// MaxStalenessSeconds returns the maximum view staleness in seconds.
	MaxStalenessSeconds() int
}

// ProviderPolicyDescriptor provides read-only access to provider registry state.
type ProviderPolicyDescriptor interface {
	// AllowedProviderIDs returns a sorted list of allowed provider IDs.
	AllowedProviderIDs() []string

	// BlockedProviderIDs returns a sorted list of blocked provider IDs.
	BlockedProviderIDs() []string
}

// PayeePolicyDescriptor provides read-only access to payee registry state.
type PayeePolicyDescriptor interface {
	// AllowedPayeeIDs returns a sorted list of allowed payee IDs.
	AllowedPayeeIDs() []string

	// BlockedPayeeIDs returns a sorted list of blocked payee IDs.
	BlockedPayeeIDs() []string
}

// CapsPolicyDescriptor provides read-only access to caps gate configuration.
type CapsPolicyDescriptor interface {
	// PolicyDescriptor returns the current caps policy configuration.
	PolicyDescriptor() CapsPolicySnapshot
}

// ComputePolicySnapshot computes a policy snapshot from the input sources.
func ComputePolicySnapshot(input PolicySnapshotInput) (PolicySnapshot, PolicySnapshotHash) {
	// Get view policy, defaulting if no descriptor provided
	maxStaleness := DefaultMaxStalenessSeconds
	if input.ViewDescriptor != nil {
		maxStaleness = input.ViewDescriptor.MaxStalenessSeconds()
	}

	snapshot := PolicySnapshot{
		Version: PolicySnapshotVersion,
		ProviderPolicy: ProviderPolicySnapshot{
			AllowedProviderIDs: input.ProviderDescriptor.AllowedProviderIDs(),
			BlockedProviderIDs: input.ProviderDescriptor.BlockedProviderIDs(),
		},
		PayeePolicy: PayeePolicySnapshot{
			AllowedPayeeIDs: input.PayeeDescriptor.AllowedPayeeIDs(),
			BlockedPayeeIDs: input.PayeeDescriptor.BlockedPayeeIDs(),
		},
		CapsPolicy: input.CapsDescriptor.PolicyDescriptor(),
		ViewPolicy: ViewPolicySnapshot{
			MaxStalenessSeconds: maxStaleness,
		},
	}

	// Ensure all slices are sorted for determinism
	sort.Strings(snapshot.ProviderPolicy.AllowedProviderIDs)
	sort.Strings(snapshot.ProviderPolicy.BlockedProviderIDs)
	sort.Strings(snapshot.PayeePolicy.AllowedPayeeIDs)
	sort.Strings(snapshot.PayeePolicy.BlockedPayeeIDs)
	sort.Strings(snapshot.CapsPolicy.PerCircleDailyCapCents)
	sort.Strings(snapshot.CapsPolicy.PerIntersectionDailyCapCents)
	sort.Strings(snapshot.CapsPolicy.PerPayeeDailyCapCents)

	hash := ComputePolicySnapshotHash(snapshot)
	return snapshot, hash
}

// VerifyPolicySnapshot verifies that the current policy matches the expected hash.
func VerifyPolicySnapshot(input PolicySnapshotInput, expectedHash PolicySnapshotHash) (PolicySnapshot, PolicySnapshotHash, bool) {
	snapshot, actualHash := ComputePolicySnapshot(input)
	return snapshot, actualHash, actualHash == expectedHash
}

// FormatCurrencyCap formats a currency cap for canonical representation.
func FormatCurrencyCap(currency string, cents int64) string {
	return fmt.Sprintf("%s:%d", currency, cents)
}

// ParseCurrencyCaps parses a map of currency->cents into sorted canonical format.
func ParseCurrencyCaps(caps map[string]int64) []string {
	if caps == nil {
		return []string{}
	}

	result := make([]string, 0, len(caps))
	for currency, cents := range caps {
		result = append(result, FormatCurrencyCap(currency, cents))
	}
	sort.Strings(result)
	return result
}

// CapsGatePolicyAdapter wraps a caps gate to provide CapsPolicyDescriptor interface.
type CapsGatePolicyAdapter struct {
	// GetPolicyFunc returns the current caps policy.
	GetPolicyFunc func() (enabled bool, circleCaps, intersectionCaps, payeeCaps map[string]int64, maxAttemptsCircle, maxAttemptsIntersection int)
}

// PolicyDescriptor returns the current caps policy as a CapsPolicySnapshot.
func (a *CapsGatePolicyAdapter) PolicyDescriptor() CapsPolicySnapshot {
	enabled, circleCaps, intersectionCaps, payeeCaps, maxAttemptsCircle, maxAttemptsIntersection := a.GetPolicyFunc()

	return CapsPolicySnapshot{
		Enabled:                       enabled,
		PerCircleDailyCapCents:        ParseCurrencyCaps(circleCaps),
		PerIntersectionDailyCapCents:  ParseCurrencyCaps(intersectionCaps),
		PerPayeeDailyCapCents:         ParseCurrencyCaps(payeeCaps),
		MaxAttemptsPerDayCircle:       maxAttemptsCircle,
		MaxAttemptsPerDayIntersection: maxAttemptsIntersection,
	}
}

// NewCapsGatePolicyAdapter creates a policy adapter from a GetPolicy function.
func NewCapsGatePolicyAdapter(getPolicyFunc func() (enabled bool, circleCaps, intersectionCaps, payeeCaps map[string]int64, maxAttemptsCircle, maxAttemptsIntersection int)) *CapsGatePolicyAdapter {
	return &CapsGatePolicyAdapter{GetPolicyFunc: getPolicyFunc}
}

// ProviderRegistryAdapter wraps a provider registry to provide ProviderPolicyDescriptor interface.
type ProviderRegistryAdapter struct {
	AllowedFunc func() []string
	BlockedFunc func() []string
}

// AllowedProviderIDs returns a sorted list of allowed provider IDs.
func (a *ProviderRegistryAdapter) AllowedProviderIDs() []string {
	return a.AllowedFunc()
}

// BlockedProviderIDs returns a sorted list of blocked provider IDs.
func (a *ProviderRegistryAdapter) BlockedProviderIDs() []string {
	return a.BlockedFunc()
}

// PayeeRegistryAdapter wraps a payee registry to provide PayeePolicyDescriptor interface.
type PayeeRegistryAdapter struct {
	AllowedFunc func() []string
	BlockedFunc func() []string
}

// AllowedPayeeIDs returns a sorted list of allowed payee IDs.
func (a *PayeeRegistryAdapter) AllowedPayeeIDs() []string {
	return a.AllowedFunc()
}

// BlockedPayeeIDs returns a sorted list of blocked payee IDs.
func (a *PayeeRegistryAdapter) BlockedPayeeIDs() []string {
	return a.BlockedFunc()
}
