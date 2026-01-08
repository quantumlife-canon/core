// Package attentionenvelope provides domain types for Phase 39: Temporal Attention Envelopes.
//
// Attention envelopes are time-boxed, explicit, revocable windows that modify
// pressure input BEFORE Phase 32 processing. They allow temporary heightened
// responsiveness for legitimate situations (on-call, travel, emergencies)
// without changing downstream pipeline behavior.
//
// CRITICAL INVARIANTS:
//   - No envelope = no change (calm is default)
//   - Explicit start (POST only), never auto-enabled
//   - Bounded duration (15m, 1h, 4h, day), auto-expires
//   - Revocable (stop early via POST)
//   - Effects bounded: max 1 step horizon, +1 magnitude, +1 cap
//   - Commerce excluded: envelope NEVER escalates commerce pressure
//   - Does NOT bypass Phase 33/34 permission/preview
//   - Does NOT force interrupts
//   - No time.Now() - clock injection only
//   - No goroutines
//   - stdlib only
//   - Hash-only storage
//
// Reference: docs/ADR/ADR-0076-phase39-attention-envelopes.md
package attentionenvelope

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// Storage constraints
const (
	// MaxEnvelopeRecords is the maximum number of envelope records to retain.
	MaxEnvelopeRecords = 200

	// MaxRetentionDays is the maximum number of days to retain envelope records.
	MaxRetentionDays = 30
)

// EnvelopeKind represents the type of attention envelope.
// CRITICAL: Determines the effect rules applied.
type EnvelopeKind string

const (
	// EnvelopeKindNone indicates no envelope (baseline behavior).
	EnvelopeKindNone EnvelopeKind = "none"

	// EnvelopeKindOnCall indicates on-call duty requiring faster response.
	// Effects: Horizon +1 earlier, Magnitude +1, Cap +1
	EnvelopeKindOnCall EnvelopeKind = "on_call"

	// EnvelopeKindWorking indicates focused work session.
	// Effects: Magnitude +1 only (no horizon shift, no cap increase)
	EnvelopeKindWorking EnvelopeKind = "working"

	// EnvelopeKindTravel indicates travel transit with time-sensitive connections.
	// Effects: Horizon +1 earlier only (no magnitude bias, no cap increase)
	EnvelopeKindTravel EnvelopeKind = "travel"

	// EnvelopeKindEmergency indicates family/health emergency situation.
	// Effects: Horizon +1 earlier, Magnitude +1, Cap +1
	EnvelopeKindEmergency EnvelopeKind = "emergency"
)

// AllEnvelopeKinds returns all envelope kinds in deterministic order.
func AllEnvelopeKinds() []EnvelopeKind {
	return []EnvelopeKind{
		EnvelopeKindNone,
		EnvelopeKindOnCall,
		EnvelopeKindWorking,
		EnvelopeKindTravel,
		EnvelopeKindEmergency,
	}
}

// Validate checks if the envelope kind is valid.
func (k EnvelopeKind) Validate() error {
	switch k {
	case EnvelopeKindNone, EnvelopeKindOnCall, EnvelopeKindWorking,
		EnvelopeKindTravel, EnvelopeKindEmergency:
		return nil
	default:
		return fmt.Errorf("invalid envelope kind: %s", k)
	}
}

// DisplayText returns calm, human-readable text for the kind.
func (k EnvelopeKind) DisplayText() string {
	switch k {
	case EnvelopeKindNone:
		return "None"
	case EnvelopeKindOnCall:
		return "On-call"
	case EnvelopeKindWorking:
		return "Working"
	case EnvelopeKindTravel:
		return "Travel"
	case EnvelopeKindEmergency:
		return "Emergency"
	default:
		return "Unknown"
	}
}

// DurationBucket represents the fixed envelope duration options.
// CRITICAL: Only these durations allowed. No custom durations.
type DurationBucket string

