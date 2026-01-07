// Package deviceidentity provides domain types for Phase 30A: Identity + Replay.
// This package implements device-rooted identity using Ed25519 keys for signed requests.
//
// CRITICAL INVARIANTS:
// - stdlib only
// - No time.Now() - clock injection only
// - No goroutines
// - Hash-only storage (fingerprints, not raw keys in UI)
// - Bounded retention for bindings (max 5 devices per circle)
//
// Reference: docs/ADR/ADR-0061-phase30A-identity-and-replay.md
package deviceidentity

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// DevicePublicKey is the hex-encoded Ed25519 public key.
// This is the device's identity - derived from the private key stored on device.
type DevicePublicKey string

// Validate checks if the public key is valid hex and correct length.
func (k DevicePublicKey) Validate() error {
	if k == "" {
		return errors.New("public key is required")
	}
	decoded, err := hex.DecodeString(string(k))
	if err != nil {
		return fmt.Errorf("invalid hex encoding: %w", err)
	}
	if len(decoded) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid public key length: got %d, want %d", len(decoded), ed25519.PublicKeySize)
	}
	return nil
}

// ToBytes decodes the hex public key to bytes.
func (k DevicePublicKey) ToBytes() (ed25519.PublicKey, error) {
	decoded, err := hex.DecodeString(string(k))
	if err != nil {
		return nil, err
	}
	return ed25519.PublicKey(decoded), nil
}

// Fingerprint computes the SHA256 fingerprint of the public key.
func (k DevicePublicKey) Fingerprint() Fingerprint {
	hash := sha256.Sum256([]byte(k))
	return Fingerprint(hex.EncodeToString(hash[:]))
}

// Fingerprint is the SHA256 hash of a public key (64 hex chars).
// This is what we store and display - never raw keys in persistence or UI.
type Fingerprint string

// Validate checks if the fingerprint is valid hex and correct length.
func (f Fingerprint) Validate() error {
	if f == "" {
		return errors.New("fingerprint is required")
	}
	if len(f) != 64 {
		return fmt.Errorf("invalid fingerprint length: got %d, want 64", len(f))
	}
	_, err := hex.DecodeString(string(f))
	if err != nil {
		return fmt.Errorf("invalid hex encoding: %w", err)
	}
	return nil
}

// Short returns first 8 chars for display (calm, non-technical).
func (f Fingerprint) Short() string {
	if len(f) < 8 {
		return string(f)
	}
	return string(f)[:8]
}

// CanonicalString returns a deterministic, pipe-delimited representation.
func (f Fingerprint) CanonicalString() string {
	return fmt.Sprintf("v1|fingerprint|%s", f)
}

// PeriodKey represents a 15-minute time bucket for replay protection.
// Format: YYYY-MM-DDTHH:MM (floored to :00, :15, :30, :45)
type PeriodKey string

// NewPeriodKey creates a period key from the given time.
// Time is floored to the nearest 15-minute boundary.
func NewPeriodKey(t time.Time) PeriodKey {
	utc := t.UTC()
	minute := utc.Minute()
	flooredMinute := (minute / 15) * 15
	floored := time.Date(utc.Year(), utc.Month(), utc.Day(), utc.Hour(), flooredMinute, 0, 0, time.UTC)
	return PeriodKey(floored.Format("2006-01-02T15:04"))
}

// Validate checks if the period key is valid format.
func (p PeriodKey) Validate() error {
	if p == "" {
		return errors.New("period key is required")
	}
	_, err := time.Parse("2006-01-02T15:04", string(p))
	if err != nil {
		return fmt.Errorf("invalid period key format: %w", err)
	}
	return nil
}

// Time parses the period key back to a time.
func (p PeriodKey) Time() (time.Time, error) {
	return time.Parse("2006-01-02T15:04", string(p))
}

// CanonicalString returns a deterministic representation.
func (p PeriodKey) CanonicalString() string {
	return fmt.Sprintf("v1|period|%s", p)
}

// Signature is a hex-encoded Ed25519 signature.
type Signature string

