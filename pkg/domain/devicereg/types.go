// Package devicereg provides domain types for Phase 37: Device Registration + Deep-Link.
//
// CRITICAL INVARIANTS:
// - stdlib only
// - No time.Now() - clock injection only
// - No goroutines
// - Hash-only storage (raw tokens ONLY in sealed secret boundary)
// - No identifiers in deep links
// - Bounded retention (max 200 records OR 30 days)
//
// Reference: docs/ADR/ADR-0074-phase37-device-registration-deeplink.md
package devicereg

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// DevicePlatform identifies the device operating system.
type DevicePlatform string

const (
	// DevicePlatformIOS represents iOS devices.
	DevicePlatformIOS DevicePlatform = "ios"
)

// Validate checks if the platform is valid.
func (p DevicePlatform) Validate() error {
	switch p {
	case DevicePlatformIOS:
		return nil
	default:
		return fmt.Errorf("invalid device platform: %s", p)
	}
}

// CanonicalString returns a deterministic representation.
func (p DevicePlatform) CanonicalString() string {
	return fmt.Sprintf("v1|platform|%s", p)
}

// DeviceRegState indicates registration state.
type DeviceRegState string

const (
	// DeviceRegStateRegistered indicates active registration.
	DeviceRegStateRegistered DeviceRegState = "registered"
	// DeviceRegStateRevoked indicates revoked registration.
	DeviceRegStateRevoked DeviceRegState = "revoked"
)

// Validate checks if the state is valid.
func (s DeviceRegState) Validate() error {
	switch s {
	case DeviceRegStateRegistered, DeviceRegStateRevoked:
		return nil
	default:
		return fmt.Errorf("invalid device reg state: %s", s)
	}
}

// CanonicalString returns a deterministic representation.
func (s DeviceRegState) CanonicalString() string {
	return fmt.Sprintf("v1|reg_state|%s", s)
}

// DeepLinkTarget enumerates valid landing screens.
type DeepLinkTarget string

const (
	// DeepLinkTargetInterrupts lands on /interrupts/preview.
	DeepLinkTargetInterrupts DeepLinkTarget = "interrupts"
	// DeepLinkTargetTrust lands on /trust/action/receipt.
	DeepLinkTargetTrust DeepLinkTarget = "trust"
	// DeepLinkTargetShadow lands on /shadow/receipt.
	DeepLinkTargetShadow DeepLinkTarget = "shadow"
	// DeepLinkTargetReality lands on /reality.
	DeepLinkTargetReality DeepLinkTarget = "reality"
	// DeepLinkTargetToday lands on /today (default).
	DeepLinkTargetToday DeepLinkTarget = "today"
)

// Validate checks if the target is valid.
func (t DeepLinkTarget) Validate() error {
	switch t {
	case DeepLinkTargetInterrupts, DeepLinkTargetTrust, DeepLinkTargetShadow,
		DeepLinkTargetReality, DeepLinkTargetToday:
		return nil
	default:
		return fmt.Errorf("invalid deep link target: %s", t)
	}
}

// ToPath returns the web path for this target.
func (t DeepLinkTarget) ToPath() string {
	switch t {
	case DeepLinkTargetInterrupts:
		return "/interrupts/preview"
	case DeepLinkTargetTrust:
		return "/trust/action/receipt"
	case DeepLinkTargetShadow:
		return "/shadow/receipt"
	case DeepLinkTargetReality:
		return "/reality"
	case DeepLinkTargetToday:
		return "/today"
	default:
		return "/today"
	}
}

// CanonicalString returns a deterministic representation.
func (t DeepLinkTarget) CanonicalString() string {
	return fmt.Sprintf("v1|target|%s", t)
}

// AllDeepLinkTargets returns all valid targets.
func AllDeepLinkTargets() []DeepLinkTarget {
	return []DeepLinkTarget{
		DeepLinkTargetInterrupts,
		DeepLinkTargetTrust,
		DeepLinkTargetShadow,
		DeepLinkTargetReality,
		DeepLinkTargetToday,
	}
}

// DeviceRegistrationReceipt is the hash-only record of a registration.
// CRITICAL: Raw token NEVER stored here. Only hashes.
type DeviceRegistrationReceipt struct {
	// PeriodKey is the day bucket (YYYY-MM-DD).
	PeriodKey string

	// Platform is the device platform.
	Platform DevicePlatform

	// CircleIDHash is SHA256 of circle ID (hash-only).
	CircleIDHash string

	// TokenHash is SHA256 of the raw device token.
	TokenHash string

	// SealedRefHash is SHA256 of the sealed reference.
	SealedRefHash string

	// State is the registration state.
	State DeviceRegState

	// StatusHash is deterministic hash of this receipt.
	StatusHash string

	// ReceiptID is the unique identifier for this receipt.
	ReceiptID string
}

