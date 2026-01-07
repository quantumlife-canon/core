// Package interruptpreview defines the Phase 34 Permitted Interrupt Preview.
//
// This package provides web-only preview of permitted interrupt candidates.
// It surfaces abstract buckets only — never raw identifiers, names, or content.
//
// CRITICAL INVARIANTS:
//   - NO notifications. Web-only preview.
//   - NO background work. No goroutines.
//   - NO raw identifiers. Hash-only, bucket-only.
//   - Deterministic: same inputs => same outputs.
//   - Single-whisper rule respected.
//   - User must explicitly click to see preview.
//
// Reference: docs/ADR/ADR-0070-phase34-interrupt-preview-web-only.md
package interruptpreview

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
)

// ═══════════════════════════════════════════════════════════════════════════
// Bucket Types
// ═══════════════════════════════════════════════════════════════════════════

// CircleTypeBucket represents the type of pressure circle.
type CircleTypeBucket string

const (
	CircleTypeHuman       CircleTypeBucket = "human"
	CircleTypeInstitution CircleTypeBucket = "institution"
	// Commerce is never permitted to interrupt
)

// ValidCircleTypes is the set of valid circle types for preview.
var ValidCircleTypes = map[CircleTypeBucket]bool{
	CircleTypeHuman:       true,
	CircleTypeInstitution: true,
}

// Validate checks if the circle type is valid.
func (c CircleTypeBucket) Validate() error {
	if !ValidCircleTypes[c] {
		return fmt.Errorf("invalid circle type: %s", c)
	}
	return nil
}

// String returns the string representation.
func (c CircleTypeBucket) String() string {
	return string(c)
}

// DisplayLabel returns a human-friendly label.
func (c CircleTypeBucket) DisplayLabel() string {
	switch c {
	case CircleTypeHuman:
		return "Someone you know"
	case CircleTypeInstitution:
		return "An organization"
	default:
		return "Unknown source"
	}
}

// HorizonBucket represents the urgency horizon.
type HorizonBucket string

const (
	HorizonNow   HorizonBucket = "now"
	HorizonSoon  HorizonBucket = "soon"
	HorizonLater HorizonBucket = "later"
)

// ValidHorizons is the set of valid horizons.
var ValidHorizons = map[HorizonBucket]bool{
	HorizonNow:   true,
	HorizonSoon:  true,
	HorizonLater: true,
}

// Validate checks if the horizon is valid.
func (h HorizonBucket) Validate() error {
	if !ValidHorizons[h] {
		return fmt.Errorf("invalid horizon: %s", h)
	}
	return nil
}

// String returns the string representation.
func (h HorizonBucket) String() string {
	return string(h)
}

// DisplayLabel returns a human-friendly label.
func (h HorizonBucket) DisplayLabel() string {
	switch h {
	case HorizonNow:
		return "Needs attention now"
	case HorizonSoon:
		return "Needs attention soon"
	case HorizonLater:
		return "Can wait"
	default:
		return "Unknown timing"
	}
}

// MagnitudeBucket represents abstract quantity.
type MagnitudeBucket string

const (
	MagnitudeNothing MagnitudeBucket = "nothing"
	MagnitudeAFew    MagnitudeBucket = "a_few"
	MagnitudeSeveral MagnitudeBucket = "several"
)

// ValidMagnitudes is the set of valid magnitudes.
var ValidMagnitudes = map[MagnitudeBucket]bool{
	MagnitudeNothing: true,
	MagnitudeAFew:    true,
	MagnitudeSeveral: true,
}

// Validate checks if the magnitude is valid.
func (m MagnitudeBucket) Validate() error {
	if !ValidMagnitudes[m] {
		return fmt.Errorf("invalid magnitude: %s", m)
	}
	return nil
}

// String returns the string representation.
func (m MagnitudeBucket) String() string {
	return string(m)
}

// DisplayLabel returns a human-friendly label.
func (m MagnitudeBucket) DisplayLabel() string {
	switch m {
	case MagnitudeNothing:
		return "Nothing specific"
	case MagnitudeAFew:
		return "A few items"
	case MagnitudeSeveral:
		return "Several items"
	default:
		return "Unknown"
	}
}

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

// ReasonBucket explains why something is permitted/shown.
type ReasonBucket string

const (
	ReasonHumanNow             ReasonBucket = "human_now"
	ReasonInstitutionSoon      ReasonBucket = "institution_soon"
	ReasonPolicyAllows         ReasonBucket = "policy_allows"
	ReasonWithinRateLimit      ReasonBucket = "within_rate_limit"
	ReasonHighestPriorityMatch ReasonBucket = "highest_priority_match"
)

