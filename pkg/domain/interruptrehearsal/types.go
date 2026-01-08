// Package interruptrehearsal defines the Phase 41 Live Interrupt Loop (APNs) domain types.
//
// This package provides rehearsal delivery + proof for interrupt candidates.
// It orchestrates "preview→deliver→receipt" using existing Phase 32-34 pipeline outputs.
//
// CRITICAL INVARIANTS:
//   - NO goroutines. NO time.Now() - clock injection only.
//   - NO new decision logic - must reuse Phase 32→33→34 pipeline outputs.
//   - Abstract payload only. No identifiers. No names. No merchants.
//   - Device token secrecy: raw token only in sealed boundary.
//   - Delivery cap: max 2/day per circle (Phase 35/36 cap respected).
//   - Deterministic IDs/hashes: same inputs + same clock period => same hashes.
//   - Single whisper rule respected - Phase 41 adds NO new whispers on /today.
//
// Reference: docs/ADR/ADR-0078-phase41-live-interrupt-loop-apns.md
package interruptrehearsal

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

// ═══════════════════════════════════════════════════════════════════════════
// Constants
// ═══════════════════════════════════════════════════════════════════════════

// MaxDeliveriesPerDay is the hard cap for deliveries per circle per day.
// Must match Phase 35/36 cap semantics.
const MaxDeliveriesPerDay = 2

// MaxRetentionDays is the maximum retention period for receipts.
const MaxRetentionDays = 30

// MaxRecords is the maximum number of records to store (FIFO eviction).
const MaxRecords = 500

// DeepLinkTarget is the target for the push deep link.
// Maps to /open?t=interrupts (Phase 37).
const DeepLinkTarget = "interrupts"

// PushTitle is the constant title for the push notification.
// CRITICAL: This is a constant literal. No customization allowed.
const PushTitle = "QuantumLife"

// PushBody is the constant body for the push notification.
// CRITICAL: This is a constant literal. No customization allowed.
const PushBody = "Something needs you. Open QuantumLife."

// ═══════════════════════════════════════════════════════════════════════════
// Rehearsal Kind
// ═══════════════════════════════════════════════════════════════════════════

// RehearsalKind identifies the type of rehearsal.
type RehearsalKind string

const (
	// RehearsalInterruptDelivery is the kind for interrupt delivery rehearsal.
	RehearsalInterruptDelivery RehearsalKind = "rehearsal_interrupt_delivery"
)

// ValidRehearsalKinds is the set of valid rehearsal kinds.
var ValidRehearsalKinds = map[RehearsalKind]bool{
	RehearsalInterruptDelivery: true,
}

// Validate checks if the rehearsal kind is valid.
func (k RehearsalKind) Validate() error {
	if !ValidRehearsalKinds[k] {
		return fmt.Errorf("invalid rehearsal kind: %s", k)
	}
	return nil
}

// String returns the string representation.
func (k RehearsalKind) String() string {
	return string(k)
}

// CanonicalString returns the canonical representation.
func (k RehearsalKind) CanonicalString() string {
	return string(k)
}

// ═══════════════════════════════════════════════════════════════════════════
// Rehearsal Status
// ═══════════════════════════════════════════════════════════════════════════

// RehearsalStatus represents the status of a rehearsal.
type RehearsalStatus string

const (
	// StatusRequested indicates rehearsal was requested and is eligible.
	StatusRequested RehearsalStatus = "status_requested"

	// StatusRejected indicates rehearsal was rejected (see RejectReason).
	StatusRejected RehearsalStatus = "status_rejected"

	// StatusAttempted indicates delivery was attempted.
	StatusAttempted RehearsalStatus = "status_attempted"

	// StatusDelivered indicates delivery succeeded.
	StatusDelivered RehearsalStatus = "status_delivered"

	// StatusFailed indicates delivery failed.
	StatusFailed RehearsalStatus = "status_failed"
)

// ValidRehearsalStatuses is the set of valid statuses.
var ValidRehearsalStatuses = map[RehearsalStatus]bool{
	StatusRequested: true,
	StatusRejected:  true,
	StatusAttempted: true,
	StatusDelivered: true,
	StatusFailed:    true,
}

