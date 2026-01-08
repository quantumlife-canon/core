// Package heldproof provides the engine for Phase 43: Held Under Agreement Proof Ledger.
//
// This engine builds proof signals and pages from Phase 42 QUEUE_PROOF outcomes.
// It is proof-only: no decisions, no behavior changes.
//
// CRITICAL INVARIANTS:
//   - NO time.Now() - clock injection required.
//   - NO goroutines.
//   - Commerce excluded: circle_type=commerce is rejected.
//   - Max 3 signals per page.
//   - Max 1 signal per circle type.
//   - Deterministic ordering by EvidenceHash.
//
// Reference: docs/ADR/ADR-0080-phase43-held-under-agreement-proof-ledger.md
package heldproof

import (
	"sort"
	"time"

	hp "quantumlife/pkg/domain/heldproof"
)

// ============================================================================
// Input Types (internal only)
// ============================================================================

// HeldProofDecisionInput represents an abstract decision input.
// CRITICAL: Only abstract buckets and hashes allowed. No identifiers.
type HeldProofDecisionInput struct {
	// CircleType is the abstract circle type (human/institution only; commerce forbidden).
	CircleType hp.HeldProofCircleType

	// Horizon is the abstract time horizon bucket.
	Horizon hp.HeldProofHorizonBucket

	// Magnitude is the abstract magnitude bucket.
	Magnitude hp.HeldProofMagnitudeBucket

	// QueuedProof must be true to generate a signal.
	QueuedProof bool

	// SourceHash is the sha256 hex of upstream abstract record.
	SourceHash string
}

// HeldProofInputs contains all inputs for building held proof signals.
type HeldProofInputs struct {
	// DayKey is the canonical YYYY-MM-DD.
	DayKey string

	// Decisions are the abstract decision inputs.
	Decisions []HeldProofDecisionInput

	// ContractActive indicates if a delegation contract is active.
	ContractActive bool
}

// ============================================================================
// Interfaces
// ============================================================================

// SignalStore stores held proof signals.
type SignalStore interface {
	AppendSignal(dayKey string, sig hp.HeldProofSignal) error
	ListSignals(dayKey string) []hp.HeldProofSignal
}

// AckStore stores held proof acknowledgments.
type AckStore interface {
	RecordViewed(dayKey, statusHash string) error
	RecordDismissed(dayKey, statusHash string) error
	IsDismissed(dayKey, statusHash string) bool
	HasViewed(dayKey, statusHash string) bool
}

// Clock provides time injection.
type Clock interface {
	Now() time.Time
}

// ============================================================================
// Engine
// ============================================================================

// Engine builds held proof signals and pages.
type Engine struct {
	signalStore SignalStore
	ackStore    AckStore
	clk         Clock
}

// NewEngine creates a new held proof engine.
func NewEngine(signalStore SignalStore, ackStore AckStore, clk Clock) *Engine {
	return &Engine{
		signalStore: signalStore,
		ackStore:    ackStore,
		clk:         clk,
	}
}

// ============================================================================
// Signal Building
// ============================================================================

// BuildSignals builds held proof signals from decision inputs.
// Rules:
//   - Include only items where QueuedProof=true.
//   - Reject/skip if CircleType==commerce.
//   - Deterministic ordering: sort by EvidenceHash lexicographically.
//   - Cap: max 3 signals.
//   - Cap: max 1 signal per CircleType.
func (e *Engine) BuildSignals(in HeldProofInputs) ([]hp.HeldProofSignal, error) {
	var signals []hp.HeldProofSignal
	seenCircleTypes := make(map[hp.HeldProofCircleType]bool)

	for _, d := range in.Decisions {
		// Skip if not queued for proof
		if !d.QueuedProof {
			continue
		}

		// CRITICAL: Reject commerce
		if d.CircleType.IsCommerce() {
			continue
		}

		// Skip if we already have a signal for this circle type
		if seenCircleTypes[d.CircleType] {
			continue
		}

		// Compute evidence hash
		evidenceHash := hp.ComputeEvidenceHash(
			in.DayKey,
			hp.KindDelegatedHolding,
			d.CircleType,
			d.Horizon,
			d.Magnitude,
			d.SourceHash,
		)

		sig := hp.HeldProofSignal{
			Kind:         hp.KindDelegatedHolding,
			CircleType:   d.CircleType,
			Horizon:      d.Horizon,
			Magnitude:    d.Magnitude,
			EvidenceHash: evidenceHash,
		}

		if err := sig.Validate(); err != nil {
			continue
		}

		signals = append(signals, sig)
		seenCircleTypes[d.CircleType] = true
	}

	// Sort by EvidenceHash for determinism
	sort.Slice(signals, func(i, j int) bool {
		return signals[i].EvidenceHash < signals[j].EvidenceHash
	})

	// Cap at max 3
	if len(signals) > hp.MaxSignalsPerPage {
		signals = signals[:hp.MaxSignalsPerPage]
	}

	return signals, nil
}

