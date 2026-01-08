// Package timewindow provides domain types for Phase 40: Time-Window Pressure Sources.
//
// Time windows model real-world "time windows" (appointments, deadlines, pickups,
// waiting humans) as abstract pressure inputs. This is OBSERVATION ONLY - no
// delivery, no execution, no notifications.
//
// Phase 40 feeds into the existing pipeline:
// External Pressure (31.4) → Decision Gate (32) → Permission (33) → Preview (34) → Delivery (36)
//
// CRITICAL INVARIANTS:
//   - NO raw timestamps, dates, times
//   - NO email addresses, subjects, senders
//   - NO merchant/vendor strings
//   - Observation ONLY - cannot deliver, cannot interrupt
//   - Hash-only storage; deterministic: same inputs => same hashes
//   - No goroutines. No time.Now() - clock injection only.
//   - Max 3 signals per build
//   - Evidence hashes capped at 3 per signal
//   - Commerce MUST NOT appear as a source
//
// Reference: docs/ADR/ADR-0077-phase40-time-window-pressure-sources.md
package timewindow

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	ae "quantumlife/pkg/domain/attentionenvelope"
)

// Storage constraints
const (
	// MaxSignals is the maximum number of signals per build result.
	MaxSignals = 3

	// MaxEvidenceHashes is the maximum number of evidence hashes per signal.
	MaxEvidenceHashes = 3

	// MaxRecords is the maximum number of records to retain.
	MaxRecords = 500

	// MaxRetentionDays is the maximum number of days to retain records.
	MaxRetentionDays = 30
)

// WindowSourceKind represents the origin of a time window signal.
type WindowSourceKind string

const (
	// SourceCalendar indicates observation from calendar events.
	SourceCalendar WindowSourceKind = "source_calendar"

	// SourceInboxInstitution indicates observation from institutional inbox messages.
	SourceInboxInstitution WindowSourceKind = "source_inbox_institution"

	// SourceInboxHuman indicates observation from human inbox messages.
	SourceInboxHuman WindowSourceKind = "source_inbox_human"

	// SourceDeviceHint indicates observation from device signals.
	SourceDeviceHint WindowSourceKind = "source_device_hint"
)

// AllWindowSourceKinds returns all source kinds in precedence order.
// CRITICAL: calendar > inbox_institution > inbox_human > device_hint
func AllWindowSourceKinds() []WindowSourceKind {
	return []WindowSourceKind{
		SourceCalendar,
		SourceInboxInstitution,
		SourceInboxHuman,
		SourceDeviceHint,
	}
}

// Validate checks if the source kind is valid.
func (k WindowSourceKind) Validate() error {
	switch k {
	case SourceCalendar, SourceInboxInstitution, SourceInboxHuman, SourceDeviceHint:
		return nil
	default:
		return fmt.Errorf("invalid window source kind: %s", k)
	}
}

// CanonicalString returns the canonical string representation.
func (k WindowSourceKind) CanonicalString() string {
	return string(k)
}

// DisplayText returns calm, human-readable text.
func (k WindowSourceKind) DisplayText() string {
	switch k {
	case SourceCalendar:
		return "calendar"
	case SourceInboxInstitution:
		return "institution"
	case SourceInboxHuman:
		return "human"
	case SourceDeviceHint:
		return "device"
	default:
		return "unknown"
	}
}

// Precedence returns the source precedence (lower = higher priority).
func (k WindowSourceKind) Precedence() int {
	switch k {
	case SourceCalendar:
		return 0
	case SourceInboxInstitution:
		return 1
	case SourceInboxHuman:
		return 2
	case SourceDeviceHint:
		return 3
	default:
		return 99
	}
}

// WindowKind represents the time horizon of a window.
type WindowKind string

const (
	// WindowNow indicates the window is happening now (within 15 minutes).
	WindowNow WindowKind = "window_now"

	// WindowSoon indicates the window is soon (within 1 hour).
	WindowSoon WindowKind = "window_soon"

	// WindowToday indicates the window is today (within 24 hours).
	WindowToday WindowKind = "window_today"

	// WindowLater indicates the window is later (beyond today).
	WindowLater WindowKind = "window_later"
)

// AllWindowKinds returns all window kinds in deterministic order.
func AllWindowKinds() []WindowKind {
	return []WindowKind{
		WindowNow,
		WindowSoon,
		WindowToday,
		WindowLater,
	}
}

