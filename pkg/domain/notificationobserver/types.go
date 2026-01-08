// Package notificationobserver provides domain types for Phase 38: Mobile Notification Metadata Observer.
//
// This package observes notification metadata from mobile OS WITHOUT reading content.
// It produces abstract pressure signals that feed into the pressure pipeline (Phase 31.4).
//
// CRITICAL INVARIANTS:
//   - NO notification content - only OS-provided category metadata
//   - NO app names - only abstract class buckets
//   - NO device identifiers - hash-only storage
//   - NO free-text fields allowed
//   - NO time.Now() - clock injection only
//   - NO goroutines
//   - NO network calls
//   - NO decision logic - observation ONLY
//   - NO delivery triggers - cannot send notifications
//   - stdlib only
//
// Reference: docs/ADR/ADR-0075-phase38-notification-metadata-observer.md
package notificationobserver

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
)

// NotificationSourceKind identifies the source of notification metadata.
type NotificationSourceKind string

const (
	// SourceMobileOS indicates metadata from the mobile operating system.
	// This is the ONLY valid source - we observe OS-level metadata only.
	SourceMobileOS NotificationSourceKind = "source_mobile_os"
)

// Validate checks if the source kind is valid.
func (s NotificationSourceKind) Validate() error {
	switch s {
	case SourceMobileOS:
		return nil
	default:
		return fmt.Errorf("invalid notification source kind: %s", s)
	}
}

// CanonicalString returns a deterministic representation.
func (s NotificationSourceKind) CanonicalString() string {
	return fmt.Sprintf("v1|notif_source|%s", s)
}

// NotificationAppClass represents abstract app categories.
// CRITICAL: These are abstract buckets, NOT app names.
// The mobile OS provides category hints without exposing app identity.
type NotificationAppClass string

const (
	// AppClassTransport includes rideshare, taxi, transit apps.
	// Examples (never exposed): Uber, Lyft, transit apps
	AppClassTransport NotificationAppClass = "transport"

	// AppClassHealth includes medical, pharmacy, wellness apps.
	// Examples (never exposed): healthcare providers, pharmacies
	AppClassHealth NotificationAppClass = "health"

	// AppClassInstitution includes schools, government, banks.
	// Examples (never exposed): schools, government services, banks
	AppClassInstitution NotificationAppClass = "institution"

	// AppClassCommerce includes delivery, retail, payment apps.
	// Examples (never exposed): delivery services, shopping apps
	AppClassCommerce NotificationAppClass = "commerce"

	// AppClassUnknown is the fallback for unclassified apps.
	AppClassUnknown NotificationAppClass = "unknown"
)

// AllAppClasses returns all app classes in deterministic order.
func AllAppClasses() []NotificationAppClass {
	return []NotificationAppClass{
		AppClassTransport,
		AppClassHealth,
		AppClassInstitution,
		AppClassCommerce,
		AppClassUnknown,
	}
}

// Validate checks if the app class is valid.
func (c NotificationAppClass) Validate() error {
	switch c {
	case AppClassTransport, AppClassHealth, AppClassInstitution,
		AppClassCommerce, AppClassUnknown:
		return nil
	default:
		return fmt.Errorf("invalid notification app class: %s", c)
	}
}

// CanonicalString returns a deterministic representation.
func (c NotificationAppClass) CanonicalString() string {
	return fmt.Sprintf("v1|app_class|%s", c)
}

// DisplayText returns calm, human-readable text for the app class.
// CRITICAL: Never expose app names.
func (c NotificationAppClass) DisplayText() string {
	switch c {
	case AppClassTransport:
		return "Transport"
	case AppClassHealth:
		return "Health"
	case AppClassInstitution:
		return "Institution"
	case AppClassCommerce:
		return "Commerce"
	case AppClassUnknown:
		return "External"
	default:
		return "External"
	}
}

// MagnitudeBucket represents abstract notification counts.
// CRITICAL: Raw counts are converted to buckets BEFORE input.
type MagnitudeBucket string

const (
	// MagnitudeNothing indicates no notifications.
	MagnitudeNothing MagnitudeBucket = "nothing"

	// MagnitudeAFew indicates 1-3 notifications.
	MagnitudeAFew MagnitudeBucket = "a_few"

	// MagnitudeSeveral indicates 4+ notifications.
	MagnitudeSeveral MagnitudeBucket = "several"
)

