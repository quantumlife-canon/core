// Package observerconsent provides domain types for Phase 55: Observer Consent Activation UI.
//
// This package provides explicit consent controls for observer capabilities via the existing
// Coverage Plan mechanism. Consent controls ONLY what may be observed. It NEVER changes what
// the system may do (interrupt, execute, deliver).
//
// CRITICAL: No time.Now() in this package - clock must be injected.
// CRITICAL: No goroutines in this package.
// CRITICAL: Canonical strings use pipe-delimited format, NOT JSON.
// CRITICAL: Period key is ALWAYS derived server-side - clients MUST NOT provide it.
//
// Reference: docs/ADR/ADR-0092-phase55-observer-consent-activation-ui.md
package observerconsent

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"

	domaincoverageplan "quantumlife/pkg/domain/coverageplan"
)

// ObserverKind represents the type of observer being consented to.
// Derived from capability, not user-provided.
type ObserverKind string

const (
	KindReceipt         ObserverKind = "receipt"
	KindCalendar        ObserverKind = "calendar"
	KindFinanceCommerce ObserverKind = "finance_commerce"
	KindCommerce        ObserverKind = "commerce"
	KindNotification    ObserverKind = "notification"
	KindDeviceHint      ObserverKind = "device_hint"
	KindUnknown         ObserverKind = "unknown"
)

// Validate checks if the ObserverKind is valid.
func (k ObserverKind) Validate() error {
	switch k {
	case KindReceipt, KindCalendar, KindFinanceCommerce, KindCommerce, KindNotification, KindDeviceHint, KindUnknown:
		return nil
	default:
		return fmt.Errorf("invalid ObserverKind: %s", k)
	}
}

// CanonicalString returns the canonical string representation.
func (k ObserverKind) CanonicalString() string {
	return string(k)
}

// String returns the string representation.
func (k ObserverKind) String() string {
	return string(k)
}

// DisplayLabel returns a user-friendly label.
func (k ObserverKind) DisplayLabel() string {
	switch k {
	case KindReceipt:
		return "Receipt scanning"
	case KindCalendar:
		return "Calendar observation"
	case KindFinanceCommerce:
		return "Finance observation"
	case KindCommerce:
		return "Commerce patterns"
	case KindNotification:
		return "Notification metadata"
	case KindDeviceHint:
		return "Device hints"
	default:
		return "Unknown"
	}
}

// AllObserverKinds returns all valid observer kinds in stable order.
func AllObserverKinds() []ObserverKind {
	return []ObserverKind{
		KindCalendar,
		KindCommerce,
		KindDeviceHint,
		KindFinanceCommerce,
		KindNotification,
		KindReceipt,
		KindUnknown,
	}
}

// ConsentAction represents the action being requested.
type ConsentAction string

const (
	ActionEnable  ConsentAction = "enable"
	ActionDisable ConsentAction = "disable"
)

// Validate checks if the ConsentAction is valid.
func (a ConsentAction) Validate() error {
	switch a {
	case ActionEnable, ActionDisable:
		return nil
	default:
		return fmt.Errorf("invalid ConsentAction: %s", a)
	}
}

// CanonicalString returns the canonical string representation.
func (a ConsentAction) CanonicalString() string {
	return string(a)
}

// String returns the string representation.
func (a ConsentAction) String() string {
	return string(a)
}

// ConsentResult represents the outcome of a consent request.
type ConsentResult string

const (
	ResultApplied  ConsentResult = "applied"
	ResultNoChange ConsentResult = "no_change"
	ResultRejected ConsentResult = "rejected"
)

// Validate checks if the ConsentResult is valid.
func (r ConsentResult) Validate() error {
	switch r {
	case ResultApplied, ResultNoChange, ResultRejected:
		return nil
	default:
		return fmt.Errorf("invalid ConsentResult: %s", r)
	}
}

// CanonicalString returns the canonical string representation.
func (r ConsentResult) CanonicalString() string {
	return string(r)
}

// String returns the string representation.
func (r ConsentResult) String() string {
	return string(r)
}

// RejectReason represents why a consent request was rejected.
type RejectReason string

const (
	RejectNone           RejectReason = ""
	RejectInvalid        RejectReason = "reject_invalid"
	RejectNotAllowlisted RejectReason = "reject_not_allowlisted"
	RejectMissingCircle  RejectReason = "reject_missing_circle"
	RejectPeriodInvalid  RejectReason = "reject_period_invalid"
)

// Validate checks if the RejectReason is valid.
func (r RejectReason) Validate() error {
	switch r {
	case RejectNone, RejectInvalid, RejectNotAllowlisted, RejectMissingCircle, RejectPeriodInvalid:
		return nil
	default:
		return fmt.Errorf("invalid RejectReason: %s", r)
	}
}