const (
	// Duration15m is a 15-minute envelope (1 period).
	Duration15m DurationBucket = "15m"

	// Duration1h is a 1-hour envelope (4 periods).
	Duration1h DurationBucket = "1h"

	// Duration4h is a 4-hour envelope (16 periods).
	Duration4h DurationBucket = "4h"

	// DurationDay is a 24-hour envelope (96 periods).
	DurationDay DurationBucket = "day"
)

// AllDurationBuckets returns all duration buckets in deterministic order.
func AllDurationBuckets() []DurationBucket {
	return []DurationBucket{
		Duration15m,
		Duration1h,
		Duration4h,
		DurationDay,
	}
}

// Validate checks if the duration bucket is valid.
func (d DurationBucket) Validate() error {
	switch d {
	case Duration15m, Duration1h, Duration4h, DurationDay:
		return nil
	default:
		return fmt.Errorf("invalid duration bucket: %s", d)
	}
}

// DisplayText returns calm, human-readable text for the duration.
func (d DurationBucket) DisplayText() string {
	switch d {
	case Duration15m:
		return "15 minutes"
	case Duration1h:
		return "1 hour"
	case Duration4h:
		return "4 hours"
	case DurationDay:
		return "24 hours"
	default:
		return "Unknown"
	}
}

// ToDuration returns the time.Duration equivalent.
func (d DurationBucket) ToDuration() time.Duration {
	switch d {
	case Duration15m:
		return 15 * time.Minute
	case Duration1h:
		return 1 * time.Hour
	case Duration4h:
		return 4 * time.Hour
	case DurationDay:
		return 24 * time.Hour
	default:
		return 0
	}
}

// EnvelopeReason represents the user's stated reason for the envelope.
// CRITICAL: Enums only. No free-text reasons allowed.
type EnvelopeReason string

const (
	// ReasonAwaitingImportant indicates waiting for important response.
	ReasonAwaitingImportant EnvelopeReason = "awaiting_important"

	// ReasonDeadline indicates working toward a deadline.
	ReasonDeadline EnvelopeReason = "deadline"

	// ReasonTravelTransit indicates in transit with time-sensitive connections.
	ReasonTravelTransit EnvelopeReason = "travel_transit"

	// ReasonOnCallDuty indicates on-call rotation duty.
	ReasonOnCallDuty EnvelopeReason = "on_call_duty"

	// ReasonFamilyMatter indicates family or health matter requiring attention.
	ReasonFamilyMatter EnvelopeReason = "family_matter"
)

// AllEnvelopeReasons returns all envelope reasons in deterministic order.
func AllEnvelopeReasons() []EnvelopeReason {
	return []EnvelopeReason{
		ReasonAwaitingImportant,
		ReasonDeadline,
		ReasonTravelTransit,
		ReasonOnCallDuty,
		ReasonFamilyMatter,
	}
}

// Validate checks if the envelope reason is valid.
func (r EnvelopeReason) Validate() error {
	switch r {
	case ReasonAwaitingImportant, ReasonDeadline, ReasonTravelTransit,
		ReasonOnCallDuty, ReasonFamilyMatter:
		return nil
	default:
		return fmt.Errorf("invalid envelope reason: %s", r)
	}
}

// DisplayText returns calm, human-readable text for the reason.
func (r EnvelopeReason) DisplayText() string {
	switch r {
	case ReasonAwaitingImportant:
		return "Awaiting something important"
	case ReasonDeadline:
		return "Working toward a deadline"
	case ReasonTravelTransit:
		return "In transit"
	case ReasonOnCallDuty:
		return "On-call duty"
	case ReasonFamilyMatter:
		return "Family matter"
	default:
		return "Unknown"
	}
}

// EnvelopeState represents the current state of an envelope.
type EnvelopeState string

const (
	// StateActive indicates the envelope is currently active.
	StateActive EnvelopeState = "active"

	// StateStopped indicates the envelope was stopped early by user.
	StateStopped EnvelopeState = "stopped"

	// StateExpired indicates the envelope expired naturally.
	StateExpired EnvelopeState = "expired"
)

// AllEnvelopeStates returns all envelope states in deterministic order.
func AllEnvelopeStates() []EnvelopeState {
	return []EnvelopeState{
		StateActive,
		StateStopped,
		StateExpired,
	}
}

