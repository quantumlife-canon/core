// Package marketsignal provides domain types for Phase 48: Market Signal Binding.
//
// This package binds unmet necessities to available marketplace packs WITHOUT:
// - Recommendations
// - Nudges
// - Ranking
// - Persuasion
// - Execution
//
// This is signal exposure only, not a funnel.
//
// CRITICAL: No time.Now() in this package - clock must be injected.
// CRITICAL: No goroutines in this package.
// CRITICAL: All signals have effect_no_power and proof_only visibility.
//
// Reference: docs/ADR/ADR-0086-phase48-market-signal-binding.md
package marketsignal

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
)

// MarketSignalKind represents the type of market signal.
type MarketSignalKind string

const (
	// MarketSignalCoverageGap indicates a gap between necessity and coverage.
	MarketSignalCoverageGap MarketSignalKind = "coverage_gap"
)

// Validate checks if the MarketSignalKind is valid.
func (k MarketSignalKind) Validate() error {
	switch k {
	case MarketSignalCoverageGap:
		return nil
	default:
		return fmt.Errorf("invalid MarketSignalKind: %s", k)
	}
}

// CanonicalString returns the canonical string representation.
func (k MarketSignalKind) CanonicalString() string {
	return string(k)
}

// String returns the string representation.
func (k MarketSignalKind) String() string {
	return string(k)
}

// AllMarketSignalKinds returns all valid signal kinds in stable order.
func AllMarketSignalKinds() []MarketSignalKind {
	return []MarketSignalKind{
		MarketSignalCoverageGap,
	}
}

// MarketSignalEffect represents what power the signal grants.
// In Phase 48, this MUST always be EffectNoPower.
type MarketSignalEffect string

const (
	// EffectNoPower is the ONLY allowed value in Phase 48.
	// Market signals provide observation but do NOT grant permission.
	EffectNoPower MarketSignalEffect = "effect_no_power"
)

// Validate checks if the MarketSignalEffect is valid.
// In Phase 48, only EffectNoPower is valid.
func (e MarketSignalEffect) Validate() error {
	if e == EffectNoPower {
		return nil
	}
	return fmt.Errorf("invalid MarketSignalEffect: %s (only effect_no_power allowed)", e)
}

// CanonicalString returns the canonical string representation.
func (e MarketSignalEffect) CanonicalString() string {
	return string(e)
}

// String returns the string representation.
func (e MarketSignalEffect) String() string {
	return string(e)
}

// MarketSignalVisibility represents how the signal is surfaced.
// In Phase 48, this MUST always be VisibilityProofOnly.
type MarketSignalVisibility string

const (
	// VisibilityProofOnly means signals are only shown in proof pages.
	// They are never pushed, never prioritized, never urgent.
	VisibilityProofOnly MarketSignalVisibility = "proof_only"
)

// Validate checks if the MarketSignalVisibility is valid.
// In Phase 48, only VisibilityProofOnly is valid.
func (v MarketSignalVisibility) Validate() error {
	if v == VisibilityProofOnly {
		return nil
	}
	return fmt.Errorf("invalid MarketSignalVisibility: %s (only proof_only allowed)", v)
}

// CanonicalString returns the canonical string representation.
func (v MarketSignalVisibility) CanonicalString() string {
	return string(v)
}

// String returns the string representation.
func (v MarketSignalVisibility) String() string {
	return string(v)
}

// NecessityKind represents the kind of necessity (from Phase 45).
// Re-exported here to avoid direct import of Phase 45 types in signals.
type NecessityKind string

const (
	NecessityKindLow     NecessityKind = "necessity_low"
	NecessityKindMedium  NecessityKind = "necessity_medium"
	NecessityKindHigh    NecessityKind = "necessity_high"
	NecessityKindUnknown NecessityKind = "necessity_unknown"
)

// Validate checks if the NecessityKind is valid.
func (n NecessityKind) Validate() error {
	switch n {
	case NecessityKindLow, NecessityKindMedium, NecessityKindHigh, NecessityKindUnknown:
		return nil
	default:
		return fmt.Errorf("invalid NecessityKind: %s", n)
	}
}

