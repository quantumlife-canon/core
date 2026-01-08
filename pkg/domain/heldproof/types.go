// Package heldproof provides domain types for Phase 43: Held Under Agreement Proof Ledger.
//
// This package provides proof-only types for displaying when delegated holding
// contracts (Phase 42) produce QUEUE_PROOF outcomes. No behavior changes.
//
// CRITICAL INVARIANTS:
//   - Proof-only. No decisions. No behavior changes.
//   - Hash-only storage. No raw identifiers, timestamps, or amounts.
//   - Abstract buckets only: magnitude/horizon/circle_type.
//   - Commerce excluded: circle_type=commerce is forbidden and must be rejected.
//   - Deterministic: same inputs + clock => same hashes and outcomes.
//   - No goroutines. No time.Now() - clock injection only.
//   - Bounded retention: 30 days OR max records, FIFO eviction.
//
// Reference: docs/ADR/ADR-0080-phase43-held-under-agreement-proof-ledger.md
package heldproof

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
)

// ============================================================================
// Constants
// ============================================================================

// Retention limits
const (
	MaxRetentionDays     = 30
	MaxSignalRecords     = 500
	MaxAckRecords        = 200
	MaxSignalsPerPage    = 3
	MaxSignalsPerCircle  = 1
)

// Fixed UX copy
const (
	DefaultTitle   = "Held, by agreement."
	DefaultCueText = "We held some things — by agreement."
	DefaultPath    = "/proof/held"

	LineAFew    = "A few things were held so you didn't have to decide yet."
	LineSeveral = "Several pressures were held under your agreement."
)

// ============================================================================
// Enums
// ============================================================================

// HeldProofKind indicates the source of the held proof signal.
type HeldProofKind string

const (
	// KindDelegatedHolding is from Phase 42 delegated holding contracts.
	KindDelegatedHolding HeldProofKind = "heldproof_delegated_holding"
)

// AllHeldProofKinds returns all kinds in deterministic order.
func AllHeldProofKinds() []HeldProofKind {
	return []HeldProofKind{KindDelegatedHolding}
}

// Validate checks if the kind is valid.
func (k HeldProofKind) Validate() error {
	switch k {
	case KindDelegatedHolding:
		return nil
	default:
		return errors.New("invalid HeldProofKind")
	}
}

// CanonicalString returns the canonical string representation.
func (k HeldProofKind) CanonicalString() string {
	return string(k)
}

// HeldProofMagnitudeBucket represents abstract magnitude.
type HeldProofMagnitudeBucket string

const (
	MagnitudeNothing HeldProofMagnitudeBucket = "nothing"
	MagnitudeAFew    HeldProofMagnitudeBucket = "a_few"
	MagnitudeSeveral HeldProofMagnitudeBucket = "several"
)

// AllMagnitudeBuckets returns all magnitude buckets in deterministic order.
func AllMagnitudeBuckets() []HeldProofMagnitudeBucket {
	return []HeldProofMagnitudeBucket{MagnitudeNothing, MagnitudeAFew, MagnitudeSeveral}
}

// Validate checks if the magnitude is valid.
func (m HeldProofMagnitudeBucket) Validate() error {
	switch m {
	case MagnitudeNothing, MagnitudeAFew, MagnitudeSeveral:
		return nil
	default:
		return errors.New("invalid HeldProofMagnitudeBucket")
	}
}

// CanonicalString returns the canonical string representation.
func (m HeldProofMagnitudeBucket) CanonicalString() string {
	return string(m)
}

// HeldProofHorizonBucket represents abstract time horizon.
type HeldProofHorizonBucket string

const (
	HorizonNow   HeldProofHorizonBucket = "now"
	HorizonSoon  HeldProofHorizonBucket = "soon"
	HorizonLater HeldProofHorizonBucket = "later"
)

// AllHorizonBuckets returns all horizon buckets in deterministic order.
func AllHorizonBuckets() []HeldProofHorizonBucket {
	return []HeldProofHorizonBucket{HorizonNow, HorizonSoon, HorizonLater}
}

// Validate checks if the horizon is valid.
func (h HeldProofHorizonBucket) Validate() error {
	switch h {
	case HorizonNow, HorizonSoon, HorizonLater:
		return nil
	default:
		return errors.New("invalid HeldProofHorizonBucket")
	}
}

// CanonicalString returns the canonical string representation.
func (h HeldProofHorizonBucket) CanonicalString() string {
	return string(h)
}

// HeldProofCircleType represents the type of circle.
// NOTE: Commerce is forbidden and must be rejected by the engine.
type HeldProofCircleType string