// Validate checks if the envelope state is valid.
func (s EnvelopeState) Validate() error {
	switch s {
	case StateActive, StateStopped, StateExpired:
		return nil
	default:
		return fmt.Errorf("invalid envelope state: %s", s)
	}
}

// AttentionEnvelope represents a time-boxed attention window.
// CRITICAL: Contains only abstract buckets and hashes. No raw timestamps exposed.
type AttentionEnvelope struct {
	// EnvelopeID is a deterministic hash of the envelope.
	EnvelopeID string

	// CircleIDHash identifies the circle this envelope applies to.
	CircleIDHash string

	// Kind is the type of attention envelope.
	Kind EnvelopeKind

	// Duration is the fixed duration bucket.
	Duration DurationBucket

	// Reason is the user's stated reason (enum only).
	Reason EnvelopeReason

	// State is the current envelope state.
	State EnvelopeState

	// StartedPeriod is the 15-minute period when envelope started.
	// Format: "YYYY-MM-DDTHH:MM" (floored to 15-minute boundary)
	StartedPeriod string

	// ExpiresAtPeriod is the 15-minute period when envelope expires.
	// Format: "YYYY-MM-DDTHH:MM" (computed from StartedPeriod + Duration)
	ExpiresAtPeriod string

	// StatusHash is a deterministic hash of the current state.
	StatusHash string
}

// CanonicalString returns the pipe-delimited, version-prefixed canonical form.
func (e *AttentionEnvelope) CanonicalString() string {
	return fmt.Sprintf("ENVELOPE|v1|%s|%s|%s|%s|%s|%s|%s",
		e.CircleIDHash,
		e.Kind,
		e.Duration,
		e.Reason,
		e.State,
		e.StartedPeriod,
		e.ExpiresAtPeriod,
	)
}

// ComputeEnvelopeID computes a deterministic envelope ID.
func (e *AttentionEnvelope) ComputeEnvelopeID() string {
	content := fmt.Sprintf("ENVELOPE_ID|v1|%s|%s|%s|%s|%s",
		e.CircleIDHash,
		e.Kind,
		e.Duration,
		e.Reason,
		e.StartedPeriod,
	)
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:16])
}

