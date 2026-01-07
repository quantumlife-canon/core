// Package pressuredecision provides domain types for Phase 32: Pressure Decision Gate.
//
// The decision gate classifies external pressure into one of three states:
// - HOLD: Pressure is acknowledged but not surfaced (default)
// - SURFACE: Pressure may appear in calm mirror views
// - INTERRUPT_CANDIDATE: Pressure is eligible to compete for interruption
//
// CRITICAL INVARIANTS:
//   - Classification only. NO notifications. NO execution. NO UI buttons.
//   - NO LLM authority. Deterministic rules only.
//   - Same inputs + same clock => same output.
//   - NO raw identifiers, NO person identifiers, NO timestamps, NO raw urgency text.
//   - Max 2 INTERRUPT_CANDIDATEs per period (day).
//   - HOLD is the default. INTERRUPT_CANDIDATE must be rare.
//   - No goroutines. Clock injection required.
//
// Reference: docs/ADR/ADR-0068-phase32-pressure-decision-gate.md
package pressuredecision

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// PressureDecisionKind represents the classification outcome.
type PressureDecisionKind string

const (
	// DecisionHold means pressure is acknowledged but not surfaced.
	// This is the DEFAULT. Silence is success.
	DecisionHold PressureDecisionKind = "hold"

	// DecisionSurface means pressure may appear in calm mirror views.
	// Does not interrupt. User sees it only if they look.
	DecisionSurface PressureDecisionKind = "surface"

	// DecisionInterruptCandidate means pressure may compete for interruption.
	// Rate-limited to max 2 per day. Requires future phase to actually interrupt.
	DecisionInterruptCandidate PressureDecisionKind = "interrupt_candidate"
)

// AllDecisionKinds returns all decision kinds in deterministic order.
func AllDecisionKinds() []PressureDecisionKind {
	return []PressureDecisionKind{
		DecisionHold,
		DecisionSurface,
		DecisionInterruptCandidate,
	}
}

// Validate checks if the decision kind is valid.
func (k PressureDecisionKind) Validate() error {
	switch k {
	case DecisionHold, DecisionSurface, DecisionInterruptCandidate:
		return nil
	default:
		return fmt.Errorf("invalid pressure decision kind: %s", k)
	}
}

// Priority returns the priority level (higher = more urgent).
// Used for comparison and downgrade logic.
func (k PressureDecisionKind) Priority() int {
	switch k {
	case DecisionInterruptCandidate:
		return 2
	case DecisionSurface:
		return 1
	case DecisionHold:
		return 0
	default:
		return -1
	}
}

// ReasonBucket provides an abstract explanation for the decision.
// CRITICAL: These are enums only. No free-text reasons allowed.
type ReasonBucket string

const (
	// ReasonDefault indicates the default HOLD decision was applied.
	ReasonDefault ReasonBucket = "default"

	// ReasonCommerceNeverInterrupts indicates commerce pressure was held.
	ReasonCommerceNeverInterrupts ReasonBucket = "commerce_never_interrupts"

	// ReasonHumanNow indicates human circle with immediate pressure.
	ReasonHumanNow ReasonBucket = "human_now"

	// ReasonInstitutionDeadline indicates institutional deadline pressure.
	ReasonInstitutionDeadline ReasonBucket = "institution_deadline"

	// ReasonTrustFragileDowngrade indicates decision was capped due to fragile trust.
	ReasonTrustFragileDowngrade ReasonBucket = "trust_fragile_downgrade"

	// ReasonRateLimitDowngrade indicates decision was downgraded due to rate limit.
	ReasonRateLimitDowngrade ReasonBucket = "rate_limit_downgrade"

	// ReasonNoMagnitude indicates no meaningful pressure existed.
	ReasonNoMagnitude ReasonBucket = "no_magnitude"

	// ReasonHorizonLater indicates pressure horizon was too far out.
	ReasonHorizonLater ReasonBucket = "horizon_later"
)

// AllReasonBuckets returns all reason buckets in deterministic order.
func AllReasonBuckets() []ReasonBucket {
	return []ReasonBucket{
		ReasonDefault,
		ReasonCommerceNeverInterrupts,
		ReasonHumanNow,
		ReasonInstitutionDeadline,
		ReasonTrustFragileDowngrade,
		ReasonRateLimitDowngrade,
		ReasonNoMagnitude,
		ReasonHorizonLater,
	}
}