// CanonicalString returns the canonical string representation.
func (r RejectReason) CanonicalString() string {
	if r == RejectNone {
		return "none"
	}
	return string(r)
}

// String returns the string representation.
func (r RejectReason) String() string {
	return string(r)
}

// ConsentAckKind represents the type of proof acknowledgment.
type ConsentAckKind string

const (
	AckDismissed ConsentAckKind = "ack_dismissed"
)

// Validate checks if the ConsentAckKind is valid.
func (k ConsentAckKind) Validate() error {
	switch k {
	case AckDismissed:
		return nil
	default:
		return fmt.Errorf("invalid ConsentAckKind: %s", k)
	}
}

// CanonicalString returns the canonical string representation.
func (k ConsentAckKind) CanonicalString() string {
	return string(k)
}

// String returns the string representation.
func (k ConsentAckKind) String() string {
	return string(k)
}

// ObserverConsentRequest represents a request to enable/disable an observer capability.
type ObserverConsentRequest struct {
	CircleIDHash string                            // SHA256 hash of circle ID
	Action       ConsentAction                     // enable or disable
	Capability   domaincoverageplan.CoverageCapability // The capability to toggle
}

// Validate checks if the request is valid.
func (r ObserverConsentRequest) Validate() error {
	if r.CircleIDHash == "" {
		return errors.New("CircleIDHash is required")
	}
	if err := r.Action.Validate(); err != nil {
		return err
	}
	if err := r.Capability.Validate(); err != nil {
		return err
	}
	return nil
}

// CanonicalStringV1 returns a pipe-delimited canonical string.
func (r ObserverConsentRequest) CanonicalStringV1() string {
	return fmt.Sprintf("%s|%s|%s",
		r.CircleIDHash,
		r.Action.CanonicalString(),
		r.Capability.CanonicalString(),
	)
}

// Hash returns the SHA256 hash of the canonical string.
func (r ObserverConsentRequest) Hash() string {
	return HashString(r.CanonicalStringV1())
}

// ObserverConsentReceipt represents an immutable record of a consent action.
type ObserverConsentReceipt struct {
	PeriodKey    string                            // Server-derived period (YYYY-MM-DD)
	CircleIDHash string                            // SHA256 hash of circle ID
	Action       ConsentAction                     // enable or disable
	Capability   domaincoverageplan.CoverageCapability // The capability that was toggled
	Kind         ObserverKind                      // Derived from capability
	Result       ConsentResult                     // applied, no_change, or rejected
	RejectReason RejectReason                      // Why rejected (if applicable)
	ReceiptHash  string                            // SHA256 hash of canonical receipt string
}

// Validate checks if the receipt is valid.
func (r ObserverConsentReceipt) Validate() error {
	if r.PeriodKey == "" {
		return errors.New("PeriodKey is required")
	}
	if r.CircleIDHash == "" {
		return errors.New("CircleIDHash is required")
	}
	if err := r.Action.Validate(); err != nil {
		return err
	}
	if err := r.Capability.Validate(); err != nil {
		// Allow validation to pass for rejected receipts with invalid capability
		if r.Result != ResultRejected {
			return err
		}
	}
	if err := r.Kind.Validate(); err != nil {
		return err
	}
	if err := r.Result.Validate(); err != nil {
		return err
	}
	if err := r.RejectReason.Validate(); err != nil {
		return err
	}
	return nil
}

// CanonicalStringV1 returns a pipe-delimited canonical string.
func (r ObserverConsentReceipt) CanonicalStringV1() string {
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s",
		r.PeriodKey,
		r.CircleIDHash,
		r.Action.CanonicalString(),
		r.Capability.CanonicalString(),
		r.Kind.CanonicalString(),
		r.Result.CanonicalString(),
		r.RejectReason.CanonicalString(),
	)
}

// ComputeReceiptHash computes the SHA256 hash of the canonical receipt string.
func (r ObserverConsentReceipt) ComputeReceiptHash() string {
	return HashString(r.CanonicalStringV1())
}

// Hash returns the receipt hash.
func (r ObserverConsentReceipt) Hash() string {
	if r.ReceiptHash != "" {
		return r.ReceiptHash
	}
	return r.ComputeReceiptHash()
}

// DedupKey returns the deduplication key for this receipt.
func (r ObserverConsentReceipt) DedupKey() string {
	return fmt.Sprintf("%s|%s|%s|%s",
		r.PeriodKey,
		r.CircleIDHash,
		r.Action.CanonicalString(),
		r.Capability.CanonicalString(),
	)
}

// ObserverConsentAck represents an acknowledgment of the consent proof page.
type ObserverConsentAck struct {
	PeriodKey    string         // Server-derived period (YYYY-MM-DD)
	CircleIDHash string         // SHA256 hash of circle ID
	AckKind      ConsentAckKind // Type of acknowledgment
	StatusHash   string         // SHA256 hash of ack state
}

