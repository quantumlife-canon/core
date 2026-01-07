// Package interruptpolicy defines the Phase 33 Interrupt Permission Contract.
//
// This package provides policy + proof for interrupt candidates without
// any delivery capability. It answers: "Is an INTERRUPT_CANDIDATE permitted
// to interrupt (in principle)?"
//
// CRITICAL INVARIANTS:
//   - NO interrupt delivery. No alerts. No messages. No external signals.
//   - Policy evaluation only. No side effects.
//   - Deterministic: same inputs => same outputs.
//   - No goroutines. Clock injection required.
//   - Hash-only storage. No raw identifiers.
//   - Default stance: NO interrupts allowed.
//
// Reference: docs/ADR/ADR-0069-phase33-interrupt-permission-contract.md
package interruptpolicy

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

// InterruptAllowance defines what types of interrupts are permitted.
// Default is AllowNone — user must explicitly enable any interrupts.
type InterruptAllowance string

const (
	// AllowNone permits no interrupts (default).
	AllowNone InterruptAllowance = "allow_none"

	// AllowHumansNow permits only human circles with NOW horizon.
	AllowHumansNow InterruptAllowance = "allow_humans_now"

	// AllowInstitutionsSoon permits institution circles with SOON or NOW horizon.
	AllowInstitutionsSoon InterruptAllowance = "allow_institutions_soon"

	// AllowTwoPerDay permits up to 2 interrupts per day from any eligible source.
	// This is a rate cap, not a category allowance.
	AllowTwoPerDay InterruptAllowance = "allow_two_per_day"
)

// ValidAllowances is the set of valid allowance values.
var ValidAllowances = map[InterruptAllowance]bool{
	AllowNone:             true,
	AllowHumansNow:        true,
	AllowInstitutionsSoon: true,
	AllowTwoPerDay:        true,
}

// Validate checks if the allowance is valid.
func (a InterruptAllowance) Validate() error {
	if !ValidAllowances[a] {
		return fmt.Errorf("invalid allowance: %s", a)
	}
	return nil
}

// String returns the string representation.
func (a InterruptAllowance) String() string {
	return string(a)
}

// ReasonBucket explains why a permission decision was made.
type ReasonBucket string

const (
	// ReasonNone indicates no specific reason (default allow).
	ReasonNone ReasonBucket = "reason_none"

	// ReasonPolicyDenies indicates policy explicitly denies.
	ReasonPolicyDenies ReasonBucket = "reason_policy_denies"

	// ReasonRateLimited indicates rate limit exceeded.
	ReasonRateLimited ReasonBucket = "reason_rate_limited"

	// ReasonCategoryBlocked indicates category is always blocked (e.g., commerce).
	ReasonCategoryBlocked ReasonBucket = "reason_category_blocked"

	// ReasonTrustFragile indicates trust status prevents interrupts.
	ReasonTrustFragile ReasonBucket = "reason_trust_fragile"

	// ReasonHorizonMismatch indicates horizon doesn't match policy.
	ReasonHorizonMismatch ReasonBucket = "reason_horizon_mismatch"

	// ReasonCategoryMismatch indicates circle type doesn't match policy.
	ReasonCategoryMismatch ReasonBucket = "reason_category_mismatch"
)

// ValidReasons is the set of valid reason values.
var ValidReasons = map[ReasonBucket]bool{
	ReasonNone:             true,
	ReasonPolicyDenies:     true,
	ReasonRateLimited:      true,
	ReasonCategoryBlocked:  true,
	ReasonTrustFragile:     true,
	ReasonHorizonMismatch:  true,
	ReasonCategoryMismatch: true,
}

// Validate checks if the reason is valid.
func (r ReasonBucket) Validate() error {
	if !ValidReasons[r] {
		return fmt.Errorf("invalid reason: %s", r)
	}
	return nil
}

// String returns the string representation.
func (r ReasonBucket) String() string {
	return string(r)
}

// MagnitudeBucket represents abstract quantities (no raw counts).
type MagnitudeBucket string

const (
	// MagnitudeNothing indicates zero items.
	MagnitudeNothing MagnitudeBucket = "nothing"

	// MagnitudeAFew indicates 1-2 items.
	MagnitudeAFew MagnitudeBucket = "a_few"

	// MagnitudeSeveral indicates 3+ items.
	MagnitudeSeveral MagnitudeBucket = "several"
)

