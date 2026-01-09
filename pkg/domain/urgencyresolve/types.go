// Package urgencyresolve provides domain types for Phase 53: Urgency Resolution Layer.
//
// CRITICAL INVARIANTS:
// - NO POWER: This package is cap-only, clamp-only. It MUST NOT deliver push,
//   execute anything, or add any observers. Proof only.
// - HASH-ONLY: Never store or render raw identifiers, emails, URLs, merchant names,
//   amounts, timestamps, or content. Only hashes, buckets, status flags.
// - DETERMINISTIC: Same inputs + same clock period = same resolution hash.
// - PIPE-DELIMITED: All canonical strings use pipe-delimited format, NOT JSON.
//   Format: v1|circle=<hash>|period=<key>|...
// - COMMERCE NEVER ESCALATES: Commerce circle type always gets cap_hold_only.
// - CAPS ONLY REDUCE: Caps can only reduce escalation; never increase power.
// - REASONS MAX 3: Reasons are deterministically sorted and capped at 3.
//
// Reference: docs/ADR/ADR-0091-phase53-urgency-resolution-layer.md
package urgencyresolve

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ============================================================================
// Enums
// ============================================================================

// UrgencyLevel represents the urgency level of a resolution.
type UrgencyLevel string

const (
	// UrgNone means no urgency.
	UrgNone UrgencyLevel = "urg_none"
	// UrgLow means low urgency.
	UrgLow UrgencyLevel = "urg_low"
	// UrgMedium means medium urgency.
	UrgMedium UrgencyLevel = "urg_medium"
	// UrgHigh means high urgency.
	UrgHigh UrgencyLevel = "urg_high"
)

// Validate checks if the UrgencyLevel is valid.
func (u UrgencyLevel) Validate() error {
	switch u {
	case UrgNone, UrgLow, UrgMedium, UrgHigh:
		return nil
	default:
		return fmt.Errorf("invalid UrgencyLevel: %s", u)
	}
}

// CanonicalString returns the canonical string for hashing.
func (u UrgencyLevel) CanonicalString() string {
	return string(u)
}

// Order returns the numeric order of the level (0=none, 1=low, 2=medium, 3=high).
func (u UrgencyLevel) Order() int {
	switch u {
	case UrgNone:
		return 0
	case UrgLow:
		return 1
	case UrgMedium:
		return 2
	case UrgHigh:
		return 3
	default:
		return 0
	}
}

// LevelFromOrder returns the UrgencyLevel for a given order.
func LevelFromOrder(order int) UrgencyLevel {
	switch order {
	case 0:
		return UrgNone
	case 1:
		return UrgLow
	case 2:
		return UrgMedium
	case 3:
		return UrgHigh
	default:
		if order < 0 {
			return UrgNone
		}
		return UrgHigh
	}
}

// ============================================================================

// EscalationCap represents the maximum escalation allowed.
type EscalationCap string

const (
	// CapHoldOnly means hold only, no escalation allowed.
	CapHoldOnly EscalationCap = "cap_hold_only"
	// CapSurfaceOnly means surface only, no interrupt.
	CapSurfaceOnly EscalationCap = "cap_surface_only"
	// CapInterruptCandidateOnly means interrupt candidate only.
	CapInterruptCandidateOnly EscalationCap = "cap_interrupt_candidate_only"
)

// Validate checks if the EscalationCap is valid.
func (c EscalationCap) Validate() error {
	switch c {
	case CapHoldOnly, CapSurfaceOnly, CapInterruptCandidateOnly:
		return nil
	default:
		return fmt.Errorf("invalid EscalationCap: %s", c)
	}
}

// CanonicalString returns the canonical string for hashing.
func (c EscalationCap) CanonicalString() string {
	return string(c)
}

// Order returns the numeric order of the cap (0=hold, 1=surface, 2=interrupt).
func (c EscalationCap) Order() int {
	switch c {
	case CapHoldOnly:
		return 0
	case CapSurfaceOnly:
		return 1
	case CapInterruptCandidateOnly:
		return 2
	default:
		return 0
	}
}