// Validate checks if the signature is valid hex and correct length.
func (s Signature) Validate() error {
	if s == "" {
		return errors.New("signature is required")
	}
	decoded, err := hex.DecodeString(string(s))
	if err != nil {
		return fmt.Errorf("invalid hex encoding: %w", err)
	}
	if len(decoded) != ed25519.SignatureSize {
		return fmt.Errorf("invalid signature length: got %d, want %d", len(decoded), ed25519.SignatureSize)
	}
	return nil
}

// ToBytes decodes the hex signature to bytes.
func (s Signature) ToBytes() ([]byte, error) {
	return hex.DecodeString(string(s))
}

// SignedRequest represents a cryptographically signed request.
// Used for replay export/import endpoints requiring explicit authorization.
type SignedRequest struct {
	// Method is the HTTP method (POST only for sensitive operations).
	Method string

	// Path is the request path (e.g., /replay/export).
	Path string

	// BodyHash is SHA256 of the request body (empty string hash if no body).
	BodyHash string

	// PeriodKey is the 15-minute bucket to prevent replay.
	PeriodKey PeriodKey

	// PublicKey is the device's public key.
	PublicKey DevicePublicKey

	// Signature signs: Method|Path|BodyHash|PeriodKey
	Signature Signature
}

// CanonicalString returns the string that should be signed.
func (r *SignedRequest) CanonicalString() string {
	return fmt.Sprintf("%s|%s|%s|%s",
		r.Method,
		r.Path,
		r.BodyHash,
		r.PeriodKey,
	)
}

// Validate checks all fields are present and valid.
func (r *SignedRequest) Validate() error {
	if r.Method == "" {
		return errors.New("method is required")
	}
	if r.Method != "POST" {
		return errors.New("only POST method is allowed for signed requests")
	}
	if r.Path == "" {
		return errors.New("path is required")
	}
	if r.BodyHash == "" {
		return errors.New("body hash is required")
	}
	if err := r.PeriodKey.Validate(); err != nil {
		return fmt.Errorf("invalid period key: %w", err)
	}
	if err := r.PublicKey.Validate(); err != nil {
		return fmt.Errorf("invalid public key: %w", err)
	}
	if err := r.Signature.Validate(); err != nil {
		return fmt.Errorf("invalid signature: %w", err)
	}
	return nil
}

// CircleBinding represents a device fingerprint bound to a circle.
// Hash-only storage - we store the fingerprint, not the raw public key.
type CircleBinding struct {
	CircleIDHash  string      // SHA256 of circle ID (hash-only)
	Fingerprint   Fingerprint // Device fingerprint
	BoundAtPeriod PeriodKey   // When binding occurred (bucket only)
	BindingHash   string      // Deterministic hash for this binding
}

// NewCircleBinding creates a new binding.
func NewCircleBinding(circleID string, fingerprint Fingerprint, boundAt time.Time) *CircleBinding {
	periodKey := NewPeriodKey(boundAt)
	b := &CircleBinding{
		CircleIDHash:  HashString(circleID),
		Fingerprint:   fingerprint,
		BoundAtPeriod: periodKey,
	}
	b.BindingHash = b.computeBindingHash()
	return b
}

// CanonicalString returns a deterministic, pipe-delimited representation.
func (b *CircleBinding) CanonicalString() string {
	return fmt.Sprintf("v1|circle_binding|%s|%s|%s",
		b.CircleIDHash,
		b.Fingerprint,
		b.BoundAtPeriod,
	)
}

// computeBindingHash computes the binding hash.
func (b *CircleBinding) computeBindingHash() string {
	hash := sha256.Sum256([]byte(b.CanonicalString()))
	return hex.EncodeToString(hash[:16])
}

// DeviceIdentityPage is the UI model for /identity.
// Shows only abstract data - fingerprint short form and magnitude buckets.
type DeviceIdentityPage struct {
	// FingerprintShort is the first 8 chars of the fingerprint.
	FingerprintShort string

	// BoundDevicesMagnitude is the abstract count (nothing/a_few/several).
	BoundDevicesMagnitude MagnitudeBucket

	// IsBound indicates if this device is bound to the current circle.
	IsBound bool

	// StatusHash is deterministic hash for this page state.
	StatusHash string
}