// MagnitudeFromCount converts a raw count to a magnitude bucket.
// CRITICAL: This is the only place raw counts are converted.
func MagnitudeFromCount(count int) MagnitudeBucket {
	switch {
	case count <= 0:
		return MagnitudeNothing
	case count <= 2:
		return MagnitudeAFew
	default:
		return MagnitudeSeveral
	}
}

// MaxInterruptsPerDay is the absolute cap on permitted interrupts.
// Even if policy allows more, this is the hard limit.
const MaxInterruptsPerDay = 2

// InterruptPolicy defines user's interrupt permission settings.
// CRITICAL: Hash-only storage. No raw identifiers.
type InterruptPolicy struct {
	// CircleIDHash is the SHA256 hash of the circle ID.
	CircleIDHash string `json:"circle_id_hash"`

	// PeriodKey is the daily bucket (YYYY-MM-DD format).
	PeriodKey string `json:"period_key"`

	// Allowance defines what types of interrupts are permitted.
	Allowance InterruptAllowance `json:"allowance"`

	// MaxPerDay is the user's configured max (clamped 0..2).
	MaxPerDay int `json:"max_per_day"`

	// CreatedBucket is the time bucket when policy was created.
	// Format: HH:MM floored to 15-minute intervals.
	CreatedBucket string `json:"created_bucket"`

	// PolicyHash is the deterministic hash of this policy.
	PolicyHash string `json:"policy_hash"`
}

// Validate checks if the policy is valid.
func (p *InterruptPolicy) Validate() error {
	if p.CircleIDHash == "" {
		return fmt.Errorf("circle_id_hash required")
	}
	if p.PeriodKey == "" {
		return fmt.Errorf("period_key required")
	}
	if err := p.Allowance.Validate(); err != nil {
		return err
	}
	if p.MaxPerDay < 0 || p.MaxPerDay > MaxInterruptsPerDay {
		return fmt.Errorf("max_per_day must be 0..%d", MaxInterruptsPerDay)
	}
	return nil
}

// ClampMaxPerDay ensures MaxPerDay is within valid range.
func (p *InterruptPolicy) ClampMaxPerDay() {
	if p.MaxPerDay < 0 {
		p.MaxPerDay = 0
	}
	if p.MaxPerDay > MaxInterruptsPerDay {
		p.MaxPerDay = MaxInterruptsPerDay
	}
}

// CanonicalString returns the pipe-delimited canonical representation.
// Format: INTERRUPT_POLICY|v1|circleIDHash|periodKey|allowance|maxPerDay|createdBucket
func (p *InterruptPolicy) CanonicalString() string {
	return fmt.Sprintf("INTERRUPT_POLICY|v1|%s|%s|%s|%d|%s",
		p.CircleIDHash,
		p.PeriodKey,
		p.Allowance,
		p.MaxPerDay,
		p.CreatedBucket,
	)
}

// ComputePolicyHash computes the deterministic hash of this policy.
func (p *InterruptPolicy) ComputePolicyHash() string {
	h := sha256.Sum256([]byte(p.CanonicalString()))
	return fmt.Sprintf("%x", h[:16])
}

// DefaultInterruptPolicy returns the default policy (no interrupts allowed).
func DefaultInterruptPolicy(circleIDHash, periodKey, createdBucket string) *InterruptPolicy {
	p := &InterruptPolicy{
		CircleIDHash:  circleIDHash,
		PeriodKey:     periodKey,
		Allowance:     AllowNone,
		MaxPerDay:     0,
		CreatedBucket: createdBucket,
	}
	p.PolicyHash = p.ComputePolicyHash()
	return p
}

// InterruptCandidate represents an INTERRUPT_CANDIDATE from Phase 32.
// Contains only abstract fields and hashes — no raw identifiers.
type InterruptCandidate struct {
	// CandidateHash is the hash identifying this candidate.
	CandidateHash string `json:"candidate_hash"`

	// CircleType is the type of pressure circle (human, institution, commerce).
	CircleType string `json:"circle_type"`

	// Horizon is the urgency horizon (now, soon, later).
	Horizon string `json:"horizon"`

	// Magnitude is the abstract magnitude bucket.
	Magnitude MagnitudeBucket `json:"magnitude"`
}

// Validate checks if the candidate is valid.
func (c *InterruptCandidate) Validate() error {
	if c.CandidateHash == "" {
		return fmt.Errorf("candidate_hash required")
	}
	if c.CircleType == "" {
		return fmt.Errorf("circle_type required")
	}
	if c.Horizon == "" {
		return fmt.Errorf("horizon required")
	}
	return nil
}

