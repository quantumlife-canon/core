// Package proofhub provides domain types for Phase 52: Proof Hub + Connected Status.
//
// CRITICAL INVARIANTS:
// - NO POWER: This package is observation/proof only. It MUST NOT change any
//   runtime behavior (no execution, no delivery, no polling, no goroutines).
// - HASH-ONLY: Never store or render raw identifiers, emails, URLs, merchant names,
//   amounts, timestamps, or content. Only hashes, buckets, status flags.
// - DETERMINISTIC: Same inputs + same clock period = same status hash.
// - PIPE-DELIMITED: All canonical strings use pipe-delimited format, NOT JSON.
//   Format: v1|circle=<hash>|period=<key>|...
// - NO TIMESTAMPS: Only recency buckets (never, recent, stale), never exact times.
// - NO COUNTS: Only magnitude buckets (nothing, a_few, several), never exact counts.
//
// Reference: docs/ADR/ADR-0090-phase52-proof-hub-connected-status.md
package proofhub

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

// ProviderStatus represents the health status of a provider.
type ProviderStatus string

const (
	// StatusUnknown means the provider status is unknown.
	StatusUnknown ProviderStatus = "status_unknown"
	// StatusOK means the provider is healthy.
	StatusOK ProviderStatus = "status_ok"
	// StatusMissing means the provider is not configured.
	StatusMissing ProviderStatus = "status_missing"
	// StatusError means the provider has an error.
	StatusError ProviderStatus = "status_error"
)

// Validate checks if the ProviderStatus is valid.
func (s ProviderStatus) Validate() error {
	switch s {
	case StatusUnknown, StatusOK, StatusMissing, StatusError:
		return nil
	default:
		return fmt.Errorf("invalid ProviderStatus: %q", s)
	}
}

// String returns the string representation.
func (s ProviderStatus) String() string {
	return string(s)
}

// CanonicalString returns the canonical string for hashing.
func (s ProviderStatus) CanonicalString() string {
	return string(s)
}

// ============================================================================

// ConnectStatus represents whether something is connected.
type ConnectStatus string

const (
	// ConnectNo means not connected.
	ConnectNo ConnectStatus = "connect_no"
	// ConnectYes means connected.
	ConnectYes ConnectStatus = "connect_yes"
)

// Validate checks if the ConnectStatus is valid.
func (c ConnectStatus) Validate() error {
	switch c {
	case ConnectNo, ConnectYes:
		return nil
	default:
		return fmt.Errorf("invalid ConnectStatus: %q", c)
	}
}

// String returns the string representation.
func (c ConnectStatus) String() string {
	return string(c)
}

// CanonicalString returns the canonical string for hashing.
func (c ConnectStatus) CanonicalString() string {
	return string(c)
}

// ============================================================================

// SyncRecencyBucket represents how recent a sync was (bucketed).
type SyncRecencyBucket string

const (
	// SyncNever means never synced.
	SyncNever SyncRecencyBucket = "sync_never"
	// SyncRecent means synced recently.
	SyncRecent SyncRecencyBucket = "sync_recent"
	// SyncStale means sync is stale.
	SyncStale SyncRecencyBucket = "sync_stale"
)

// Validate checks if the SyncRecencyBucket is valid.
func (s SyncRecencyBucket) Validate() error {
	switch s {
	case SyncNever, SyncRecent, SyncStale:
		return nil
	default:
		return fmt.Errorf("invalid SyncRecencyBucket: %q", s)
	}
}

// String returns the string representation.
func (s SyncRecencyBucket) String() string {
	return string(s)
}

// CanonicalString returns the canonical string for hashing.
func (s SyncRecencyBucket) CanonicalString() string {
	return string(s)
}

// ============================================================================

// MagnitudeBucket represents a coarse count bucket for summaries.
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
		return fmt.Errorf("invalid MagnitudeBucket: %q", m)
	}
}

// String returns the string representation.
func (m MagnitudeBucket) String() string {
	return string(m)
}

// CanonicalString returns the canonical string for hashing.
func (m MagnitudeBucket) CanonicalString() string {
	return string(m)
}

// ComputeMagnitudeBucket returns the magnitude bucket for a count.
func ComputeMagnitudeBucket(count int) MagnitudeBucket {
	switch {
	case count == 0:
		return MagNothing
	case count <= 3:
		return MagAFew
	default:
		return MagSeveral
	}
}

// ============================================================================

// ProofHubSectionKind identifies what kind of section this is.
type ProofHubSectionKind string