// CapFromOrder returns the EscalationCap for a given order.
func CapFromOrder(order int) EscalationCap {
	switch order {
	case 0:
		return CapHoldOnly
	case 1:
		return CapSurfaceOnly
	case 2:
		return CapInterruptCandidateOnly
	default:
		if order < 0 {
			return CapHoldOnly
		}
		return CapInterruptCandidateOnly
	}
}

// MinCap returns the more restrictive of two caps.
func MinCap(a, b EscalationCap) EscalationCap {
	if a.Order() < b.Order() {
		return a
	}
	return b
}

// ============================================================================

// UrgencyReasonBucket represents reasons for urgency resolution.
type UrgencyReasonBucket string

const (
	// ReasonTimeWindow means a time window signal contributed.
	ReasonTimeWindow UrgencyReasonBucket = "reason_time_window"
	// ReasonInstitutionDeadline means an institution deadline contributed.
	ReasonInstitutionDeadline UrgencyReasonBucket = "reason_institution_deadline"
	// ReasonHumanNow means a human-now signal contributed.
	ReasonHumanNow UrgencyReasonBucket = "reason_human_now"
	// ReasonTrustProtection means trust protection clamped the resolution.
	ReasonTrustProtection UrgencyReasonBucket = "reason_trust_protection"
	// ReasonVendorContractCap means a vendor contract cap was applied.
	ReasonVendorContractCap UrgencyReasonBucket = "reason_vendor_contract_cap"
	// ReasonSemanticsNecessity means semantics necessity clamped.
	ReasonSemanticsNecessity UrgencyReasonBucket = "reason_semantics_necessity"
	// ReasonEnvelopeActive means an active envelope contributed.
	ReasonEnvelopeActive UrgencyReasonBucket = "reason_envelope_active"
	// ReasonDefaultHold means default hold was applied.
	ReasonDefaultHold UrgencyReasonBucket = "reason_default_hold"
)

// Validate checks if the UrgencyReasonBucket is valid.
func (r UrgencyReasonBucket) Validate() error {
	switch r {
	case ReasonTimeWindow, ReasonInstitutionDeadline, ReasonHumanNow,
		ReasonTrustProtection, ReasonVendorContractCap, ReasonSemanticsNecessity,
		ReasonEnvelopeActive, ReasonDefaultHold:
		return nil
	default:
		return fmt.Errorf("invalid UrgencyReasonBucket: %s", r)
	}
}

// CanonicalString returns the canonical string for hashing.
func (r UrgencyReasonBucket) CanonicalString() string {
	return string(r)
}

// SortReasons sorts reasons deterministically and returns max 3.
func SortReasons(reasons []UrgencyReasonBucket) []UrgencyReasonBucket {
	if len(reasons) == 0 {
		return reasons
	}
	// Sort by string value for determinism
	sorted := make([]UrgencyReasonBucket, len(reasons))
	copy(sorted, reasons)
	sort.Slice(sorted, func(i, j int) bool {
		return string(sorted[i]) < string(sorted[j])
	})
	// Cap at 3
	if len(sorted) > 3 {
		sorted = sorted[:3]
	}
	return sorted
}

// ============================================================================

// ResolutionStatus represents the status of a resolution.
type ResolutionStatus string

const (
	// StatusOK means no clamp happened.
	StatusOK ResolutionStatus = "status_ok"
	// StatusClamped means a clamp reduced escalation.
	StatusClamped ResolutionStatus = "status_clamped"
	// StatusRejected means invalid inputs were rejected.
	StatusRejected ResolutionStatus = "status_rejected"
)

// Validate checks if the ResolutionStatus is valid.
func (s ResolutionStatus) Validate() error {
	switch s {
	case StatusOK, StatusClamped, StatusRejected:
		return nil
	default:
		return fmt.Errorf("invalid ResolutionStatus: %s", s)
	}
}

