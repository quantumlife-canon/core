// Package urgencydelivery defines the Phase 54 Urgency → Delivery Binding domain types.
//
// This package provides explicit, deterministic binding between the urgency
// resolution layer (Phase 53) and the interrupt delivery pipeline (Phases 32-36).
//
// CRITICAL INVARIANTS:
//   - NO BACKGROUND EXECUTION: No goroutines, no timers, no polling.
//   - POST-TRIGGERED ONLY: Any delivery attempt happens only via explicit POST.
//   - NO NEW DECISION LOGIC: Reuses existing pipelines (31.4, 32, 33, 34, 36, 53).
//   - COMMERCE EXCLUDED: Never escalated, never delivered, never overridden.
//   - ABSTRACT PAYLOAD ONLY: Two-line payload, no identifiers.
//   - DETERMINISTIC: Same inputs + same injected clock → same output hash/ID.
//   - HASH-ONLY STORAGE: Never stores raw identifiers.
//   - No time.Now() in pkg/ or internal/. Use injected clock.
//   - BOUNDED RETENTION: 30 days and max 200 records.
//
// Reference: docs/ADR/ADR-0092-phase54-urgency-delivery-binding.md
package urgencydelivery

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// ═══════════════════════════════════════════════════════════════════════════
// Binding Run Kind
// ═══════════════════════════════════════════════════════════════════════════

// BindingRunKind identifies how the binding run was triggered.
type BindingRunKind string

const (
	// RunManual means explicit POST triggered the run.
	RunManual BindingRunKind = "run_manual"
)

// Validate checks if the BindingRunKind is valid.
func (k BindingRunKind) Validate() error {
	switch k {
	case RunManual:
		return nil
	default:
		return fmt.Errorf("invalid BindingRunKind: %s", k)
	}
}

// CanonicalString returns the canonical string for hashing.
func (k BindingRunKind) CanonicalString() string {
	return string(k)
}

// ═══════════════════════════════════════════════════════════════════════════
// Binding Outcome Kind
// ═══════════════════════════════════════════════════════════════════════════

// BindingOutcomeKind represents the outcome of a binding run.
type BindingOutcomeKind string

const (
	// OutcomeDelivered means delivery was successfully attempted.
	OutcomeDelivered BindingOutcomeKind = "outcome_delivered"
	// OutcomeNotDelivered means no delivery occurred.
	OutcomeNotDelivered BindingOutcomeKind = "outcome_not_delivered"
)

// Validate checks if the BindingOutcomeKind is valid.
func (k BindingOutcomeKind) Validate() error {
	switch k {
	case OutcomeDelivered, OutcomeNotDelivered:
		return nil
	default:
		return fmt.Errorf("invalid BindingOutcomeKind: %s", k)
	}
}

// CanonicalString returns the canonical string for hashing.
func (k BindingOutcomeKind) CanonicalString() string {
	return string(k)
}

// ═══════════════════════════════════════════════════════════════════════════
// Binding Rejection Reason
// ═══════════════════════════════════════════════════════════════════════════

// BindingRejectionReason explains why delivery was not attempted.
// CRITICAL: Must be bucketed/abstract; no identifiers.
type BindingRejectionReason string

const (
	// RejectNone means no rejection (delivery occurred).
	RejectNone BindingRejectionReason = ""
	// RejectNoCandidate means no candidate available for delivery.
	RejectNoCandidate BindingRejectionReason = "reject_no_candidate"
	// RejectCommerceExcluded means commerce circles are never delivered.
	RejectCommerceExcluded BindingRejectionReason = "reject_commerce_excluded"
	// RejectPolicyDisallows means policy does not permit delivery.
	RejectPolicyDisallows BindingRejectionReason = "reject_policy_disallows"
	// RejectNotPermittedByUrgency means urgency level too low.
	RejectNotPermittedByUrgency BindingRejectionReason = "reject_not_permitted_by_urgency"
	// RejectRateLimited means daily cap has been reached.
	RejectRateLimited BindingRejectionReason = "reject_rate_limited"
	// RejectNoDevice means no device is registered.
	RejectNoDevice BindingRejectionReason = "reject_no_device"
	// RejectTransportUnavailable means transport is not available.
	RejectTransportUnavailable BindingRejectionReason = "reject_transport_unavailable"
	// RejectSealedKeyMissing means APNs sealed key is not available.
	RejectSealedKeyMissing BindingRejectionReason = "reject_sealed_key_missing"
	// RejectEnforcementClamped means enforcement layer clamped the request.
	RejectEnforcementClamped BindingRejectionReason = "reject_enforcement_clamped"
	// RejectInternalError means an internal error occurred (details not leaked).
	RejectInternalError BindingRejectionReason = "reject_internal_error"
)