// AllMagnitudes returns all magnitudes in deterministic order.
func AllMagnitudes() []MagnitudeBucket {
	return []MagnitudeBucket{
		MagnitudeNothing,
		MagnitudeAFew,
		MagnitudeSeveral,
	}
}

// Validate checks if the magnitude is valid.
func (m MagnitudeBucket) Validate() error {
	switch m {
	case MagnitudeNothing, MagnitudeAFew, MagnitudeSeveral:
		return nil
	default:
		return fmt.Errorf("invalid magnitude bucket: %s", m)
	}
}

// CanonicalString returns a deterministic representation.
func (m MagnitudeBucket) CanonicalString() string {
	return fmt.Sprintf("v1|magnitude|%s", m)
}

// DisplayText returns calm, human-readable text.
func (m MagnitudeBucket) DisplayText() string {
	switch m {
	case MagnitudeNothing:
		return "nothing"
	case MagnitudeAFew:
		return "a few"
	case MagnitudeSeveral:
		return "several"
	default:
		return "unknown"
	}
}

// ToMagnitude converts a raw count to a magnitude bucket.
// This is the ONLY place where raw counts are converted.
func ToMagnitude(count int) MagnitudeBucket {
	switch {
	case count == 0:
		return MagnitudeNothing
	case count <= 3:
		return MagnitudeAFew
	default:
		return MagnitudeSeveral
	}
}

// HorizonBucket represents abstract delivery timing.
// CRITICAL: No raw timestamps.
type HorizonBucket string

const (
	// HorizonNow indicates immediate attention may be warranted.
	HorizonNow HorizonBucket = "now"

	// HorizonSoon indicates attention within hours.
	HorizonSoon HorizonBucket = "soon"

	// HorizonLater indicates no immediate urgency.
	HorizonLater HorizonBucket = "later"
)

// AllHorizons returns all horizons in deterministic order.
func AllHorizons() []HorizonBucket {
	return []HorizonBucket{
		HorizonNow,
		HorizonSoon,
		HorizonLater,
	}
}

// Validate checks if the horizon is valid.
func (h HorizonBucket) Validate() error {
	switch h {
	case HorizonNow, HorizonSoon, HorizonLater:
		return nil
	default:
		return fmt.Errorf("invalid horizon bucket: %s", h)
	}
}

// CanonicalString returns a deterministic representation.
func (h HorizonBucket) CanonicalString() string {
	return fmt.Sprintf("v1|horizon|%s", h)
}

// DisplayText returns calm, human-readable text.
func (h HorizonBucket) DisplayText() string {
	switch h {
	case HorizonNow:
		return "now"
	case HorizonSoon:
		return "soon"
	case HorizonLater:
		return "later"
	default:
		return "unknown"
	}
}

// NotificationPressureSignal is the abstract pressure signal from notification metadata.
// CRITICAL: Contains ONLY abstract buckets and hashes, never raw data.
type NotificationPressureSignal struct {
	// Source identifies where this signal came from.
	Source NotificationSourceKind

	// AppClass is the abstract app category.
	AppClass NotificationAppClass

	// Magnitude is the abstract notification count.
	Magnitude MagnitudeBucket

	// Horizon is the abstract timing urgency.
	Horizon HorizonBucket

	// PeriodKey is the daily bucket (format: "YYYY-MM-DD").
	PeriodKey string

	// EvidenceHash is a deterministic hash of the inputs.
	EvidenceHash string

	// StatusHash is a deterministic hash of this signal.
	StatusHash string

	// SignalID is the unique identifier for this signal.
	SignalID string
}

// Validate checks if the signal is valid.
func (s *NotificationPressureSignal) Validate() error {
	if err := s.Source.Validate(); err != nil {
		return fmt.Errorf("invalid source: %w", err)
	}
	if err := s.AppClass.Validate(); err != nil {
		return fmt.Errorf("invalid app class: %w", err)
	}
	if err := s.Magnitude.Validate(); err != nil {
		return fmt.Errorf("invalid magnitude: %w", err)
	}
	if err := s.Horizon.Validate(); err != nil {
		return fmt.Errorf("invalid horizon: %w", err)
	}
	if s.PeriodKey == "" {
		return errors.New("period_key is required")
	}
	if s.EvidenceHash == "" {
		return errors.New("evidence_hash is required")
	}
	return nil
}

