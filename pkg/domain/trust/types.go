// Package trust provides types for the Trust Accrual Layer.
//
// Phase 20: Trust Accrual Layer (Proof Over Time)
//
// CRITICAL INVARIANTS:
//   - Silence is the default outcome
//   - Trust signals are NEVER pushed
//   - Trust signals are NEVER frequent
//   - Trust signals are NEVER actionable
//   - Only abstract buckets (nothing / a_few / several)
//   - NO timestamps, counts, vendors, people, or content
//   - Append-only, hash-only storage
//   - Deterministic: same inputs + clock => same hashes
//   - No goroutines, no time.Now()
//
// This package makes restraint observable retrospectively,
// without creating engagement pressure.
//
// Reference: docs/ADR/ADR-0048-phase20-trust-accrual-layer.md
package trust

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"quantumlife/pkg/domain/shadowllm"
)

// =============================================================================
// Enums
// =============================================================================

// TrustPeriod represents the time granularity for trust summaries.
// CRITICAL: These are abstract periods, not specific dates.
type TrustPeriod string

const (
	// PeriodWeek represents a weekly aggregation period.
	PeriodWeek TrustPeriod = "week"

	// PeriodMonth represents a monthly aggregation period.
	PeriodMonth TrustPeriod = "month"
)

// Validate checks if the period is valid.
func (p TrustPeriod) Validate() bool {
	switch p {
	case PeriodWeek, PeriodMonth:
		return true
	default:
		return false
	}
}

// TrustSignalKind represents what kind of restraint was demonstrated.
type TrustSignalKind string

const (
	// SignalQuietHeld means obligations were held quietly without surfacing.
	SignalQuietHeld TrustSignalKind = "quiet_held"

	// SignalInterruptionPrevented means potential interruptions were suppressed.
	SignalInterruptionPrevented TrustSignalKind = "interruption_prevented"

	// SignalNothingRequired means no action was needed - silence was natural.
	SignalNothingRequired TrustSignalKind = "nothing_required"
)

// Validate checks if the signal kind is valid.
func (s TrustSignalKind) Validate() bool {
	switch s {
	case SignalQuietHeld, SignalInterruptionPrevented, SignalNothingRequired:
		return true
	default:
		return false
	}
}

// HumanReadable returns a calm, non-performative description.
// CRITICAL: These must not celebrate, claim value, or persuade.
func (s TrustSignalKind) HumanReadable() string {
	switch s {
	case SignalQuietHeld:
		return "Things were held quietly."
	case SignalInterruptionPrevented:
		return "Interruptions were prevented."
	case SignalNothingRequired:
		return "Nothing required attention."
	default:
		return ""
	}
}

// =============================================================================
// TrustSummary
// =============================================================================

// TrustSummary represents aggregated evidence of restraint for a period.
//
// CRITICAL CONSTRAINTS:
//   - NO raw numbers
//   - NO specific dates
//   - NO identifiers
//   - Only abstract magnitude buckets
//   - Deterministic hashing
type TrustSummary struct {
	// SummaryID uniquely identifies this summary.
	SummaryID string

	// SummaryHash is the SHA256 hash of the canonical string.
	SummaryHash string

	// Period is the time granularity (week | month).
	Period TrustPeriod

	// PeriodKey is an abstract period identifier (e.g., "2024-W03" for week 3).
	// CRITICAL: This is NOT a timestamp. It's a bucket identifier.
	PeriodKey string

	// SignalKind indicates what kind of restraint was demonstrated.
	SignalKind TrustSignalKind

	// MagnitudeBucket indicates the relative amount of restraint.
	// Uses existing abstract buckets: nothing | a_few | several
	MagnitudeBucket shadowllm.MagnitudeBucket

	// DismissedBucket records when this summary was dismissed (if ever).
	// Empty string means not dismissed. Format: abstract bucket only.
	DismissedBucket string

	// CreatedBucket is a 5-minute bucket for determinism.
	CreatedBucket string

	// CreatedAt is the exact creation time (internal use only).
	CreatedAt time.Time
}

// CanonicalString returns the pipe-delimited canonical representation.
// Used for deterministic hashing.
func (s *TrustSummary) CanonicalString() string {
	return "TRUST_SUMMARY|v1|" +
		string(s.Period) + "|" +
		s.PeriodKey + "|" +
		string(s.SignalKind) + "|" +
		string(s.MagnitudeBucket) + "|" +
		s.CreatedBucket
}

// ComputeHash computes the SHA256 hash of the canonical string.
func (s *TrustSummary) ComputeHash() string {
	h := sha256.Sum256([]byte(s.CanonicalString()))
	return hex.EncodeToString(h[:])
}

