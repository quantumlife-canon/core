// Package pushtransport defines the Phase 35 Push Transport domain types.
//
// This package provides abstract interrupt delivery via push transport.
// It is transport-only — it does NOT make delivery decisions.
// Decisions are made by Phase 33 (policy) and Phase 34 (preview).
//
// CRITICAL INVARIANTS:
//   - Transport-only. No new decision logic.
//   - Abstract payload only. No identifiers in push body.
//   - Hash-only storage. Token is hashed before persistence.
//   - No goroutines. No time.Now().
//   - Commerce never interrupts.
//
// Reference: docs/ADR/ADR-0071-phase35-push-transport-abstract-interrupt-delivery.md
package pushtransport

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// ═══════════════════════════════════════════════════════════════════════════
// Push Provider Kind
// ═══════════════════════════════════════════════════════════════════════════

// PushProviderKind identifies the transport mechanism.
type PushProviderKind string

const (
	ProviderAPNs    PushProviderKind = "apns"
	ProviderWebhook PushProviderKind = "webhook"
	ProviderStub    PushProviderKind = "stub"
)

// ValidProviderKinds is the set of valid provider kinds.
var ValidProviderKinds = map[PushProviderKind]bool{
	ProviderAPNs:    true,
	ProviderWebhook: true,
	ProviderStub:    true,
}

// Validate checks if the provider kind is valid.
func (p PushProviderKind) Validate() error {
	if !ValidProviderKinds[p] {
		return fmt.Errorf("invalid provider kind: %s", p)
	}
	return nil
}

// String returns the string representation.
func (p PushProviderKind) String() string {
	return string(p)
}

