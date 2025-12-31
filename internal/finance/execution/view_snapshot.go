// Package execution provides v9.13 View Freshness Binding types.
//
// CRITICAL: v9.13 ensures read-before-write and view freshness:
// 1. ViewSnapshot MUST be obtained BEFORE any provider write call
// 2. ViewSnapshot MUST be within MaxStaleness of execution time
// 3. ViewSnapshotHash binds the envelope to a specific view state
// 4. Any view staleness or hash mismatch blocks execution
//
// This prevents executing based on stale or mismatched view data
// (account snapshot, payee eligibility, balance assumptions, shared view state).
//
// Reference: ADR-0014, Canon Addendum v9
package execution

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ViewSnapshotVersion is the version of the view snapshot format.
// Bump this if the snapshot structure changes to invalidate old hashes.
const ViewSnapshotVersion = "v9.13"

// DefaultMaxStalenessSeconds is the default maximum view staleness (5 minutes).
const DefaultMaxStalenessSeconds = 300

// ViewSnapshotHash is a type alias for view snapshot hash values.
type ViewSnapshotHash string

// ViewSnapshot captures minimal read-side state required for execution.
// This is the "view" that was current when the execution was approved.
//
// CRITICAL: Keep this minimal and boring. Only fields required for
// correctness/safety. No complex nested structures.
type ViewSnapshot struct {
	// SnapshotID uniquely identifies this snapshot.
	SnapshotID string

	// CapturedAt is when this snapshot was taken (from injected clock).
	// CRITICAL: Must use injected clock, never time.Now().
	CapturedAt time.Time

	// CircleID is the circle viewing/executing.
	CircleID string

	// IntersectionID is the shared context (optional).
	// Empty string means no intersection context.
	IntersectionID string

	// PayeeID is the registered payee for the execution.
	PayeeID string

	// Currency is the currency for the execution (ISO 4217).
	Currency string

	// AmountCents is the amount being executed.
	AmountCents int64

	// PayeeAllowed indicates if the payee is currently allowed in policy.
	PayeeAllowed bool

	// ProviderID is the write provider that will be used.
	ProviderID string

	// ProviderAllowed indicates if the provider is currently allowed.
	ProviderAllowed bool

	// AccountVisibility indicates what accounts are visible for this execution.
	// This is a minimal representation - just the canonical account IDs.
	AccountVisibility []string

	// SharedViewHash captures the shared view state if in intersection context.
	// Empty if no intersection or no shared view constraints.
	SharedViewHash string

	// BalanceCheckPassed indicates if balance requirements are met.
	// This is a simple boolean; we don't expose actual balances.
	BalanceCheckPassed bool

	// Notes is optional metadata for audit purposes only.
	// Never affects hash computation. Never shown to users.
	Notes string
}

// ComputeViewSnapshotHash computes a deterministic SHA-256 hash of the snapshot.
// Uses canonical string serialization with stable field ordering.
//
// CRITICAL: This uses explicit string building, NOT JSON marshal,
// to ensure deterministic ordering across all Go versions.
func ComputeViewSnapshotHash(snapshot ViewSnapshot) ViewSnapshotHash {
	canonical := CanonicalViewSnapshotString(snapshot)
	h := sha256.New()
	h.Write([]byte(canonical))
	return ViewSnapshotHash(hex.EncodeToString(h.Sum(nil)))
}