// ComputeID computes a stable ID for this summary.
// Based on period and period key - same period always yields same ID.
func (s *TrustSummary) ComputeID() string {
	idStr := "TRUST_ID|" +
		string(s.Period) + "|" +
		s.PeriodKey
	h := sha256.Sum256([]byte(idStr))
	return hex.EncodeToString(h[:16]) // 32 hex chars
}

// Validate checks if the summary is valid.
func (s *TrustSummary) Validate() error {
	if !s.Period.Validate() {
		return ErrInvalidPeriod
	}
	if s.PeriodKey == "" {
		return ErrMissingPeriodKey
	}
	if !s.SignalKind.Validate() {
		return ErrInvalidSignalKind
	}
	if !s.MagnitudeBucket.Validate() {
		return ErrInvalidMagnitude
	}
	if s.CreatedBucket == "" {
		return ErrMissingCreatedBucket
	}
	return nil
}

// IsDismissed returns true if this summary has been dismissed.
func (s *TrustSummary) IsDismissed() bool {
	return s.DismissedBucket != ""
}

// IsMeaningful returns true if this summary represents actual restraint.
// "Nothing" magnitude means no meaningful activity occurred.
func (s *TrustSummary) IsMeaningful() bool {
	return s.MagnitudeBucket != shadowllm.MagnitudeNothing
}

// =============================================================================
// TrustDismissal
// =============================================================================

// TrustDismissal records that a user dismissed a trust summary.
// Once dismissed, the summary must not reappear for that period.
type TrustDismissal struct {
	// DismissalID uniquely identifies this dismissal.
	DismissalID string

	// DismissalHash is the SHA256 hash of the canonical string.
	DismissalHash string

	// SummaryID references the dismissed summary.
	SummaryID string

	// SummaryHash references the dismissed summary's hash.
	SummaryHash string

	// CreatedBucket is a 5-minute bucket for determinism.
	CreatedBucket string

	// CreatedAt is the exact creation time (internal use only).
	CreatedAt time.Time
}

// CanonicalString returns the pipe-delimited canonical representation.
func (d *TrustDismissal) CanonicalString() string {
	return "TRUST_DISMISSAL|v1|" +
		d.SummaryID + "|" +
		d.SummaryHash + "|" +
		d.CreatedBucket
}

// ComputeHash computes the SHA256 hash of the canonical string.
func (d *TrustDismissal) ComputeHash() string {
	h := sha256.Sum256([]byte(d.CanonicalString()))
	return hex.EncodeToString(h[:])
}

// ComputeID computes a stable ID for this dismissal.
func (d *TrustDismissal) ComputeID() string {
	idStr := "DISMISSAL_ID|" +
		d.SummaryID + "|" +
		d.CreatedBucket
	h := sha256.Sum256([]byte(idStr))
	return hex.EncodeToString(h[:16])
}

// Validate checks if the dismissal is valid.
func (d *TrustDismissal) Validate() error {
	if d.SummaryID == "" {
		return ErrMissingSummaryID
	}
	if d.SummaryHash == "" {
		return ErrMissingSummaryHash
	}
	if d.CreatedBucket == "" {
		return ErrMissingCreatedBucket
	}
	return nil
}

// =============================================================================
// Helpers
// =============================================================================

// FiveMinuteBucket computes a 5-minute bucket from a time.
// CRITICAL: This is for determinism, not for display.
func FiveMinuteBucket(t time.Time) string {
	t = t.UTC()
	minute := (t.Minute() / 5) * 5
	return t.Format("2006-01-02T15:") + itoa(minute)
}

// WeekKey computes a week key (e.g., "2024-W03") from a time.
// CRITICAL: This is an abstract bucket, not a timestamp.
func WeekKey(t time.Time) string {
	year, week := t.UTC().ISOWeek()
	return itoa(year) + "-W" + padTwo(week)
}

// MonthKey computes a month key (e.g., "2024-01") from a time.
// CRITICAL: This is an abstract bucket, not a timestamp.
func MonthKey(t time.Time) string {
	t = t.UTC()
	return t.Format("2006-01")
}

// itoa converts int to string without fmt dependency.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

// padTwo pads a number to two digits.
func padTwo(n int) string {
	if n < 10 {
		return "0" + itoa(n)
	}
	return itoa(n)
}

// =============================================================================
// Errors
// =============================================================================

type trustError string

func (e trustError) Error() string { return string(e) }

const (
	ErrInvalidPeriod        trustError = "invalid trust period"
	ErrMissingPeriodKey     trustError = "missing period key"
	ErrInvalidSignalKind    trustError = "invalid signal kind"
	ErrInvalidMagnitude     trustError = "invalid magnitude bucket"
	ErrMissingCreatedBucket trustError = "missing created bucket"
	ErrMissingSummaryID     trustError = "missing summary ID"
	ErrMissingSummaryHash   trustError = "missing summary hash"
)
