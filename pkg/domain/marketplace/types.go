// Package marketplace provides domain types for Phase 46: Circle Registry + Packs (Marketplace v0).
// This is a meaning-only + observer-intent-only layer.
// Packs MUST NOT grant permission to SURFACE/INTERRUPT/DELIVER/EXECUTE.
// Observer bindings are "intents" only - no real wiring occurs.
package marketplace

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
)

// PackSlug is a validated, URL-safe pack identifier.
// Format: lowercase alphanumeric with hyphens, 3-50 chars, no leading/trailing hyphens.
type PackSlug string

// slugPattern validates pack slugs.
var slugPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{1,48}[a-z0-9]$`)

// Validate checks if the PackSlug is valid.
func (s PackSlug) Validate() error {
	if s == "" {
		return errors.New("PackSlug is required")
	}
	if len(s) < 3 || len(s) > 50 {
		return fmt.Errorf("PackSlug must be 3-50 characters: %s", s)
	}
	if !slugPattern.MatchString(string(s)) {
		return fmt.Errorf("PackSlug must be lowercase alphanumeric with hyphens, no leading/trailing hyphens: %s", s)
	}
	return nil
}

// CanonicalString returns the canonical string representation.
func (s PackSlug) CanonicalString() string {
	return string(s)
}

// String returns the string representation.
func (s PackSlug) String() string {
	return string(s)
}

// PackKind represents the category of pack.
type PackKind string

const (
	PackKindSemantics        PackKind = "pack_kind_semantics"         // Semantics presets only
	PackKindObserverBinding  PackKind = "pack_kind_observer_binding"  // Observer intents only
	PackKindCombined         PackKind = "pack_kind_combined"          // Both semantics + observer intents
)

// Validate checks if the PackKind is valid.
func (k PackKind) Validate() error {
	switch k {
	case PackKindSemantics, PackKindObserverBinding, PackKindCombined:
		return nil
	default:
		return fmt.Errorf("invalid PackKind: %s", k)
	}
}

// CanonicalString returns the canonical string representation.
func (k PackKind) CanonicalString() string {
	return string(k)
}

// String returns the string representation.
func (k PackKind) String() string {
	return string(k)
}

// AllPackKinds returns all valid pack kinds in stable order.
func AllPackKinds() []PackKind {
	return []PackKind{
		PackKindSemantics,
		PackKindObserverBinding,
		PackKindCombined,
	}
}

// PackTier represents the trust/complexity tier of a pack.
type PackTier string

const (
	PackTierCurated   PackTier = "pack_tier_curated"   // Curated by system, high trust
	PackTierCommunity PackTier = "pack_tier_community" // Community contributed
	PackTierCustom    PackTier = "pack_tier_custom"    // User-created custom pack
)

// Validate checks if the PackTier is valid.
func (t PackTier) Validate() error {
	switch t {
	case PackTierCurated, PackTierCommunity, PackTierCustom:
		return nil
	default:
		return fmt.Errorf("invalid PackTier: %s", t)
	}
}

// CanonicalString returns the canonical string representation.
func (t PackTier) CanonicalString() string {
	return string(t)
}

// String returns the string representation.
func (t PackTier) String() string {
	return string(t)
}

// AllPackTiers returns all valid pack tiers in stable order.
func AllPackTiers() []PackTier {
	return []PackTier{
		PackTierCurated,
		PackTierCommunity,
		PackTierCustom,
	}
}

// PackVisibility represents pack visibility in marketplace.
type PackVisibility string

const (
	PackVisibilityPublic   PackVisibility = "pack_visibility_public"   // Visible to all
	PackVisibilityUnlisted PackVisibility = "pack_visibility_unlisted" // Accessible by slug only
	PackVisibilityPrivate  PackVisibility = "pack_visibility_private"  // User's own packs only
)

// Validate checks if the PackVisibility is valid.
func (v PackVisibility) Validate() error {
	switch v {
	case PackVisibilityPublic, PackVisibilityUnlisted, PackVisibilityPrivate:
		return nil
	default:
		return fmt.Errorf("invalid PackVisibility: %s", v)
	}
}

// CanonicalString returns the canonical string representation.
func (v PackVisibility) CanonicalString() string {
	return string(v)
}

// String returns the string representation.
func (v PackVisibility) String() string {
	return string(v)
}

// PackStatus represents the installation status of a pack.
type PackStatus string

const (
	PackStatusAvailable    PackStatus = "pack_status_available"    // Not installed
	PackStatusInstalled    PackStatus = "pack_status_installed"    // Currently installed
	PackStatusPendingApply PackStatus = "pack_status_pending_apply" // Intent recorded, not yet applied
)

// Validate checks if the PackStatus is valid.
func (s PackStatus) Validate() error {
	switch s {
	case PackStatusAvailable, PackStatusInstalled, PackStatusPendingApply:
		return nil
	default:
		return fmt.Errorf("invalid PackStatus: %s", s)
	}
}

// CanonicalString returns the canonical string representation.
func (s PackStatus) CanonicalString() string {
	return string(s)
}

// String returns the string representation.
func (s PackStatus) String() string {
	return string(s)
}

// BindingKind represents what kind of observer binding this is.
// CRITICAL: These are INTENT-ONLY - no actual wiring occurs.
type BindingKind string

const (
	BindingKindObserveOnly BindingKind = "binding_kind_observe_only" // Log/observe changes
	BindingKindAnnotate    BindingKind = "binding_kind_annotate"     // Add metadata annotations
	BindingKindEnrich      BindingKind = "binding_kind_enrich"       // Enrich with derived data
)

// Validate checks if the BindingKind is valid.
func (b BindingKind) Validate() error {
	switch b {
	case BindingKindObserveOnly, BindingKindAnnotate, BindingKindEnrich:
		return nil
	default:
		return fmt.Errorf("invalid BindingKind: %s", b)
	}
}

// CanonicalString returns the canonical string representation.
func (b BindingKind) CanonicalString() string {
	return string(b)
}

// String returns the string representation.
func (b BindingKind) String() string {
	return string(b)
}

// AllBindingKinds returns all valid binding kinds in stable order.
func AllBindingKinds() []BindingKind {
	return []BindingKind{
		BindingKindObserveOnly,
		BindingKindAnnotate,
		BindingKindEnrich,
	}
}

// PackEffect represents what power the pack grants.
// In Phase 46, this MUST always be EffectNoPower.
type PackEffect string

const (
	// EffectNoPower is the ONLY allowed value in Phase 46.
	// Packs provide meaning and intent but do NOT grant permission.
	EffectNoPower PackEffect = "effect_no_power"
)

// Validate checks if the PackEffect is valid.
// In Phase 46, only EffectNoPower is valid.
func (e PackEffect) Validate() error {
	if e == EffectNoPower {
		return nil
	}
	return fmt.Errorf("invalid PackEffect: %s (only effect_no_power allowed)", e)
}

// CanonicalString returns the canonical string representation.
func (e PackEffect) CanonicalString() string {
	return string(e)
}

// String returns the string representation.
func (e PackEffect) String() string {
	return string(e)
}

// SemanticsPreset represents default semantics to apply for a circle pattern.
type SemanticsPreset struct {
	CirclePatternHash string // Hash of pattern (e.g., "bank_*", "family_*")
	SemanticKind      string // semantic_human, semantic_institution, etc.
	UrgencyModel      string // urgency_never_interrupt, etc.
	NecessityLevel    string // necessity_low, necessity_medium, necessity_high
}

// Validate checks if the SemanticsPreset is valid.
func (p SemanticsPreset) Validate() error {
	if p.CirclePatternHash == "" {
		return errors.New("CirclePatternHash is required")
	}
	if p.SemanticKind == "" {
		return errors.New("SemanticKind is required")
	}
	if p.UrgencyModel == "" {
		return errors.New("UrgencyModel is required")
	}
	if p.NecessityLevel == "" {
		return errors.New("NecessityLevel is required")
	}
	return nil
}

// CanonicalStringV1 returns a pipe-delimited canonical string.
func (p SemanticsPreset) CanonicalStringV1() string {
	return fmt.Sprintf("%s|%s|%s|%s",
		p.CirclePatternHash,
		p.SemanticKind,
		p.UrgencyModel,
		p.NecessityLevel,
	)
}

// ObserverBinding represents an INTENT to bind an observer to a circle pattern.
// CRITICAL: This is intent-only - no real wiring occurs in Phase 46.
type ObserverBinding struct {
	CirclePatternHash string      // Hash of pattern
	ObserverSlug      string      // Which observer to bind (intent only)
	BindingKind       BindingKind // What kind of binding
	Effect            PackEffect  // MUST be EffectNoPower
}

// Validate checks if the ObserverBinding is valid.
func (b ObserverBinding) Validate() error {
	if b.CirclePatternHash == "" {
		return errors.New("CirclePatternHash is required")
	}
	if b.ObserverSlug == "" {
		return errors.New("ObserverSlug is required")
	}
	if err := b.BindingKind.Validate(); err != nil {
		return err
	}
	if err := b.Effect.Validate(); err != nil {
		return err
	}
	return nil
}

// CanonicalStringV1 returns a pipe-delimited canonical string.
func (b ObserverBinding) CanonicalStringV1() string {
	return fmt.Sprintf("%s|%s|%s|%s",
		b.CirclePatternHash,
		b.ObserverSlug,
		b.BindingKind.CanonicalString(),
		b.Effect.CanonicalString(),
	)
}

// PackTemplate represents a curated pack definition.
type PackTemplate struct {
	Slug             PackSlug          // Unique pack identifier
	Kind             PackKind          // What the pack contains
	Tier             PackTier          // Trust tier
	Visibility       PackVisibility    // Visibility in marketplace
	Title            string            // Human-readable title
	Description      string            // What this pack does
	SemanticsPresets []SemanticsPreset // Semantics defaults (may be empty)
	ObserverBindings []ObserverBinding // Observer intents (may be empty)
	Effect           PackEffect        // MUST be EffectNoPower
	VersionHash      string            // Hash of pack contents for versioning
}

// Validate checks if the PackTemplate is valid.
func (p PackTemplate) Validate() error {
	if err := p.Slug.Validate(); err != nil {
		return err
	}
	if err := p.Kind.Validate(); err != nil {
		return err
	}
	if err := p.Tier.Validate(); err != nil {
		return err
	}
	if err := p.Visibility.Validate(); err != nil {
		return err
	}
	if p.Title == "" {
		return errors.New("Title is required")
	}
	if p.Description == "" {
		return errors.New("Description is required")
	}
	if err := p.Effect.Validate(); err != nil {
		return err
	}
	// Validate all presets
	for i, preset := range p.SemanticsPresets {
		if err := preset.Validate(); err != nil {
			return fmt.Errorf("SemanticsPreset[%d]: %w", i, err)
		}
	}
	// Validate all bindings
	for i, binding := range p.ObserverBindings {
		if err := binding.Validate(); err != nil {
			return fmt.Errorf("ObserverBinding[%d]: %w", i, err)
		}
	}
	// Validate kind matches contents
	switch p.Kind {
	case PackKindSemantics:
		if len(p.SemanticsPresets) == 0 {
			return errors.New("PackKindSemantics requires at least one SemanticsPreset")
		}
		if len(p.ObserverBindings) > 0 {
			return errors.New("PackKindSemantics must not have ObserverBindings")
		}
	case PackKindObserverBinding:
		if len(p.ObserverBindings) == 0 {
			return errors.New("PackKindObserverBinding requires at least one ObserverBinding")
		}
		if len(p.SemanticsPresets) > 0 {
			return errors.New("PackKindObserverBinding must not have SemanticsPresets")
		}
	case PackKindCombined:
		if len(p.SemanticsPresets) == 0 && len(p.ObserverBindings) == 0 {
			return errors.New("PackKindCombined requires at least one preset or binding")
		}
	}
	return nil
}

// CanonicalStringV1 returns a pipe-delimited canonical string.
func (p PackTemplate) CanonicalStringV1() string {
	presetsCanonical := ""
	for i, preset := range p.SemanticsPresets {
		if i > 0 {
			presetsCanonical += ";"
		}
		presetsCanonical += preset.CanonicalStringV1()
	}
	bindingsCanonical := ""
	for i, binding := range p.ObserverBindings {
		if i > 0 {
			bindingsCanonical += ";"
		}
		bindingsCanonical += binding.CanonicalStringV1()
	}
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s|[%s]|[%s]|%s",
		p.Slug.CanonicalString(),
		p.Kind.CanonicalString(),
		p.Tier.CanonicalString(),
		p.Visibility.CanonicalString(),
		p.Title,
		p.Description,
		presetsCanonical,
		bindingsCanonical,
		p.Effect.CanonicalString(),
	)
}

// ComputeVersionHash computes a version hash for the pack contents.
func (p PackTemplate) ComputeVersionHash() string {
	return HashString(p.CanonicalStringV1())
}

// PackInstallIntent represents the user's intent to install a pack.
// This is a declaration of intent, not an action.
type PackInstallIntent struct {
	PackSlugHash  string     // Hash of pack slug
	VersionHash   string     // Version of pack being installed
	Effect        PackEffect // MUST be EffectNoPower
}

// Validate checks if the PackInstallIntent is valid.
func (i PackInstallIntent) Validate() error {
	if i.PackSlugHash == "" {
		return errors.New("PackSlugHash is required")
	}
	if i.VersionHash == "" {
		return errors.New("VersionHash is required")
	}
	if err := i.Effect.Validate(); err != nil {
		return err
	}
	return nil
}

// CanonicalStringV1 returns a pipe-delimited canonical string.
func (i PackInstallIntent) CanonicalStringV1() string {
	return fmt.Sprintf("%s|%s|%s",
		i.PackSlugHash,
		i.VersionHash,
		i.Effect.CanonicalString(),
	)
}

// PackInstallRecord represents a stored pack installation.
type PackInstallRecord struct {
	PeriodKey     string     // Daily bucket key (YYYY-MM-DD)
	PackSlugHash  string     // Hash of pack slug
	VersionHash   string     // Version installed
	StatusHash    string     // sha256(PeriodKey|PackSlugHash|VersionHash)
	Status        PackStatus // Installation status
	Effect        PackEffect // MUST be EffectNoPower
}

// Validate checks if the PackInstallRecord is valid.
func (r PackInstallRecord) Validate() error {
	if r.PeriodKey == "" {
		return errors.New("PeriodKey is required")
	}
	if r.PackSlugHash == "" {
		return errors.New("PackSlugHash is required")
	}
	if r.VersionHash == "" {
		return errors.New("VersionHash is required")
	}
	if r.StatusHash == "" {
		return errors.New("StatusHash is required")
	}
	if err := r.Status.Validate(); err != nil {
		return err
	}
	if err := r.Effect.Validate(); err != nil {
		return err
	}
	return nil
}

// CanonicalStringV1 returns a pipe-delimited canonical string.
func (r PackInstallRecord) CanonicalStringV1() string {
	return fmt.Sprintf("%s|%s|%s|%s|%s",
		r.PeriodKey,
		r.PackSlugHash,
		r.VersionHash,
		r.Status.CanonicalString(),
		r.Effect.CanonicalString(),
	)
}

// PackRemovalRecord represents a pack removal event.
type PackRemovalRecord struct {
	PeriodKey    string // Daily bucket key (YYYY-MM-DD)
	PackSlugHash string // Hash of removed pack
	VersionHash  string // Version that was removed
	StatusHash   string // sha256(PeriodKey|PackSlugHash|"removed")
}

// Validate checks if the PackRemovalRecord is valid.
func (r PackRemovalRecord) Validate() error {
	if r.PeriodKey == "" {
		return errors.New("PeriodKey is required")
	}
	if r.PackSlugHash == "" {
		return errors.New("PackSlugHash is required")
	}
	if r.VersionHash == "" {
		return errors.New("VersionHash is required")
	}
	if r.StatusHash == "" {
		return errors.New("StatusHash is required")
	}
	return nil
}

// CanonicalStringV1 returns a pipe-delimited canonical string.
func (r PackRemovalRecord) CanonicalStringV1() string {
	return fmt.Sprintf("%s|%s|%s|removed",
		r.PeriodKey,
		r.PackSlugHash,
		r.VersionHash,
	)
}

// MarketplaceHomePage represents the UI model for the marketplace home.
type MarketplaceHomePage struct {
	Title         string
	Lines         []string
	AvailablePacks []PackCard
	InstalledPacks []PackCard
	StatusHash    string
}

// PackCard represents a pack in the marketplace list UI.
type PackCard struct {
	SlugHash     string
	Title        string
	Description  string
	Kind         PackKind
	Tier         PackTier
	Status       PackStatus
	VersionHash  string
	Effect       PackEffect // Always EffectNoPower
}

// Validate checks if the PackCard is valid.
func (c PackCard) Validate() error {
	if c.SlugHash == "" {
		return errors.New("SlugHash is required")
	}
	if c.Title == "" {
		return errors.New("Title is required")
	}
	if err := c.Kind.Validate(); err != nil {
		return err
	}
	if err := c.Tier.Validate(); err != nil {
		return err
	}
	if err := c.Status.Validate(); err != nil {
		return err
	}
	if err := c.Effect.Validate(); err != nil {
		return err
	}
	return nil
}

// PackDetailPage represents the UI model for a pack detail view.
type PackDetailPage struct {
	Title            string
	Lines            []string
	Pack             PackCard
	SemanticsPresets []SemanticsPresetDisplay
	ObserverBindings []ObserverBindingDisplay
	CanInstall       bool
	CanRemove        bool
	StatusHash       string
}

// SemanticsPresetDisplay represents a preset for display.
type SemanticsPresetDisplay struct {
	PatternHash    string
	SemanticKind   string
	UrgencyModel   string
	NecessityLevel string
}

// ObserverBindingDisplay represents a binding for display.
type ObserverBindingDisplay struct {
	PatternHash  string
	ObserverSlug string
	BindingKind  BindingKind
	Effect       PackEffect
}

// MarketplaceProofPage represents the UI model for marketplace proof.
type MarketplaceProofPage struct {
	Title          string
	Lines          []string
	InstalledPacks []InstalledProofLine
	RemovedPacks   []RemovedProofLine
	StatusHash     string
}

// InstalledProofLine represents an installed pack in proof view.
type InstalledProofLine struct {
	PackSlugHash   string
	VersionHash    string
	InstalledDate  string // PeriodKey when installed
	PresetsCount   int
	BindingsCount  int
	Effect         PackEffect // Always EffectNoPower
}

// RemovedProofLine represents a removed pack in proof view.
type RemovedProofLine struct {
	PackSlugHash  string
	VersionHash   string
	RemovedDate   string // PeriodKey when removed
}

// MarketplaceProofAck represents an acknowledgment of marketplace proof.
type MarketplaceProofAck struct {
	PeriodKey  string
	StatusHash string
	AckKind    string // "viewed"|"dismissed"
}

// Allowed ack kinds.
const (
	AckKindViewed    = "viewed"
	AckKindDismissed = "dismissed"
)

// Validate checks if the ack is valid.
func (a MarketplaceProofAck) Validate() error {
	if a.PeriodKey == "" {
		return errors.New("PeriodKey is required")
	}
	if a.StatusHash == "" {
		return errors.New("StatusHash is required")
	}
	switch a.AckKind {
	case AckKindViewed, AckKindDismissed:
		return nil
	default:
		return fmt.Errorf("invalid AckKind: %s", a.AckKind)
	}
}

// CanonicalStringV1 returns a pipe-delimited canonical string.
func (a MarketplaceProofAck) CanonicalStringV1() string {
	return fmt.Sprintf("%s|%s|%s", a.PeriodKey, a.StatusHash, a.AckKind)
}

// MarketplaceCue represents the whisper cue for marketplace.
type MarketplaceCue struct {
	Available  bool
	Text       string // "Explore packs for your circles."
	Path       string // "/marketplace"
	StatusHash string
}

// HashString computes SHA256 hash of a string and returns hex-encoded result.
func HashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// ComputeStatusHash computes the status hash for a record.
func ComputeStatusHash(periodKey, packSlugHash, versionHash string) string {
	canonical := fmt.Sprintf("%s|%s|%s", periodKey, packSlugHash, versionHash)
	return HashString(canonical)
}

// ComputeRemovalStatusHash computes the status hash for a removal record.
func ComputeRemovalStatusHash(periodKey, packSlugHash string) string {
	canonical := fmt.Sprintf("%s|%s|removed", periodKey, packSlugHash)
	return HashString(canonical)
}

// MaxDisplayPacks is the maximum number of packs to display in UI.
const MaxDisplayPacks = 50

// PackCountBucket returns a bucketed count description.
func PackCountBucket(count int) string {
	if count == 0 {
		return "nothing"
	}
	if count <= 3 {
		return "a_few"
	}
	return "several"
}