// CanonicalString returns the canonical string for hashing.
func (s ResolutionStatus) CanonicalString() string {
	return string(s)
}

// ============================================================================

// CircleTypeBucket represents the type of circle.
type CircleTypeBucket string

const (
	// BucketHuman means a human circle.
	BucketHuman CircleTypeBucket = "bucket_human"
	// BucketInstitution means an institution circle.
	BucketInstitution CircleTypeBucket = "bucket_institution"
	// BucketCommerce means a commerce circle.
	BucketCommerce CircleTypeBucket = "bucket_commerce"
	// BucketUnknown means unknown circle type.
	BucketUnknown CircleTypeBucket = "bucket_unknown"
)

// Validate checks if the CircleTypeBucket is valid.
func (c CircleTypeBucket) Validate() error {
	switch c {
	case BucketHuman, BucketInstitution, BucketCommerce, BucketUnknown:
		return nil
	default:
		return fmt.Errorf("invalid CircleTypeBucket: %s", c)
	}
}

// CanonicalString returns the canonical string for hashing.
func (c CircleTypeBucket) CanonicalString() string {
	return string(c)
}

// ============================================================================

// RecencyBucket represents recency of an event.
type RecencyBucket string

const (
	// RecNever means never happened.
	RecNever RecencyBucket = "rec_never"
	// RecRecent means happened recently.
	RecRecent RecencyBucket = "rec_recent"
	// RecStale means happened but is stale.
	RecStale RecencyBucket = "rec_stale"
)

// Validate checks if the RecencyBucket is valid.
func (r RecencyBucket) Validate() error {
	switch r {
	case RecNever, RecRecent, RecStale:
		return nil
	default:
		return fmt.Errorf("invalid RecencyBucket: %s", r)
	}
}

// CanonicalString returns the canonical string for hashing.
func (r RecencyBucket) CanonicalString() string {
	return string(r)
}

// ============================================================================

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

// ============================================================================

// MagnitudeBucket represents coarse count bucket.
type MagnitudeBucket string

const (
	// MagNothing means 0 items.
	MagNothing MagnitudeBucket = "mag_nothing"
	// MagAFew means 1-3 items.
	MagAFew MagnitudeBucket = "mag_a_few"
	// MagSeveral means 4+ items.
	MagSeveral MagnitudeBucket = "mag_several"
)

// Validate checks if the MagnitudeBucket is valid.
func (m MagnitudeBucket) Validate() error {
	switch m {
	case MagNothing, MagAFew, MagSeveral:
		return nil
	default:
		return fmt.Errorf("invalid MagnitudeBucket: %s", m)
	}
}

// CanonicalString returns the canonical string for hashing.
func (m MagnitudeBucket) CanonicalString() string {
	return string(m)
}

// ============================================================================

// WindowSignalBucket represents time window signal state.
type WindowSignalBucket string

const (
	// WindowNone means no window signal.
	WindowNone WindowSignalBucket = "window_none"
	// WindowActive means window is active.
	WindowActive WindowSignalBucket = "window_active"
	// WindowExpired means window expired.
	WindowExpired WindowSignalBucket = "window_expired"
)

// Validate checks if the WindowSignalBucket is valid.
func (w WindowSignalBucket) Validate() error {
	switch w {
	case WindowNone, WindowActive, WindowExpired:
		return nil
	default:
		return fmt.Errorf("invalid WindowSignalBucket: %s", w)
	}
}

// CanonicalString returns the canonical string for hashing.
func (w WindowSignalBucket) CanonicalString() string {
	return string(w)
}

// ============================================================================

// InterruptAllowanceBucket represents interrupt policy allowance.
type InterruptAllowanceBucket string

const (
	// AllowanceNone means no interrupt allowed.
	AllowanceNone InterruptAllowanceBucket = "allowance_none"
	// AllowanceSurface means surface only allowed.
	AllowanceSurface InterruptAllowanceBucket = "allowance_surface"
	// AllowanceInterrupt means full interrupt allowed.
	AllowanceInterrupt InterruptAllowanceBucket = "allowance_interrupt"
)