// BuildSignalFromDecision builds a single signal from a decision.
// CRITICAL: Rejects commerce.
func (e *Engine) BuildSignalFromDecision(dayKey string, d HeldProofDecisionInput) (*hp.HeldProofSignal, error) {
	if !d.QueuedProof {
		return nil, nil
	}

	if d.CircleType.IsCommerce() {
		return nil, nil
	}

	evidenceHash := hp.ComputeEvidenceHash(
		dayKey,
		hp.KindDelegatedHolding,
		d.CircleType,
		d.Horizon,
		d.Magnitude,
		d.SourceHash,
	)

	sig := &hp.HeldProofSignal{
		Kind:         hp.KindDelegatedHolding,
		CircleType:   d.CircleType,
		Horizon:      d.Horizon,
		Magnitude:    d.Magnitude,
		EvidenceHash: evidenceHash,
	}

	if err := sig.Validate(); err != nil {
		return nil, err
	}

	return sig, nil
}

// ============================================================================
// Page Building
// ============================================================================

// BuildPage builds a held proof page from signals.
// Returns (nil, "", nil) if no signals.
func (e *Engine) BuildPage(signals []hp.HeldProofSignal, period hp.HeldProofPeriod) (*hp.HeldProofPage, string, error) {
	if len(signals) == 0 {
		return nil, "", nil
	}

	// Filter out commerce and enforce caps
	var filtered []hp.HeldProofSignal
	seenTypes := make(map[hp.HeldProofCircleType]bool)

	for _, sig := range signals {
		if sig.CircleType.IsCommerce() {
			continue
		}
		if seenTypes[sig.CircleType] {
			continue
		}
		filtered = append(filtered, sig)
		seenTypes[sig.CircleType] = true

		if len(filtered) >= hp.MaxSignalsPerPage {
			break
		}
	}

	if len(filtered) == 0 {
		return nil, "", nil
	}

	// Build chips (unique circle types, sorted)
	chipSet := make(map[string]bool)
	for _, sig := range filtered {
		chipSet[sig.CircleType.CanonicalString()] = true
	}
	var chips []string
	for chip := range chipSet {
		chips = append(chips, chip)
	}
	sort.Strings(chips)

	// Derive magnitude from count
	magnitude := hp.MagnitudeFromCount(len(filtered))

	// Get line from magnitude
	line := hp.LineFromMagnitude(magnitude)
	if line == "" {
		return nil, "", nil
	}

	page := &hp.HeldProofPage{
		Title:     hp.DefaultTitle,
		Line:      line,
		Chips:     chips,
		Magnitude: magnitude,
	}

	// Compute status hash
	page.StatusHash = page.ComputeHash()

	return page, page.StatusHash, nil
}

// ============================================================================
// Cue Building
// ============================================================================

// BuildCue builds a held proof cue for /today.
// Returns nil if page is nil or dismissed or viewed.
func (e *Engine) BuildCue(page *hp.HeldProofPage, dismissed bool, viewed bool) *hp.HeldProofCue {
	if page == nil {
		return nil
	}

	if dismissed {
		return &hp.HeldProofCue{
			Available:  false,
			CueText:    hp.DefaultCueText,
			Path:       hp.DefaultPath,
			StatusHash: page.StatusHash,
		}
	}

	// Policy: cue disappears after view
	if viewed {
		return &hp.HeldProofCue{
			Available:  false,
			CueText:    hp.DefaultCueText,
			Path:       hp.DefaultPath,
			StatusHash: page.StatusHash,
		}
	}

	return &hp.HeldProofCue{
		Available:  true,
		CueText:    hp.DefaultCueText,
		Path:       hp.DefaultPath,
		StatusHash: page.StatusHash,
	}
}

// ============================================================================
// Store Integration
// ============================================================================