// Validate checks if the reason bucket is valid.
func (r ReasonBucket) Validate() error {
	switch r {
	case ReasonDefault, ReasonCommerceNeverInterrupts, ReasonHumanNow,
		ReasonInstitutionDeadline, ReasonTrustFragileDowngrade,
		ReasonRateLimitDowngrade, ReasonNoMagnitude, ReasonHorizonLater:
		return nil
	default:
		return fmt.Errorf("invalid reason bucket: %s", r)
	}
}

// CircleType represents the type of circle for decision rules.
// CRITICAL: Maps to CircleKind from externalpressure but adds semantic meaning.
type CircleType string

const (
	// CircleTypeHuman represents a human-owned sovereign circle.
	// Examples: me, family, spouse, friend.
	CircleTypeHuman CircleType = "human"

	// CircleTypeInstitution represents an institutional sovereign circle.
	// Examples: HMRC, bank, employer, NHS.
	CircleTypeInstitution CircleType = "institution"

	// CircleTypeCommerce represents commerce-derived external circles.
	// Examples: delivery patterns, transport habits, retail frequency.
	// CRITICAL: Commerce NEVER interrupts alone.
	CircleTypeCommerce CircleType = "commerce"
)

// AllCircleTypes returns all circle types in deterministic order.
func AllCircleTypes() []CircleType {
	return []CircleType{
		CircleTypeHuman,
		CircleTypeInstitution,
		CircleTypeCommerce,
	}
}

// Validate checks if the circle type is valid.
func (t CircleType) Validate() error {
	switch t {
	case CircleTypeHuman, CircleTypeInstitution, CircleTypeCommerce:
		return nil
	default:
		return fmt.Errorf("invalid circle type: %s", t)
	}
}

// CanInterrupt returns whether this circle type can produce interrupt candidates.
func (t CircleType) CanInterrupt() bool {
	switch t {
	case CircleTypeHuman:
		return true
	case CircleTypeInstitution:
		return false // Institutions surface only, don't interrupt
	case CircleTypeCommerce:
		return false // Commerce NEVER interrupts
	default:
		return false
	}
}

// TrustBaselineStatus represents the trust state from Phase 20.
type TrustBaselineStatus string

const (
	// TrustStatusNormal indicates normal trust baseline.
	TrustStatusNormal TrustBaselineStatus = "normal"

	// TrustStatusFragile indicates fragile trust baseline.
	// When fragile, max decision is SURFACE (no interrupt candidates).
	TrustStatusFragile TrustBaselineStatus = "fragile"

	// TrustStatusUnknown indicates trust baseline is not available.
	// Treated as normal for decision purposes.
	TrustStatusUnknown TrustBaselineStatus = "unknown"
)

// Validate checks if the trust status is valid.
func (s TrustBaselineStatus) Validate() error {
	switch s {
	case TrustStatusNormal, TrustStatusFragile, TrustStatusUnknown:
		return nil
	default:
		return fmt.Errorf("invalid trust baseline status: %s", s)
	}
}

// MaxInterruptCandidatesPerDay is the rate limit for interrupt candidates.
const MaxInterruptCandidatesPerDay = 2

// PressureDecisionInput contains all inputs for decision computation.
// CRITICAL: Contains only abstract buckets and hashes. No raw data.
type PressureDecisionInput struct {
	// CircleIDHash identifies the circle (hash only).
	CircleIDHash string

	// CircleType classifies the circle for rule application.
	CircleType CircleType

	// Magnitude is the abstract pressure magnitude.
	Magnitude PressureMagnitude

	// Horizon is the abstract time horizon.
	Horizon PressureHorizon

	// TrustStatus is the current trust baseline (if available).
	TrustStatus TrustBaselineStatus

	// InterruptCandidatesToday is the count of interrupt candidates already today.
	InterruptCandidatesToday int

	// PeriodKey is the day bucket (format: "YYYY-MM-DD").
	PeriodKey string
}

// CanonicalString returns the pipe-delimited, version-prefixed canonical form.
func (i *PressureDecisionInput) CanonicalString() string {
	return fmt.Sprintf("DECISION_INPUT|v1|%s|%s|%s|%s|%s|%d|%s",
		i.CircleIDHash,
		i.CircleType,
		i.Magnitude,
		i.Horizon,
		i.TrustStatus,
		i.InterruptCandidatesToday,
		i.PeriodKey,
	)
}