// ValidReasons is the set of valid reasons.
var ValidReasons = map[ReasonBucket]bool{
	ReasonHumanNow:             true,
	ReasonInstitutionSoon:      true,
	ReasonPolicyAllows:         true,
	ReasonWithinRateLimit:      true,
	ReasonHighestPriorityMatch: true,
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

// DisplayLabel returns a human-friendly label.
func (r ReasonBucket) DisplayLabel() string {
	switch r {
	case ReasonHumanNow:
		return "From someone, needing attention now"
	case ReasonInstitutionSoon:
		return "From an organization, with a deadline"
	case ReasonPolicyAllows:
		return "Matches your settings"
	case ReasonWithinRateLimit:
		return "Within your daily limit"
	case ReasonHighestPriorityMatch:
		return "Selected as most relevant"
	default:
		return "Permitted"
	}
}

// AllowanceBucket represents the policy allowance that permitted this.
type AllowanceBucket string

const (
	AllowanceNone             AllowanceBucket = "allow_none"
	AllowanceHumansNow        AllowanceBucket = "allow_humans_now"
	AllowanceInstitutionsSoon AllowanceBucket = "allow_institutions_soon"
	AllowanceTwoPerDay        AllowanceBucket = "allow_two_per_day"
)

// ValidAllowances is the set of valid allowances.
var ValidAllowances = map[AllowanceBucket]bool{
	AllowanceNone:             true,
	AllowanceHumansNow:        true,
	AllowanceInstitutionsSoon: true,
	AllowanceTwoPerDay:        true,
}

// Validate checks if the allowance is valid.
func (a AllowanceBucket) Validate() error {
	if !ValidAllowances[a] {
		return fmt.Errorf("invalid allowance: %s", a)
	}
	return nil
}

// String returns the string representation.
func (a AllowanceBucket) String() string {
	return string(a)
}

// DisplayLabel returns a human-friendly label.
func (a AllowanceBucket) DisplayLabel() string {
	switch a {
	case AllowanceNone:
		return "No interrupts allowed"
	case AllowanceHumansNow:
		return "Humans only (immediate)"
	case AllowanceInstitutionsSoon:
		return "Institutions only (pressing)"
	case AllowanceTwoPerDay:
		return "Up to two per day"
	default:
		return "Unknown policy"
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Acknowledgment Types
// ═══════════════════════════════════════════════════════════════════════════

// PreviewAckKind represents the type of user acknowledgment.
type PreviewAckKind string

const (
	AckViewed    PreviewAckKind = "viewed"
	AckDismissed PreviewAckKind = "dismissed"
	AckHeld      PreviewAckKind = "held"
)

// ValidAckKinds is the set of valid ack kinds.
var ValidAckKinds = map[PreviewAckKind]bool{
	AckViewed:    true,
	AckDismissed: true,
	AckHeld:      true,
}

// Validate checks if the ack kind is valid.
func (a PreviewAckKind) Validate() error {
	if !ValidAckKinds[a] {
		return fmt.Errorf("invalid ack kind: %s", a)
	}
	return nil
}

// String returns the string representation.
func (a PreviewAckKind) String() string {
	return string(a)
}

// ═══════════════════════════════════════════════════════════════════════════
// Preview Candidate
// ═══════════════════════════════════════════════════════════════════════════

// PreviewCandidate represents a permitted interrupt candidate for preview.
// CRITICAL: Contains ONLY abstract fields and hashes. Never raw identifiers.
type PreviewCandidate struct {
	// CandidateHash is the SHA256 hash identifying this candidate.
	CandidateHash string `json:"candidate_hash"`

	// CircleType is the type of pressure circle.
	CircleType CircleTypeBucket `json:"circle_type"`

	// Horizon is the urgency horizon.
	Horizon HorizonBucket `json:"horizon"`

	// Magnitude is the abstract quantity.
	Magnitude MagnitudeBucket `json:"magnitude"`

	// ReasonBucket explains why this is permitted.
	ReasonBucket ReasonBucket `json:"reason_bucket"`

	// Allowance is the policy allowance that permitted this.
	Allowance AllowanceBucket `json:"allowance"`

	// SelectionHash is the hash used for deterministic selection.
	SelectionHash string `json:"selection_hash"`
}

// Validate checks if the candidate is valid.
func (c *PreviewCandidate) Validate() error {
	if c.CandidateHash == "" {
		return fmt.Errorf("candidate_hash required")
	}
	if err := c.CircleType.Validate(); err != nil {
		return err
	}
	if err := c.Horizon.Validate(); err != nil {
		return err
	}
	if err := c.Magnitude.Validate(); err != nil {
		return err
	}
	if err := c.ReasonBucket.Validate(); err != nil {
		return err
	}
	if err := c.Allowance.Validate(); err != nil {
		return err
	}
	return nil
}

// CanonicalString returns the pipe-delimited canonical representation.
func (c *PreviewCandidate) CanonicalString() string {
	return fmt.Sprintf("PREVIEW_CANDIDATE|v1|%s|%s|%s|%s|%s|%s",
		c.CandidateHash,
		c.CircleType,
		c.Horizon,
		c.Magnitude,
		c.ReasonBucket,
		c.Allowance,
	)
}

// ComputeSelectionHash computes the hash for deterministic selection.
func (c *PreviewCandidate) ComputeSelectionHash(periodKey string) string {
	input := fmt.Sprintf("%s|%s", c.CanonicalString(), periodKey)
	h := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", h)
}

// ═══════════════════════════════════════════════════════════════════════════
// Preview Cue
// ═══════════════════════════════════════════════════════════════════════════

// PreviewCue represents the subtle cue shown on /today.
type PreviewCue struct {
	// Available indicates if a preview is available.
	Available bool `json:"available"`

	// Text is the cue text to display.
	Text string `json:"text"`

	// LinkPath is the path to the preview page.
	LinkPath string `json:"link_path"`

	// StatusHash is the deterministic hash of this cue state.
	StatusHash string `json:"status_hash"`

	// Priority is the cue priority (higher number = lower priority).
	Priority int `json:"priority"`
}

// DefaultPreviewCueText is the default cue text.
const DefaultPreviewCueText = "If you want to look now — there's something time-sensitive."

// DefaultPreviewCuePath is the default path to the preview page.
const DefaultPreviewCuePath = "/interrupts/preview"

// DefaultPreviewCuePriority is the priority for Phase 34 cue.
// Higher number = lower priority. Must be below Phase 33 and above sanity-check.
const DefaultPreviewCuePriority = 110

// CanonicalString returns the pipe-delimited canonical representation.
func (c *PreviewCue) CanonicalString() string {
	availStr := "no"
	if c.Available {
		availStr = "yes"
	}
	return fmt.Sprintf("PREVIEW_CUE|v1|%s|%s|%d",
		availStr,
		c.LinkPath,
		c.Priority,
	)
}

// ComputeStatusHash computes the status hash for this cue.
func (c *PreviewCue) ComputeStatusHash() string {
	h := sha256.Sum256([]byte(c.CanonicalString()))
	return fmt.Sprintf("%x", h[:16])
}

// DefaultPreviewCue returns the default (unavailable) preview cue.
func DefaultPreviewCue() *PreviewCue {
	cue := &PreviewCue{
		Available: false,
		Text:      DefaultPreviewCueText,
		LinkPath:  DefaultPreviewCuePath,
		Priority:  DefaultPreviewCuePriority,
	}
	cue.StatusHash = cue.ComputeStatusHash()
	return cue
}

// ═══════════════════════════════════════════════════════════════════════════
// Preview Page
// ═══════════════════════════════════════════════════════════════════════════

// PreviewPage represents the preview page data.
// CRITICAL: No raw identifiers. Abstract buckets only.
type PreviewPage struct {
	// Title is the page title.
	Title string `json:"title"`

	// Subtitle is the page subtitle.
	Subtitle string `json:"subtitle"`

	// Lines are calm copy lines.
	Lines []string `json:"lines"`

	// CircleTypeLabel is the display label for circle type.
	CircleTypeLabel string `json:"circle_type_label"`

	// HorizonLabel is the display label for horizon.
	HorizonLabel string `json:"horizon_label"`

	// MagnitudeLabel is the display label for magnitude.
	MagnitudeLabel string `json:"magnitude_label"`

	// ReasonLabel is the display label for reason.
	ReasonLabel string `json:"reason_label"`

	// AllowanceLabel is the display label for allowance.
	AllowanceLabel string `json:"allowance_label"`

	// HoldPath is the path for the hold action.
	HoldPath string `json:"hold_path"`

	// DismissPath is the path for the dismiss action.
	DismissPath string `json:"dismiss_path"`

	// BackLink is the path to return to.
	BackLink string `json:"back_link"`

	// StatusHash is the deterministic hash of this page state.
	StatusHash string `json:"status_hash"`

	// CandidateHash is the hash of the selected candidate.
	CandidateHash string `json:"candidate_hash"`

	// PeriodKey is the current period.
	PeriodKey string `json:"period_key"`

	// CircleIDHash is the circle for this preview.
	CircleIDHash string `json:"circle_id_hash"`
}

// CanonicalString returns the pipe-delimited canonical representation.
func (p *PreviewPage) CanonicalString() string {
	return fmt.Sprintf("PREVIEW_PAGE|v1|%s|%s|%s|%s|%s|%s|%s",
		p.Title,
		p.CandidateHash,
		p.CircleTypeLabel,
		p.HorizonLabel,
		p.ReasonLabel,
		p.AllowanceLabel,
		p.PeriodKey,
	)
}

// ComputeStatusHash computes the status hash for this page.
func (p *PreviewPage) ComputeStatusHash() string {
	h := sha256.Sum256([]byte(p.CanonicalString()))
	return fmt.Sprintf("%x", h[:16])
}

// DefaultPreviewPage returns an empty preview page.
func DefaultPreviewPage(periodKey, circleIDHash string) *PreviewPage {
	p := &PreviewPage{
		Title:        "Available, if you want it.",
		Subtitle:     "This is time-sensitive, but still your choice.",
		Lines:        []string{"Your boundaries are still being respected."},
		HoldPath:     "/interrupts/preview/hold",
		DismissPath:  "/interrupts/preview/dismiss",
		BackLink:     "/today",
		PeriodKey:    periodKey,
		CircleIDHash: circleIDHash,
	}
	p.StatusHash = p.ComputeStatusHash()
	return p
}

// ═══════════════════════════════════════════════════════════════════════════
// Preview Proof Page
// ═══════════════════════════════════════════════════════════════════════════

// PreviewProofPage represents the proof page for preview.
type PreviewProofPage struct {
	// Title is the page title.
	Title string `json:"title"`

	// Lines are calm copy lines.
	Lines []string `json:"lines"`

	// PreviewAvailable indicates if any preview was available.
	PreviewAvailable bool `json:"preview_available"`

	// UserDismissed indicates if the user dismissed/held.
	UserDismissed bool `json:"user_dismissed"`

	// StatusHash is the deterministic hash of this proof state.
	StatusHash string `json:"status_hash"`

	// BackLink is the path to return to.
	BackLink string `json:"back_link"`

	// PeriodKey is the current period.
	PeriodKey string `json:"period_key"`
}

// CanonicalString returns the pipe-delimited canonical representation.
func (p *PreviewProofPage) CanonicalString() string {
	availStr := "no"
	if p.PreviewAvailable {
		availStr = "yes"
	}
	dismissedStr := "no"
	if p.UserDismissed {
		dismissedStr = "yes"
	}
	return fmt.Sprintf("PREVIEW_PROOF_PAGE|v1|%s|%s|%s",
		availStr,
		dismissedStr,
		p.PeriodKey,
	)
}

// ComputeStatusHash computes the status hash for this proof page.
func (p *PreviewProofPage) ComputeStatusHash() string {
	h := sha256.Sum256([]byte(p.CanonicalString()))
	return fmt.Sprintf("%x", h[:16])
}

// DefaultPreviewProofPage returns the default proof page.
func DefaultPreviewProofPage(periodKey string) *PreviewProofPage {
	p := &PreviewProofPage{
		Title:            "Permission, kept.",
		Lines:            defaultProofLines(),
		PreviewAvailable: false,
		UserDismissed:    false,
		BackLink:         "/today",
		PeriodKey:        periodKey,
	}
	p.StatusHash = p.ComputeStatusHash()
	return p
}

// defaultProofLines returns the default proof lines.
func defaultProofLines() []string {
	return []string{
		"Previews are only shown when permitted by your policy.",
		"Your boundaries are being respected.",
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Preview Ack Record
// ═══════════════════════════════════════════════════════════════════════════

// PreviewAck represents a preview acknowledgment record.
// CRITICAL: Hash-only. No raw identifiers.
type PreviewAck struct {
	// AckID is the unique identifier for this ack.
	AckID string `json:"ack_id"`

	// CircleIDHash identifies the circle.
	CircleIDHash string `json:"circle_id_hash"`

	// PeriodKey is the daily bucket.
	PeriodKey string `json:"period_key"`

	// CandidateHash is the hash of the candidate.
	CandidateHash string `json:"candidate_hash"`

	// Kind is the type of acknowledgment.
	Kind PreviewAckKind `json:"kind"`

	// AckBucket is when the ack was recorded.
	AckBucket string `json:"ack_bucket"`

	// StatusHash is the page status that was acked.
	StatusHash string `json:"status_hash"`
}

// Validate checks if the ack is valid.
func (a *PreviewAck) Validate() error {
	if a.CircleIDHash == "" {
		return fmt.Errorf("circle_id_hash required")
	}
	if a.PeriodKey == "" {
		return fmt.Errorf("period_key required")
	}
	if a.CandidateHash == "" {
		return fmt.Errorf("candidate_hash required")
	}
	if err := a.Kind.Validate(); err != nil {
		return err
	}
	return nil
}

// CanonicalString returns the pipe-delimited canonical representation.
func (a *PreviewAck) CanonicalString() string {
	return fmt.Sprintf("PREVIEW_ACK|v1|%s|%s|%s|%s|%s|%s|%s",
		a.AckID,
		a.CircleIDHash,
		a.PeriodKey,
		a.CandidateHash,
		a.Kind,
		a.AckBucket,
		a.StatusHash,
	)
}

// ComputeAckID computes the ack ID.
func (a *PreviewAck) ComputeAckID() string {
	input := fmt.Sprintf("%s|%s|%s|%s|%s",
		a.CircleIDHash,
		a.PeriodKey,
		a.CandidateHash,
		a.Kind,
		a.AckBucket,
	)
	h := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", h[:16])
}

// ═══════════════════════════════════════════════════════════════════════════
// Forbidden Pattern Validation
// ═══════════════════════════════════════════════════════════════════════════

// ForbiddenPatterns are patterns that must never appear in preview content.
var ForbiddenPatterns = []*regexp.Regexp{
	regexp.MustCompile(`@`),                                                  // email addresses
	regexp.MustCompile(`https?://`),                                          // URLs
	regexp.MustCompile(`[£$€]\s*\d`),                                         // currency amounts
	regexp.MustCompile(`\d{3}[-.\s]?\d{3}[-.\s]?\d{4}`),                      // phone numbers
	regexp.MustCompile(`(?i)(uber|deliveroo|amazon|paypal|invoice|receipt)`), // merchant tokens
	regexp.MustCompile(`\d{4}-\d{2}-\d{2}`),                                  // dates
	regexp.MustCompile(`\d{1,2}:\d{2}`),                                      // times
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
// Input Types
// ═══════════════════════════════════════════════════════════════════════════

// PreviewInput is the input to the preview engine.
type PreviewInput struct {
	// CircleIDHash identifies the circle.
	CircleIDHash string `json:"circle_id_hash"`

	// PeriodKey is the daily bucket.
	PeriodKey string `json:"period_key"`

	// PermittedCandidates are the candidates that passed Phase 33.
	PermittedCandidates []*PreviewCandidate `json:"permitted_candidates"`

	// IsDismissed indicates if the user already dismissed for this period.
	IsDismissed bool `json:"is_dismissed"`

	// IsHeld indicates if the user already held for this period.
	IsHeld bool `json:"is_held"`

	// TimeBucket is the current time bucket for determinism.
	TimeBucket string `json:"time_bucket"`
}

// Validate checks if the input is valid.
func (i *PreviewInput) Validate() error {
	if i.CircleIDHash == "" {
		return fmt.Errorf("circle_id_hash required")
	}
	if i.PeriodKey == "" {
		return fmt.Errorf("period_key required")
	}
	return nil
}

// CanonicalString returns the pipe-delimited canonical representation.
func (i *PreviewInput) CanonicalString() string {
	candidateHashes := make([]string, len(i.PermittedCandidates))
	for idx, c := range i.PermittedCandidates {
		candidateHashes[idx] = c.CandidateHash
	}
	dismissedStr := "no"
	if i.IsDismissed {
		dismissedStr = "yes"
	}
	heldStr := "no"
	if i.IsHeld {
		heldStr = "yes"
	}
	return fmt.Sprintf("PREVIEW_INPUT|v1|%s|%s|%s|%s|%d|%s",
		i.CircleIDHash,
		i.PeriodKey,
		dismissedStr,
		heldStr,
		len(i.PermittedCandidates),
		strings.Join(candidateHashes, ","),
	)
}

// ComputeInputHash computes the hash of this input.
func (i *PreviewInput) ComputeInputHash() string {
	h := sha256.Sum256([]byte(i.CanonicalString()))
	return fmt.Sprintf("%x", h[:16])
}