// PersistSignal persists a signal to the store.
func (e *Engine) PersistSignal(dayKey string, sig hp.HeldProofSignal) error {
	if e.signalStore == nil {
		return nil
	}
	return e.signalStore.AppendSignal(dayKey, sig)
}

// LoadSignals loads signals for a day from the store.
func (e *Engine) LoadSignals(dayKey string) []hp.HeldProofSignal {
	if e.signalStore == nil {
		return nil
	}
	return e.signalStore.ListSignals(dayKey)
}

// RecordViewed records that the proof page was viewed.
func (e *Engine) RecordViewed(dayKey, statusHash string) error {
	if e.ackStore == nil {
		return nil
	}
	return e.ackStore.RecordViewed(dayKey, statusHash)
}

// RecordDismissed records that the proof page was dismissed.
func (e *Engine) RecordDismissed(dayKey, statusHash string) error {
	if e.ackStore == nil {
		return nil
	}
	return e.ackStore.RecordDismissed(dayKey, statusHash)
}

// IsDismissed checks if the proof page was dismissed.
func (e *Engine) IsDismissed(dayKey, statusHash string) bool {
	if e.ackStore == nil {
		return false
	}
	return e.ackStore.IsDismissed(dayKey, statusHash)
}

// HasViewed checks if the proof page was viewed.
func (e *Engine) HasViewed(dayKey, statusHash string) bool {
	if e.ackStore == nil {
		return false
	}
	return e.ackStore.HasViewed(dayKey, statusHash)
}

// ============================================================================
// High-Level Operations
// ============================================================================

// GetCurrentDayKey returns the current day key from the clock.
func (e *Engine) GetCurrentDayKey() string {
	return e.clk.Now().UTC().Format("2006-01-02")
}

// BuildPageForDay builds the proof page for the current day.
func (e *Engine) BuildPageForDay() (*hp.HeldProofPage, string, error) {
	dayKey := e.GetCurrentDayKey()
	signals := e.LoadSignals(dayKey)

	period := hp.HeldProofPeriod{DayKey: dayKey}
	return e.BuildPage(signals, period)
}

// BuildCueForDay builds the cue for the current day.
func (e *Engine) BuildCueForDay() *hp.HeldProofCue {
	dayKey := e.GetCurrentDayKey()
	signals := e.LoadSignals(dayKey)

	period := hp.HeldProofPeriod{DayKey: dayKey}
	page, statusHash, err := e.BuildPage(signals, period)
	if err != nil || page == nil {
		return nil
	}

	dismissed := e.IsDismissed(dayKey, statusHash)
	viewed := e.HasViewed(dayKey, statusHash)

	return e.BuildCue(page, dismissed, viewed)
}

// ============================================================================
// Phase 42 Integration
// ============================================================================

// Phase42QueueProofOutcome represents a QUEUE_PROOF outcome from Phase 42.
// CRITICAL: Contains only abstract data. No raw identifiers.
type Phase42QueueProofOutcome struct {
	// CircleType is the abstract circle type (human/institution only; commerce is rejected).
	CircleType hp.HeldProofCircleType

	// Horizon is the abstract time horizon bucket.
	Horizon hp.HeldProofHorizonBucket

	// Magnitude is the abstract magnitude bucket.
	Magnitude hp.HeldProofMagnitudeBucket

	// SourceHash is the sha256 hex of the Phase 42 decision record.
	SourceHash string
}

// HandleQueueProofOutcome processes a Phase 42 QUEUE_PROOF outcome.
// Returns the signal that was created, or nil if rejected (e.g., commerce).
// CRITICAL: Rejects commerce.
func (e *Engine) HandleQueueProofOutcome(dayKey string, outcome Phase42QueueProofOutcome) *hp.HeldProofSignal {
	// Reject commerce
	if outcome.CircleType.IsCommerce() {
		return nil
	}

	// Build decision input
	input := HeldProofDecisionInput{
		CircleType:  outcome.CircleType,
		Horizon:     outcome.Horizon,
		Magnitude:   outcome.Magnitude,
		QueuedProof: true,
		SourceHash:  outcome.SourceHash,
	}

	// Build signal
	sig, err := e.BuildSignalFromDecision(dayKey, input)
	if err != nil || sig == nil {
		return nil
	}

	// Persist signal
	if err := e.PersistSignal(dayKey, *sig); err != nil {
		return nil
	}

	return sig
}