// Validate checks all fields are present and valid.
func (r *DeviceRegistrationReceipt) Validate() error {
	if r.PeriodKey == "" {
		return errors.New("period_key is required")
	}
	if err := r.Platform.Validate(); err != nil {
		return fmt.Errorf("invalid platform: %w", err)
	}
	if r.CircleIDHash == "" {
		return errors.New("circle_id_hash is required")
	}
	if r.TokenHash == "" {
		return errors.New("token_hash is required")
	}
	if r.SealedRefHash == "" {
		return errors.New("sealed_ref_hash is required")
	}
	if err := r.State.Validate(); err != nil {
		return fmt.Errorf("invalid state: %w", err)
	}

	// Check for forbidden patterns in hashes
	if err := CheckForbiddenPatterns(r.CircleIDHash); err != nil {
		return fmt.Errorf("forbidden pattern in circle_id_hash: %w", err)
	}
	if err := CheckForbiddenPatterns(r.TokenHash); err != nil {
		return fmt.Errorf("forbidden pattern in token_hash: %w", err)
	}

	return nil
}

// CanonicalString returns a deterministic, pipe-delimited representation.
func (r *DeviceRegistrationReceipt) CanonicalString() string {
	return fmt.Sprintf("DEVICE_REG_RECEIPT|v1|%s|%s|%s|%s|%s|%s",
		r.PeriodKey,
		r.Platform,
		r.CircleIDHash,
		r.TokenHash,
		r.SealedRefHash,
		r.State,
	)
}

// ComputeStatusHash computes the deterministic status hash.
func (r *DeviceRegistrationReceipt) ComputeStatusHash() string {
	hash := sha256.Sum256([]byte(r.CanonicalString()))
	return hex.EncodeToString(hash[:16])
}

// ComputeReceiptID computes the unique receipt ID.
func (r *DeviceRegistrationReceipt) ComputeReceiptID() string {
	canonical := fmt.Sprintf("RECEIPT_ID|%s|%s|%s", r.PeriodKey, r.CircleIDHash, r.TokenHash)
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:16])
}

// TokenHashPrefix returns first 8 chars of token hash for display.
func (r *DeviceRegistrationReceipt) TokenHashPrefix() string {
	if len(r.TokenHash) < 8 {
		return r.TokenHash
	}
	return r.TokenHash[:8]
}

// StatusHashPrefix returns first 8 chars of status hash for display.
func (r *DeviceRegistrationReceipt) StatusHashPrefix() string {
	if len(r.StatusHash) < 8 {
		return r.StatusHash
	}
	return r.StatusHash[:8]
}

// DeviceRegistrationProofPage is the UI model for /proof/device.
type DeviceRegistrationProofPage struct {
	// Title is the page title.
	Title string

	// Lines are calm description lines.
	Lines []string

	// TokenHashPrefix is first 8 chars for display.
	TokenHashPrefix string

	// StatusHashPrefix is first 8 chars for display.
	StatusHashPrefix string

	// DismissAvailable indicates if dismiss action is available.
	DismissAvailable bool

	// StatusHash is the deterministic hash of this page.
	StatusHash string

	// HasRegistration indicates if there's an active registration.
	HasRegistration bool

	// Platform shows which platform is registered.
	Platform DevicePlatform
}

// ComputeStatusHash computes the status hash for the proof page.
func (p *DeviceRegistrationProofPage) ComputeStatusHash() string {
	canonical := fmt.Sprintf("PROOF_PAGE|v1|%s|%s|%s|%t",
		p.Title,
		p.TokenHashPrefix,
		p.StatusHashPrefix,
		p.HasRegistration,
	)
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:16])
}

// DevicesPage is the UI model for /devices.
type DevicesPage struct {
	// Title is the page title.
	Title string

	// HasiOSRegistration indicates if iOS is registered.
	HasiOSRegistration bool

	// RegistrationMagnitude is abstract count.
	RegistrationMagnitude MagnitudeBucket

	// StatusHash is the deterministic hash.
	StatusHash string
}