// Validate checks if the status is valid.
func (s RehearsalStatus) Validate() error {
	if !ValidRehearsalStatuses[s] {
		return fmt.Errorf("invalid rehearsal status: %s", s)
	}
	return nil
}

// String returns the string representation.
func (s RehearsalStatus) String() string {
	return string(s)
}

// CanonicalString returns the canonical representation.
func (s RehearsalStatus) CanonicalString() string {
	return string(s)
}

// DisplayLabel returns a human-friendly label.
func (s RehearsalStatus) DisplayLabel() string {
	switch s {
	case StatusRequested:
		return "Requested"
	case StatusRejected:
		return "Not sent"
	case StatusAttempted:
		return "Attempted, quietly."
	case StatusDelivered:
		return "Delivered, quietly."
	case StatusFailed:
		return "Delivery issue"
	default:
		return "Unknown"
	}
}

// IsTerminal returns true if this is a terminal status.
func (s RehearsalStatus) IsTerminal() bool {
	return s == StatusRejected || s == StatusDelivered || s == StatusFailed
}

// ═══════════════════════════════════════════════════════════════════════════
// Reject Reason
// ═══════════════════════════════════════════════════════════════════════════

// RehearsalRejectReason explains why a rehearsal was rejected.
type RehearsalRejectReason string

const (
	// RejectNone indicates no rejection (successful or not rejected).
	RejectNone RehearsalRejectReason = "reject_none"

	// RejectNoDevice indicates no device registered.
	RejectNoDevice RehearsalRejectReason = "reject_no_device"

	// RejectPolicyDisallows indicates policy does not allow interrupts.
	RejectPolicyDisallows RehearsalRejectReason = "reject_policy_disallows"

	// RejectNoCandidate indicates no interrupt candidate available.
	RejectNoCandidate RehearsalRejectReason = "reject_no_candidate"

	// RejectRateLimited indicates daily cap reached.
	RejectRateLimited RehearsalRejectReason = "reject_rate_limited"

	// RejectTransportUnavailable indicates transport is not available.
	RejectTransportUnavailable RehearsalRejectReason = "reject_transport_unavailable"

	// RejectSealedKeyMissing indicates APNs sealed key is not configured.
	RejectSealedKeyMissing RehearsalRejectReason = "reject_sealed_key_missing"
)

// ValidRejectReasons is the set of valid reject reasons.
var ValidRejectReasons = map[RehearsalRejectReason]bool{
	RejectNone:                 true,
	RejectNoDevice:             true,
	RejectPolicyDisallows:      true,
	RejectNoCandidate:          true,
	RejectRateLimited:          true,
	RejectTransportUnavailable: true,
	RejectSealedKeyMissing:     true,
}

// Validate checks if the reject reason is valid.
func (r RehearsalRejectReason) Validate() error {
	if !ValidRejectReasons[r] {
		return fmt.Errorf("invalid reject reason: %s", r)
	}
	return nil
}

// String returns the string representation.
func (r RehearsalRejectReason) String() string {
	return string(r)
}

// CanonicalString returns the canonical representation.
func (r RehearsalRejectReason) CanonicalString() string {
	return string(r)
}