// Validate checks if the InterruptAllowanceBucket is valid.
func (a InterruptAllowanceBucket) Validate() error {
	switch a {
	case AllowanceNone, AllowanceSurface, AllowanceInterrupt:
		return nil
	default:
		return fmt.Errorf("invalid InterruptAllowanceBucket: %s", a)
	}
}

// CanonicalString returns the canonical string for hashing.
func (a InterruptAllowanceBucket) CanonicalString() string {
	return string(a)
}

// ============================================================================
// Structs
// ============================================================================

// UrgencyInputs contains all inputs for urgency resolution.
type UrgencyInputs struct {
	// CircleIDHash is the hashed circle ID.
	CircleIDHash string
	// PeriodKey is the period for this resolution.
	PeriodKey string
	// PressureOutcomeKind is the kind of pressure outcome.
	PressureOutcomeKind string
	// CircleType is the type of circle.
	CircleType CircleTypeBucket
	// HorizonBucket is the time horizon.
	HorizonBucket HorizonBucket
	// MagnitudeBucket is the pressure magnitude.
	MagnitudeBucket MagnitudeBucket
	// EnvelopeActive indicates if an envelope is active.
	EnvelopeActive bool
	// WindowSignal is the time window signal state.
	WindowSignal WindowSignalBucket
	// VendorCap is the vendor contract cap (if any).
	VendorCap EscalationCap
	// NecessityDeclared indicates if necessity was declared.
	NecessityDeclared bool
	// TrustFragile indicates if trust is fragile.
	TrustFragile bool
	// InterruptAllowance is the interrupt policy allowance.
	InterruptAllowance InterruptAllowanceBucket
}

// Validate checks if the UrgencyInputs are valid.
func (i UrgencyInputs) Validate() error {
	if i.CircleIDHash == "" {
		return errors.New("CircleIDHash is required")
	}
	if i.PeriodKey == "" {
		return errors.New("PeriodKey is required")
	}
	if strings.Contains(i.PeriodKey, "|") {
		return errors.New("PeriodKey cannot contain pipe delimiter")
	}
	if err := i.CircleType.Validate(); err != nil {
		return err
	}
	if err := i.HorizonBucket.Validate(); err != nil {
		return err
	}
	if err := i.MagnitudeBucket.Validate(); err != nil {
		return err
	}
	if err := i.WindowSignal.Validate(); err != nil {
		return err
	}
	if err := i.VendorCap.Validate(); err != nil {
		return err
	}
	if err := i.InterruptAllowance.Validate(); err != nil {
		return err
	}
	// Check for forbidden patterns
	if ContainsForbiddenPattern(i.CircleIDHash) {
		return errors.New("CircleIDHash contains forbidden pattern")
	}
	if ContainsForbiddenPattern(i.PeriodKey) {
		return errors.New("PeriodKey contains forbidden pattern")
	}
	if ContainsForbiddenPattern(i.PressureOutcomeKind) {
		return errors.New("PressureOutcomeKind contains forbidden pattern")
	}
	return nil
}

// CanonicalString returns the pipe-delimited canonical string for hashing.
func (i UrgencyInputs) CanonicalString() string {
	parts := []string{
		"v1",
		"circle=" + i.CircleIDHash,
		"period=" + i.PeriodKey,
		"pressure=" + i.PressureOutcomeKind,
		"circle_type=" + i.CircleType.CanonicalString(),
		"horizon=" + i.HorizonBucket.CanonicalString(),
		"magnitude=" + i.MagnitudeBucket.CanonicalString(),
		"envelope=" + boolToStr(i.EnvelopeActive),
		"window=" + i.WindowSignal.CanonicalString(),
		"vendor_cap=" + i.VendorCap.CanonicalString(),
		"necessity=" + boolToStr(i.NecessityDeclared),
		"trust_fragile=" + boolToStr(i.TrustFragile),
		"interrupt=" + i.InterruptAllowance.CanonicalString(),
	}
	return strings.Join(parts, "|")
}