// ComputeStatusHash computes a deterministic status hash.
func (e *AttentionEnvelope) ComputeStatusHash() string {
	h := sha256.Sum256([]byte(e.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// Validate checks if the envelope is valid.
func (e *AttentionEnvelope) Validate() error {
	if e.EnvelopeID == "" {
		return errors.New("missing envelope_id")
	}
	if e.CircleIDHash == "" {
		return errors.New("missing circle_id_hash")
	}
	if err := e.Kind.Validate(); err != nil {
		return err
	}
	if err := e.Duration.Validate(); err != nil {
		return err
	}
	if err := e.Reason.Validate(); err != nil {
		return err
	}
	if err := e.State.Validate(); err != nil {
		return err
	}
	if e.StartedPeriod == "" {
		return errors.New("missing started_period")
	}
	if e.ExpiresAtPeriod == "" {
		return errors.New("missing expires_at_period")
	}
	if e.StatusHash == "" {
		return errors.New("missing status_hash")
	}
	return nil
}

// EnvelopeReceipt represents a record of envelope action.
// CRITICAL: Contains only hashes and buckets. No raw timestamps.
type EnvelopeReceipt struct {
	// ReceiptID is a deterministic hash of the receipt.
	ReceiptID string

	// EnvelopeHash is the hash of the envelope this receipt belongs to.
	EnvelopeHash string

	// CircleIDHash identifies the circle.
	CircleIDHash string

	// Action is the action taken (started, stopped, expired).
	Action EnvelopeAction

	// PeriodKey is the day bucket when action occurred.
	// Format: "YYYY-MM-DD"
	PeriodKey string

	// StatusHash is a deterministic hash of the receipt state.
	StatusHash string
}

// EnvelopeAction represents the action that generated a receipt.
type EnvelopeAction string

const (
	// ActionStarted indicates envelope was started.
	ActionStarted EnvelopeAction = "started"

	// ActionStopped indicates envelope was stopped early.
	ActionStopped EnvelopeAction = "stopped"

	// ActionExpired indicates envelope expired naturally.
	ActionExpired EnvelopeAction = "expired"

	// ActionApplied indicates envelope was applied to pressure input.
	ActionApplied EnvelopeAction = "applied"
)

// Validate checks if the action is valid.
func (a EnvelopeAction) Validate() error {
	switch a {
	case ActionStarted, ActionStopped, ActionExpired, ActionApplied:
		return nil
	default:
		return fmt.Errorf("invalid envelope action: %s", a)
	}
}

// DisplayText returns calm, human-readable text for the action.
func (a EnvelopeAction) DisplayText() string {
	switch a {
	case ActionStarted:
		return "Started"
	case ActionStopped:
		return "Stopped"
	case ActionExpired:
		return "Expired"
	case ActionApplied:
		return "Applied"
	default:
		return "Unknown"
	}
}

// CanonicalString returns the pipe-delimited, version-prefixed canonical form.
func (r *EnvelopeReceipt) CanonicalString() string {
	return fmt.Sprintf("ENVELOPE_RECEIPT|v1|%s|%s|%s|%s",
		r.EnvelopeHash,
		r.CircleIDHash,
		r.Action,
		r.PeriodKey,
	)
}

// ComputeReceiptID computes a deterministic receipt ID.
func (r *EnvelopeReceipt) ComputeReceiptID() string {
	h := sha256.Sum256([]byte(r.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// ComputeStatusHash computes a deterministic status hash.
func (r *EnvelopeReceipt) ComputeStatusHash() string {
	content := fmt.Sprintf("RECEIPT_STATUS|v1|%s|%s|%s",
		r.ReceiptID,
		r.EnvelopeHash,
		r.Action,
	)
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:16])
}

// Validate checks if the receipt is valid.
func (r *EnvelopeReceipt) Validate() error {
	if r.ReceiptID == "" {
		return errors.New("missing receipt_id")
	}
	if r.EnvelopeHash == "" {
		return errors.New("missing envelope_hash")
	}
	if r.CircleIDHash == "" {
		return errors.New("missing circle_id_hash")
	}
	if err := r.Action.Validate(); err != nil {
		return err
	}
	if r.PeriodKey == "" {
		return errors.New("missing period_key")
	}
	if r.StatusHash == "" {
		return errors.New("missing status_hash")
	}
	return nil
}

// EnvelopeProofPage represents the proof page data.
// CRITICAL: Contains ONLY abstract buckets and hashes. No raw timestamps.
type EnvelopeProofPage struct {
	// CircleIDHash identifies the circle.
	CircleIDHash string

	// CurrentEnvelopeHash is the hash of the current envelope (if active).
	// Empty if no active envelope.
	CurrentEnvelopeHash string

	// CurrentKind is the kind of the current envelope (if active).
	CurrentKind EnvelopeKind

	// CurrentDuration is the duration of the current envelope (if active).
	CurrentDuration DurationBucket

	// CurrentState is the state of the current envelope.
	CurrentState EnvelopeState

	// RecentReceiptCount is the count of recent receipts (bucketed).
	// Buckets: 0, 1-3, 4+
	RecentReceiptCount string

	// PageHash is a deterministic hash of the page content.
	PageHash string
}

// CanonicalString returns the pipe-delimited, version-prefixed canonical form.
func (p *EnvelopeProofPage) CanonicalString() string {
	return fmt.Sprintf("ENVELOPE_PROOF|v1|%s|%s|%s|%s|%s|%s",
		p.CircleIDHash,
		p.CurrentEnvelopeHash,
		p.CurrentKind,
		p.CurrentDuration,
		p.CurrentState,
		p.RecentReceiptCount,
	)
}

// ComputePageHash computes a deterministic page hash.
func (p *EnvelopeProofPage) ComputePageHash() string {
	h := sha256.Sum256([]byte(p.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// NewPeriodKey creates a 15-minute period key from a time.
// Format: "YYYY-MM-DDTHH:MM" floored to 15-minute boundary.
func NewPeriodKey(t time.Time) string {
	utc := t.UTC()
	minute := (utc.Minute() / 15) * 15
	floored := time.Date(utc.Year(), utc.Month(), utc.Day(), utc.Hour(), minute, 0, 0, time.UTC)
	return floored.Format("2006-01-02T15:04")
}

// NewDayKey creates a day bucket key from a time.
// Format: "YYYY-MM-DD"
func NewDayKey(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

// ParsePeriodKey parses a period key into a time.
func ParsePeriodKey(key string) (time.Time, error) {
	return time.Parse("2006-01-02T15:04", key)
}

// ComputeExpiryPeriod computes the expiry period from start period and duration.
func ComputeExpiryPeriod(startPeriod string, duration DurationBucket) (string, error) {
	startTime, err := ParsePeriodKey(startPeriod)
	if err != nil {
		return "", fmt.Errorf("invalid start period: %w", err)
	}
	expiryTime := startTime.Add(duration.ToDuration())
	return NewPeriodKey(expiryTime), nil
}

// IsExpired checks if a period has passed relative to a clock.
func IsExpired(expiryPeriod string, clock time.Time) (bool, error) {
	expiryTime, err := ParsePeriodKey(expiryPeriod)
	if err != nil {
		return false, fmt.Errorf("invalid expiry period: %w", err)
	}
	// Expired if clock is at or past the expiry period start
	return !clock.Before(expiryTime), nil
}

// BucketReceiptCount converts a count to an abstract bucket string.
func BucketReceiptCount(count int) string {
	switch {
	case count == 0:
		return "none"
	case count <= 3:
		return "a_few"
	default:
		return "several"
	}
}

// ReceiptCountBucket represents the abstract count of receipts.
// CRITICAL: No raw counts exposed. Buckets only.
type ReceiptCountBucket string

const (
	// ReceiptCountNone indicates no receipts.
	ReceiptCountNone ReceiptCountBucket = "none"

	// ReceiptCountFew indicates 1-3 receipts.
	ReceiptCountFew ReceiptCountBucket = "a_few"

	// ReceiptCountSeveral indicates 4+ receipts.
	ReceiptCountSeveral ReceiptCountBucket = "several"
)

// DisplayText returns calm, human-readable text for the count.
func (c ReceiptCountBucket) DisplayText() string {
	switch c {
	case ReceiptCountNone:
		return "None"
	case ReceiptCountFew:
		return "A few"
	case ReceiptCountSeveral:
		return "Several"
	default:
		return "Unknown"
	}
}

// ToReceiptCountBucket converts a count to a ReceiptCountBucket.
func ToReceiptCountBucket(count int) ReceiptCountBucket {
	switch {
	case count == 0:
		return ReceiptCountNone
	case count <= 3:
		return ReceiptCountFew
	default:
		return ReceiptCountSeveral
	}
}

// EnvelopeProofPageV2 represents the proof page data.
// CRITICAL: Contains ONLY abstract buckets and hashes. No raw timestamps.
type EnvelopeProofPageV2 struct {
	// CircleIDHash identifies the circle.
	CircleIDHash string

	// ReceiptCountBucket is the abstract count of receipts.
	ReceiptCountBucket ReceiptCountBucket

	// StatusHash is a deterministic hash of the page content.
	StatusHash string
}

// BuildEnvelopeProofPage constructs an envelope proof page from receipts.
// CRITICAL: Returns only abstract buckets and hashes.
func BuildEnvelopeProofPage(receipts []*EnvelopeReceipt) *EnvelopeProofPageV2 {
	page := &EnvelopeProofPageV2{
		ReceiptCountBucket: ToReceiptCountBucket(len(receipts)),
	}

	// Compute status hash from all receipt hashes
	content := "PROOF_PAGE|v1"
	for _, r := range receipts {
		content += "|" + r.EnvelopeHash
	}
	h := sha256.Sum256([]byte(content))
	page.StatusHash = hex.EncodeToString(h[:16])

	return page
}