// CanonicalString returns a pipe-delimited, version-prefixed canonical form.
func (s *NotificationPressureSignal) CanonicalString() string {
	return fmt.Sprintf("NOTIF_SIGNAL|v1|%s|%s|%s|%s|%s|%s",
		s.Source,
		s.AppClass,
		s.Magnitude,
		s.Horizon,
		s.PeriodKey,
		s.EvidenceHash,
	)
}

// ComputeStatusHash computes the deterministic status hash.
func (s *NotificationPressureSignal) ComputeStatusHash() string {
	hash := sha256.Sum256([]byte(s.CanonicalString()))
	return hex.EncodeToString(hash[:16])
}

// ComputeSignalID computes the unique signal ID.
// Key: source + app_class + period_key (max 1 signal per app class per period)
func (s *NotificationPressureSignal) ComputeSignalID() string {
	key := fmt.Sprintf("SIGNAL_ID|%s|%s|%s", s.Source, s.AppClass, s.PeriodKey)
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:16])
}

// NotificationObserverInput represents the input to the observer.
// CRITICAL: Contains ONLY abstract buckets, never raw data.
type NotificationObserverInput struct {
	// AppClass is the abstract app category (from OS).
	AppClass NotificationAppClass

	// Magnitude is the pre-bucketed notification count.
	Magnitude MagnitudeBucket

	// Horizon is the pre-bucketed timing urgency.
	Horizon HorizonBucket

	// PeriodKey is the daily bucket.
	PeriodKey string
}

// Validate checks if the input is valid.
func (i *NotificationObserverInput) Validate() error {
	if err := i.AppClass.Validate(); err != nil {
		return fmt.Errorf("invalid app class: %w", err)
	}
	if err := i.Magnitude.Validate(); err != nil {
		return fmt.Errorf("invalid magnitude: %w", err)
	}
	if err := i.Horizon.Validate(); err != nil {
		return fmt.Errorf("invalid horizon: %w", err)
	}
	if i.PeriodKey == "" {
		return errors.New("period_key is required")
	}
	return nil
}

// CanonicalString returns a deterministic representation.
func (i *NotificationObserverInput) CanonicalString() string {
	return fmt.Sprintf("NOTIF_INPUT|v1|%s|%s|%s|%s",
		i.AppClass,
		i.Magnitude,
		i.Horizon,
		i.PeriodKey,
	)
}

// ComputeEvidenceHash computes the evidence hash for this input.
func (i *NotificationObserverInput) ComputeEvidenceHash() string {
	hash := sha256.Sum256([]byte(i.CanonicalString()))
	return hex.EncodeToString(hash[:16])
}

// CheckForbiddenContent checks if a value contains forbidden content.
// CRITICAL: Any identifiable content must be rejected.
func CheckForbiddenContent(value string) error {
	if value == "" {
		return nil
	}

	// Check for email patterns
	for _, c := range value {
		if c == '@' {
			return errors.New("email pattern detected")
		}
	}

	// Check for URL patterns
	if containsAny(value, "http://", "https://", "www.") {
		return errors.New("URL pattern detected")
	}

	// Check for currency patterns
	if containsAny(value, "$", "£", "€", "¥") {
		return errors.New("currency pattern detected")
	}

	// Check for time patterns (HH:MM)
	if hasTimePattern(value) {
		return errors.New("time pattern detected")
	}

	return nil
}

// containsAny checks if s contains any of the patterns.
func containsAny(s string, patterns ...string) bool {
	for _, p := range patterns {
		if len(p) > len(s) {
			continue
		}
		for i := 0; i <= len(s)-len(p); i++ {
			if s[i:i+len(p)] == p {
				return true
			}
		}
	}
	return false
}

// hasTimePattern checks for HH:MM time patterns.
func hasTimePattern(s string) bool {
	for i := 0; i < len(s)-4; i++ {
		if isDigit(s[i]) && isDigit(s[i+1]) && s[i+2] == ':' &&
			isDigit(s[i+3]) && isDigit(s[i+4]) {
			return true
		}
	}
	return false
}

// isDigit checks if a byte is a digit.
func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
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

// Bounded retention constants.
const (
	// MaxSignalRecords is the maximum number of signal records.
	MaxSignalRecords = 200

	// MaxRetentionDays is the maximum retention period in days.
	MaxRetentionDays = 30
)

// MaxSignalsPerAppClassPerPeriod is the limit: 1 signal per app class per period.
const MaxSignalsPerAppClassPerPeriod = 1