func boolToStr(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// ============================================================================

// UrgencyResolution is the result of urgency resolution.
type UrgencyResolution struct {
	// CircleIDHash is the hashed circle ID.
	CircleIDHash string
	// PeriodKey is the period for this resolution.
	PeriodKey string
	// Level is the resolved urgency level.
	Level UrgencyLevel
	// Cap is the escalation cap applied.
	Cap EscalationCap
	// Reasons are the reasons for this resolution (max 3, sorted).
	Reasons []UrgencyReasonBucket
	// Status is the resolution status.
	Status ResolutionStatus
	// ResolutionHash is the SHA256 hash of the resolution.
	ResolutionHash string
}

// Validate checks if the UrgencyResolution is valid.
func (r UrgencyResolution) Validate() error {
	if r.CircleIDHash == "" {
		return errors.New("CircleIDHash is required")
	}
	if r.PeriodKey == "" {
		return errors.New("PeriodKey is required")
	}
	if err := r.Level.Validate(); err != nil {
		return err
	}
	if err := r.Cap.Validate(); err != nil {
		return err
	}
	if len(r.Reasons) > 3 {
		return errors.New("Reasons cannot exceed 3")
	}
	for _, reason := range r.Reasons {
		if err := reason.Validate(); err != nil {
			return err
		}
	}
	if err := r.Status.Validate(); err != nil {
		return err
	}
	return nil
}

// CanonicalString returns the pipe-delimited canonical string for hashing.
func (r UrgencyResolution) CanonicalString() string {
	reasonStrs := make([]string, len(r.Reasons))
	for i, reason := range r.Reasons {
		reasonStrs[i] = reason.CanonicalString()
	}
	parts := []string{
		"v1",
		"circle=" + r.CircleIDHash,
		"period=" + r.PeriodKey,
		"level=" + r.Level.CanonicalString(),
		"cap=" + r.Cap.CanonicalString(),
		"reasons=" + strings.Join(reasonStrs, ","),
		"status=" + r.Status.CanonicalString(),
	}
	return strings.Join(parts, "|")
}

// ComputeHash computes the SHA256 hash of the resolution.
func (r UrgencyResolution) ComputeHash() string {
	canonical := r.CanonicalString()
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:])
}

// ============================================================================

// UrgencyProofPage is the proof page for urgency resolution.
type UrgencyProofPage struct {
	// Title is the page title.
	Title string
	// Lines are calm descriptive lines (max 8).
	Lines []string
	// Level is the resolved urgency level.
	Level UrgencyLevel
	// Cap is the escalation cap applied.
	Cap EscalationCap
	// ReasonChips are reason chips for display (max 3).
	ReasonChips []string
	// StatusHash is the resolution hash for verification.
	StatusHash string
	// PeriodKey is the period for this resolution.
	PeriodKey string
}

// Validate checks if the UrgencyProofPage is valid.
func (p UrgencyProofPage) Validate() error {
	if p.Title == "" {
		return errors.New("Title is required")
	}
	if len(p.Lines) > 8 {
		return errors.New("Lines cannot exceed 8")
	}
	if len(p.ReasonChips) > 3 {
		return errors.New("ReasonChips cannot exceed 3")
	}
	if err := p.Level.Validate(); err != nil {
		return err
	}
	if err := p.Cap.Validate(); err != nil {
		return err
	}
	// Check for forbidden patterns in lines
	for _, line := range p.Lines {
		if ContainsForbiddenPattern(line) {
			return errors.New("Line contains forbidden pattern")
		}
	}
	return nil
}

// CanonicalString returns the canonical string for the page.
func (p UrgencyProofPage) CanonicalString() string {
	parts := []string{
		"page",
		"title=" + p.Title,
		"level=" + p.Level.CanonicalString(),
		"cap=" + p.Cap.CanonicalString(),
		"chips=" + strings.Join(p.ReasonChips, ","),
		"hash=" + p.StatusHash,
	}
	return strings.Join(parts, "|")
}