// Validate checks if the window kind is valid.
func (k WindowKind) Validate() error {
	switch k {
	case WindowNow, WindowSoon, WindowToday, WindowLater:
		return nil
	default:
		return fmt.Errorf("invalid window kind: %s", k)
	}
}

// CanonicalString returns the canonical string representation.
func (k WindowKind) CanonicalString() string {
	return string(k)
}

// DisplayText returns calm, human-readable text.
func (k WindowKind) DisplayText() string {
	switch k {
	case WindowNow:
		return "now"
	case WindowSoon:
		return "soon"
	case WindowToday:
		return "today"
	case WindowLater:
		return "later"
	default:
		return "unknown"
	}
}

// ShiftEarlier shifts the window kind by one step earlier.
// CRITICAL: Max 1 step. WindowNow stays WindowNow.
func (k WindowKind) ShiftEarlier() WindowKind {
	switch k {
	case WindowLater:
		return WindowToday
	case WindowToday:
		return WindowSoon
	case WindowSoon:
		return WindowNow
	default:
		return k // WindowNow stays WindowNow
	}
}

// WindowReasonBucket represents the abstract reason for a window.
type WindowReasonBucket string

const (
	// ReasonPickup indicates a pickup window (package, person, etc.).
	ReasonPickup WindowReasonBucket = "reason_pickup"

	// ReasonAppointment indicates an appointment window.
	ReasonAppointment WindowReasonBucket = "reason_appointment"

	// ReasonDeadline indicates a deadline window.
	ReasonDeadline WindowReasonBucket = "reason_deadline"

	// ReasonTravel indicates a travel window.
	ReasonTravel WindowReasonBucket = "reason_travel"

	// ReasonWaiting indicates someone is waiting.
	ReasonWaiting WindowReasonBucket = "reason_waiting"

	// ReasonHealth indicates a health-related window.
	ReasonHealth WindowReasonBucket = "reason_health"

	// ReasonUnknown indicates an unknown reason.
	ReasonUnknown WindowReasonBucket = "reason_unknown"
)

// AllWindowReasonBuckets returns all reason buckets in deterministic order.
func AllWindowReasonBuckets() []WindowReasonBucket {
	return []WindowReasonBucket{
		ReasonPickup,
		ReasonAppointment,
		ReasonDeadline,
		ReasonTravel,
		ReasonWaiting,
		ReasonHealth,
		ReasonUnknown,
	}
}

// Validate checks if the reason bucket is valid.
func (r WindowReasonBucket) Validate() error {
	switch r {
	case ReasonPickup, ReasonAppointment, ReasonDeadline, ReasonTravel,
		ReasonWaiting, ReasonHealth, ReasonUnknown:
		return nil
	default:
		return fmt.Errorf("invalid window reason bucket: %s", r)
	}
}

// CanonicalString returns the canonical string representation.
func (r WindowReasonBucket) CanonicalString() string {
	return string(r)
}