// Validate checks if the ack is valid.
func (a ObserverConsentAck) Validate() error {
	if a.PeriodKey == "" {
		return errors.New("PeriodKey is required")
	}
	if a.CircleIDHash == "" {
		return errors.New("CircleIDHash is required")
	}
	if err := a.AckKind.Validate(); err != nil {
		return err
	}
	return nil
}

// CanonicalStringV1 returns a pipe-delimited canonical string.
func (a ObserverConsentAck) CanonicalStringV1() string {
	return fmt.Sprintf("%s|%s|%s",
		a.PeriodKey,
		a.CircleIDHash,
		a.AckKind.CanonicalString(),
	)
}

// ComputeStatusHash computes the SHA256 hash of the canonical ack string.
func (a ObserverConsentAck) ComputeStatusHash() string {
	return HashString(a.CanonicalStringV1())
}

// Hash returns the status hash.
func (a ObserverConsentAck) Hash() string {
	if a.StatusHash != "" {
		return a.StatusHash
	}
	return a.ComputeStatusHash()
}

// DedupKey returns the deduplication key for this ack.
func (a ObserverConsentAck) DedupKey() string {
	return fmt.Sprintf("%s|%s|%s",
		a.PeriodKey,
		a.CircleIDHash,
		a.AckKind.CanonicalString(),
	)
}

// ObserverCapabilityStatus represents the UI status of an observer capability.
type ObserverCapabilityStatus struct {
	Capability  domaincoverageplan.CoverageCapability
	Kind        ObserverKind
	Label       string
	Description string
	Enabled     bool
	Allowlisted bool
}

// ObserverSettingsPage represents the settings page model.
type ObserverSettingsPage struct {
	Title        string
	Description  string
	Capabilities []ObserverCapabilityStatus
	StatusHash   string
}

// ObserverProofPage represents the proof page model.
type ObserverProofPage struct {
	Title       string
	Lines       []string
	Receipts    []ObserverConsentReceipt
	StatusHash  string
	MaxReceipts int
}

// HashString computes SHA256 hash of a string and returns hex-encoded result.
func HashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// HashCircleID computes SHA256 hash of a circle ID.
func HashCircleID(circleID string) string {
	return HashString(circleID)
}

// KindFromCapability derives the ObserverKind from a CoverageCapability.
func KindFromCapability(cap domaincoverageplan.CoverageCapability) ObserverKind {
	switch cap {
	case domaincoverageplan.CapReceiptObserver:
		return KindReceipt
	case domaincoverageplan.CapCommerceObserver:
		return KindCommerce
	case domaincoverageplan.CapFinanceCommerceObserver:
		return KindFinanceCommerce
	case domaincoverageplan.CapNotificationMetadata:
		return KindNotification
	default:
		return KindUnknown
	}
}

// AllowlistedCapabilities returns the list of capabilities that can be toggled via Phase 55.
// CRITICAL: Only observer/measurement capabilities. Never action/execution capabilities.
func AllowlistedCapabilities() []domaincoverageplan.CoverageCapability {
	return []domaincoverageplan.CoverageCapability{
		domaincoverageplan.CapReceiptObserver,
		domaincoverageplan.CapCommerceObserver,
		domaincoverageplan.CapFinanceCommerceObserver,
		domaincoverageplan.CapNotificationMetadata,
	}
}

// IsAllowlisted checks if a capability can be toggled via Phase 55.
func IsAllowlisted(cap domaincoverageplan.CoverageCapability) bool {
	for _, allowed := range AllowlistedCapabilities() {
		if allowed == cap {
			return true
		}
	}
	return false
}

// NormalizeCapabilities sorts and deduplicates capabilities.
func NormalizeCapabilities(caps []domaincoverageplan.CoverageCapability) []domaincoverageplan.CoverageCapability {
	if len(caps) == 0 {
		return []domaincoverageplan.CoverageCapability{}
	}

	// Deduplicate
	seen := make(map[domaincoverageplan.CoverageCapability]bool)
	result := make([]domaincoverageplan.CoverageCapability, 0, len(caps))
	for _, cap := range caps {
		if !seen[cap] {
			seen[cap] = true
			result = append(result, cap)
		}
	}

	// Sort lexicographically
	sort.Slice(result, func(i, j int) bool {
		return result[i] < result[j]
	})

	return result
}

// ForbiddenClientFields returns the list of field names that clients MUST NOT provide.
func ForbiddenClientFields() []string {
	return []string{
		"period",
		"periodKey",
		"period_key",
		"periodKeyHash",
		"email",
		"url",
		"name",
		"vendor",
		"token",
		"device",
	}
}

// Bounded retention constants.
const (
	MaxConsentRecords       = 200
	MaxConsentRetentionDays = 30
	MaxAckRecords           = 200
	MaxAckRetentionDays     = 30
	MaxProofDisplayReceipts = 12
)