// Validate checks if the BindingRejectionReason is valid.
func (r BindingRejectionReason) Validate() error {
	switch r {
	case RejectNone, RejectNoCandidate, RejectCommerceExcluded, RejectPolicyDisallows,
		RejectNotPermittedByUrgency, RejectRateLimited, RejectNoDevice,
		RejectTransportUnavailable, RejectSealedKeyMissing, RejectEnforcementClamped,
		RejectInternalError:
		return nil
	default:
		return fmt.Errorf("invalid BindingRejectionReason: %s", r)
	}
}

// CanonicalString returns the canonical string for hashing.
func (r BindingRejectionReason) CanonicalString() string {
	if r == RejectNone {
		return "none"
	}
	return string(r)
}

// DisplayLabel returns a human-friendly label.
func (r BindingRejectionReason) DisplayLabel() string {
	switch r {
	case RejectNone:
		return ""
	case RejectNoCandidate:
		return "Nothing to deliver"
	case RejectCommerceExcluded:
		return "Commerce is excluded"
	case RejectPolicyDisallows:
		return "Policy does not permit"
	case RejectNotPermittedByUrgency:
		return "Not urgent enough"
	case RejectRateLimited:
		return "Daily limit reached"
	case RejectNoDevice:
		return "No device registered"
	case RejectTransportUnavailable:
		return "Transport unavailable"
	case RejectSealedKeyMissing:
		return "Configuration incomplete"
	case RejectEnforcementClamped:
		return "Enforcement active"
	case RejectInternalError:
		return "System issue"
	default:
		return "Unknown"
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Urgency Bucket
// ═══════════════════════════════════════════════════════════════════════════

// UrgencyBucket represents the urgency level bucket.
// Maps from Phase 53 UrgencyLevel.
type UrgencyBucket string

const (
	// UrgencyNone means no urgency.
	UrgencyNone UrgencyBucket = "urgency_none"
	// UrgencyLow means low urgency.
	UrgencyLow UrgencyBucket = "urgency_low"
	// UrgencyMedium means medium urgency.
	UrgencyMedium UrgencyBucket = "urgency_medium"
	// UrgencyHigh means high urgency.
	UrgencyHigh UrgencyBucket = "urgency_high"
)

// Validate checks if the UrgencyBucket is valid.
func (u UrgencyBucket) Validate() error {
	switch u {
	case UrgencyNone, UrgencyLow, UrgencyMedium, UrgencyHigh:
		return nil
	default:
		return fmt.Errorf("invalid UrgencyBucket: %s", u)
	}
}

// CanonicalString returns the canonical string for hashing.
func (u UrgencyBucket) CanonicalString() string {
	return string(u)
}

// Order returns the numeric order (0=none, 1=low, 2=medium, 3=high).
func (u UrgencyBucket) Order() int {
	switch u {
	case UrgencyNone:
		return 0
	case UrgencyLow:
		return 1
	case UrgencyMedium:
		return 2
	case UrgencyHigh:
		return 3
	default:
		return 0
	}
}

// AllowsDelivery returns true if this urgency level permits delivery attempt.
// ONLY urgency_medium or urgency_high can attempt delivery.
func (u UrgencyBucket) AllowsDelivery() bool {
	return u == UrgencyMedium || u == UrgencyHigh
}

// ═══════════════════════════════════════════════════════════════════════════
// Delivery Intent Kind
// ═══════════════════════════════════════════════════════════════════════════

// DeliveryIntentKind represents the intent for delivery.
type DeliveryIntentKind string

const (
	// IntentHold means hold, do not deliver.
	IntentHold DeliveryIntentKind = "intent_hold"
	// IntentSurfaceOnly means surface only (no push).
	IntentSurfaceOnly DeliveryIntentKind = "intent_surface_only"
	// IntentInterruptCandidate means interrupt candidate (may deliver).
	IntentInterruptCandidate DeliveryIntentKind = "intent_interrupt_candidate"
	// IntentDeliver means attempt delivery.
	IntentDeliver DeliveryIntentKind = "intent_deliver"
)

// Validate checks if the DeliveryIntentKind is valid.
func (i DeliveryIntentKind) Validate() error {
	switch i {
	case IntentHold, IntentSurfaceOnly, IntentInterruptCandidate, IntentDeliver:
		return nil
	default:
		return fmt.Errorf("invalid DeliveryIntentKind: %s", i)
	}
}

// CanonicalString returns the canonical string for hashing.
func (i DeliveryIntentKind) CanonicalString() string {
	return string(i)
}

// ═══════════════════════════════════════════════════════════════════════════
// Circle Type Bucket
// ═══════════════════════════════════════════════════════════════════════════

// CircleTypeBucket represents the type of circle.
type CircleTypeBucket string

const (
	// CircleTypeHuman means a human circle.
	CircleTypeHuman CircleTypeBucket = "bucket_human"
	// CircleTypeInstitution means an institution circle.
	CircleTypeInstitution CircleTypeBucket = "bucket_institution"
	// CircleTypeCommerce means a commerce circle.
	CircleTypeCommerce CircleTypeBucket = "bucket_commerce"
	// CircleTypeUnknown means unknown circle type.
	CircleTypeUnknown CircleTypeBucket = "bucket_unknown"
)

// Validate checks if the CircleTypeBucket is valid.
func (c CircleTypeBucket) Validate() error {
	switch c {
	case CircleTypeHuman, CircleTypeInstitution, CircleTypeCommerce, CircleTypeUnknown:
		return nil
	default:
		return fmt.Errorf("invalid CircleTypeBucket: %s", c)
	}
}

// CanonicalString returns the canonical string for hashing.
func (c CircleTypeBucket) CanonicalString() string {
	return string(c)
}

// IsCommerce returns true if this is a commerce circle.
func (c CircleTypeBucket) IsCommerce() bool {
	return c == CircleTypeCommerce
}

// ═══════════════════════════════════════════════════════════════════════════
// Horizon Bucket
// ═══════════════════════════════════════════════════════════════════════════

// HorizonBucket represents time horizon.
type HorizonBucket string

const (
	// HorizonNow means immediate.
	HorizonNow HorizonBucket = "horizon_now"
	// HorizonSoon means soon.
	HorizonSoon HorizonBucket = "horizon_soon"
	// HorizonLater means later.
	HorizonLater HorizonBucket = "horizon_later"
	// HorizonNone means no horizon.
	HorizonNone HorizonBucket = "horizon_none"
)

// Validate checks if the HorizonBucket is valid.
func (h HorizonBucket) Validate() error {
	switch h {
	case HorizonNow, HorizonSoon, HorizonLater, HorizonNone:
		return nil
	default:
		return fmt.Errorf("invalid HorizonBucket: %s", h)
	}
}

// CanonicalString returns the canonical string for hashing.
func (h HorizonBucket) CanonicalString() string {
	return string(h)
}

// ═══════════════════════════════════════════════════════════════════════════
// Magnitude Bucket
// ═══════════════════════════════════════════════════════════════════════════

// MagnitudeBucket represents coarse count bucket.
type MagnitudeBucket string

const (
	// MagnitudeNothing means 0 items.
	MagnitudeNothing MagnitudeBucket = "mag_nothing"
	// MagnitudeAFew means 1-3 items.
	MagnitudeAFew MagnitudeBucket = "mag_a_few"
	// MagnitudeSeveral means 4+ items.
	MagnitudeSeveral MagnitudeBucket = "mag_several"
)

// Validate checks if the MagnitudeBucket is valid.
func (m MagnitudeBucket) Validate() error {
	switch m {
	case MagnitudeNothing, MagnitudeAFew, MagnitudeSeveral:
		return nil
	default:
		return fmt.Errorf("invalid MagnitudeBucket: %s", m)
	}
}

// CanonicalString returns the canonical string for hashing.
func (m MagnitudeBucket) CanonicalString() string {
	return string(m)
}

// ═══════════════════════════════════════════════════════════════════════════
// Policy Allowance Bucket
// ═══════════════════════════════════════════════════════════════════════════

// PolicyAllowanceBucket represents policy allowance from Phase 33.
type PolicyAllowanceBucket string

const (
	// PolicyAllowed means policy permits delivery.
	PolicyAllowed PolicyAllowanceBucket = "policy_allowed"
	// PolicyDenied means policy denies delivery.
	PolicyDenied PolicyAllowanceBucket = "policy_denied"
)

// Validate checks if the PolicyAllowanceBucket is valid.
func (p PolicyAllowanceBucket) Validate() error {
	switch p {
	case PolicyAllowed, PolicyDenied:
		return nil
	default:
		return fmt.Errorf("invalid PolicyAllowanceBucket: %s", p)
	}
}

// CanonicalString returns the canonical string for hashing.
func (p PolicyAllowanceBucket) CanonicalString() string {
	return string(p)
}

// ═══════════════════════════════════════════════════════════════════════════
// Envelope Activity Bucket
// ═══════════════════════════════════════════════════════════════════════════

// EnvelopeActivityBucket represents envelope activity state.
type EnvelopeActivityBucket string

const (
	// EnvelopeNone means no active envelope.
	EnvelopeNone EnvelopeActivityBucket = "envelope_none"
	// EnvelopeActive means an envelope is active.
	EnvelopeActive EnvelopeActivityBucket = "envelope_active"
)

// Validate checks if the EnvelopeActivityBucket is valid.
func (e EnvelopeActivityBucket) Validate() error {
	switch e {
	case EnvelopeNone, EnvelopeActive:
		return nil
	default:
		return fmt.Errorf("invalid EnvelopeActivityBucket: %s", e)
	}
}

// CanonicalString returns the canonical string for hashing.
func (e EnvelopeActivityBucket) CanonicalString() string {
	return string(e)
}

// ═══════════════════════════════════════════════════════════════════════════
// Enforcement Clamp Bucket
// ═══════════════════════════════════════════════════════════════════════════

// EnforcementClampBucket represents enforcement clamp state from Phase 44.2.
type EnforcementClampBucket string

const (
	// EnforcementNotClamped means not clamped by enforcement.
	EnforcementNotClamped EnforcementClampBucket = "enforcement_not_clamped"
	// EnforcementClamped means clamped by enforcement.
	EnforcementClamped EnforcementClampBucket = "enforcement_clamped"
)

// Validate checks if the EnforcementClampBucket is valid.
func (e EnforcementClampBucket) Validate() error {
	switch e {
	case EnforcementNotClamped, EnforcementClamped:
		return nil
	default:
		return fmt.Errorf("invalid EnforcementClampBucket: %s", e)
	}
}

// CanonicalString returns the canonical string for hashing.
func (e EnforcementClampBucket) CanonicalString() string {
	return string(e)
}

// ═══════════════════════════════════════════════════════════════════════════
// Binding Inputs
// ═══════════════════════════════════════════════════════════════════════════

// BindingInputs contains all inputs for the binding engine.
// CRITICAL: All references are hashed. No raw identifiers.
type BindingInputs struct {
	// CircleIDHash is the hashed circle ID.
	CircleIDHash string
	// PeriodKey is the server-derived period key (YYYY-MM-DD).
	PeriodKey string
	// HasDevice indicates if a device is registered.
	HasDevice bool
	// TransportAvailable indicates if transport is available.
	TransportAvailable bool
	// SealedKeyAvailable indicates if APNs sealed key is available.
	SealedKeyAvailable bool
	// UrgencyBucket is the urgency level from Phase 53.
	UrgencyBucket UrgencyBucket
	// CandidateHash is the candidate hash (empty if none).
	CandidateHash string
	// CandidateCircleTypeBucket is the candidate's circle type.
	CandidateCircleTypeBucket CircleTypeBucket
	// CandidateHorizonBucket is the candidate's horizon.
	CandidateHorizonBucket HorizonBucket
	// CandidateMagnitudeBucket is the candidate's magnitude.
	CandidateMagnitudeBucket MagnitudeBucket
	// PolicyAllowanceBucket is the policy allowance from Phase 33.
	PolicyAllowanceBucket PolicyAllowanceBucket
	// EnvelopeActivityBucket is the envelope activity state.
	EnvelopeActivityBucket EnvelopeActivityBucket
	// EnforcementClampBucket is the enforcement clamp state from Phase 44.2.
	EnforcementClampBucket EnforcementClampBucket
	// DeliveredTodayCount is the number delivered today (for rate limiting).
	DeliveredTodayCount int
}

// Validate checks if the BindingInputs are valid.
func (i BindingInputs) Validate() error {
	if i.CircleIDHash == "" {
		return errors.New("CircleIDHash is required")
	}
	if i.PeriodKey == "" {
		return errors.New("PeriodKey is required")
	}
	if strings.Contains(i.PeriodKey, "|") {
		return errors.New("PeriodKey cannot contain pipe delimiter")
	}
	if err := i.UrgencyBucket.Validate(); err != nil {
		return err
	}
	if err := i.CandidateCircleTypeBucket.Validate(); err != nil {
		return err
	}
	if err := i.CandidateHorizonBucket.Validate(); err != nil {
		return err
	}
	if err := i.CandidateMagnitudeBucket.Validate(); err != nil {
		return err
	}
	if err := i.PolicyAllowanceBucket.Validate(); err != nil {
		return err
	}
	if err := i.EnvelopeActivityBucket.Validate(); err != nil {
		return err
	}
	if err := i.EnforcementClampBucket.Validate(); err != nil {
		return err
	}
	// Check for forbidden patterns
	if containsForbiddenPattern(i.CircleIDHash) {
		return errors.New("CircleIDHash contains forbidden pattern")
	}
	if containsForbiddenPattern(i.CandidateHash) {
		return errors.New("CandidateHash contains forbidden pattern")
	}
	return nil
}

// CanonicalString returns the pipe-delimited canonical string for hashing.
func (i BindingInputs) CanonicalString() string {
	parts := []string{
		"v1",
		"circle=" + i.CircleIDHash,
		"period=" + i.PeriodKey,
		"has_device=" + boolToStr(i.HasDevice),
		"transport=" + boolToStr(i.TransportAvailable),
		"sealed_key=" + boolToStr(i.SealedKeyAvailable),
		"urgency=" + i.UrgencyBucket.CanonicalString(),
		"candidate=" + i.CandidateHash,
		"circle_type=" + i.CandidateCircleTypeBucket.CanonicalString(),
		"horizon=" + i.CandidateHorizonBucket.CanonicalString(),
		"magnitude=" + i.CandidateMagnitudeBucket.CanonicalString(),
		"policy=" + i.PolicyAllowanceBucket.CanonicalString(),
		"envelope=" + i.EnvelopeActivityBucket.CanonicalString(),
		"enforcement=" + i.EnforcementClampBucket.CanonicalString(),
		"delivered_today=" + fmt.Sprintf("%d", i.DeliveredTodayCount),
	}
	return strings.Join(parts, "|")
}

// Hash computes the SHA256 hash of the canonical string.
func (i BindingInputs) Hash() string {
	h := sha256.Sum256([]byte(i.CanonicalString()))
	return hex.EncodeToString(h[:])
}

// ═══════════════════════════════════════════════════════════════════════════
// Binding Decision
// ═══════════════════════════════════════════════════════════════════════════

// BindingDecision is the result of the binding engine.
type BindingDecision struct {
	// Intent is the delivery intent.
	Intent DeliveryIntentKind
	// ShouldAttemptDelivery indicates if delivery should be attempted.
	ShouldAttemptDelivery bool
	// RejectionReason explains why delivery was rejected (empty if delivered).
	RejectionReason BindingRejectionReason
	// DeterministicDecisionHash is a deterministic hash of the decision.
	DeterministicDecisionHash string
}

// Validate checks if the BindingDecision is valid.
func (d BindingDecision) Validate() error {
	if err := d.Intent.Validate(); err != nil {
		return err
	}
	if err := d.RejectionReason.Validate(); err != nil {
		return err
	}
	// If should attempt delivery, reason must be empty
	if d.ShouldAttemptDelivery && d.RejectionReason != RejectNone {
		return errors.New("RejectionReason must be empty when ShouldAttemptDelivery is true")
	}
	// If not attempting delivery, reason must be set
	if !d.ShouldAttemptDelivery && d.RejectionReason == RejectNone {
		return errors.New("RejectionReason is required when ShouldAttemptDelivery is false")
	}
	return nil
}

// CanonicalString returns the pipe-delimited canonical string for hashing.
func (d BindingDecision) CanonicalString() string {
	parts := []string{
		"decision",
		"intent=" + d.Intent.CanonicalString(),
		"attempt=" + boolToStr(d.ShouldAttemptDelivery),
		"rejection=" + d.RejectionReason.CanonicalString(),
	}
	return strings.Join(parts, "|")
}

// ComputeHash computes the deterministic decision hash.
func (d BindingDecision) ComputeHash() string {
	h := sha256.Sum256([]byte(d.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// ═══════════════════════════════════════════════════════════════════════════
// Urgency Delivery Receipt
// ═══════════════════════════════════════════════════════════════════════════

// UrgencyDeliveryReceipt is the proof of a binding run.
// CRITICAL: Hash-only storage. No raw identifiers.
type UrgencyDeliveryReceipt struct {
	// ReceiptHash is the unique hash of this receipt.
	ReceiptHash string
	// CircleIDHash is the hashed circle ID.
	CircleIDHash string
	// PeriodKey is the period key.
	PeriodKey string
	// RunKind is how the run was triggered.
	RunKind BindingRunKind
	// OutcomeKind is the outcome of the run.
	OutcomeKind BindingOutcomeKind
	// UrgencyBucket is the urgency level.
	UrgencyBucket UrgencyBucket
	// CandidateHash is the candidate hash (empty if none).
	CandidateHash string
	// Intent is the delivery intent.
	Intent DeliveryIntentKind
	// RejectionReason explains why delivery was rejected (empty if delivered).
	RejectionReason BindingRejectionReason
	// AttemptIDHash is the hash of the Phase 36 attempt ID (empty if not delivered).
	AttemptIDHash string
	// StatusHash is a deterministic hash for UI cue dedupe.
	StatusHash string
	// CreatedBucket is "this_period" (no timestamp).
	CreatedBucket string
}

// Validate checks if the UrgencyDeliveryReceipt is valid.
func (r UrgencyDeliveryReceipt) Validate() error {
	if r.ReceiptHash == "" {
		return errors.New("ReceiptHash is required")
	}
	if r.CircleIDHash == "" {
		return errors.New("CircleIDHash is required")
	}
	if r.PeriodKey == "" {
		return errors.New("PeriodKey is required")
	}
	if err := r.RunKind.Validate(); err != nil {
		return err
	}
	if err := r.OutcomeKind.Validate(); err != nil {
		return err
	}
	if err := r.UrgencyBucket.Validate(); err != nil {
		return err
	}
	if err := r.Intent.Validate(); err != nil {
		return err
	}
	if err := r.RejectionReason.Validate(); err != nil {
		return err
	}
	// Check for forbidden patterns
	if containsForbiddenPattern(r.CircleIDHash) {
		return errors.New("CircleIDHash contains forbidden pattern")
	}
	if containsForbiddenPattern(r.CandidateHash) {
		return errors.New("CandidateHash contains forbidden pattern")
	}
	return nil
}

// CanonicalString returns the pipe-delimited canonical string for hashing.
func (r UrgencyDeliveryReceipt) CanonicalString() string {
	parts := []string{
		"v1",
		"circle=" + r.CircleIDHash,
		"period=" + r.PeriodKey,
		"run=" + r.RunKind.CanonicalString(),
		"outcome=" + r.OutcomeKind.CanonicalString(),
		"urgency=" + r.UrgencyBucket.CanonicalString(),
		"candidate=" + r.CandidateHash,
		"intent=" + r.Intent.CanonicalString(),
		"rejection=" + r.RejectionReason.CanonicalString(),
		"attempt=" + r.AttemptIDHash,
	}
	return strings.Join(parts, "|")
}

// ComputeReceiptHash computes the deterministic receipt hash.
func (r UrgencyDeliveryReceipt) ComputeReceiptHash() string {
	h := sha256.Sum256([]byte(r.CanonicalString()))
	return hex.EncodeToString(h[:])
}

// ComputeStatusHash computes the deterministic status hash for UI dedupe.
func (r UrgencyDeliveryReceipt) ComputeStatusHash() string {
	content := fmt.Sprintf("status|%s|%s|%s|%s",
		r.CircleIDHash,
		r.PeriodKey,
		r.OutcomeKind.CanonicalString(),
		r.Intent.CanonicalString(),
	)
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:16])
}

// DedupKey returns the deduplication key for this receipt.
// Same circle+candidate+period should not create multiple receipts.
func (r UrgencyDeliveryReceipt) DedupKey() string {
	return fmt.Sprintf("%s|%s|%s", r.CircleIDHash, r.CandidateHash, r.PeriodKey)
}

// ═══════════════════════════════════════════════════════════════════════════
// Receipt Line (for Proof Page)
// ═══════════════════════════════════════════════════════════════════════════

// ReceiptLine is a single line in the proof page.
type ReceiptLine struct {
	// OutcomeKind is the outcome of the run.
	OutcomeKind BindingOutcomeKind
	// UrgencyBucket is the urgency level.
	UrgencyBucket UrgencyBucket
	// Intent is the delivery intent.
	Intent DeliveryIntentKind
	// RejectionReason explains why delivery was rejected (empty if delivered).
	RejectionReason BindingRejectionReason
	// ReceiptHashPrefix is the short prefix of the receipt hash.
	ReceiptHashPrefix string
}

// CanonicalString returns the canonical string for the line.
func (l ReceiptLine) CanonicalString() string {
	return fmt.Sprintf("line|%s|%s|%s|%s|%s",
		l.OutcomeKind.CanonicalString(),
		l.UrgencyBucket.CanonicalString(),
		l.Intent.CanonicalString(),
		l.RejectionReason.CanonicalString(),
		l.ReceiptHashPrefix,
	)
}

// FromReceipt creates a ReceiptLine from a receipt.
func ReceiptLineFromReceipt(r UrgencyDeliveryReceipt) ReceiptLine {
	prefix := r.ReceiptHash
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}
	return ReceiptLine{
		OutcomeKind:       r.OutcomeKind,
		UrgencyBucket:     r.UrgencyBucket,
		Intent:            r.Intent,
		RejectionReason:   r.RejectionReason,
		ReceiptHashPrefix: prefix,
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Proof Page
// ═══════════════════════════════════════════════════════════════════════════

// ProofPage is the proof page for urgency delivery binding.
// CRITICAL: No raw identifiers. Abstract buckets only.
type ProofPage struct {
	// Title is the page title.
	Title string
	// Lines are calm descriptive lines (max 8).
	Lines []string
	// RecentReceipts are recent receipt lines (max 6).
	RecentReceipts []ReceiptLine
	// StatusHash is a deterministic hash for verification.
	StatusHash string
}

// MaxProofPageReceipts is the maximum number of receipts on the proof page.
const MaxProofPageReceipts = 6

// MaxProofPageLines is the maximum number of lines on the proof page.
const MaxProofPageLines = 8

// Validate checks if the ProofPage is valid.
func (p ProofPage) Validate() error {
	if p.Title == "" {
		return errors.New("Title is required")
	}
	if len(p.Lines) > MaxProofPageLines {
		return fmt.Errorf("Lines cannot exceed %d", MaxProofPageLines)
	}
	if len(p.RecentReceipts) > MaxProofPageReceipts {
		return fmt.Errorf("RecentReceipts cannot exceed %d", MaxProofPageReceipts)
	}
	// Check for forbidden patterns in lines
	for _, line := range p.Lines {
		if containsForbiddenPattern(line) {
			return errors.New("Line contains forbidden pattern")
		}
	}
	return nil
}

// CanonicalString returns the canonical string for hashing.
func (p ProofPage) CanonicalString() string {
	receiptHashes := make([]string, len(p.RecentReceipts))
	for i, r := range p.RecentReceipts {
		receiptHashes[i] = r.ReceiptHashPrefix
	}
	parts := []string{
		"page",
		"title=" + p.Title,
		"receipts=" + strings.Join(receiptHashes, ","),
	}
	return strings.Join(parts, "|")
}

// ComputeStatusHash computes the deterministic status hash.
func (p ProofPage) ComputeStatusHash() string {
	h := sha256.Sum256([]byte(p.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// DefaultProofPage returns the default proof page.
func DefaultProofPage() ProofPage {
	p := ProofPage{
		Title: "Delivery binding, quietly.",
		Lines: []string{
			"Delivery is explicit, not automatic.",
			"We only deliver when you ask.",
			"Nothing was delivered this period.",
		},
		RecentReceipts: []ReceiptLine{},
	}
	p.StatusHash = p.ComputeStatusHash()
	return p
}

// ═══════════════════════════════════════════════════════════════════════════
// Constants
// ═══════════════════════════════════════════════════════════════════════════

// MaxDeliveriesPerDay is the absolute cap on deliveries per day.
// CRITICAL: Must align with Phase 33/35/36/41 cap.
const MaxDeliveriesPerDay = 2

// MaxReceiptRecords is the maximum number of receipts to retain.
const MaxReceiptRecords = 200

// MaxRetentionDays is the maximum number of days to retain receipts.
const MaxRetentionDays = 30

// CreatedBucketThisPeriod is the bucket for "this period" creation.
const CreatedBucketThisPeriod = "this_period"

// ═══════════════════════════════════════════════════════════════════════════
// Helpers
// ═══════════════════════════════════════════════════════════════════════════

func boolToStr(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// ForbiddenPatterns are patterns that must never appear in urgency delivery output.
var ForbiddenPatterns = []string{
	"@",            // email indicator
	"http://",      // URL
	"https://",     // URL
	".com",         // domain
	".org",         // domain
	".net",         // domain
	"vendor_id",    // vendor identifier
	"pack_id",      // pack identifier
	"merchant",     // merchant name
	"amount",       // amount value
	"currency",     // currency
	"sender",       // sender identifier
	"subject",      // email subject
	"recipient",    // recipient identifier
	"device_token", // device token
}

// containsForbiddenPattern checks if a string contains any forbidden pattern.
func containsForbiddenPattern(s string) bool {
	lower := strings.ToLower(s)
	for _, pattern := range ForbiddenPatterns {
		if strings.Contains(lower, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

// ContainsForbiddenPattern is the exported version.
func ContainsForbiddenPattern(s string) bool {
	return containsForbiddenPattern(s)
}
