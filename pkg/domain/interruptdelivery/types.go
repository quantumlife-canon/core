// Package interruptdelivery defines the Phase 36 Interrupt Delivery Orchestrator domain types.
//
// This package provides explicit, deterministic delivery orchestration for the
// interrupt pipeline: External Pressure (31.4) → Decision Gate (32) →
// Permission Contract (33) → Preview (34) → Transport (35/35b).
//
// CRITICAL INVARIANTS:
//   - Delivery is EXPLICIT. No background execution. No goroutines.
//   - POST-only delivery. Human must trigger explicitly.
//   - Max 2 deliveries per day. Hard cap enforced at orchestration time.
//   - Hash-only storage. No raw identifiers.
//   - Deterministic ordering. Candidates sorted by hash.
//   - Transport-agnostic. Uses Phase 35 transport interface.
//   - HOLD is the default outcome.
//
// Reference: docs/ADR/ADR-0073-phase36-interrupt-delivery-orchestrator.md
package interruptdelivery

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// ═══════════════════════════════════════════════════════════════════════════
// Transport Kind
// ═══════════════════════════════════════════════════════════════════════════

// TransportKind identifies the delivery mechanism.
type TransportKind string

const (
	TransportStub    TransportKind = "stub"
	TransportAPNs    TransportKind = "apns"
	TransportWebhook TransportKind = "webhook"
)

// ValidTransportKinds is the set of valid transport kinds.
var ValidTransportKinds = map[TransportKind]bool{
	TransportStub:    true,
	TransportAPNs:    true,
	TransportWebhook: true,
}

// Validate checks if the transport kind is valid.
func (t TransportKind) Validate() error {
	if !ValidTransportKinds[t] {
		return fmt.Errorf("invalid transport kind: %s", t)
	}
	return nil
}

// String returns the string representation.
func (t TransportKind) String() string {
	return string(t)
}

// ═══════════════════════════════════════════════════════════════════════════
// Result Bucket
// ═══════════════════════════════════════════════════════════════════════════

// ResultBucket represents the outcome of a delivery attempt.
type ResultBucket string

const (
	ResultSent     ResultBucket = "sent"
	ResultSkipped  ResultBucket = "skipped"
	ResultRejected ResultBucket = "rejected"
	ResultDeduped  ResultBucket = "deduped"
)

// ValidResultBuckets is the set of valid result buckets.
var ValidResultBuckets = map[ResultBucket]bool{
	ResultSent:     true,
	ResultSkipped:  true,
	ResultRejected: true,
	ResultDeduped:  true,
}

// Validate checks if the result bucket is valid.
func (r ResultBucket) Validate() error {
	if !ValidResultBuckets[r] {
		return fmt.Errorf("invalid result bucket: %s", r)
	}
	return nil
}

// String returns the string representation.
func (r ResultBucket) String() string {
	return string(r)
}