// CanonicalString returns the pipe-delimited canonical representation.
func (c *InterruptCandidate) CanonicalString() string {
	return fmt.Sprintf("INTERRUPT_CANDIDATE|v1|%s|%s|%s|%s",
		c.CandidateHash,
		c.CircleType,
		c.Horizon,
		c.Magnitude,
	)
}

// InterruptPermissionDecision is the result of permission evaluation.
type InterruptPermissionDecision struct {
	// CandidateHash identifies which candidate this decision is for.
	CandidateHash string `json:"candidate_hash"`

	// Allowed indicates whether the interrupt is permitted.
	Allowed bool `json:"allowed"`

	// ReasonBucket explains the decision.
	ReasonBucket ReasonBucket `json:"reason_bucket"`

	// DeterministicHash is the verifiable hash of this decision.
	DeterministicHash string `json:"deterministic_hash"`
}

// CanonicalString returns the pipe-delimited canonical representation.
func (d *InterruptPermissionDecision) CanonicalString() string {
	allowedStr := "denied"
	if d.Allowed {
		allowedStr = "allowed"
	}
	return fmt.Sprintf("INTERRUPT_PERMISSION_DECISION|v1|%s|%s|%s",
		d.CandidateHash,
		allowedStr,
		d.ReasonBucket,
	)
}

// ComputeDeterministicHash computes the hash of this decision.
func (d *InterruptPermissionDecision) ComputeDeterministicHash() string {
	h := sha256.Sum256([]byte(d.CanonicalString()))
	return fmt.Sprintf("%x", h[:16])
}

// InterruptProofPage represents the proof page data.
// CRITICAL: No raw identifiers. Abstract buckets only.
type InterruptProofPage struct {
	// Title is the page title (calm, minimal).
	Title string `json:"title"`

	// Lines are calm copy lines for the page.
	Lines []string `json:"lines"`

	// PermittedMagnitude is the abstract count of permitted interrupts.
	PermittedMagnitude MagnitudeBucket `json:"permitted_magnitude"`

	// DeniedMagnitude is the abstract count of denied interrupts.
	DeniedMagnitude MagnitudeBucket `json:"denied_magnitude"`

	// StatusHash is the deterministic hash of this proof state.
	StatusHash string `json:"status_hash"`

	// DismissPath is the path for dismissing the cue.
	DismissPath string `json:"dismiss_path"`

	// DismissMethod is the HTTP method for dismissing.
	DismissMethod string `json:"dismiss_method"`

	// BackLink is the path to return to.
	BackLink string `json:"back_link"`

	// PolicySummary is a calm description of current policy.
	PolicySummary string `json:"policy_summary"`

	// PeriodKey is the current period for dismissal tracking.
	PeriodKey string `json:"period_key"`

	// CircleIDHash is the circle for dismissal tracking.
	CircleIDHash string `json:"circle_id_hash"`
}

// CanonicalString returns the pipe-delimited canonical representation.
func (p *InterruptProofPage) CanonicalString() string {
	return fmt.Sprintf("INTERRUPT_PROOF_PAGE|v1|%s|%s|%s|%s",
		p.PermittedMagnitude,
		p.DeniedMagnitude,
		p.PolicySummary,
		p.PeriodKey,
	)
}

// ComputeStatusHash computes the status hash for this proof page.
func (p *InterruptProofPage) ComputeStatusHash() string {
	h := sha256.Sum256([]byte(p.CanonicalString()))
	return fmt.Sprintf("%x", h[:16])
}

// DefaultInterruptProofPage returns the default proof page (nothing permitted).
func DefaultInterruptProofPage(periodKey, circleIDHash string) *InterruptProofPage {
	p := &InterruptProofPage{
		Title:              "Interruptions, quietly.",
		Lines:              defaultProofLines(),
		PermittedMagnitude: MagnitudeNothing,
		DeniedMagnitude:    MagnitudeNothing,
		DismissPath:        "/proof/interrupts/dismiss",
		DismissMethod:      "POST",
		BackLink:           "/today",
		PolicySummary:      "Interruptions are off.",
		PeriodKey:          periodKey,
		CircleIDHash:       circleIDHash,
	}
	p.StatusHash = p.ComputeStatusHash()
	return p
}