// NewDeviceIdentityPage creates a page for display.
func NewDeviceIdentityPage(fingerprint Fingerprint, boundCount int, isBound bool) *DeviceIdentityPage {
	page := &DeviceIdentityPage{
		FingerprintShort:      fingerprint.Short(),
		BoundDevicesMagnitude: ComputeMagnitude(boundCount),
		IsBound:               isBound,
	}
	page.StatusHash = page.computeStatusHash()
	return page
}

// computeStatusHash computes the status hash for this page.
func (p *DeviceIdentityPage) computeStatusHash() string {
	canonical := fmt.Sprintf("v1|identity_page|%s|%s|%t",
		p.FingerprintShort,
		p.BoundDevicesMagnitude,
		p.IsBound,
	)
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:16])
}

// MagnitudeBucket represents abstract quantities (never raw counts in UI).
type MagnitudeBucket string

const (
	MagnitudeNothing MagnitudeBucket = "nothing"
	MagnitudeAFew    MagnitudeBucket = "a_few"   // 1-3
	MagnitudeSeveral MagnitudeBucket = "several" // 4-5
	MagnitudeMany    MagnitudeBucket = "many"    // >5 (should not happen with max 5 bound)
)

// ComputeMagnitude converts a count to an abstract bucket.
func ComputeMagnitude(count int) MagnitudeBucket {
	switch {
	case count == 0:
		return MagnitudeNothing
	case count <= 3:
		return MagnitudeAFew
	case count <= 5:
		return MagnitudeSeveral
	default:
		return MagnitudeMany
	}
}

// HashString computes SHA256 of input and returns first 32 hex chars.
func HashString(s string) string {
	hash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(hash[:16])
}

// ComputeBodyHash computes SHA256 of body content.
// Empty body returns hash of empty string.
func ComputeBodyHash(body []byte) string {
	hash := sha256.Sum256(body)
	return hex.EncodeToString(hash[:])
}

// VerificationResult contains the result of signature verification.
type VerificationResult struct {
	Valid           bool
	Error           string
	Fingerprint     Fingerprint
	IsBoundToCircle bool
}

// BindResult contains the result of binding a device.
type BindResult struct {
	Success    bool
	Error      string
	Binding    *CircleBinding
	BoundCount int
	AtMaxLimit bool
}

// Constants
const (
	// MaxDevicesPerCircle is the maximum devices that can be bound to a circle.
	MaxDevicesPerCircle = 5

	// DefaultRetentionDays is the default retention for replay bundles.
	DefaultRetentionDays = 30
)

// ParseSignedRequestFromHeaders extracts signed request components from headers.
// Headers: X-QL-Method, X-QL-Path, X-QL-BodyHash, X-QL-Period, X-QL-PublicKey, X-QL-Signature
func ParseSignedRequestFromHeaders(headers map[string]string) (*SignedRequest, error) {
	req := &SignedRequest{
		Method:    headers["X-QL-Method"],
		Path:      headers["X-QL-Path"],
		BodyHash:  headers["X-QL-BodyHash"],
		PeriodKey: PeriodKey(headers["X-QL-Period"]),
		PublicKey: DevicePublicKey(headers["X-QL-PublicKey"]),
		Signature: Signature(headers["X-QL-Signature"]),
	}

	// Validate all fields
	var errs []string
	if req.Method == "" {
		errs = append(errs, "missing X-QL-Method")
	}
	if req.Path == "" {
		errs = append(errs, "missing X-QL-Path")
	}
	if req.BodyHash == "" {
		errs = append(errs, "missing X-QL-BodyHash")
	}
	if req.PeriodKey == "" {
		errs = append(errs, "missing X-QL-Period")
	}
	if req.PublicKey == "" {
		errs = append(errs, "missing X-QL-PublicKey")
	}
	if req.Signature == "" {
		errs = append(errs, "missing X-QL-Signature")
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("missing headers: %s", strings.Join(errs, ", "))
	}

	return req, nil
}