// CanonicalViewSnapshotString builds a canonical string representation.
// Fields are in explicit alphabetical order for stability.
// Notes field is excluded from hash computation.
func CanonicalViewSnapshotString(snapshot ViewSnapshot) string {
	var sb strings.Builder

	// Version prefix
	sb.WriteString("version:")
	sb.WriteString(ViewSnapshotVersion)
	sb.WriteString("|")

	// AccountVisibility (sorted for determinism)
	sb.WriteString("account_visibility:")
	sortedAccounts := make([]string, len(snapshot.AccountVisibility))
	copy(sortedAccounts, snapshot.AccountVisibility)
	sort.Strings(sortedAccounts)
	sb.WriteString(strings.Join(sortedAccounts, ","))
	sb.WriteString("|")

	// AmountCents
	sb.WriteString("amount_cents:")
	sb.WriteString(fmt.Sprintf("%d", snapshot.AmountCents))
	sb.WriteString("|")

	// BalanceCheckPassed
	sb.WriteString("balance_check_passed:")
	sb.WriteString(fmt.Sprintf("%t", snapshot.BalanceCheckPassed))
	sb.WriteString("|")

	// CapturedAt (RFC3339 for determinism)
	sb.WriteString("captured_at:")
	sb.WriteString(snapshot.CapturedAt.UTC().Format(time.RFC3339))
	sb.WriteString("|")

	// CircleID
	sb.WriteString("circle_id:")
	sb.WriteString(snapshot.CircleID)
	sb.WriteString("|")

	// Currency
	sb.WriteString("currency:")
	sb.WriteString(snapshot.Currency)
	sb.WriteString("|")

	// IntersectionID
	sb.WriteString("intersection_id:")
	sb.WriteString(snapshot.IntersectionID)
	sb.WriteString("|")

	// PayeeAllowed
	sb.WriteString("payee_allowed:")
	sb.WriteString(fmt.Sprintf("%t", snapshot.PayeeAllowed))
	sb.WriteString("|")

	// PayeeID
	sb.WriteString("payee_id:")
	sb.WriteString(snapshot.PayeeID)
	sb.WriteString("|")

	// ProviderAllowed
	sb.WriteString("provider_allowed:")
	sb.WriteString(fmt.Sprintf("%t", snapshot.ProviderAllowed))
	sb.WriteString("|")

	// ProviderID
	sb.WriteString("provider_id:")
	sb.WriteString(snapshot.ProviderID)
	sb.WriteString("|")

	// SharedViewHash
	sb.WriteString("shared_view_hash:")
	sb.WriteString(snapshot.SharedViewHash)
	sb.WriteString("|")

	// SnapshotID
	sb.WriteString("snapshot_id:")
	sb.WriteString(snapshot.SnapshotID)

	// Notes intentionally excluded from hash

	return sb.String()
}

// ViewFreshnessResult contains the result of a freshness check.
type ViewFreshnessResult struct {
	// Fresh is true if the view is within MaxStaleness.
	Fresh bool

	// StalenessMs is the actual staleness in milliseconds.
	StalenessMs int64

	// MaxStalenessMs is the configured maximum staleness.
	MaxStalenessMs int64

	// Reason contains a neutral explanation if not fresh.
	Reason string
}

// CheckViewFreshness verifies the snapshot is within MaxStaleness of now.
// Uses the provided clock for deterministic time.
func CheckViewFreshness(snapshot ViewSnapshot, now time.Time, maxStalenessSeconds int) ViewFreshnessResult {
	staleness := now.Sub(snapshot.CapturedAt)
	stalenessMs := staleness.Milliseconds()
	maxStalenessMs := int64(maxStalenessSeconds) * 1000

	if stalenessMs > maxStalenessMs {
		return ViewFreshnessResult{
			Fresh:          false,
			StalenessMs:    stalenessMs,
			MaxStalenessMs: maxStalenessMs,
			Reason: fmt.Sprintf("view stale: %dms > max %dms",
				stalenessMs, maxStalenessMs),
		}
	}

	return ViewFreshnessResult{
		Fresh:          true,
		StalenessMs:    stalenessMs,
		MaxStalenessMs: maxStalenessMs,
	}
}

// ViewHashVerifyResult contains the result of a view hash verification.
type ViewHashVerifyResult struct {
	// Match is true if hashes match.
	Match bool

	// ExpectedHash is the hash from the envelope.
	ExpectedHash string

	// ActualHash is the computed hash from current view.
	ActualHash string

	// Reason contains a neutral explanation if mismatch.
	Reason string
}

// VerifyViewSnapshotHash compares expected and actual view hashes.
func VerifyViewSnapshotHash(expectedHash, actualHash ViewSnapshotHash) ViewHashVerifyResult {
	if expectedHash == actualHash {
		return ViewHashVerifyResult{
			Match:        true,
			ExpectedHash: string(expectedHash),
			ActualHash:   string(actualHash),
		}
	}

	// Truncate hashes for safe logging
	expectedPrefix := string(expectedHash)
	if len(expectedPrefix) > 16 {
		expectedPrefix = expectedPrefix[:16] + "..."
	}
	actualPrefix := string(actualHash)
	if len(actualPrefix) > 16 {
		actualPrefix = actualPrefix[:16] + "..."
	}

	return ViewHashVerifyResult{
		Match:        false,
		ExpectedHash: string(expectedHash),
		ActualHash:   string(actualHash),
		Reason: fmt.Sprintf("view hash mismatch: expected %s, got %s",
			expectedPrefix, actualPrefix),
	}
}