const (
	// SectionIdentity is the identity section.
	SectionIdentity ProofHubSectionKind = "section_identity"
	// SectionConnections is the connections section.
	SectionConnections ProofHubSectionKind = "section_connections"
	// SectionSync is the sync section.
	SectionSync ProofHubSectionKind = "section_sync"
	// SectionShadow is the shadow section.
	SectionShadow ProofHubSectionKind = "section_shadow"
	// SectionLedger is the ledger section.
	SectionLedger ProofHubSectionKind = "section_ledger"
	// SectionInvariants is the invariants section.
	SectionInvariants ProofHubSectionKind = "section_invariants"
)

// Validate checks if the ProofHubSectionKind is valid.
func (k ProofHubSectionKind) Validate() error {
	switch k {
	case SectionIdentity, SectionConnections, SectionSync, SectionShadow, SectionLedger, SectionInvariants:
		return nil
	default:
		return fmt.Errorf("invalid ProofHubSectionKind: %q", k)
	}
}

// String returns the string representation.
func (k ProofHubSectionKind) String() string {
	return string(k)
}

// CanonicalString returns the canonical string for hashing.
func (k ProofHubSectionKind) CanonicalString() string {
	return string(k)
}

// ============================================================================

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
		return fmt.Errorf("invalid AckKind: %q", k)
	}
}

// String returns the string representation.
func (k AckKind) String() string {
	return string(k)
}

// ============================================================================
// Core Structures
// ============================================================================

// ProofHubInputs contains the inputs for building a proof hub page.
// All fields are already bucketed/hashed - no raw identifiers.
type ProofHubInputs struct {
	// CircleIDHash is the hashed circle ID (already hashed).
	CircleIDHash string
	// NowPeriodKey is the period key derived server-side (e.g., "2025-W03").
	NowPeriodKey string

	// Gmail connection status
	GmailConnected        bool
	GmailLastSyncBucket   SyncRecencyBucket
	GmailNoticedMagnitude MagnitudeBucket

	// TrueLayer connection status
	TrueLayerConnected        bool
	TrueLayerLastSyncBucket   SyncRecencyBucket
	TrueLayerNoticedMagnitude MagnitudeBucket

	// Shadow provider status
	ShadowProviderKind string // bucketed: stub | azure_openai_chat | azure_openai_embed | unknown
	ShadowRealAllowed  bool
	ShadowHealthStatus ProviderStatus

	// Device registration status
	DeviceRegistered bool

	// Ledger status
	TransparencyLinesMagnitude MagnitudeBucket
	LastLedgerPeriodBucket     SyncRecencyBucket
	EnforcementAuditRecent     bool
	InterruptPolicyConfigured  bool
}

// Validate checks all fields of the inputs.
func (i ProofHubInputs) Validate() error {
	if i.CircleIDHash == "" {
		return errors.New("CircleIDHash cannot be empty")
	}
	if i.NowPeriodKey == "" {
		return errors.New("NowPeriodKey cannot be empty")
	}
	if strings.Contains(i.NowPeriodKey, "|") {
		return errors.New("NowPeriodKey cannot contain pipe character")
	}
	if err := i.GmailLastSyncBucket.Validate(); err != nil {
		return fmt.Errorf("GmailLastSyncBucket: %w", err)
	}
	if err := i.GmailNoticedMagnitude.Validate(); err != nil {
		return fmt.Errorf("GmailNoticedMagnitude: %w", err)
	}
	if err := i.TrueLayerLastSyncBucket.Validate(); err != nil {
		return fmt.Errorf("TrueLayerLastSyncBucket: %w", err)
	}
	if err := i.TrueLayerNoticedMagnitude.Validate(); err != nil {
		return fmt.Errorf("TrueLayerNoticedMagnitude: %w", err)
	}
	if err := i.ShadowHealthStatus.Validate(); err != nil {
		return fmt.Errorf("ShadowHealthStatus: %w", err)
	}
	if err := i.TransparencyLinesMagnitude.Validate(); err != nil {
		return fmt.Errorf("TransparencyLinesMagnitude: %w", err)
	}
	if err := i.LastLedgerPeriodBucket.Validate(); err != nil {
		return fmt.Errorf("LastLedgerPeriodBucket: %w", err)
	}
	return nil
}