// ComputeHash computes a deterministic hash of the input.
func (i *PressureDecisionInput) ComputeHash() string {
	h := sha256.Sum256([]byte(i.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// Validate checks if the input is valid.
func (i *PressureDecisionInput) Validate() error {
	if i.CircleIDHash == "" {
		return fmt.Errorf("missing circle_id_hash")
	}
	if err := i.CircleType.Validate(); err != nil {
		return err
	}
	if err := i.Magnitude.Validate(); err != nil {
		return err
	}
	if err := i.Horizon.Validate(); err != nil {
		return err
	}
	if err := i.TrustStatus.Validate(); err != nil {
		return err
	}
	if i.InterruptCandidatesToday < 0 {
		return fmt.Errorf("invalid interrupt_candidates_today: %d", i.InterruptCandidatesToday)
	}
	if i.PeriodKey == "" {
		return fmt.Errorf("missing period_key")
	}
	return nil
}

// PressureDecision represents the classification result.
// CRITICAL: Contains only abstract buckets and hashes. No raw data.
type PressureDecision struct {
	// DecisionID is a deterministic hash of the decision.
	DecisionID string

	// CircleIDHash identifies the circle (hash only).
	CircleIDHash string

	// Decision is the classification outcome.
	Decision PressureDecisionKind

	// ReasonBucket explains the decision abstractly.
	ReasonBucket ReasonBucket

	// PeriodKey is the day bucket (format: "YYYY-MM-DD").
	PeriodKey string

	// InputHash is the hash of the input used to compute this decision.
	InputHash string

	// StatusHash is a deterministic hash of the decision state.
	StatusHash string
}

// CanonicalString returns the pipe-delimited, version-prefixed canonical form.
func (d *PressureDecision) CanonicalString() string {
	return fmt.Sprintf("PRESSURE_DECISION|v1|%s|%s|%s|%s|%s",
		d.CircleIDHash,
		d.Decision,
		d.ReasonBucket,
		d.PeriodKey,
		d.InputHash,
	)
}

// ComputeDecisionID computes a deterministic decision ID.
func (d *PressureDecision) ComputeDecisionID() string {
	h := sha256.Sum256([]byte(d.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// ComputeStatusHash computes a deterministic status hash.
func (d *PressureDecision) ComputeStatusHash() string {
	content := fmt.Sprintf("STATUS|v1|%s|%s|%s|%s",
		d.CircleIDHash,
		d.Decision,
		d.ReasonBucket,
		d.PeriodKey,
	)
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:16])
}

// Validate checks if the decision is valid.
func (d *PressureDecision) Validate() error {
	if d.DecisionID == "" {
		return fmt.Errorf("missing decision_id")
	}
	if d.CircleIDHash == "" {
		return fmt.Errorf("missing circle_id_hash")
	}
	if err := d.Decision.Validate(); err != nil {
		return err
	}
	if err := d.ReasonBucket.Validate(); err != nil {
		return err
	}
	if d.PeriodKey == "" {
		return fmt.Errorf("missing period_key")
	}
	if d.InputHash == "" {
		return fmt.Errorf("missing input_hash")
	}
	if d.StatusHash == "" {
		return fmt.Errorf("missing status_hash")
	}
	return nil
}

// PressureDecisionRecord is the persistence format for decisions.
// CRITICAL: Hash-only storage. No raw content.
type PressureDecisionRecord struct {
	// RecordID is a unique identifier for the record.
	RecordID string

	// DecisionID is the hash of the decision.
	DecisionID string

	// CircleIDHash identifies the circle.
	CircleIDHash string

	// Decision is the classification outcome.
	Decision PressureDecisionKind

	// ReasonBucket explains the decision.
	ReasonBucket ReasonBucket

	// PeriodKey is the day bucket.
	PeriodKey string

	// InputHash is the hash of the input.
	InputHash string

	// StatusHash is the hash of the decision state.
	StatusHash string
}

// CanonicalString returns the pipe-delimited canonical form.
func (r *PressureDecisionRecord) CanonicalString() string {
	return fmt.Sprintf("DECISION_RECORD|v1|%s|%s|%s|%s|%s|%s|%s",
		r.RecordID,
		r.DecisionID,
		r.CircleIDHash,
		r.Decision,
		r.ReasonBucket,
		r.PeriodKey,
		r.InputHash,
	)
}

// ComputeRecordID computes a deterministic record ID.
func (r *PressureDecisionRecord) ComputeRecordID() string {
	content := fmt.Sprintf("RECORD_ID|v1|%s|%s|%s",
		r.DecisionID,
		r.CircleIDHash,
		r.PeriodKey,
	)
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:16])
}

// FromDecision creates a record from a decision.
func (r *PressureDecisionRecord) FromDecision(d *PressureDecision) {
	r.DecisionID = d.DecisionID
	r.CircleIDHash = d.CircleIDHash
	r.Decision = d.Decision
	r.ReasonBucket = d.ReasonBucket
	r.PeriodKey = d.PeriodKey
	r.InputHash = d.InputHash
	r.StatusHash = d.StatusHash
	r.RecordID = r.ComputeRecordID()
}

// PressureMagnitude represents the abstract pressure magnitude.
// Re-exported from externalpressure for convenience.
type PressureMagnitude string

const (
	// MagnitudeNothing indicates no meaningful pressure.
	MagnitudeNothing PressureMagnitude = "nothing"
	// MagnitudeAFew indicates a small amount of pressure.
	MagnitudeAFew PressureMagnitude = "a_few"
	// MagnitudeSeveral indicates significant pressure.
	MagnitudeSeveral PressureMagnitude = "several"
)

// Validate checks if the magnitude is valid.
func (m PressureMagnitude) Validate() error {
	switch m {
	case MagnitudeNothing, MagnitudeAFew, MagnitudeSeveral:
		return nil
	default:
		return fmt.Errorf("invalid pressure magnitude: %s", m)
	}
}

// PressureHorizon represents the abstract time horizon.
// Adds "now" for immediate pressure detection.
type PressureHorizon string

const (
	// HorizonNow indicates immediate pressure (within hours).
	HorizonNow PressureHorizon = "now"
	// HorizonSoon indicates near-term pressure (within days).
	HorizonSoon PressureHorizon = "soon"
	// HorizonLater indicates distant pressure (weeks+).
	HorizonLater PressureHorizon = "later"
	// HorizonUnknown indicates no horizon information.
	HorizonUnknown PressureHorizon = "unknown"
)

// Validate checks if the horizon is valid.
func (h PressureHorizon) Validate() error {
	switch h {
	case HorizonNow, HorizonSoon, HorizonLater, HorizonUnknown:
		return nil
	default:
		return fmt.Errorf("invalid pressure horizon: %s", h)
	}
}

// DecisionBatch represents a batch of decisions for a period.
type DecisionBatch struct {
	// PeriodKey is the day bucket.
	PeriodKey string

	// Decisions contains all decisions for the period.
	Decisions []*PressureDecision

	// InterruptCandidateCount is the count of interrupt candidates.
	InterruptCandidateCount int

	// SurfaceCount is the count of surface decisions.
	SurfaceCount int

	// HoldCount is the count of hold decisions.
	HoldCount int

	// BatchHash is a deterministic hash of the batch.
	BatchHash string
}

// CanonicalString returns the pipe-delimited canonical form.
func (b *DecisionBatch) CanonicalString() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("DECISION_BATCH|v1|%s|%d|%d|%d",
		b.PeriodKey,
		b.InterruptCandidateCount,
		b.SurfaceCount,
		b.HoldCount,
	))

	for _, d := range b.Decisions {
		sb.WriteString("|")
		sb.WriteString(d.DecisionID)
	}

	return sb.String()
}

// ComputeBatchHash computes a deterministic batch hash.
func (b *DecisionBatch) ComputeBatchHash() string {
	h := sha256.Sum256([]byte(b.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// AddDecision adds a decision to the batch and updates counts.
func (b *DecisionBatch) AddDecision(d *PressureDecision) {
	b.Decisions = append(b.Decisions, d)
	switch d.Decision {
	case DecisionInterruptCandidate:
		b.InterruptCandidateCount++
	case DecisionSurface:
		b.SurfaceCount++
	case DecisionHold:
		b.HoldCount++
	}
	b.BatchHash = b.ComputeBatchHash()
}

// NewDecisionBatch creates a new decision batch for a period.
func NewDecisionBatch(periodKey string) *DecisionBatch {
	return &DecisionBatch{
		PeriodKey: periodKey,
		Decisions: make([]*PressureDecision, 0),
	}
}