// defaultProofLines returns the calm copy for the proof page.
func defaultProofLines() []string {
	return []string{
		"Your boundaries are being respected.",
		"We will still ask before action.",
	}
}

// InterruptPolicyRecord is the persistence record for a policy.
// CRITICAL: Hash-only. No raw identifiers.
type InterruptPolicyRecord struct {
	// RecordID is the unique identifier for this record.
	RecordID string `json:"record_id"`

	// CircleIDHash identifies the circle.
	CircleIDHash string `json:"circle_id_hash"`

	// PeriodKey is the daily bucket.
	PeriodKey string `json:"period_key"`

	// Allowance is the policy allowance.
	Allowance InterruptAllowance `json:"allowance"`

	// MaxPerDay is the configured max.
	MaxPerDay int `json:"max_per_day"`

	// CreatedBucket is when the policy was created.
	CreatedBucket string `json:"created_bucket"`

	// PolicyHash is the deterministic hash.
	PolicyHash string `json:"policy_hash"`
}

// CanonicalString returns the pipe-delimited canonical representation.
func (r *InterruptPolicyRecord) CanonicalString() string {
	return fmt.Sprintf("INTERRUPT_POLICY_RECORD|v1|%s|%s|%s|%s|%d|%s|%s",
		r.RecordID,
		r.CircleIDHash,
		r.PeriodKey,
		r.Allowance,
		r.MaxPerDay,
		r.CreatedBucket,
		r.PolicyHash,
	)
}

// ComputeRecordID computes the record ID from the policy hash and created bucket.
func (r *InterruptPolicyRecord) ComputeRecordID() string {
	input := fmt.Sprintf("%s|%s|%s", r.CircleIDHash, r.PeriodKey, r.CreatedBucket)
	h := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", h[:16])
}

// FromPolicy creates a record from a policy.
func (r *InterruptPolicyRecord) FromPolicy(p *InterruptPolicy) {
	r.CircleIDHash = p.CircleIDHash
	r.PeriodKey = p.PeriodKey
	r.Allowance = p.Allowance
	r.MaxPerDay = p.MaxPerDay
	r.CreatedBucket = p.CreatedBucket
	r.PolicyHash = p.PolicyHash
	r.RecordID = r.ComputeRecordID()
}

// ToPolicy converts a record back to a policy.
func (r *InterruptPolicyRecord) ToPolicy() *InterruptPolicy {
	return &InterruptPolicy{
		CircleIDHash:  r.CircleIDHash,
		PeriodKey:     r.PeriodKey,
		Allowance:     r.Allowance,
		MaxPerDay:     r.MaxPerDay,
		CreatedBucket: r.CreatedBucket,
		PolicyHash:    r.PolicyHash,
	}
}

// InterruptProofAck records that a user dismissed the proof cue.
// CRITICAL: Hash-only. No raw identifiers.
type InterruptProofAck struct {
	// AckID is the unique identifier for this acknowledgment.
	AckID string `json:"ack_id"`

	// CircleIDHash identifies the circle.
	CircleIDHash string `json:"circle_id_hash"`

	// PeriodKey is the daily bucket.
	PeriodKey string `json:"period_key"`

	// AckBucket is when the ack was recorded.
	AckBucket string `json:"ack_bucket"`

	// StatusHash is the proof page status that was dismissed.
	StatusHash string `json:"status_hash"`
}

// CanonicalString returns the pipe-delimited canonical representation.
func (a *InterruptProofAck) CanonicalString() string {
	return fmt.Sprintf("INTERRUPT_PROOF_ACK|v1|%s|%s|%s|%s|%s",
		a.AckID,
		a.CircleIDHash,
		a.PeriodKey,
		a.AckBucket,
		a.StatusHash,
	)
}

// ComputeAckID computes the ack ID.
func (a *InterruptProofAck) ComputeAckID() string {
	input := fmt.Sprintf("%s|%s|%s", a.CircleIDHash, a.PeriodKey, a.AckBucket)
	h := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", h[:16])
}

// InterruptPermissionInput is the input to the permission engine.
type InterruptPermissionInput struct {
	// CircleIDHash identifies the circle.
	CircleIDHash string `json:"circle_id_hash"`

	// PeriodKey is the daily bucket.
	PeriodKey string `json:"period_key"`

	// Policy is the current effective policy (nil means default).
	Policy *InterruptPolicy `json:"policy"`

	// Candidates are the INTERRUPT_CANDIDATE decisions to evaluate.
	Candidates []*InterruptCandidate `json:"candidates"`

	// TrustFragile indicates if trust is in fragile state.
	TrustFragile bool `json:"trust_fragile"`

	// TimeBucket is the current time bucket for determinism.
	TimeBucket string `json:"time_bucket"`
}