// DisplayText returns calm, human-readable text.
func (r WindowReasonBucket) DisplayText() string {
	switch r {
	case ReasonPickup:
		return "pickup"
	case ReasonAppointment:
		return "appointment"
	case ReasonDeadline:
		return "deadline"
	case ReasonTravel:
		return "travel"
	case ReasonWaiting:
		return "waiting"
	case ReasonHealth:
		return "health"
	case ReasonUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}

// WindowCircleType represents the type of circle for a window.
type WindowCircleType string

const (
	// CircleHuman indicates a human circle (family, friend, colleague).
	CircleHuman WindowCircleType = "circle_human"

	// CircleInstitution indicates an institutional circle (bank, hospital, etc.).
	CircleInstitution WindowCircleType = "circle_institution"

	// CircleSelf indicates the self circle.
	CircleSelf WindowCircleType = "circle_self"
)

// AllWindowCircleTypes returns all circle types in deterministic order.
func AllWindowCircleTypes() []WindowCircleType {
	return []WindowCircleType{
		CircleHuman,
		CircleInstitution,
		CircleSelf,
	}
}

// Validate checks if the circle type is valid.
func (c WindowCircleType) Validate() error {
	switch c {
	case CircleHuman, CircleInstitution, CircleSelf:
		return nil
	default:
		return fmt.Errorf("invalid window circle type: %s", c)
	}
}

// CanonicalString returns the canonical string representation.
func (c WindowCircleType) CanonicalString() string {
	return string(c)
}

// DisplayText returns calm, human-readable text.
func (c WindowCircleType) DisplayText() string {
	switch c {
	case CircleHuman:
		return "human"
	case CircleInstitution:
		return "institution"
	case CircleSelf:
		return "self"
	default:
		return "unknown"
	}
}

// WindowMagnitudeBucket represents the abstract magnitude of windows.
type WindowMagnitudeBucket string

const (
	// MagnitudeNothing indicates no meaningful windows.
	MagnitudeNothing WindowMagnitudeBucket = "nothing"

	// MagnitudeAFew indicates a small number of windows (1-3).
	MagnitudeAFew WindowMagnitudeBucket = "a_few"

	// MagnitudeSeveral indicates multiple windows (4+).
	MagnitudeSeveral WindowMagnitudeBucket = "several"
)

// AllWindowMagnitudeBuckets returns all magnitude buckets in deterministic order.
func AllWindowMagnitudeBuckets() []WindowMagnitudeBucket {
	return []WindowMagnitudeBucket{
		MagnitudeNothing,
		MagnitudeAFew,
		MagnitudeSeveral,
	}
}

// Validate checks if the magnitude bucket is valid.
func (m WindowMagnitudeBucket) Validate() error {
	switch m {
	case MagnitudeNothing, MagnitudeAFew, MagnitudeSeveral:
		return nil
	default:
		return fmt.Errorf("invalid window magnitude bucket: %s", m)
	}
}

// CanonicalString returns the canonical string representation.
func (m WindowMagnitudeBucket) CanonicalString() string {
	return string(m)
}

// DisplayText returns calm, human-readable text.
func (m WindowMagnitudeBucket) DisplayText() string {
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

// IncrementMagnitude increases magnitude by one bucket.
// CRITICAL: Max +1. several stays several.
func (m WindowMagnitudeBucket) IncrementMagnitude() WindowMagnitudeBucket {
	switch m {
	case MagnitudeNothing:
		return MagnitudeAFew
	case MagnitudeAFew:
		return MagnitudeSeveral
	default:
		return MagnitudeSeveral
	}
}

// ToMagnitudeBucket converts a count to a magnitude bucket.
func ToMagnitudeBucket(count int) WindowMagnitudeBucket {
	switch {
	case count == 0:
		return MagnitudeNothing
	case count <= 3:
		return MagnitudeAFew
	default:
		return MagnitudeSeveral
	}
}

// WindowEvidenceKind represents the type of evidence for a window.
type WindowEvidenceKind string

const (
	// EvidenceCalendarEventHash indicates evidence from a calendar event.
	EvidenceCalendarEventHash WindowEvidenceKind = "evidence_calendar_event_hash"

	// EvidenceMessageHash indicates evidence from a message.
	EvidenceMessageHash WindowEvidenceKind = "evidence_message_hash"

	// EvidenceThreadHash indicates evidence from a thread.
	EvidenceThreadHash WindowEvidenceKind = "evidence_thread_hash"

	// EvidenceDeviceSignalHash indicates evidence from a device signal.
	EvidenceDeviceSignalHash WindowEvidenceKind = "evidence_device_signal_hash"
)

// AllWindowEvidenceKinds returns all evidence kinds in deterministic order.
func AllWindowEvidenceKinds() []WindowEvidenceKind {
	return []WindowEvidenceKind{
		EvidenceCalendarEventHash,
		EvidenceMessageHash,
		EvidenceThreadHash,
		EvidenceDeviceSignalHash,
	}
}

// Validate checks if the evidence kind is valid.
func (e WindowEvidenceKind) Validate() error {
	switch e {
	case EvidenceCalendarEventHash, EvidenceMessageHash, EvidenceThreadHash, EvidenceDeviceSignalHash:
		return nil
	default:
		return fmt.Errorf("invalid window evidence kind: %s", e)
	}
}

// CanonicalString returns the canonical string representation.
func (e WindowEvidenceKind) CanonicalString() string {
	return string(e)
}

// TimeWindowSignal represents a single time window signal.
// CRITICAL: Contains only abstract buckets and hashes. No raw data.
type TimeWindowSignal struct {
	// Source indicates where this signal originated.
	Source WindowSourceKind

	// CircleType is the type of circle this signal relates to.
	CircleType WindowCircleType

	// Kind is the time horizon of the window.
	Kind WindowKind

	// Reason is the abstract reason bucket.
	Reason WindowReasonBucket

	// Magnitude is the abstract magnitude bucket.
	Magnitude WindowMagnitudeBucket

	// EvidenceHashes contains up to MaxEvidenceHashes evidence hashes.
	// CRITICAL: SHA256 only, max 3.
	EvidenceHashes []string

	// StatusHash is a deterministic hash of this signal.
	StatusHash string
}

// CanonicalString returns the pipe-delimited, version-prefixed canonical form.
func (s *TimeWindowSignal) CanonicalString() string {
	// Sort evidence hashes for determinism
	sortedHashes := make([]string, len(s.EvidenceHashes))
	copy(sortedHashes, s.EvidenceHashes)
	sort.Strings(sortedHashes)

	return fmt.Sprintf("WINDOW_SIGNAL|v1|%s|%s|%s|%s|%s|%s",
		s.Source.CanonicalString(),
		s.CircleType.CanonicalString(),
		s.Kind.CanonicalString(),
		s.Reason.CanonicalString(),
		s.Magnitude.CanonicalString(),
		strings.Join(sortedHashes, ","),
	)
}

// ComputeStatusHash computes a deterministic status hash.
func (s *TimeWindowSignal) ComputeStatusHash() string {
	h := sha256.Sum256([]byte(s.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// Validate checks if the signal is valid.
func (s *TimeWindowSignal) Validate() error {
	if err := s.Source.Validate(); err != nil {
		return err
	}
	if err := s.CircleType.Validate(); err != nil {
		return err
	}
	if err := s.Kind.Validate(); err != nil {
		return err
	}
	if err := s.Reason.Validate(); err != nil {
		return err
	}
	if err := s.Magnitude.Validate(); err != nil {
		return err
	}
	if len(s.EvidenceHashes) > MaxEvidenceHashes {
		return fmt.Errorf("too many evidence hashes: %d > %d", len(s.EvidenceHashes), MaxEvidenceHashes)
	}
	return nil
}

// CalendarWindowInputs represents calendar-derived window inputs.
type CalendarWindowInputs struct {
	// HasUpcoming indicates whether there are upcoming calendar events.
	HasUpcoming bool

	// UpcomingCountBucket is the abstract count of upcoming events.
	UpcomingCountBucket WindowMagnitudeBucket

	// NextStartsIn is the time horizon for the next event.
	NextStartsIn WindowKind

	// EvidenceHashes contains calendar event evidence hashes.
	EvidenceHashes []string
}

// CanonicalString returns the canonical string representation.
func (c *CalendarWindowInputs) CanonicalString() string {
	sortedHashes := make([]string, len(c.EvidenceHashes))
	copy(sortedHashes, c.EvidenceHashes)
	sort.Strings(sortedHashes)

	return fmt.Sprintf("CAL_INPUT|v1|%t|%s|%s|%s",
		c.HasUpcoming,
		c.UpcomingCountBucket.CanonicalString(),
		c.NextStartsIn.CanonicalString(),
		strings.Join(sortedHashes, ","),
	)
}

// InboxWindowInputs represents inbox-derived window inputs.
type InboxWindowInputs struct {
	// InstitutionalCountBucket is the abstract count of institutional messages.
	InstitutionalCountBucket WindowMagnitudeBucket

	// HumanCountBucket is the abstract count of human messages.
	HumanCountBucket WindowMagnitudeBucket

	// InstitutionWindowKind is the time horizon for institutional messages.
	InstitutionWindowKind WindowKind

	// HumanWindowKind is the time horizon for human messages.
	HumanWindowKind WindowKind

	// EvidenceHashes contains message evidence hashes.
	EvidenceHashes []string
}

// CanonicalString returns the canonical string representation.
func (i *InboxWindowInputs) CanonicalString() string {
	sortedHashes := make([]string, len(i.EvidenceHashes))
	copy(sortedHashes, i.EvidenceHashes)
	sort.Strings(sortedHashes)

	return fmt.Sprintf("INBOX_INPUT|v1|%s|%s|%s|%s|%s",
		i.InstitutionalCountBucket.CanonicalString(),
		i.HumanCountBucket.CanonicalString(),
		i.InstitutionWindowKind.CanonicalString(),
		i.HumanWindowKind.CanonicalString(),
		strings.Join(sortedHashes, ","),
	)
}

// DeviceHintInputs represents device-derived window inputs.
type DeviceHintInputs struct {
	// TransportSignals is the abstract count of transport signals.
	TransportSignals WindowMagnitudeBucket

	// HealthSignals is the abstract count of health signals.
	HealthSignals WindowMagnitudeBucket

	// InstitutionSignals is the abstract count of institution signals.
	InstitutionSignals WindowMagnitudeBucket

	// EvidenceHashes contains device signal evidence hashes.
	EvidenceHashes []string
}

// CanonicalString returns the canonical string representation.
func (d *DeviceHintInputs) CanonicalString() string {
	sortedHashes := make([]string, len(d.EvidenceHashes))
	copy(sortedHashes, d.EvidenceHashes)
	sort.Strings(sortedHashes)

	return fmt.Sprintf("DEVICE_INPUT|v1|%s|%s|%s|%s",
		d.TransportSignals.CanonicalString(),
		d.HealthSignals.CanonicalString(),
		d.InstitutionSignals.CanonicalString(),
		strings.Join(sortedHashes, ","),
	)
}

// AttentionEnvelopeSummary is a read-only summary of attention envelope state.
// Imported from Phase 39. Used to modify window kind.
type AttentionEnvelopeSummary struct {
	// IsActive indicates whether an envelope is active.
	IsActive bool

	// Kind is the envelope kind (if active).
	Kind ae.EnvelopeKind
}

// TimeWindowInputs captures all inputs needed to build time window signals.
type TimeWindowInputs struct {
	// CircleIDHash identifies the circle.
	CircleIDHash string

	// NowBucket is the 15-minute period bucket.
	// Format: "YYYY-MM-DDTHH:MM" floored to 15-minute boundary.
	NowBucket string

	// Calendar contains calendar-derived inputs.
	Calendar CalendarWindowInputs

	// Inbox contains inbox-derived inputs.
	Inbox InboxWindowInputs

	// DeviceHints contains device-derived inputs.
	DeviceHints DeviceHintInputs

	// EnvelopeSummary contains read-only envelope state.
	EnvelopeSummary AttentionEnvelopeSummary
}

// CanonicalString returns the canonical string representation.
func (t *TimeWindowInputs) CanonicalString() string {
	return fmt.Sprintf("WINDOW_INPUTS|v1|%s|%s|%s|%s|%s|%t|%s",
		t.CircleIDHash,
		t.NowBucket,
		t.Calendar.CanonicalString(),
		t.Inbox.CanonicalString(),
		t.DeviceHints.CanonicalString(),
		t.EnvelopeSummary.IsActive,
		t.EnvelopeSummary.Kind,
	)
}

// ComputeInputHash computes a deterministic hash of the inputs.
func (t *TimeWindowInputs) ComputeInputHash() string {
	h := sha256.Sum256([]byte(t.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// TimeWindowBuildStatus represents the status of a build result.
type TimeWindowBuildStatus string

const (
	// StatusOK indicates signals were built successfully.
	StatusOK TimeWindowBuildStatus = "ok"

	// StatusEmpty indicates no qualifying windows.
	StatusEmpty TimeWindowBuildStatus = "empty"

	// StatusBlocked indicates building was blocked (e.g., commerce).
	StatusBlocked TimeWindowBuildStatus = "blocked"
)

// AllTimeWindowBuildStatuses returns all statuses in deterministic order.
func AllTimeWindowBuildStatuses() []TimeWindowBuildStatus {
	return []TimeWindowBuildStatus{
		StatusOK,
		StatusEmpty,
		StatusBlocked,
	}
}

// Validate checks if the status is valid.
func (s TimeWindowBuildStatus) Validate() error {
	switch s {
	case StatusOK, StatusEmpty, StatusBlocked:
		return nil
	default:
		return fmt.Errorf("invalid time window build status: %s", s)
	}
}

// CanonicalString returns the canonical string representation.
func (s TimeWindowBuildStatus) CanonicalString() string {
	return string(s)
}

// TimeWindowBuildResult represents the result of building time window signals.
type TimeWindowBuildResult struct {
	// Signals contains up to MaxSignals signals.
	Signals []TimeWindowSignal

	// ResultHash is a deterministic hash of the result.
	ResultHash string

	// Status is the build status.
	Status TimeWindowBuildStatus

	// InputHash is the hash of the inputs used.
	InputHash string

	// PeriodKey is the period this result was built for.
	PeriodKey string

	// CircleIDHash identifies the circle.
	CircleIDHash string
}

// CanonicalString returns the pipe-delimited, version-prefixed canonical form.
func (r *TimeWindowBuildResult) CanonicalString() string {
	var signalHashes []string
	for _, s := range r.Signals {
		signalHashes = append(signalHashes, s.StatusHash)
	}
	sort.Strings(signalHashes)

	return fmt.Sprintf("WINDOW_RESULT|v1|%s|%s|%s|%s|%s",
		r.CircleIDHash,
		r.PeriodKey,
		r.Status.CanonicalString(),
		r.InputHash,
		strings.Join(signalHashes, ","),
	)
}

// ComputeResultHash computes a deterministic result hash.
func (r *TimeWindowBuildResult) ComputeResultHash() string {
	h := sha256.Sum256([]byte(r.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// Validate checks if the result is valid.
func (r *TimeWindowBuildResult) Validate() error {
	if err := r.Status.Validate(); err != nil {
		return err
	}
	if len(r.Signals) > MaxSignals {
		return fmt.Errorf("too many signals: %d > %d", len(r.Signals), MaxSignals)
	}
	for i := range r.Signals {
		if err := r.Signals[i].Validate(); err != nil {
			return err
		}
	}
	if r.ResultHash == "" {
		return fmt.Errorf("missing result_hash")
	}
	return nil
}

// GetOverallMagnitude returns the highest magnitude across all signals.
func (r *TimeWindowBuildResult) GetOverallMagnitude() WindowMagnitudeBucket {
	if len(r.Signals) == 0 {
		return MagnitudeNothing
	}

	maxMag := MagnitudeNothing
	for _, s := range r.Signals {
		if s.Magnitude == MagnitudeSeveral {
			return MagnitudeSeveral
		}
		if s.Magnitude == MagnitudeAFew {
			maxMag = MagnitudeAFew
		}
	}
	return maxMag
}

// GetSourceChips returns unique source display texts for UI chips.
func (r *TimeWindowBuildResult) GetSourceChips() []string {
	seen := make(map[string]bool)
	var chips []string

	for _, s := range r.Signals {
		text := s.Source.DisplayText()
		if !seen[text] {
			seen[text] = true
			chips = append(chips, text)
		}
	}

	sort.Strings(chips)
	return chips
}

// NewPeriodKey creates a 15-minute period key from a time.
// Format: "YYYY-MM-DDTHH:MM" floored to 15-minute boundary.
func NewPeriodKey(t time.Time) string {
	utc := t.UTC()
	minute := (utc.Minute() / 15) * 15
	floored := time.Date(utc.Year(), utc.Month(), utc.Day(), utc.Hour(), minute, 0, 0, time.UTC)
	return floored.Format("2006-01-02T15:04")
}

// NewDayKey creates a day bucket key from a time.
// Format: "YYYY-MM-DD"
func NewDayKey(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

// ParsePeriodKey parses a period key into a time.
func ParsePeriodKey(key string) (time.Time, error) {
	return time.Parse("2006-01-02T15:04", key)
}

// WindowsProofPage represents the proof page data for time windows.
// CRITICAL: Contains ONLY abstract buckets and hashes. No raw data.
type WindowsProofPage struct {
	// CircleIDHash identifies the circle.
	CircleIDHash string

	// MagnitudeBucket is the overall magnitude.
	MagnitudeBucket WindowMagnitudeBucket

	// SourceChips contains source display texts.
	SourceChips []string

	// ResultHash is the result hash.
	ResultHash string

	// Status is the build status.
	Status TimeWindowBuildStatus

	// PageHash is a deterministic hash of the page.
	PageHash string
}

// CanonicalString returns the canonical string representation.
func (p *WindowsProofPage) CanonicalString() string {
	sortedChips := make([]string, len(p.SourceChips))
	copy(sortedChips, p.SourceChips)
	sort.Strings(sortedChips)

	return fmt.Sprintf("WINDOWS_PAGE|v1|%s|%s|%s|%s|%s",
		p.CircleIDHash,
		p.MagnitudeBucket.CanonicalString(),
		strings.Join(sortedChips, ","),
		p.ResultHash,
		p.Status.CanonicalString(),
	)
}

// ComputePageHash computes a deterministic page hash.
func (p *WindowsProofPage) ComputePageHash() string {
	h := sha256.Sum256([]byte(p.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// BuildWindowsProofPage builds a proof page from a build result.
func BuildWindowsProofPage(circleIDHash string, result *TimeWindowBuildResult) *WindowsProofPage {
	if result == nil {
		return &WindowsProofPage{
			CircleIDHash:    circleIDHash,
			MagnitudeBucket: MagnitudeNothing,
			Status:          StatusEmpty,
		}
	}

	page := &WindowsProofPage{
		CircleIDHash:    circleIDHash,
		MagnitudeBucket: result.GetOverallMagnitude(),
		SourceChips:     result.GetSourceChips(),
		ResultHash:      result.ResultHash,
		Status:          result.Status,
	}
	page.PageHash = page.ComputePageHash()
	return page
}