// CanonicalString returns the pipe-delimited canonical string for hashing.
func (i ProofHubInputs) CanonicalString() string {
	parts := []string{
		"v1",
		"circle=" + i.CircleIDHash,
		"period=" + i.NowPeriodKey,
		"gmail_conn=" + boolToStr(i.GmailConnected),
		"gmail_sync=" + i.GmailLastSyncBucket.CanonicalString(),
		"gmail_mag=" + i.GmailNoticedMagnitude.CanonicalString(),
		"truelayer_conn=" + boolToStr(i.TrueLayerConnected),
		"truelayer_sync=" + i.TrueLayerLastSyncBucket.CanonicalString(),
		"truelayer_mag=" + i.TrueLayerNoticedMagnitude.CanonicalString(),
		"shadow_kind=" + i.ShadowProviderKind,
		"shadow_real=" + boolToStr(i.ShadowRealAllowed),
		"shadow_health=" + i.ShadowHealthStatus.CanonicalString(),
		"device_reg=" + boolToStr(i.DeviceRegistered),
		"ledger_mag=" + i.TransparencyLinesMagnitude.CanonicalString(),
		"ledger_sync=" + i.LastLedgerPeriodBucket.CanonicalString(),
		"audit_recent=" + boolToStr(i.EnforcementAuditRecent),
		"interrupt_cfg=" + boolToStr(i.InterruptPolicyConfigured),
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

// ProofHubBadge is a single badge displayed in a section.
type ProofHubBadge struct {
	// Label is from a restricted set (e.g., "Gmail", "TrueLayer", "Device").
	Label string
	// Kind is the badge kind (e.g., "connection", "sync", "health").
	Kind string
	// Value is the bucketed value (never raw).
	Value string
}

// Validate checks the badge fields.
func (b ProofHubBadge) Validate() error {
	if b.Label == "" {
		return errors.New("badge Label cannot be empty")
	}
	if b.Kind == "" {
		return errors.New("badge Kind cannot be empty")
	}
	if b.Value == "" {
		return errors.New("badge Value cannot be empty")
	}
	return nil
}

// CanonicalString returns the canonical string for hashing.
func (b ProofHubBadge) CanonicalString() string {
	return fmt.Sprintf("badge|%s|%s|%s", b.Label, b.Kind, b.Value)
}

// ============================================================================

// ProofHubSection is a section of the proof hub page.
type ProofHubSection struct {
	// Kind identifies what kind of section this is.
	Kind ProofHubSectionKind
	// Title is the section title.
	Title string
	// Badges are the badges in this section.
	Badges []ProofHubBadge
	// Lines are calm descriptive lines (no IDs, no content).
	Lines []string
}

// Validate checks the section fields.
func (s ProofHubSection) Validate() error {
	if err := s.Kind.Validate(); err != nil {
		return fmt.Errorf("section Kind: %w", err)
	}
	if s.Title == "" {
		return errors.New("section Title cannot be empty")
	}
	for i, b := range s.Badges {
		if err := b.Validate(); err != nil {
			return fmt.Errorf("section badge %d: %w", i, err)
		}
	}
	return nil
}

// CanonicalString returns the canonical string for hashing.
func (s ProofHubSection) CanonicalString() string {
	var parts []string
	parts = append(parts, fmt.Sprintf("section|%s|%s", s.Kind, s.Title))
	for _, b := range s.Badges {
		parts = append(parts, b.CanonicalString())
	}
	for _, line := range s.Lines {
		parts = append(parts, "line|"+line)
	}
	return strings.Join(parts, "|")
}

// ============================================================================

// ProofHubCue is an optional cue shown on the proof hub or /today page.
type ProofHubCue struct {
	// Available indicates if the cue should be shown.
	Available bool
	// Text is the calm cue text (no urgency).
	Text string
	// Path is the link path.
	Path string
}

// Validate checks the cue fields.
func (c ProofHubCue) Validate() error {
	if c.Available && c.Text == "" {
		return errors.New("cue Text cannot be empty when Available")
	}
	if c.Available && c.Path == "" {
		return errors.New("cue Path cannot be empty when Available")
	}
	return nil
}

// ============================================================================

// ProofHubAction is an optional action on the proof hub page.
type ProofHubAction struct {
	// Label is the action label.
	Label string
	// Method is GET or POST.
	Method string
	// Path is the action path.
	Path string
	// FormFields are optional form fields (only hashes, no raw ids).
	FormFields map[string]string
}

// Validate checks the action fields.
func (a ProofHubAction) Validate() error {
	if a.Label == "" {
		return errors.New("action Label cannot be empty")
	}
	if a.Method != "GET" && a.Method != "POST" {
		return fmt.Errorf("action Method must be GET or POST, got %q", a.Method)
	}
	if a.Path == "" {
		return errors.New("action Path cannot be empty")
	}
	return nil
}

// ============================================================================

// ProofHubPage is the rendered proof hub page.
type ProofHubPage struct {
	// Title is the page title.
	Title string
	// PeriodKey is the period for this page.
	PeriodKey string
	// StatusHash is the hash of the inputs.
	StatusHash string
	// Sections are the page sections.
	Sections []ProofHubSection
	// Cue is an optional cue.
	Cue *ProofHubCue
	// PrimaryAction is an optional primary action.
	PrimaryAction *ProofHubAction
	// SecondaryAction is an optional secondary action.
	SecondaryAction *ProofHubAction
}

// Validate checks the page fields.
func (p ProofHubPage) Validate() error {
	if p.Title == "" {
		return errors.New("page Title cannot be empty")
	}
	if p.PeriodKey == "" {
		return errors.New("page PeriodKey cannot be empty")
	}
	if p.StatusHash == "" {
		return errors.New("page StatusHash cannot be empty")
	}
	for i, s := range p.Sections {
		if err := s.Validate(); err != nil {
			return fmt.Errorf("page section %d: %w", i, err)
		}
	}
	if p.Cue != nil {
		if err := p.Cue.Validate(); err != nil {
			return fmt.Errorf("page Cue: %w", err)
		}
	}
	if p.PrimaryAction != nil {
		if err := p.PrimaryAction.Validate(); err != nil {
			return fmt.Errorf("page PrimaryAction: %w", err)
		}
	}
	if p.SecondaryAction != nil {
		if err := p.SecondaryAction.Validate(); err != nil {
			return fmt.Errorf("page SecondaryAction: %w", err)
		}
	}
	return nil
}

// CanonicalString returns the canonical string for the page.
func (p ProofHubPage) CanonicalString() string {
	var parts []string
	parts = append(parts, fmt.Sprintf("page|%s|%s|%s", p.Title, p.PeriodKey, p.StatusHash))
	for _, s := range p.Sections {
		parts = append(parts, s.CanonicalString())
	}
	return strings.Join(parts, "|")
}

// ============================================================================

// ProofHubAck is an acknowledgment record for dismissing the proof hub cue.
type ProofHubAck struct {
	// CircleIDHash is the hashed circle ID.
	CircleIDHash string
	// PeriodKey is the period for this ack.
	PeriodKey string
	// StatusHash is the status hash that was acked.
	StatusHash string
	// AckKind is the kind of acknowledgment.
	AckKind AckKind
}

// Validate checks the ack fields.
func (a ProofHubAck) Validate() error {
	if a.CircleIDHash == "" {
		return errors.New("ack CircleIDHash cannot be empty")
	}
	if a.PeriodKey == "" {
		return errors.New("ack PeriodKey cannot be empty")
	}
	if a.StatusHash == "" {
		return errors.New("ack StatusHash cannot be empty")
	}
	if err := a.AckKind.Validate(); err != nil {
		return fmt.Errorf("ack AckKind: %w", err)
	}
	return nil
}

// CanonicalString returns the canonical string for hashing.
func (a ProofHubAck) CanonicalString() string {
	return fmt.Sprintf("ack|%s|%s|%s|%s", a.CircleIDHash, a.PeriodKey, a.StatusHash, a.AckKind)
}

// ============================================================================
// Hashing Functions
// ============================================================================

// HashProofHubStatus computes the SHA256 hash of the inputs canonical string.
func HashProofHubStatus(inputs ProofHubInputs) string {
	canonical := inputs.CanonicalString()
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:])
}