// CanonicalString returns the canonical string representation.
func (n NecessityKind) CanonicalString() string {
	return string(n)
}

// String returns the string representation.
func (n NecessityKind) String() string {
	return string(n)
}

// AllNecessityKinds returns all valid necessity kinds in stable order.
func AllNecessityKinds() []NecessityKind {
	return []NecessityKind{
		NecessityKindHigh,
		NecessityKindLow,
		NecessityKindMedium,
		NecessityKindUnknown,
	}
}

// CoverageGapKind represents the type of coverage gap.
type CoverageGapKind string

const (
	GapNoObserver   CoverageGapKind = "gap_no_observer"
	GapPartialCover CoverageGapKind = "gap_partial_cover"
)

// Validate checks if the CoverageGapKind is valid.
func (g CoverageGapKind) Validate() error {
	switch g {
	case GapNoObserver, GapPartialCover:
		return nil
	default:
		return fmt.Errorf("invalid CoverageGapKind: %s", g)
	}
}

// CanonicalString returns the canonical string representation.
func (g CoverageGapKind) CanonicalString() string {
	return string(g)
}

// String returns the string representation.
func (g CoverageGapKind) String() string {
	return string(g)
}

// AllCoverageGapKinds returns all valid gap kinds in stable order.
func AllCoverageGapKinds() []CoverageGapKind {
	return []CoverageGapKind{
		GapNoObserver,
		GapPartialCover,
	}
}

// MarketSignal represents a market signal binding unmet necessity to available pack.
// CRITICAL: This is signal exposure only - no recommendations, no nudges, no ranking.
type MarketSignal struct {
	SignalID     string                 // Deterministic hash of signal content
	CircleHash   string                 // SHA256 hash of circle ID
	NecessityKind NecessityKind         // What necessity level was declared
	CoverageGap  CoverageGapKind        // What kind of coverage gap exists
	PackIDHash   string                 // SHA256 hash of pack slug
	Kind         MarketSignalKind       // Type of signal (always coverage_gap)
	Effect       MarketSignalEffect     // MUST be effect_no_power
	Visibility   MarketSignalVisibility // MUST be proof_only
	PeriodKey    string                 // Period key (YYYY-MM-DD)
}

// Validate checks if the MarketSignal is valid.
func (s MarketSignal) Validate() error {
	if s.SignalID == "" {
		return errors.New("SignalID is required")
	}
	if s.CircleHash == "" {
		return errors.New("CircleHash is required")
	}
	if err := s.NecessityKind.Validate(); err != nil {
		return fmt.Errorf("NecessityKind: %w", err)
	}
	if err := s.CoverageGap.Validate(); err != nil {
		return fmt.Errorf("CoverageGap: %w", err)
	}
	if s.PackIDHash == "" {
		return errors.New("PackIDHash is required")
	}
	if err := s.Kind.Validate(); err != nil {
		return fmt.Errorf("Kind: %w", err)
	}
	if err := s.Effect.Validate(); err != nil {
		return fmt.Errorf("Effect: %w", err)
	}
	if err := s.Visibility.Validate(); err != nil {
		return fmt.Errorf("Visibility: %w", err)
	}
	if s.PeriodKey == "" {
		return errors.New("PeriodKey is required")
	}
	return nil
}

// CanonicalString returns a pipe-delimited canonical string.
func (s MarketSignal) CanonicalString() string {
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s",
		s.CircleHash,
		s.NecessityKind.CanonicalString(),
		s.CoverageGap.CanonicalString(),
		s.PackIDHash,
		s.Kind.CanonicalString(),
		s.Effect.CanonicalString(),
		s.Visibility.CanonicalString(),
		s.PeriodKey,
	)
}

// ComputeSignalID computes the deterministic signal ID from canonical string.
func (s MarketSignal) ComputeSignalID() string {
	return HashString(s.CanonicalString())
}

// MarketProofAckKind represents acknowledgment type.
type MarketProofAckKind string

const (
	AckViewed    MarketProofAckKind = "ack_viewed"
	AckDismissed MarketProofAckKind = "ack_dismissed"
)