// DisplayLabel returns a human-friendly label.
func (r ResultBucket) DisplayLabel() string {
	switch r {
	case ResultSent:
		return "Delivered"
	case ResultSkipped:
		return "Not delivered"
	case ResultRejected:
		return "Rejected"
	case ResultDeduped:
		return "Already delivered"
	default:
		return "Unknown"
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Reason Bucket
// ═══════════════════════════════════════════════════════════════════════════

// ReasonBucket explains why a delivery attempt had a particular outcome.
type ReasonBucket string

const (
	ReasonNone           ReasonBucket = "none"
	ReasonPolicyDenies   ReasonBucket = "policy_denies"
	ReasonCapReached     ReasonBucket = "cap_reached"
	ReasonNotConfigured  ReasonBucket = "not_configured"
	ReasonAlreadySent    ReasonBucket = "already_sent"
	ReasonTransportError ReasonBucket = "transport_error"
	ReasonNoCandidate    ReasonBucket = "no_candidate"
	ReasonTrustFragile   ReasonBucket = "trust_fragile"
)

// ValidReasonBuckets is the set of valid reason buckets.
var ValidReasonBuckets = map[ReasonBucket]bool{
	ReasonNone:           true,
	ReasonPolicyDenies:   true,
	ReasonCapReached:     true,
	ReasonNotConfigured:  true,
	ReasonAlreadySent:    true,
	ReasonTransportError: true,
	ReasonNoCandidate:    true,
	ReasonTrustFragile:   true,
}

// Validate checks if the reason bucket is valid.
func (r ReasonBucket) Validate() error {
	if !ValidReasonBuckets[r] {
		return fmt.Errorf("invalid reason bucket: %s", r)
	}
	return nil
}

// String returns the string representation.
func (r ReasonBucket) String() string {
	return string(r)
}

// DisplayLabel returns a human-friendly label.
func (r ReasonBucket) DisplayLabel() string {
	switch r {
	case ReasonNone:
		return ""
	case ReasonPolicyDenies:
		return "Policy does not permit"
	case ReasonCapReached:
		return "Daily limit reached"
	case ReasonNotConfigured:
		return "No device registered"
	case ReasonAlreadySent:
		return "Already delivered this period"
	case ReasonTransportError:
		return "Delivery issue"
	case ReasonNoCandidate:
		return "Nothing to deliver"
	case ReasonTrustFragile:
		return "Trust is being rebuilt"
	default:
		return "Unknown"
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Delivery Candidate
// ═══════════════════════════════════════════════════════════════════════════

// DeliveryCandidate represents an eligible interrupt candidate for delivery.
// CRITICAL: Contains only hashes. Never raw identifiers.
type DeliveryCandidate struct {
	// CandidateHash is the hash identifying this candidate (from Phase 32).
	CandidateHash string

	// CircleIDHash identifies the circle that generated the pressure.
	CircleIDHash string

	// DecisionHash is the Phase 32 decision hash.
	DecisionHash string

	// PeriodKey is the day bucket (YYYY-MM-DD).
	PeriodKey string
}

// Validate checks if the candidate is valid.
func (c *DeliveryCandidate) Validate() error {
	if c.CandidateHash == "" {
		return fmt.Errorf("candidate_hash required")
	}
	if c.CircleIDHash == "" {
		return fmt.Errorf("circle_id_hash required")
	}
	if c.DecisionHash == "" {
		return fmt.Errorf("decision_hash required")
	}
	if c.PeriodKey == "" {
		return fmt.Errorf("period_key required")
	}
	return nil
}

// CanonicalString returns the pipe-delimited canonical representation.
func (c *DeliveryCandidate) CanonicalString() string {
	return fmt.Sprintf("DELIVERY_CANDIDATE|v1|%s|%s|%s|%s",
		c.CandidateHash,
		c.CircleIDHash,
		c.DecisionHash,
		c.PeriodKey,
	)
}

// ComputeHash computes a deterministic hash of the candidate.
func (c *DeliveryCandidate) ComputeHash() string {
	h := sha256.Sum256([]byte(c.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// ═══════════════════════════════════════════════════════════════════════════
// Delivery Attempt
// ═══════════════════════════════════════════════════════════════════════════

// DeliveryAttempt records a single delivery attempt.
// CRITICAL: Hash-only storage. No raw identifiers.
type DeliveryAttempt struct {
	// AttemptID is the unique identifier (SHA256 of canonical).
	AttemptID string

	// CandidateHash identifies which candidate was attempted.
	CandidateHash string

	// CircleIDHash identifies the circle.
	CircleIDHash string

	// TransportKind is the delivery mechanism used.
	TransportKind TransportKind

	// ResultBucket is the outcome (sent, skipped, rejected, deduped).
	ResultBucket ResultBucket

	// ReasonBucket explains why this result occurred.
	ReasonBucket ReasonBucket

	// PeriodKey is the day bucket (YYYY-MM-DD).
	PeriodKey string

	// AttemptBucket is the time bucket (HH:MM floored to 15-min).
	AttemptBucket string

	// StatusHash is a deterministic hash of the attempt state.
	StatusHash string
}

// Validate checks if the attempt is valid.
func (a *DeliveryAttempt) Validate() error {
	if a.AttemptID == "" {
		return fmt.Errorf("attempt_id required")
	}
	if a.CircleIDHash == "" {
		return fmt.Errorf("circle_id_hash required")
	}
	if a.PeriodKey == "" {
		return fmt.Errorf("period_key required")
	}
	if err := a.TransportKind.Validate(); err != nil {
		return err
	}
	if err := a.ResultBucket.Validate(); err != nil {
		return err
	}
	if err := a.ReasonBucket.Validate(); err != nil {
		return err
	}
	return nil
}

// CanonicalString returns the pipe-delimited canonical representation.
func (a *DeliveryAttempt) CanonicalString() string {
	return fmt.Sprintf("DELIVERY_ATTEMPT|v1|%s|%s|%s|%s|%s|%s|%s",
		a.CandidateHash,
		a.CircleIDHash,
		a.TransportKind,
		a.ResultBucket,
		a.ReasonBucket,
		a.PeriodKey,
		a.AttemptBucket,
	)
}

// ComputeAttemptID computes a deterministic attempt ID.
// Uses circle+candidate+period for deduplication.
func (a *DeliveryAttempt) ComputeAttemptID() string {
	dedupKey := fmt.Sprintf("DELIVERY_DEDUP|v1|%s|%s|%s",
		a.CircleIDHash,
		a.CandidateHash,
		a.PeriodKey,
	)
	h := sha256.Sum256([]byte(dedupKey))
	return hex.EncodeToString(h[:16])
}

// ComputeStatusHash computes a deterministic hash of the full attempt state.
func (a *DeliveryAttempt) ComputeStatusHash() string {
	h := sha256.Sum256([]byte(a.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// ═══════════════════════════════════════════════════════════════════════════
// Attempt Summary
// ═══════════════════════════════════════════════════════════════════════════

// AttemptSummary provides a bucketed summary of an attempt for the receipt.
type AttemptSummary struct {
	// ResultBucket is the outcome.
	ResultBucket ResultBucket

	// ReasonBucket explains the outcome.
	ReasonBucket ReasonBucket

	// TransportKind is the delivery mechanism.
	TransportKind TransportKind

	// AttemptHash is the hash of the attempt.
	AttemptHash string
}

// CanonicalString returns the pipe-delimited canonical representation.
func (s *AttemptSummary) CanonicalString() string {
	return fmt.Sprintf("ATTEMPT_SUMMARY|v1|%s|%s|%s|%s",
		s.ResultBucket,
		s.ReasonBucket,
		s.TransportKind,
		s.AttemptHash,
	)
}

// ═══════════════════════════════════════════════════════════════════════════
// Delivery Receipt
// ═══════════════════════════════════════════════════════════════════════════

// DeliveryReceipt is the proof of a delivery run.
// CRITICAL: Contains only abstract buckets and hashes.
type DeliveryReceipt struct {
	// ReceiptID is the unique identifier (SHA256 of canonical).
	ReceiptID string

	// CircleIDHash identifies the circle.
	CircleIDHash string

	// Attempts contains bucketed summaries of each attempt.
	Attempts []AttemptSummary

	// SentCount is the number of sent attempts.
	SentCount int

	// SkippedCount is the number of skipped attempts.
	SkippedCount int

	// DedupedCount is the number of deduped attempts.
	DedupedCount int

	// PeriodKey is the day bucket.
	PeriodKey string

	// TimeBucket is when the receipt was created.
	TimeBucket string

	// StatusHash is a deterministic hash of the receipt state.
	StatusHash string
}

// Validate checks if the receipt is valid.
func (r *DeliveryReceipt) Validate() error {
	if r.ReceiptID == "" {
		return fmt.Errorf("receipt_id required")
	}
	if r.CircleIDHash == "" {
		return fmt.Errorf("circle_id_hash required")
	}
	if r.PeriodKey == "" {
		return fmt.Errorf("period_key required")
	}
	return nil
}

// CanonicalString returns the pipe-delimited canonical representation.
func (r *DeliveryReceipt) CanonicalString() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("DELIVERY_RECEIPT|v1|%s|%d|%d|%d|%s|%s",
		r.CircleIDHash,
		r.SentCount,
		r.SkippedCount,
		r.DedupedCount,
		r.PeriodKey,
		r.TimeBucket,
	))

	for _, a := range r.Attempts {
		sb.WriteString("|")
		sb.WriteString(a.AttemptHash)
	}

	return sb.String()
}

// ComputeReceiptID computes a deterministic receipt ID.
func (r *DeliveryReceipt) ComputeReceiptID() string {
	h := sha256.Sum256([]byte(r.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// ComputeStatusHash computes a deterministic hash of the receipt state.
func (r *DeliveryReceipt) ComputeStatusHash() string {
	content := fmt.Sprintf("RECEIPT_STATUS|v1|%s|%d|%d|%d|%s",
		r.CircleIDHash,
		r.SentCount,
		r.SkippedCount,
		r.DedupedCount,
		r.PeriodKey,
	)
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:16])
}

// ═══════════════════════════════════════════════════════════════════════════
// Delivery Proof Page
// ═══════════════════════════════════════════════════════════════════════════

// DeliveryProofPage represents the proof page for delivery.
// CRITICAL: No raw identifiers. Abstract buckets only.
type DeliveryProofPage struct {
	// Title is the page title.
	Title string

	// Subtitle provides context.
	Subtitle string

	// Lines are calm copy lines.
	Lines []string

	// SentLabel describes how many were sent.
	SentLabel string

	// SkippedLabel describes how many were skipped.
	SkippedLabel string

	// StatusHash is a deterministic hash of the page state.
	StatusHash string

	// EvidenceHashes are opaque hashes for verification.
	EvidenceHashes []string

	// DismissPath is the path for dismissing the cue.
	DismissPath string

	// DismissMethod is the HTTP method for dismissing.
	DismissMethod string

	// BackLink is the path to return to.
	BackLink string

	// PeriodKey is the current period.
	PeriodKey string

	// CircleIDHash is the circle.
	CircleIDHash string
}

// CanonicalString returns the pipe-delimited canonical representation.
func (p *DeliveryProofPage) CanonicalString() string {
	return fmt.Sprintf("DELIVERY_PROOF_PAGE|v1|%s|%s|%s|%s",
		p.SentLabel,
		p.SkippedLabel,
		p.PeriodKey,
		strings.Join(p.EvidenceHashes, ","),
	)
}

// ComputeStatusHash computes a deterministic hash of the page state.
func (p *DeliveryProofPage) ComputeStatusHash() string {
	h := sha256.Sum256([]byte(p.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// DefaultDeliveryProofPage returns the default proof page.
func DefaultDeliveryProofPage(periodKey, circleIDHash string) *DeliveryProofPage {
	p := &DeliveryProofPage{
		Title:          "Delivery, quietly.",
		Subtitle:       "What happened with interruptions this period.",
		Lines:          defaultProofLines(),
		SentLabel:      "Nothing",
		SkippedLabel:   "Nothing",
		DismissPath:    "/proof/delivery/dismiss",
		DismissMethod:  "POST",
		BackLink:       "/today",
		PeriodKey:      periodKey,
		CircleIDHash:   circleIDHash,
		EvidenceHashes: []string{},
	}
	p.StatusHash = p.ComputeStatusHash()
	return p
}

// defaultProofLines returns calm copy for the proof page.
func defaultProofLines() []string {
	return []string{
		"Delivery is explicit, not automatic.",
		"We only deliver when you ask.",
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Delivery Cue
// ═══════════════════════════════════════════════════════════════════════════

// DeliveryCue represents the whisper cue for /today.
type DeliveryCue struct {
	// Available indicates if a delivery occurred this period.
	Available bool

	// Text is the cue text.
	Text string

	// LinkPath is the path to the proof page.
	LinkPath string

	// StatusHash is a deterministic hash of the cue state.
	StatusHash string

	// Priority is the cue priority (higher number = lower priority).
	Priority int
}

// DefaultDeliveryCueText is the cue text after delivery.
const DefaultDeliveryCueText = "We delivered something — carefully."

// DefaultDeliveryCuePath is the path to the delivery proof page.
const DefaultDeliveryCuePath = "/proof/delivery"

// DefaultDeliveryCuePriority is the priority for Phase 36 cue.
// Lowest priority in the whisper chain.
const DefaultDeliveryCuePriority = 120

// CanonicalString returns the pipe-delimited canonical representation.
func (c *DeliveryCue) CanonicalString() string {
	availStr := "no"
	if c.Available {
		availStr = "yes"
	}
	return fmt.Sprintf("DELIVERY_CUE|v1|%s|%s|%d",
		availStr,
		c.LinkPath,
		c.Priority,
	)
}

// ComputeStatusHash computes a deterministic hash of the cue state.
func (c *DeliveryCue) ComputeStatusHash() string {
	h := sha256.Sum256([]byte(c.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// DefaultDeliveryCue returns the default (unavailable) delivery cue.
func DefaultDeliveryCue() *DeliveryCue {
	cue := &DeliveryCue{
		Available: false,
		Text:      DefaultDeliveryCueText,
		LinkPath:  DefaultDeliveryCuePath,
		Priority:  DefaultDeliveryCuePriority,
	}
	cue.StatusHash = cue.ComputeStatusHash()
	return cue
}

// ═══════════════════════════════════════════════════════════════════════════
// Delivery Ack
// ═══════════════════════════════════════════════════════════════════════════

// DeliveryAck records that a delivery proof was acknowledged.
// CRITICAL: Hash-only. No raw identifiers.
type DeliveryAck struct {
	// AckID is the unique identifier.
	AckID string

	// CircleIDHash identifies the circle.
	CircleIDHash string

	// PeriodKey is the day bucket.
	PeriodKey string

	// AckBucket is when the ack was recorded.
	AckBucket string

	// ReceiptHash is the hash of the receipt that was acked.
	ReceiptHash string
}

// CanonicalString returns the pipe-delimited canonical representation.
func (a *DeliveryAck) CanonicalString() string {
	return fmt.Sprintf("DELIVERY_ACK|v1|%s|%s|%s|%s|%s",
		a.AckID,
		a.CircleIDHash,
		a.PeriodKey,
		a.AckBucket,
		a.ReceiptHash,
	)
}

// ComputeAckID computes a deterministic ack ID.
func (a *DeliveryAck) ComputeAckID() string {
	input := fmt.Sprintf("%s|%s|%s", a.CircleIDHash, a.PeriodKey, a.AckBucket)
	h := sha256.Sum256([]byte(input))
	return hex.EncodeToString(h[:16])
}

// ═══════════════════════════════════════════════════════════════════════════
// Delivery Input
// ═══════════════════════════════════════════════════════════════════════════

// DeliveryInput contains all inputs for the delivery engine.
type DeliveryInput struct {
	// CircleIDHash identifies the circle.
	CircleIDHash string

	// PeriodKey is the current day bucket.
	PeriodKey string

	// TimeBucket is the current time bucket (15-min interval).
	TimeBucket string

	// Candidates are the INTERRUPT_CANDIDATEs from Phase 32.
	Candidates []*DeliveryCandidate

	// PolicyAllowed indicates if Phase 33 policy permits delivery.
	PolicyAllowed bool

	// TrustFragile indicates if trust is fragile (Phase 20).
	TrustFragile bool

	// PushEnabled indicates if push transport is configured.
	PushEnabled bool

	// SentToday is the count of sent attempts today.
	SentToday int

	// MaxPerDay is the daily cap (typically 2).
	MaxPerDay int

	// PriorAttempts are prior attempts for deduplication.
	PriorAttempts map[string]bool // key: candidate_hash
}

// Validate checks if the input is valid.
func (i *DeliveryInput) Validate() error {
	if i.CircleIDHash == "" {
		return fmt.Errorf("circle_id_hash required")
	}
	if i.PeriodKey == "" {
		return fmt.Errorf("period_key required")
	}
	return nil
}

// CanonicalString returns the pipe-delimited canonical representation.
func (i *DeliveryInput) CanonicalString() string {
	candidateHashes := make([]string, len(i.Candidates))
	for idx, c := range i.Candidates {
		candidateHashes[idx] = c.CandidateHash
	}
	policyStr := "denied"
	if i.PolicyAllowed {
		policyStr = "allowed"
	}
	trustStr := "stable"
	if i.TrustFragile {
		trustStr = "fragile"
	}
	pushStr := "disabled"
	if i.PushEnabled {
		pushStr = "enabled"
	}
	return fmt.Sprintf("DELIVERY_INPUT|v1|%s|%s|%s|%s|%s|%d|%d|%s",
		i.CircleIDHash,
		i.PeriodKey,
		policyStr,
		trustStr,
		pushStr,
		i.SentToday,
		len(i.Candidates),
		strings.Join(candidateHashes, ","),
	)
}

// ComputeInputHash computes a deterministic hash of the input.
func (i *DeliveryInput) ComputeInputHash() string {
	h := sha256.Sum256([]byte(i.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// ═══════════════════════════════════════════════════════════════════════════
// Constants
// ═══════════════════════════════════════════════════════════════════════════

// MaxDeliveriesPerDay is the absolute cap on deliveries per day.
const MaxDeliveriesPerDay = 2

// MagnitudeBucket represents abstract quantities.
type MagnitudeBucket string

const (
	MagnitudeNothing MagnitudeBucket = "nothing"
	MagnitudeAFew    MagnitudeBucket = "a_few"
	MagnitudeSeveral MagnitudeBucket = "several"
)

// MagnitudeFromCount converts a count to a magnitude bucket.
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

// MagnitudeLabel returns a human-friendly label.
func MagnitudeLabel(m MagnitudeBucket) string {
	switch m {
	case MagnitudeNothing:
		return "Nothing"
	case MagnitudeAFew:
		return "A couple"
	case MagnitudeSeveral:
		return "Several"
	default:
		return "Unknown"
	}
}