// ============================================================================
// Forbidden Patterns
// ============================================================================

// ForbiddenPatterns are patterns that must never appear in proof hub output.
var ForbiddenPatterns = []string{
	"@",           // email indicator
	"http://",     // URL
	"https://",    // URL
	".com",        // domain
	".org",        // domain
	".net",        // domain
	"merchant",    // raw identifier
	"sender",      // raw identifier
	"subject",     // raw identifier
	"amount",      // raw value
	"$",           // currency
	"USD",         // currency
	"GBP",         // currency
	"EUR",         // currency
}

// ValidateNoForbiddenPatterns checks that a string doesn't contain forbidden patterns.
func ValidateNoForbiddenPatterns(s string) error {
	lower := strings.ToLower(s)
	for _, pattern := range ForbiddenPatterns {
		if strings.Contains(lower, strings.ToLower(pattern)) {
			return fmt.Errorf("forbidden pattern found: %q", pattern)
		}
	}
	return nil
}

// ============================================================================
// Helper Functions
// ============================================================================

// SortSections sorts sections by kind for deterministic ordering.
func SortSections(sections []ProofHubSection) {
	order := map[ProofHubSectionKind]int{
		SectionIdentity:    0,
		SectionConnections: 1,
		SectionSync:        2,
		SectionShadow:      3,
		SectionLedger:      4,
		SectionInvariants:  5,
	}
	sort.Slice(sections, func(i, j int) bool {
		return order[sections[i].Kind] < order[sections[j].Kind]
	})
}

// SortBadges sorts badges by label for deterministic ordering.
func SortBadges(badges []ProofHubBadge) {
	sort.Slice(badges, func(i, j int) bool {
		if badges[i].Label != badges[j].Label {
			return badges[i].Label < badges[j].Label
		}
		return badges[i].Kind < badges[j].Kind
	})
}