// Validate checks if the input is valid.
func (i *InterruptPermissionInput) Validate() error {
	if i.CircleIDHash == "" {
		return fmt.Errorf("circle_id_hash required")
	}
	if i.PeriodKey == "" {
		return fmt.Errorf("period_key required")
	}
	return nil
}

// CanonicalString returns the pipe-delimited canonical representation.
func (i *InterruptPermissionInput) CanonicalString() string {
	policyHash := "none"
	if i.Policy != nil {
		policyHash = i.Policy.PolicyHash
	}
	candidateHashes := make([]string, len(i.Candidates))
	for idx, c := range i.Candidates {
		candidateHashes[idx] = c.CandidateHash
	}
	trustStr := "stable"
	if i.TrustFragile {
		trustStr = "fragile"
	}
	return fmt.Sprintf("INTERRUPT_PERMISSION_INPUT|v1|%s|%s|%s|%s|%d|%s",
		i.CircleIDHash,
		i.PeriodKey,
		policyHash,
		trustStr,
		len(i.Candidates),
		strings.Join(candidateHashes, ","),
	)
}

// ComputeInputHash computes the hash of this input.
func (i *InterruptPermissionInput) ComputeInputHash() string {
	h := sha256.Sum256([]byte(i.CanonicalString()))
	return fmt.Sprintf("%x", h[:16])
}

// InterruptPermissionResult is the output of the permission engine.
type InterruptPermissionResult struct {
	// Decisions are the permission decisions for each candidate.
	Decisions []*InterruptPermissionDecision `json:"decisions"`

	// PermittedMagnitude is the abstract count of permitted interrupts.
	PermittedMagnitude MagnitudeBucket `json:"permitted_magnitude"`

	// DeniedMagnitude is the abstract count of denied interrupts.
	DeniedMagnitude MagnitudeBucket `json:"denied_magnitude"`

	// StatusHash is the deterministic hash of this result.
	StatusHash string `json:"status_hash"`

	// InputHash is the hash of the input that produced this result.
	InputHash string `json:"input_hash"`
}

// CanonicalString returns the pipe-delimited canonical representation.
func (r *InterruptPermissionResult) CanonicalString() string {
	decisionHashes := make([]string, len(r.Decisions))
	for idx, d := range r.Decisions {
		decisionHashes[idx] = d.DeterministicHash
	}
	return fmt.Sprintf("INTERRUPT_PERMISSION_RESULT|v1|%s|%s|%s|%s",
		r.PermittedMagnitude,
		r.DeniedMagnitude,
		r.InputHash,
		strings.Join(decisionHashes, ","),
	)
}

// ComputeStatusHash computes the status hash of this result.
func (r *InterruptPermissionResult) ComputeStatusHash() string {
	h := sha256.Sum256([]byte(r.CanonicalString()))
	return fmt.Sprintf("%x", h[:16])
}

// InterruptWhisperCue represents the Phase 33 whisper cue for /today.
type InterruptWhisperCue struct {
	// Text is the calm cue text.
	Text string `json:"text"`

	// Link is the path to the proof page.
	Link string `json:"link"`

	// Visible indicates if the cue should be shown.
	Visible bool `json:"visible"`

	// Priority is the cue priority (lower = less important).
	Priority int `json:"priority"`
}

// DefaultInterruptWhisperCue returns the default whisper cue.
func DefaultInterruptWhisperCue() *InterruptWhisperCue {
	return &InterruptWhisperCue{
		Text:     "If you ever need it — interruptions are still being held.",
		Link:     "/proof/interrupts",
		Visible:  false,
		Priority: 100, // Lowest priority (highest number)
	}
}

// CircleTypeCommerce is the commerce circle type constant.
const CircleTypeCommerce = "commerce"

// CircleTypeHuman is the human circle type constant.
const CircleTypeHuman = "human"

// CircleTypeInstitution is the institution circle type constant.
const CircleTypeInstitution = "institution"

// HorizonNow is the NOW horizon constant.
const HorizonNow = "now"

// HorizonSoon is the SOON horizon constant.
const HorizonSoon = "soon"

// HorizonLater is the LATER horizon constant.
const HorizonLater = "later"