// ComputeStatusHash computes the status hash.
func (p *DevicesPage) ComputeStatusHash() string {
	canonical := fmt.Sprintf("DEVICES_PAGE|v1|%s|%t|%s",
		p.Title,
		p.HasiOSRegistration,
		p.RegistrationMagnitude,
	)
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:16])
}

// MagnitudeBucket represents abstract quantities.
type MagnitudeBucket string

const (
	// MagnitudeNothing means zero.
	MagnitudeNothing MagnitudeBucket = "nothing"
	// MagnitudeAFew means 1-3.
	MagnitudeAFew MagnitudeBucket = "a_few"
	// MagnitudeSeveral means 4+.
	MagnitudeSeveral MagnitudeBucket = "several"
)

// MagnitudeFromCount converts count to magnitude bucket.
func MagnitudeFromCount(count int) MagnitudeBucket {
	switch {
	case count == 0:
		return MagnitudeNothing
	case count <= 3:
		return MagnitudeAFew
	default:
		return MagnitudeSeveral
	}
}

// DeviceRegisterCue is the whisper cue for device registration.
type DeviceRegisterCue struct {
	// Available indicates if the cue should be shown.
	Available bool

	// Text is the cue text.
	Text string

	// LinkPath is the link destination.
	LinkPath string

	// Priority is the cue priority (higher = lower priority, shown later).
	Priority int
}

// Constants for device registration cue.
const (
	// DefaultDeviceRegCueText is the default cue text.
	DefaultDeviceRegCueText = "If you wanted to, you could register a device."

	// DefaultDeviceRegCuePath is the cue link destination.
	DefaultDeviceRegCuePath = "/devices"

	// DefaultDeviceRegCuePriority is the cue priority (lowest).
	// This should be higher than all other cue priorities.
	DefaultDeviceRegCuePriority = 100
)

// DeepLinkComputeInput contains inputs for computing deep link target.
type DeepLinkComputeInput struct {
	// InterruptPreviewAvailable indicates if interrupt preview exists.
	InterruptPreviewAvailable bool
	// InterruptPreviewAcked indicates if preview was acknowledged.
	InterruptPreviewAcked bool

	// TrustActionReceiptAvailable indicates if trust receipt exists.
	TrustActionReceiptAvailable bool
	// TrustActionReceiptDismissed indicates if receipt was dismissed.
	TrustActionReceiptDismissed bool

	// ShadowReceiptCueAvailable indicates if shadow cue exists.
	ShadowReceiptCueAvailable bool
	// ShadowReceiptCueDismissed indicates if cue was dismissed.
	ShadowReceiptCueDismissed bool

	// RealityCueAvailable indicates if reality cue exists.
	RealityCueAvailable bool
}

// ForbiddenPatterns contains regex patterns for forbidden content.
var ForbiddenPatterns = []*regexp.Regexp{
	regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`), // Email
	regexp.MustCompile(`https?://[^\s]+`),                                // URL
	regexp.MustCompile(`\$\d+(\.\d{2})?`),                                // Currency
	regexp.MustCompile(`£\d+(\.\d{2})?`),                                 // GBP
	regexp.MustCompile(`€\d+(\.\d{2})?`),                                 // EUR
}

// CheckForbiddenPatterns checks if value contains forbidden patterns.
func CheckForbiddenPatterns(value string) error {
	for _, pattern := range ForbiddenPatterns {
		if pattern.MatchString(value) {
			return errors.New("contains forbidden pattern")
		}
	}
	return nil
}

// HashString computes SHA256 of input and returns hex-encoded hash.
func HashString(s string) string {
	hash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(hash[:])
}

// HashStringShort computes SHA256 and returns first 32 hex chars.
func HashStringShort(s string) string {
	hash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(hash[:16])
}

// ValidateOpenParam validates the t parameter for /open endpoint.
func ValidateOpenParam(t string) (DeepLinkTarget, error) {
	if t == "" {
		return "", errors.New("t parameter is required")
	}

	// Sanitize: only allow alphanumeric and underscore
	for _, c := range t {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_') {
			return "", fmt.Errorf("invalid character in t parameter: %c", c)
		}
	}

	target := DeepLinkTarget(strings.ToLower(t))
	if err := target.Validate(); err != nil {
		return "", err
	}

	return target, nil
}

// Bounded retention constants.
const (
	// MaxRegistrationRecords is the maximum number of registration records.
	MaxRegistrationRecords = 200

	// MaxRetentionDays is the maximum retention period in days.
	MaxRetentionDays = 30
)