// Validate checks if the MarketProofAckKind is valid.
func (k MarketProofAckKind) Validate() error {
	switch k {
	case AckViewed, AckDismissed:
		return nil
	default:
		return fmt.Errorf("invalid MarketProofAckKind: %s", k)
	}
}

// CanonicalString returns the canonical string representation.
func (k MarketProofAckKind) CanonicalString() string {
	return string(k)
}

// String returns the string representation.
func (k MarketProofAckKind) String() string {
	return string(k)
}

// MarketProofAck represents an acknowledgment of market proof.
type MarketProofAck struct {
	CircleHash string             // SHA256 hash of circle ID
	PeriodKey  string             // Period key (YYYY-MM-DD)
	AckKind    MarketProofAckKind // Type of acknowledgment
	StatusHash string             // SHA256 hash of ack state
}

// Validate checks if the MarketProofAck is valid.
func (a MarketProofAck) Validate() error {
	if a.CircleHash == "" {
		return errors.New("CircleHash is required")
	}
	if a.PeriodKey == "" {
		return errors.New("PeriodKey is required")
	}
	if err := a.AckKind.Validate(); err != nil {
		return err
	}
	if a.StatusHash == "" {
		return errors.New("StatusHash is required")
	}
	return nil
}

// CanonicalString returns a pipe-delimited canonical string.
func (a MarketProofAck) CanonicalString() string {
	return fmt.Sprintf("%s|%s|%s",
		a.CircleHash,
		a.PeriodKey,
		a.AckKind.CanonicalString(),
	)
}

// ComputeStatusHash computes the SHA256 hash of the canonical ack string.
func (a MarketProofAck) ComputeStatusHash() string {
	return HashString(a.CanonicalString())
}

// MarketProofPage represents the UI model for market proof.
type MarketProofPage struct {
	Title      string   // Page title
	Lines      []string // Calm copy lines (no recommendations!)
	Signals    []MarketSignalDisplay
	StatusHash string
}

// MarketSignalDisplay represents a signal for display.
// CRITICAL: No pricing, no install buttons, no urgency, no calls to action.
type MarketSignalDisplay struct {
	SignalID      string
	NecessityKind string // Display-safe necessity label
	GapKind       string // Display-safe gap label
	PackLabel     string // Abstract pack name (never specific)
	Effect        MarketSignalEffect
	Visibility    MarketSignalVisibility
}

// MarketProofCue represents the whisper cue for market signals.
type MarketProofCue struct {
	Available  bool   // Whether the cue should be shown
	Text       string // Cue text (whisper style, no recommendations)
	Path       string // Path to proof page
	StatusHash string // Hash of cue state
}

// HashString computes SHA256 hash of a string and returns hex-encoded result.
func HashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// NormalizeSignals sorts and deduplicates signals by SignalID.
func NormalizeSignals(signals []MarketSignal) []MarketSignal {
	if len(signals) == 0 {
		return []MarketSignal{}
	}

	// Deduplicate by SignalID
	seen := make(map[string]bool)
	result := make([]MarketSignal, 0, len(signals))
	for _, sig := range signals {
		if !seen[sig.SignalID] {
			seen[sig.SignalID] = true
			result = append(result, sig)
		}
	}

	// Sort by SignalID for deterministic output
	sort.Slice(result, func(i, j int) bool {
		return result[i].SignalID < result[j].SignalID
	})

	return result
}

// ComputeProofStatusHash computes the status hash for a proof page.
func ComputeProofStatusHash(signals []MarketSignal) string {
	if len(signals) == 0 {
		return HashString("proof|empty")
	}
	canonical := "proof"
	for _, sig := range signals {
		canonical += "|" + sig.SignalID
	}
	return HashString(canonical)
}

// ComputeCueStatusHash computes the status hash for a cue.
func ComputeCueStatusHash(signalCount int, available bool) string {
	availStr := "hidden"
	if available {
		availStr = "shown"
	}
	return HashString(fmt.Sprintf("cue|%d|%s", signalCount, availStr))
}

// Bounded retention constants.
const (
	MaxMarketSignalRecords    = 200
	MaxMarketSignalDays       = 30
	MaxSignalsPerCirclePeriod = 3
)