// DisplayLabel returns a human-friendly label.
func (r RehearsalRejectReason) DisplayLabel() string {
	switch r {
	case RejectNone:
		return ""
	case RejectNoDevice:
		return "No device registered"
	case RejectPolicyDisallows:
		return "Interrupts are turned off"
	case RejectNoCandidate:
		return "Nothing to deliver"
	case RejectRateLimited:
		return "Daily limit reached"
	case RejectTransportUnavailable:
		return "Transport not available"
	case RejectSealedKeyMissing:
		return "Push credentials not configured"
	default:
		return "Unknown reason"
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Transport Kind
// ═══════════════════════════════════════════════════════════════════════════

// TransportKind identifies the transport mechanism.
type TransportKind string

const (
	TransportAPNs    TransportKind = "apns"
	TransportWebhook TransportKind = "webhook"
	TransportStub    TransportKind = "stub"
	TransportNone    TransportKind = "none"
)

// ValidTransportKinds is the set of valid transport kinds.
var ValidTransportKinds = map[TransportKind]bool{
	TransportAPNs:    true,
	TransportWebhook: true,
	TransportStub:    true,
	TransportNone:    true,
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

// CanonicalString returns the canonical representation.
func (t TransportKind) CanonicalString() string {
	return string(t)
}

// DisplayLabel returns a human-friendly label.
func (t TransportKind) DisplayLabel() string {
	switch t {
	case TransportAPNs:
		return "Apple Push"
	case TransportWebhook:
		return "Webhook"
	case TransportStub:
		return "Test Mode"
	case TransportNone:
		return "None"
	default:
		return "Unknown"
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Delivery Bucket
// ═══════════════════════════════════════════════════════════════════════════

// DeliveryBucket represents abstract delivery count (no actual counts).
type DeliveryBucket string

const (
	DeliveryNone DeliveryBucket = "delivery_none"
	DeliveryOne  DeliveryBucket = "delivery_one"
)

// ValidDeliveryBuckets is the set of valid delivery buckets.
var ValidDeliveryBuckets = map[DeliveryBucket]bool{
	DeliveryNone: true,
	DeliveryOne:  true,
}

// Validate checks if the delivery bucket is valid.
func (d DeliveryBucket) Validate() error {
	if !ValidDeliveryBuckets[d] {
		return fmt.Errorf("invalid delivery bucket: %s", d)
	}
	return nil
}

// String returns the string representation.
func (d DeliveryBucket) String() string {
	return string(d)
}

// CanonicalString returns the canonical representation.
func (d DeliveryBucket) CanonicalString() string {
	return string(d)
}

// ═══════════════════════════════════════════════════════════════════════════
// Latency Bucket
// ═══════════════════════════════════════════════════════════════════════════

// LatencyBucket represents abstract latency measurement.
// NOTE: Latency measurement is permitted ONLY in cmd/ wrapper.
// Internal engine accepts bucket as input.
type LatencyBucket string

const (
	LatencyFast LatencyBucket = "fast" // < 500ms
	LatencyOK   LatencyBucket = "ok"   // 500ms - 2s
	LatencySlow LatencyBucket = "slow" // > 2s
	LatencyNA   LatencyBucket = "na"   // not applicable (e.g., rejected)
)

// ValidLatencyBuckets is the set of valid latency buckets.
var ValidLatencyBuckets = map[LatencyBucket]bool{
	LatencyFast: true,
	LatencyOK:   true,
	LatencySlow: true,
	LatencyNA:   true,
}

// Validate checks if the latency bucket is valid.
func (l LatencyBucket) Validate() error {
	if !ValidLatencyBuckets[l] {
		return fmt.Errorf("invalid latency bucket: %s", l)
	}
	return nil
}

// String returns the string representation.
func (l LatencyBucket) String() string {
	return string(l)
}

// CanonicalString returns the canonical representation.
func (l LatencyBucket) CanonicalString() string {
	return string(l)
}

// ═══════════════════════════════════════════════════════════════════════════
// Error Class Bucket
// ═══════════════════════════════════════════════════════════════════════════

// ErrorClassBucket categorizes delivery errors abstractly.
type ErrorClassBucket string

const (
	ErrorClassNone      ErrorClassBucket = "none"
	ErrorClassTransient ErrorClassBucket = "transient"
	ErrorClassPermanent ErrorClassBucket = "permanent"
	ErrorClassUnknown   ErrorClassBucket = "unknown"
)

// ValidErrorClassBuckets is the set of valid error class buckets.
var ValidErrorClassBuckets = map[ErrorClassBucket]bool{
	ErrorClassNone:      true,
	ErrorClassTransient: true,
	ErrorClassPermanent: true,
	ErrorClassUnknown:   true,
}

// Validate checks if the error class bucket is valid.
func (e ErrorClassBucket) Validate() error {
	if !ValidErrorClassBuckets[e] {
		return fmt.Errorf("invalid error class bucket: %s", e)
	}
	return nil
}

// String returns the string representation.
func (e ErrorClassBucket) String() string {
	return string(e)
}

// CanonicalString returns the canonical representation.
func (e ErrorClassBucket) CanonicalString() string {
	return string(e)
}

// ═══════════════════════════════════════════════════════════════════════════
// Rehearsal Inputs
// ═══════════════════════════════════════════════════════════════════════════

// RehearsalInputs contains all inputs for computing rehearsal eligibility.
type RehearsalInputs struct {
	// CircleIDHash identifies the circle (SHA256 hash).
	CircleIDHash string

	// PeriodKey is the current day (YYYY-MM-DD).
	PeriodKey string

	// Allowance is the current policy allowance.
	Allowance string

	// MaxPerDay is the configured max deliveries per day.
	MaxPerDay int

	// DailyDeliveryCount is the count of deliveries today.
	DailyDeliveryCount int

	// HasDevice indicates if a device is registered.
	HasDevice bool

	// CandidateHash is the hash of the selected candidate (empty if none).
	CandidateHash string

	// TransportKind is the transport mechanism.
	TransportKind TransportKind

	// SealedReady indicates if APNs sealed credentials are configured.
	SealedReady bool

	// EnvelopeActive indicates if an attention envelope is active.
	EnvelopeActive bool

	// TimeBucket is the current 15-minute interval.
	TimeBucket string
}

// Validate checks if the inputs are valid.
func (i *RehearsalInputs) Validate() error {
	if i.CircleIDHash == "" {
		return fmt.Errorf("circle_id_hash is required")
	}
	if i.PeriodKey == "" {
		return fmt.Errorf("period_key is required")
	}
	return nil
}

// CanonicalString returns the canonical representation.
func (i *RehearsalInputs) CanonicalString() string {
	return fmt.Sprintf("REHEARSAL_INPUT|v1|%s|%s|%s|%d|%t|%s|%s|%t|%t",
		i.CircleIDHash,
		i.PeriodKey,
		i.Allowance,
		i.MaxPerDay,
		i.HasDevice,
		i.CandidateHash,
		i.TransportKind,
		i.SealedReady,
		i.EnvelopeActive,
	)
}

// ComputeInputHash computes the hash of this input.
func (i *RehearsalInputs) ComputeInputHash() string {
	h := sha256.Sum256([]byte(i.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// ═══════════════════════════════════════════════════════════════════════════
// Rehearsal Plan
// ═══════════════════════════════════════════════════════════════════════════

// RehearsalPlan contains the delivery plan (built before actual delivery).
type RehearsalPlan struct {
	// AttemptIDHash is the deterministic hash for this attempt.
	AttemptIDHash string

	// TransportKind is the transport to use.
	TransportKind TransportKind

	// DeepLinkTarget is the target for the deep link.
	DeepLinkTarget string

	// PayloadTitle is the push notification title.
	PayloadTitle string

	// PayloadBody is the push notification body.
	PayloadBody string

	// CandidateHash is the hash of the candidate being delivered.
	CandidateHash string
}

// Validate checks if the plan is valid.
func (p *RehearsalPlan) Validate() error {
	if p.AttemptIDHash == "" {
		return fmt.Errorf("attempt_id_hash is required")
	}
	if err := p.TransportKind.Validate(); err != nil {
		return err
	}
	return nil
}

// CanonicalString returns the canonical representation.
func (p *RehearsalPlan) CanonicalString() string {
	return fmt.Sprintf("REHEARSAL_PLAN|v1|%s|%s|%s|%s|%s|%s",
		p.AttemptIDHash,
		p.TransportKind,
		p.DeepLinkTarget,
		p.PayloadTitle,
		p.PayloadBody,
		p.CandidateHash,
	)
}

// ═══════════════════════════════════════════════════════════════════════════
// Rehearsal Receipt
// ═══════════════════════════════════════════════════════════════════════════

// RehearsalReceipt records the outcome of a rehearsal.
// CRITICAL: Hash-only. No raw identifiers.
type RehearsalReceipt struct {
	// Kind is the rehearsal kind.
	Kind RehearsalKind

	// Status is the rehearsal status.
	Status RehearsalStatus

	// RejectReason explains rejection (if status is rejected).
	RejectReason RehearsalRejectReason

	// PeriodKey is the day of the rehearsal.
	PeriodKey string

	// CircleIDHash identifies the circle.
	CircleIDHash string

	// CandidateHash is the hash of the candidate (empty if none).
	CandidateHash string

	// AttemptIDHash is the hash identifying this attempt.
	AttemptIDHash string

	// TransportKind is the transport used.
	TransportKind TransportKind

	// DeliveryBucket indicates if delivery occurred.
	DeliveryBucket DeliveryBucket

	// LatencyBucket is the latency measurement bucket.
	LatencyBucket LatencyBucket

	// ErrorClassBucket categorizes any error.
	ErrorClassBucket ErrorClassBucket

	// StatusHash is the hash of this receipt.
	StatusHash string

	// TimeBucket is the 15-minute interval of the attempt.
	TimeBucket string
}

// Validate checks if the receipt is valid.
func (r *RehearsalReceipt) Validate() error {
	if err := r.Kind.Validate(); err != nil {
		return err
	}
	if err := r.Status.Validate(); err != nil {
		return err
	}
	if err := r.RejectReason.Validate(); err != nil {
		return err
	}
	if r.CircleIDHash == "" {
		return fmt.Errorf("circle_id_hash is required")
	}
	if r.PeriodKey == "" {
		return fmt.Errorf("period_key is required")
	}
	// Validate status+reject reason consistency
	if r.Status == StatusRejected && r.RejectReason == RejectNone {
		return fmt.Errorf("rejected status requires a reject reason")
	}
	if r.Status != StatusRejected && r.RejectReason != RejectNone {
		return fmt.Errorf("reject reason only valid for rejected status")
	}
	return nil
}

// CanonicalString returns the canonical representation.
func (r *RehearsalReceipt) CanonicalString() string {
	return fmt.Sprintf("REHEARSAL_RECEIPT|v1|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s",
		r.Kind,
		r.Status,
		r.RejectReason,
		r.PeriodKey,
		r.CircleIDHash,
		r.CandidateHash,
		r.AttemptIDHash,
		r.TransportKind,
		r.DeliveryBucket,
		r.LatencyBucket,
		r.ErrorClassBucket,
		r.TimeBucket,
	)
}

// ComputeStatusHash computes the status hash for this receipt.
func (r *RehearsalReceipt) ComputeStatusHash() string {
	h := sha256.Sum256([]byte(r.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// ComputeAttemptIDHash computes a deterministic attempt ID.
// Uses circle+candidate+period to enable deduplication.
func ComputeAttemptIDHash(circleIDHash, candidateHash, periodKey string) string {
	dedupKey := fmt.Sprintf("REHEARSAL_ATTEMPT_DEDUP|v1|%s|%s|%s",
		circleIDHash,
		candidateHash,
		periodKey,
	)
	h := sha256.Sum256([]byte(dedupKey))
	return hex.EncodeToString(h[:16])
}

// ═══════════════════════════════════════════════════════════════════════════
// Rehearsal Proof Page
// ═══════════════════════════════════════════════════════════════════════════

// RehearsalProofPage represents the proof page data.
// CRITICAL: No raw identifiers. Abstract buckets only.
type RehearsalProofPage struct {
	// Title is the page title.
	Title string

	// Lines are calm copy lines explaining the state.
	Lines []string

	// ReceiptSummary is the abstract summary of the receipt.
	ReceiptSummary *ReceiptSummary

	// StatusHash is the hash of this proof page.
	StatusHash string

	// BackLink is the link to return.
	BackLink string

	// PeriodKey is the current period.
	PeriodKey string
}

// ReceiptSummary contains abstract receipt information for display.
type ReceiptSummary struct {
	// Status is the delivery status label.
	Status string

	// TransportLabel is the transport mechanism label.
	TransportLabel string

	// PeriodLabel is the period bucket label.
	PeriodLabel string

	// CandidateHashPrefix is the first 8 chars of candidate hash.
	CandidateHashPrefix string

	// AttemptHashPrefix is the first 8 chars of attempt hash.
	AttemptHashPrefix string

	// StatusHashPrefix is the first 8 chars of status hash.
	StatusHashPrefix string

	// RejectReasonLabel is the rejection reason label (if rejected).
	RejectReasonLabel string
}

// CanonicalString returns the canonical representation.
func (p *RehearsalProofPage) CanonicalString() string {
	return fmt.Sprintf("REHEARSAL_PROOF_PAGE|v1|%s|%s|%s",
		p.Title,
		p.PeriodKey,
		strings.Join(p.Lines, "|"),
	)
}

// ComputeStatusHash computes the status hash for this proof page.
func (p *RehearsalProofPage) ComputeStatusHash() string {
	h := sha256.Sum256([]byte(p.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// DefaultRehearsalProofPage returns a default proof page.
func DefaultRehearsalProofPage(periodKey string) *RehearsalProofPage {
	p := &RehearsalProofPage{
		Title: "Rehearsal delivery",
		Lines: []string{
			"No rehearsal attempted this period.",
			"Use the rehearsal page to test delivery.",
		},
		ReceiptSummary: nil,
		BackLink:       "/today",
		PeriodKey:      periodKey,
	}
	p.StatusHash = p.ComputeStatusHash()
	return p
}

// BuildProofPageFromReceipt builds a proof page from a receipt.
func BuildProofPageFromReceipt(receipt *RehearsalReceipt) *RehearsalProofPage {
	var lines []string
	var title string

	switch receipt.Status {
	case StatusDelivered:
		title = "Delivered, quietly."
		lines = []string{
			"A rehearsal push was delivered.",
			"Your boundaries are still being respected.",
		}
	case StatusAttempted:
		title = "Attempted, quietly."
		lines = []string{
			"A rehearsal push was attempted.",
			"Waiting for confirmation.",
		}
	case StatusFailed:
		title = "Delivery issue"
		lines = []string{
			"The rehearsal push encountered an issue.",
			"You can try again later.",
		}
	case StatusRejected:
		title = "Not sent."
		lines = []string{
			receipt.RejectReason.DisplayLabel(),
			"Your boundaries are still being respected.",
		}
	default:
		title = "Rehearsal"
		lines = []string{"Status unknown."}
	}

	summary := &ReceiptSummary{
		Status:         receipt.Status.DisplayLabel(),
		TransportLabel: receipt.TransportKind.DisplayLabel(),
		PeriodLabel:    "Today",
	}

	if receipt.CandidateHash != "" && len(receipt.CandidateHash) >= 8 {
		summary.CandidateHashPrefix = receipt.CandidateHash[:8]
	}
	if receipt.AttemptIDHash != "" && len(receipt.AttemptIDHash) >= 8 {
		summary.AttemptHashPrefix = receipt.AttemptIDHash[:8]
	}
	if receipt.StatusHash != "" && len(receipt.StatusHash) >= 8 {
		summary.StatusHashPrefix = receipt.StatusHash[:8]
	}
	if receipt.RejectReason != RejectNone {
		summary.RejectReasonLabel = receipt.RejectReason.DisplayLabel()
	}

	p := &RehearsalProofPage{
		Title:          title,
		Lines:          lines,
		ReceiptSummary: summary,
		BackLink:       "/today",
		PeriodKey:      receipt.PeriodKey,
	}
	p.StatusHash = p.ComputeStatusHash()
	return p
}

// ═══════════════════════════════════════════════════════════════════════════
// Rehearsal Ack
// ═══════════════════════════════════════════════════════════════════════════

// RehearsalAck records a proof dismissal acknowledgment.
// CRITICAL: Hash-only. No raw identifiers.
type RehearsalAck struct {
	// AckID is the unique identifier for this ack.
	AckID string

	// CircleIDHash identifies the circle.
	CircleIDHash string

	// PeriodKey is the daily bucket.
	PeriodKey string

	// AckBucket is when the ack was recorded.
	AckBucket string

	// StatusHash is the proof page status that was dismissed.
	StatusHash string
}

// CanonicalString returns the canonical representation.
func (a *RehearsalAck) CanonicalString() string {
	return fmt.Sprintf("REHEARSAL_ACK|v1|%s|%s|%s|%s|%s",
		a.AckID,
		a.CircleIDHash,
		a.PeriodKey,
		a.AckBucket,
		a.StatusHash,
	)
}

// ComputeAckID computes the ack ID.
func (a *RehearsalAck) ComputeAckID() string {
	input := fmt.Sprintf("%s|%s|%s", a.CircleIDHash, a.PeriodKey, a.AckBucket)
	h := sha256.Sum256([]byte(input))
	return hex.EncodeToString(h[:16])
}

// ═══════════════════════════════════════════════════════════════════════════
// Rehearse Page Data
// ═══════════════════════════════════════════════════════════════════════════

// RehearsePage represents the rehearse page data.
type RehearsePage struct {
	// Title is the page title.
	Title string

	// Lines are calm copy lines.
	Lines []string

	// PolicyAllowanceLabel is the current policy allowance.
	PolicyAllowanceLabel string

	// DeviceRegistered indicates if a device is registered.
	DeviceRegistered bool

	// CandidateAvailable indicates if a candidate is available.
	CandidateAvailable bool

	// DailyCapLabel is the delivery cap label.
	DailyCapLabel string

	// CanSend indicates if sending is currently possible.
	CanSend bool

	// BlockedReason explains why sending is blocked (if applicable).
	BlockedReason string

	// SendPath is the path to send.
	SendPath string

	// ProofPath is the path to view proof.
	ProofPath string

	// BackLink is the link to return.
	BackLink string
}

// DefaultRehearsePage returns a default rehearse page.
func DefaultRehearsePage() *RehearsePage {
	return &RehearsePage{
		Title: "Rehearsal delivery",
		Lines: []string{
			"Test that push delivery works.",
			"This sends one abstract notification.",
		},
		DailyCapLabel: fmt.Sprintf("up to %d/day", MaxDeliveriesPerDay),
		SendPath:      "/interrupts/rehearse/send",
		ProofPath:     "/proof/interrupts/rehearse",
		BackLink:      "/today",
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Forbidden Pattern Validation
// ═══════════════════════════════════════════════════════════════════════════

// ForbiddenPatterns are patterns that must never appear in rehearsal content.
var ForbiddenPatterns = []*regexp.Regexp{
	regexp.MustCompile(`@`),                                                  // email addresses
	regexp.MustCompile(`https?://`),                                          // URLs
	regexp.MustCompile(`[£$€]\s*\d`),                                         // currency amounts
	regexp.MustCompile(`\d{3}[-.\s]?\d{3}[-.\s]?\d{4}`),                      // phone numbers
	regexp.MustCompile(`(?i)(uber|deliveroo|amazon|paypal|invoice|receipt)`), // merchant tokens
}

// ContainsForbiddenPattern checks if a string contains any forbidden pattern.
func ContainsForbiddenPattern(s string) bool {
	for _, pattern := range ForbiddenPatterns {
		if pattern.MatchString(s) {
			return true
		}
	}
	return false
}

// ValidateNoForbiddenPatterns validates that all strings are safe.
func ValidateNoForbiddenPatterns(strings ...string) error {
	for _, s := range strings {
		if ContainsForbiddenPattern(s) {
			return fmt.Errorf("contains forbidden pattern: %s", s)
		}
	}
	return nil
}

// ═══════════════════════════════════════════════════════════════════════════
// Hash Prefix Helper
// ═══════════════════════════════════════════════════════════════════════════

// HashPrefix returns the first 8 characters of a hash for display.
// Returns empty string if hash is too short.
func HashPrefix(hash string) string {
	if len(hash) >= 8 {
		return hash[:8]
	}
	return ""
}