// ============================================================================

// UrgencyCue is an optional cue for urgency resolution.
type UrgencyCue struct {
	// Available indicates if the cue should be shown.
	Available bool
	// Line is the calm cue line (no urgency language).
	Line string
	// Priority is the cue priority (lower = higher priority).
	Priority int
}

// Validate checks if the UrgencyCue is valid.
func (c UrgencyCue) Validate() error {
	if c.Available && c.Line == "" {
		return errors.New("Line is required when Available")
	}
	if ContainsForbiddenPattern(c.Line) {
		return errors.New("Line contains forbidden pattern")
	}
	// Check for urgency language
	if containsUrgencyLanguage(c.Line) {
		return errors.New("Line contains urgency language")
	}
	return nil
}

func containsUrgencyLanguage(s string) bool {
	lower := strings.ToLower(s)
	urgentWords := []string{"urgent", "immediately", "asap", "critical", "emergency", "now!"}
	for _, word := range urgentWords {
		if strings.Contains(lower, word) {
			return true
		}
	}
	return false
}

// ============================================================================

// UrgencyAck is an acknowledgment record for dismissing urgency cue.
type UrgencyAck struct {
	// CircleIDHash is the hashed circle ID.
	CircleIDHash string
	// PeriodKey is the period for this ack.
	PeriodKey string
	// ResolutionHash is the resolution hash that was acked.
	ResolutionHash string
	// AckKind is the kind of acknowledgment.
	AckKind AckKind
}

// AckKind represents the kind of acknowledgment.
type AckKind string

const (
	// AckDismissed means the user dismissed the cue.
	AckDismissed AckKind = "dismissed"
)

// Validate checks if the AckKind is valid.
func (k AckKind) Validate() error {
	switch k {
	case AckDismissed:
		return nil
	default:
		return fmt.Errorf("invalid AckKind: %s", k)
	}
}

// Validate checks if the UrgencyAck is valid.
func (a UrgencyAck) Validate() error {
	if a.CircleIDHash == "" {
		return errors.New("CircleIDHash is required")
	}
	if a.PeriodKey == "" {
		return errors.New("PeriodKey is required")
	}
	if a.ResolutionHash == "" {
		return errors.New("ResolutionHash is required")
	}
	return a.AckKind.Validate()
}

// CanonicalString returns the canonical string for hashing.
func (a UrgencyAck) CanonicalString() string {
	return fmt.Sprintf("ack|%s|%s|%s|%s", a.CircleIDHash, a.PeriodKey, a.ResolutionHash, a.AckKind)
}

// ============================================================================
// Hashing Functions
// ============================================================================

// HashUrgencyInputs computes the SHA256 hash of the inputs canonical string.
func HashUrgencyInputs(inputs UrgencyInputs) string {
	canonical := inputs.CanonicalString()
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:])
}

// ============================================================================
// Forbidden Patterns
// ============================================================================

// ForbiddenPatterns are patterns that must never appear in urgency resolution output.
var ForbiddenPatterns = []string{
	"@",           // email indicator
	"http://",     // URL
	"https://",    // URL
	".com",        // domain
	".org",        // domain
	".net",        // domain
	"vendor_id",   // vendor identifier
	"vendorID",    // vendor identifier
	"pack_id",     // pack identifier
	"packID",      // pack identifier
	"merchant",    // merchant name
	"amount",      // amount value
	"currency",    // currency
	"sender",      // sender identifier
	"subject",     // email subject
	"recipient",   // recipient identifier
}

// ContainsForbiddenPattern checks if a string contains any forbidden pattern.
func ContainsForbiddenPattern(s string) bool {
	lower := strings.ToLower(s)
	for _, pattern := range ForbiddenPatterns {
		if strings.Contains(lower, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}