const (
	CircleTypeHuman       HeldProofCircleType = "human"
	CircleTypeInstitution HeldProofCircleType = "institution"
	CircleTypeCommerce    HeldProofCircleType = "commerce"
	CircleTypeUnknown     HeldProofCircleType = "unknown"
)

// AllCircleTypes returns all circle types in deterministic order.
func AllCircleTypes() []HeldProofCircleType {
	return []HeldProofCircleType{CircleTypeHuman, CircleTypeInstitution, CircleTypeCommerce, CircleTypeUnknown}
}

// Validate checks if the circle type is valid.
func (c HeldProofCircleType) Validate() error {
	switch c {
	case CircleTypeHuman, CircleTypeInstitution, CircleTypeCommerce, CircleTypeUnknown:
		return nil
	default:
		return errors.New("invalid HeldProofCircleType")
	}
}

// IsCommerce returns true if this is a commerce circle type.
// Commerce is forbidden in held proof signals.
func (c HeldProofCircleType) IsCommerce() bool {
	return c == CircleTypeCommerce
}

// CanonicalString returns the canonical string representation.
func (c HeldProofCircleType) CanonicalString() string {
	return string(c)
}

// HeldProofAckKind represents acknowledgment types.
type HeldProofAckKind string

const (
	AckViewed    HeldProofAckKind = "viewed"
	AckDismissed HeldProofAckKind = "dismissed"
)

// AllAckKinds returns all ack kinds in deterministic order.
func AllAckKinds() []HeldProofAckKind {
	return []HeldProofAckKind{AckViewed, AckDismissed}
}

// Validate checks if the ack kind is valid.
func (a HeldProofAckKind) Validate() error {
	switch a {
	case AckViewed, AckDismissed:
		return nil
	default:
		return errors.New("invalid HeldProofAckKind")
	}
}

// CanonicalString returns the canonical string representation.
func (a HeldProofAckKind) CanonicalString() string {
	return string(a)
}

// ============================================================================
// Structs
// ============================================================================

// HeldProofPeriod represents a day period.
type HeldProofPeriod struct {
	// DayKey is the canonical YYYY-MM-DD derived from injected clock.
	DayKey string
}

// Validate checks if the period is valid.
func (p *HeldProofPeriod) Validate() error {
	if p.DayKey == "" {
		return errors.New("DayKey is required")
	}
	// Basic format check: YYYY-MM-DD is 10 chars
	if len(p.DayKey) != 10 {
		return errors.New("DayKey must be YYYY-MM-DD format")
	}
	return nil
}

// HeldProofSignal represents a single held proof signal.
type HeldProofSignal struct {
	// Kind indicates the source (delegated_holding).
	Kind HeldProofKind

	// CircleType is the abstract circle type bucket.
	// CRITICAL: Commerce is forbidden.
	CircleType HeldProofCircleType

	// Horizon is the abstract time horizon bucket.
	Horizon HeldProofHorizonBucket

	// Magnitude is the abstract magnitude bucket.
	Magnitude HeldProofMagnitudeBucket

	// EvidenceHash is the sha256 hex of the canonical evidence.
	EvidenceHash string
}

// Validate checks if the signal is valid.
func (s *HeldProofSignal) Validate() error {
	if err := s.Kind.Validate(); err != nil {
		return err
	}
	if err := s.CircleType.Validate(); err != nil {
		return err
	}
	if s.CircleType.IsCommerce() {
		return errors.New("commerce circle type is forbidden")
	}
	if err := s.Horizon.Validate(); err != nil {
		return err
	}
	if err := s.Magnitude.Validate(); err != nil {
		return err
	}
	if s.EvidenceHash == "" {
		return errors.New("EvidenceHash is required")
	}
	return nil
}

// CanonicalString returns the pipe-delimited canonical representation.
func (s *HeldProofSignal) CanonicalString() string {
	var b strings.Builder
	b.WriteString("HPS|")
	b.WriteString(s.Kind.CanonicalString())
	b.WriteString("|")
	b.WriteString(s.CircleType.CanonicalString())
	b.WriteString("|")
	b.WriteString(s.Horizon.CanonicalString())
	b.WriteString("|")
	b.WriteString(s.Magnitude.CanonicalString())
	b.WriteString("|")
	b.WriteString(s.EvidenceHash)
	return b.String()
}

// ComputeHash computes the sha256 hash of the canonical string.
func (s *HeldProofSignal) ComputeHash() string {
	h := sha256.Sum256([]byte(s.CanonicalString()))
	return hex.EncodeToString(h[:])
}

// HeldProofPage represents the proof page data.
type HeldProofPage struct {
	// Title is always "Held, by agreement."
	Title string

	// Line is a single calm deterministic line.
	Line string

	// Chips are unique circle type strings (max 3, sorted).
	Chips []string

	// Magnitude is the derived magnitude bucket.
	Magnitude HeldProofMagnitudeBucket

	// StatusHash is the sha256 hex of the page canonical string.
	StatusHash string
}