// DisplayLabel returns a human-friendly label.
func (p PushProviderKind) DisplayLabel() string {
	switch p {
	case ProviderAPNs:
		return "Apple Push"
	case ProviderWebhook:
		return "Webhook"
	case ProviderStub:
		return "Test Mode"
	default:
		return "Unknown"
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Push Token Kind
// ═══════════════════════════════════════════════════════════════════════════

// PushTokenKind identifies the type of token.
type PushTokenKind string

const (
	TokenKindDeviceToken PushTokenKind = "device_token"
	TokenKindEndpointURL PushTokenKind = "endpoint_url"
)

// ValidTokenKinds is the set of valid token kinds.
var ValidTokenKinds = map[PushTokenKind]bool{
	TokenKindDeviceToken: true,
	TokenKindEndpointURL: true,
}

// Validate checks if the token kind is valid.
func (t PushTokenKind) Validate() error {
	if !ValidTokenKinds[t] {
		return fmt.Errorf("invalid token kind: %s", t)
	}
	return nil
}

// String returns the string representation.
func (t PushTokenKind) String() string {
	return string(t)
}

// ═══════════════════════════════════════════════════════════════════════════
// Attempt Status
// ═══════════════════════════════════════════════════════════════════════════

// AttemptStatus represents the outcome of a delivery attempt.
type AttemptStatus string

const (
	StatusSent    AttemptStatus = "sent"
	StatusSkipped AttemptStatus = "skipped"
	StatusFailed  AttemptStatus = "failed"
)

// ValidAttemptStatuses is the set of valid statuses.
var ValidAttemptStatuses = map[AttemptStatus]bool{
	StatusSent:    true,
	StatusSkipped: true,
	StatusFailed:  true,
}

// Validate checks if the status is valid.
func (s AttemptStatus) Validate() error {
	if !ValidAttemptStatuses[s] {
		return fmt.Errorf("invalid attempt status: %s", s)
	}
	return nil
}

// String returns the string representation.
func (s AttemptStatus) String() string {
	return string(s)
}

// DisplayLabel returns a human-friendly label.
func (s AttemptStatus) DisplayLabel() string {
	switch s {
	case StatusSent:
		return "Delivered"
	case StatusSkipped:
		return "Not delivered"
	case StatusFailed:
		return "Delivery failed"
	default:
		return "Unknown"
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Failure Bucket
// ═══════════════════════════════════════════════════════════════════════════

// FailureBucket explains why a delivery was skipped or failed.
type FailureBucket string

const (
	FailureNone           FailureBucket = "none"
	FailureNotConfigured  FailureBucket = "not_configured"
	FailureNotPermitted   FailureBucket = "not_permitted"
	FailureCapReached     FailureBucket = "cap_reached"
	FailureTransportError FailureBucket = "transport_error"
	FailureNoCandidate    FailureBucket = "no_candidate"
)

// ValidFailureBuckets is the set of valid failure buckets.
var ValidFailureBuckets = map[FailureBucket]bool{
	FailureNone:           true,
	FailureNotConfigured:  true,
	FailureNotPermitted:   true,
	FailureCapReached:     true,
	FailureTransportError: true,
	FailureNoCandidate:    true,
}

// Validate checks if the failure bucket is valid.
func (f FailureBucket) Validate() error {
	if !ValidFailureBuckets[f] {
		return fmt.Errorf("invalid failure bucket: %s", f)
	}
	return nil
}

// String returns the string representation.
func (f FailureBucket) String() string {
	return string(f)
}

// DisplayLabel returns a human-friendly label.
func (f FailureBucket) DisplayLabel() string {
	switch f {
	case FailureNone:
		return ""
	case FailureNotConfigured:
		return "No device registered"
	case FailureNotPermitted:
		return "Policy does not permit"
	case FailureCapReached:
		return "Daily limit reached"
	case FailureTransportError:
		return "Delivery issue"
	case FailureNoCandidate:
		return "Nothing to deliver"
	default:
		return "Unknown reason"
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Push Registration
// ═══════════════════════════════════════════════════════════════════════════

// PushRegistration stores device registration for push delivery.
// CRITICAL: TokenHash is the SHA256 of the raw token. Raw token is NEVER stored.
type PushRegistration struct {
	// RegistrationID is the unique identifier (computed from canonical string).
	RegistrationID string

	// CircleIDHash identifies the circle (hashed).
	CircleIDHash string

	// DeviceFingerprintHash identifies the device (from Phase 30A).
	DeviceFingerprintHash string

	// ProviderKind is the push provider (apns, webhook, stub).
	ProviderKind PushProviderKind

	// TokenKind is the type of token (device_token, endpoint_url).
	TokenKind PushTokenKind

	// TokenHash is SHA256 of the raw token. NEVER the raw token itself.
	TokenHash string

	// CreatedPeriodKey is the day this registration was created.
	CreatedPeriodKey string

	// Enabled indicates if push is enabled for this registration.
	Enabled bool
}

// CanonicalString returns the canonical pipe-delimited representation.
func (r *PushRegistration) CanonicalString() string {
	return fmt.Sprintf("PUSH_REG|v1|%s|%s|%s|%s|%s|%s|%t",
		r.CircleIDHash,
		r.DeviceFingerprintHash,
		r.ProviderKind,
		r.TokenKind,
		r.TokenHash,
		r.CreatedPeriodKey,
		r.Enabled,
	)
}

// ComputeRegistrationID computes a deterministic ID from the canonical string.
func (r *PushRegistration) ComputeRegistrationID() string {
	canonical := r.CanonicalString()
	h := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(h[:16])
}

// Validate checks if the registration is valid.
func (r *PushRegistration) Validate() error {
	if r.CircleIDHash == "" {
		return fmt.Errorf("circle_id_hash is required")
	}
	if r.TokenHash == "" {
		return fmt.Errorf("token_hash is required")
	}
	if err := r.ProviderKind.Validate(); err != nil {
		return err
	}
	if err := r.TokenKind.Validate(); err != nil {
		return err
	}
	return nil
}

// HashToken computes SHA256 hash of a raw token.
// This MUST be called before storing; raw token MUST NOT be persisted.
func HashToken(rawToken string) string {
	h := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(h[:])
}

// ═══════════════════════════════════════════════════════════════════════════
// Push Delivery Attempt
// ═══════════════════════════════════════════════════════════════════════════

// PushDeliveryAttempt records a delivery attempt.
type PushDeliveryAttempt struct {
	// AttemptID is the unique identifier (computed from canonical string).
	AttemptID string

	// CircleIDHash identifies the circle.
	CircleIDHash string

	// CandidateHash identifies the interrupt candidate being delivered.
	CandidateHash string

	// ProviderKind is the push provider used.
	ProviderKind PushProviderKind

	// Status is the outcome (sent, skipped, failed).
	Status AttemptStatus

	// FailureBucket explains why skipped or failed.
	FailureBucket FailureBucket

	// StatusHash is a deterministic hash of the attempt state.
	StatusHash string

	// PeriodKey is the day of the attempt.
	PeriodKey string

	// AttemptBucket is the time bucket (15-min interval).
	AttemptBucket string
}

// CanonicalString returns the canonical pipe-delimited representation.
func (a *PushDeliveryAttempt) CanonicalString() string {
	return fmt.Sprintf("PUSH_ATTEMPT|v1|%s|%s|%s|%s|%s|%s|%s",
		a.CircleIDHash,
		a.CandidateHash,
		a.ProviderKind,
		a.Status,
		a.FailureBucket,
		a.PeriodKey,
		a.AttemptBucket,
	)
}

// ComputeAttemptID computes a deterministic ID.
// Uses circle+candidate+period to enable deduplication.
func (a *PushDeliveryAttempt) ComputeAttemptID() string {
	// Dedup key: same circle+candidate+period = same attempt ID
	dedupKey := fmt.Sprintf("PUSH_ATTEMPT_DEDUP|v1|%s|%s|%s",
		a.CircleIDHash,
		a.CandidateHash,
		a.PeriodKey,
	)
	h := sha256.Sum256([]byte(dedupKey))
	return hex.EncodeToString(h[:16])
}

// ComputeStatusHash computes a hash of the full attempt state.
func (a *PushDeliveryAttempt) ComputeStatusHash() string {
	canonical := a.CanonicalString()
	h := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(h[:16])
}

// Validate checks if the attempt is valid.
func (a *PushDeliveryAttempt) Validate() error {
	if a.CircleIDHash == "" {
		return fmt.Errorf("circle_id_hash is required")
	}
	if a.PeriodKey == "" {
		return fmt.Errorf("period_key is required")
	}
	if err := a.Status.Validate(); err != nil {
		return err
	}
	if err := a.FailureBucket.Validate(); err != nil {
		return err
	}
	return nil
}

// ═══════════════════════════════════════════════════════════════════════════
// Push Delivery Receipt Page
// ═══════════════════════════════════════════════════════════════════════════

// PushDeliveryReceiptPage represents the proof page for push delivery.
type PushDeliveryReceiptPage struct {
	// Title is the page title.
	Title string

	// Subtitle provides context.
	Subtitle string

	// Lines are calm copy lines explaining the state.
	Lines []string

	// Status is the delivery status.
	Status AttemptStatus

	// FailureBucket explains why not delivered (if applicable).
	FailureBucket FailureBucket

	// EvidenceHashes are opaque hashes for verification.
	EvidenceHashes []string

	// StatusHash is a deterministic hash of the page state.
	StatusHash string

	// PeriodKey is the day.
	PeriodKey string

	// BackLink is the link to return.
	BackLink string
}

// CanonicalString returns the canonical pipe-delimited representation.
func (p *PushDeliveryReceiptPage) CanonicalString() string {
	var b strings.Builder
	b.WriteString("PUSH_PROOF|v1|")
	b.WriteString(p.Title)
	b.WriteString("|")
	b.WriteString(string(p.Status))
	b.WriteString("|")
	b.WriteString(string(p.FailureBucket))
	b.WriteString("|")
	b.WriteString(p.PeriodKey)
	b.WriteString("|")
	b.WriteString(strings.Join(p.EvidenceHashes, ","))
	return b.String()
}

// ComputeStatusHash computes a deterministic hash of the page state.
func (p *PushDeliveryReceiptPage) ComputeStatusHash() string {
	canonical := p.CanonicalString()
	h := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(h[:16])
}

// DefaultPushProofPage returns a default proof page.
func DefaultPushProofPage(periodKey string) *PushDeliveryReceiptPage {
	page := &PushDeliveryReceiptPage{
		Title:          "Push Delivery",
		Subtitle:       "Abstract delivery status.",
		Lines:          []string{"No delivery attempt this period."},
		Status:         StatusSkipped,
		FailureBucket:  FailureNoCandidate,
		EvidenceHashes: []string{},
		PeriodKey:      periodKey,
		BackLink:       "/today",
	}
	page.StatusHash = page.ComputeStatusHash()
	return page
}

// ═══════════════════════════════════════════════════════════════════════════
// Transport Request (for engine → transport handoff)
// ═══════════════════════════════════════════════════════════════════════════

// TransportRequest contains the data needed to execute a push delivery.
// This is returned by the engine; actual network call is done in cmd/.
type TransportRequest struct {
	// ProviderKind is the transport to use.
	ProviderKind PushProviderKind

	// TokenHash is the hashed token (for logging/audit).
	TokenHash string

	// RawToken is the actual token needed for delivery.
	// CRITICAL: This is passed through but NEVER persisted.
	RawToken string

	// Endpoint is the URL for webhook transport.
	Endpoint string

	// Payload is the push payload.
	Payload TransportPayload

	// AttemptID links to the attempt record.
	AttemptID string
}

// TransportPayload is the abstract push payload.
// CRITICAL: Body MUST be a constant string with no identifiers.
type TransportPayload struct {
	// Title is always "QuantumLife".
	Title string

	// Body is always "Something needs you. Open QuantumLife."
	Body string

	// StatusHash is an opaque hash for correlation.
	StatusHash string
}

// DefaultTransportPayload returns the standard abstract payload.
func DefaultTransportPayload(statusHash string) TransportPayload {
	return TransportPayload{
		Title:      PushTitle,
		Body:       PushBody,
		StatusHash: statusHash,
	}
}

// Push payload constants.
// CRITICAL: These are constant literals. No customization allowed.
const (
	PushTitle = "QuantumLife"
	PushBody  = "Something needs you. Open QuantumLife."
)

// TransportResult contains the outcome of a transport send.
type TransportResult struct {
	// Success indicates if the transport succeeded.
	Success bool

	// ErrorBucket categorizes any error.
	ErrorBucket FailureBucket

	// ResponseHash is a hash of the provider response (for audit).
	ResponseHash string
}

// ═══════════════════════════════════════════════════════════════════════════
// Delivery Eligibility Input
// ═══════════════════════════════════════════════════════════════════════════

// DeliveryEligibilityInput contains all inputs for computing delivery eligibility.
type DeliveryEligibilityInput struct {
	// CircleIDHash identifies the circle.
	CircleIDHash string

	// PeriodKey is the current day.
	PeriodKey string

	// TimeBucket is the current 15-minute interval.
	TimeBucket string

	// HasCandidate indicates if there's a permitted interrupt candidate.
	HasCandidate bool

	// CandidateHash is the hash of the candidate (from Phase 34).
	CandidateHash string

	// PolicyEnabled indicates if interrupt policy is enabled (Phase 33).
	PolicyEnabled bool

	// PushEnabled indicates if push transport is enabled in registration.
	PushEnabled bool

	// HasRegistration indicates if a device is registered.
	HasRegistration bool

	// Registration is the active push registration (if any).
	Registration *PushRegistration

	// DailyAttemptCount is the number of sent attempts today.
	DailyAttemptCount int

	// MaxPerDay is the daily cap (typically 2).
	MaxPerDay int
}

// ═══════════════════════════════════════════════════════════════════════════
// Constants
// ═══════════════════════════════════════════════════════════════════════════

// DefaultMaxPushPerDay is the default daily cap for push deliveries.
const DefaultMaxPushPerDay = 2