// Validate checks if the page is valid.
func (p *HeldProofPage) Validate() error {
	if p.Title == "" {
		return errors.New("Title is required")
	}
	if p.Line == "" {
		return errors.New("Line is required")
	}
	if len(p.Chips) > MaxSignalsPerPage {
		return errors.New("too many chips")
	}
	if err := p.Magnitude.Validate(); err != nil {
		return err
	}
	if p.StatusHash == "" {
		return errors.New("StatusHash is required")
	}
	return nil
}

// CanonicalString returns the pipe-delimited canonical representation.
func (p *HeldProofPage) CanonicalString() string {
	var b strings.Builder
	b.WriteString("HPP|")
	b.WriteString(p.Title)
	b.WriteString("|")
	b.WriteString(p.Line)
	b.WriteString("|")
	// Sort chips for determinism
	sortedChips := make([]string, len(p.Chips))
	copy(sortedChips, p.Chips)
	sort.Strings(sortedChips)
	b.WriteString(strings.Join(sortedChips, ","))
	b.WriteString("|")
	b.WriteString(p.Magnitude.CanonicalString())
	return b.String()
}

// ComputeHash computes the sha256 hash of the canonical string.
func (p *HeldProofPage) ComputeHash() string {
	h := sha256.Sum256([]byte(p.CanonicalString()))
	return hex.EncodeToString(h[:])
}

// HeldProofAck represents an acknowledgment of the proof page.
type HeldProofAck struct {
	// Period is the day period.
	Period HeldProofPeriod

	// AckKind is the type of acknowledgment.
	AckKind HeldProofAckKind

	// StatusHash is the page status hash being acknowledged.
	StatusHash string

	// AckHash is the sha256 hex of the canonical ack string.
	AckHash string
}

// Validate checks if the ack is valid.
func (a *HeldProofAck) Validate() error {
	if err := a.Period.Validate(); err != nil {
		return err
	}
	if err := a.AckKind.Validate(); err != nil {
		return err
	}
	if a.StatusHash == "" {
		return errors.New("StatusHash is required")
	}
	if a.AckHash == "" {
		return errors.New("AckHash is required")
	}
	return nil
}

// CanonicalString returns the pipe-delimited canonical representation.
func (a *HeldProofAck) CanonicalString() string {
	var b strings.Builder
	b.WriteString("HPA|")
	b.WriteString(a.Period.DayKey)
	b.WriteString("|")
	b.WriteString(a.AckKind.CanonicalString())
	b.WriteString("|")
	b.WriteString(a.StatusHash)
	return b.String()
}

// ComputeHash computes the sha256 hash of the canonical string.
func (a *HeldProofAck) ComputeHash() string {
	h := sha256.Sum256([]byte(a.CanonicalString()))
	return hex.EncodeToString(h[:])
}

// HeldProofCue represents the cue shown on /today.
type HeldProofCue struct {
	// Available indicates if the cue should be shown.
	Available bool

	// CueText is the text to display.
	CueText string

	// Path is always "/proof/held".
	Path string

	// StatusHash must match the page StatusHash.
	StatusHash string
}

// ============================================================================
// Helper Functions
// ============================================================================

// ComputeEvidenceHash computes the evidence hash from components.
func ComputeEvidenceHash(dayKey string, kind HeldProofKind, circleType HeldProofCircleType, horizon HeldProofHorizonBucket, magnitude HeldProofMagnitudeBucket, sourceHash string) string {
	var b strings.Builder
	b.WriteString(dayKey)
	b.WriteString("|")
	b.WriteString(kind.CanonicalString())
	b.WriteString("|")
	b.WriteString(circleType.CanonicalString())
	b.WriteString("|")
	b.WriteString(horizon.CanonicalString())
	b.WriteString("|")
	b.WriteString(magnitude.CanonicalString())
	b.WriteString("|")
	b.WriteString(sourceHash)

	h := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(h[:])
}

// MagnitudeFromCount derives magnitude bucket from signal count.
func MagnitudeFromCount(count int) HeldProofMagnitudeBucket {
	switch {
	case count == 0:
		return MagnitudeNothing
	case count == 1:
		return MagnitudeAFew
	default:
		return MagnitudeSeveral
	}
}

// LineFromMagnitude returns the appropriate line for the magnitude.
func LineFromMagnitude(mag HeldProofMagnitudeBucket) string {
	switch mag {
	case MagnitudeAFew:
		return LineAFew
	case MagnitudeSeveral:
		return LineSeveral
	default:
		return ""
	}
}
