// Package shadowllm provides types for LLM shadow-mode observation.
//
// Phase 19: LLM Shadow-Mode Contract
// Phase 19.3: Azure OpenAI Shadow Provider
//
// CRITICAL INVARIANTS:
//   - Shadow mode emits METADATA ONLY (scores, deltas, confidence) - never content
//   - Shadow mode can NEVER: surface UI text, create obligations/drafts,
//     alter interruption levels, trigger execution, write back to providers
//   - Shadow mode is OFF by default, requires explicit config flag
//   - No goroutines in pkg/domain/ or internal/. No time.Now() - clock injection only.
//   - Stub provider: Deterministic (same inputs + same seed + same clock => identical outputs/hashes)
//   - Real providers: Non-deterministic OK but receipts are auditable with provenance
//   - Stdlib only. No external dependencies.
//
// Reference: docs/ADR/ADR-0043-phase19-shadow-mode-contract.md
// Reference: docs/ADR/ADR-0044-phase19-3-azure-openai-shadow-provider.md
package shadowllm

import (
	"time"

	"quantumlife/pkg/domain/identity"
)

// ShadowMode defines the shadow-mode operation mode.
type ShadowMode string

const (
	// ShadowModeOff means shadow-mode is disabled (default).
	ShadowModeOff ShadowMode = "off"

	// ShadowModeObserve means shadow-mode runs after the quiet loop, emitting metadata only.
	ShadowModeObserve ShadowMode = "observe"
)

// IsEnabled returns true if shadow mode is not "off".
func (m ShadowMode) IsEnabled() bool {
	return m == ShadowModeObserve
}

// Validate checks if the mode is valid.
func (m ShadowMode) Validate() bool {
	return m == ShadowModeOff || m == ShadowModeObserve
}

// =============================================================================
// Phase 19.3: Provider Kind and Provenance Types
// =============================================================================

// ProviderKind identifies the shadow LLM provider type.
//
// CRITICAL: Only "stub" is fully deterministic.
// Real providers (azure_openai) produce non-deterministic results but are auditable.
type ProviderKind string

const (
	// ProviderKindNone means no provider is configured.
	ProviderKindNone ProviderKind = "none"

	// ProviderKindStub is the deterministic stub provider (default).
	ProviderKindStub ProviderKind = "stub"

	// ProviderKindAzureOpenAI is the Azure OpenAI provider.
	// CRITICAL: Requires explicit opt-in and consent.
	ProviderKindAzureOpenAI ProviderKind = "azure_openai"

	// ProviderKindLocalSLM is a placeholder for future local SLM support.
	// NOT IMPLEMENTED in Phase 19.3.
	ProviderKindLocalSLM ProviderKind = "local_slm"
)

// Validate checks if the provider kind is valid.
func (p ProviderKind) Validate() bool {
	switch p {
	case ProviderKindNone, ProviderKindStub, ProviderKindAzureOpenAI, ProviderKindLocalSLM:
		return true
	default:
		return false
	}
}

// IsReal returns true if this is a real (non-stub) provider.
func (p ProviderKind) IsReal() bool {
	return p == ProviderKindAzureOpenAI || p == ProviderKindLocalSLM
}

// RequiresConsent returns true if this provider requires explicit consent.
func (p ProviderKind) RequiresConsent() bool {
	return p.IsReal()
}

// LatencyBucket indicates response latency in abstract buckets.
// Computed in cmd layer only (not in internal/).
type LatencyBucket string

const (
	// LatencyFast indicates response under 1 second.
	LatencyFast LatencyBucket = "fast"

	// LatencyMedium indicates response 1-5 seconds.
	LatencyMedium LatencyBucket = "medium"

	// LatencySlow indicates response over 5 seconds.
	LatencySlow LatencyBucket = "slow"

	// LatencyTimeout indicates the request timed out.
	LatencyTimeout LatencyBucket = "timeout"

	// LatencyNA indicates latency is not applicable (stub provider).
	LatencyNA LatencyBucket = "na"
)

// Validate checks if the latency bucket is valid.
func (l LatencyBucket) Validate() bool {
	switch l {
	case LatencyFast, LatencyMedium, LatencySlow, LatencyTimeout, LatencyNA:
		return true
	default:
		return false
	}
}

// ReceiptStatus indicates the outcome of a shadow run.
type ReceiptStatus string

const (
	// ReceiptStatusSuccess means the shadow run completed successfully.
	ReceiptStatusSuccess ReceiptStatus = "success"

	// ReceiptStatusDisabled means shadow mode is disabled.
	ReceiptStatusDisabled ReceiptStatus = "disabled"

	// ReceiptStatusNotPermitted means the provider is not permitted (missing consent).
	ReceiptStatusNotPermitted ReceiptStatus = "not_permitted"

	// ReceiptStatusFailed means the shadow run failed (network error, etc).
	ReceiptStatusFailed ReceiptStatus = "failed"

	// ReceiptStatusInvalidOutput means the model output was invalid.
	ReceiptStatusInvalidOutput ReceiptStatus = "invalid_output"

	// ReceiptStatusPrivacyBlocked means input failed privacy validation.
	ReceiptStatusPrivacyBlocked ReceiptStatus = "privacy_blocked"
)

// Validate checks if the status is valid.
func (s ReceiptStatus) Validate() bool {
	switch s {
	case ReceiptStatusSuccess, ReceiptStatusDisabled, ReceiptStatusNotPermitted,
		ReceiptStatusFailed, ReceiptStatusInvalidOutput, ReceiptStatusPrivacyBlocked:
		return true
	default:
		return false
	}
}

// Provenance captures provider and request metadata for auditability.
//
// CRITICAL: Contains NO secrets (API keys, tokens).
// Contains ONLY metadata for audit trail.
type Provenance struct {
	// ProviderKind identifies the provider type.
	ProviderKind ProviderKind

	// ModelOrDeployment is the model/deployment name (e.g., "gpt-4o-mini").
	// May be empty for stub provider.
	ModelOrDeployment string

	// RequestPolicyHash is a hash of the privacy policy version used.
	// Allows tracking which policy version was in effect.
	RequestPolicyHash string

	// PromptTemplateVersion identifies the prompt template version.
	PromptTemplateVersion string

	// LatencyBucket indicates response latency (computed in cmd layer).
	LatencyBucket LatencyBucket

	// Status indicates the outcome of the shadow run.
	Status ReceiptStatus

	// ErrorBucket contains an abstract error category if failed.
	// Never contains actual error messages or stack traces.
	ErrorBucket string
}

// Validate checks if the provenance is valid.
func (p *Provenance) Validate() error {
	if !p.ProviderKind.Validate() {
		return ErrInvalidProviderKind
	}
	if !p.LatencyBucket.Validate() {
		return ErrInvalidLatencyBucket
	}
	if !p.Status.Validate() {
		return ErrInvalidReceiptStatus
	}
	return nil
}

// CanonicalString returns the pipe-delimited canonical representation.
func (p *Provenance) CanonicalString() string {
	return "PROVENANCE|v1|" +
		string(p.ProviderKind) + "|" +
		p.ModelOrDeployment + "|" +
		p.RequestPolicyHash + "|" +
		p.PromptTemplateVersion + "|" +
		string(p.LatencyBucket) + "|" +
		string(p.Status) + "|" +
		p.ErrorBucket
}

// Additional error types for Phase 19.3
const (
	ErrInvalidProviderKind  shadowError = "invalid provider kind"
	ErrInvalidLatencyBucket shadowError = "invalid latency bucket"
	ErrInvalidReceiptStatus shadowError = "invalid receipt status"
	ErrProviderNotPermitted shadowError = "provider not permitted"
	ErrPrivacyViolation     shadowError = "privacy violation in input"
	ErrInvalidModelOutput   shadowError = "invalid model output"
	ErrProviderTimeout      shadowError = "provider request timed out"
	ErrProviderError        shadowError = "provider error"
	ErrWhyGenericTooLong    shadowError = "why_generic exceeds max length"
)

// ShadowSignalKind identifies the type of signal emitted by shadow-mode.
type ShadowSignalKind string

const (
	// SignalKindRegretDelta represents a suggested change to regret score.
	SignalKindRegretDelta ShadowSignalKind = "regret_delta"

	// SignalKindCategoryPressure represents pressure to surface a category.
	SignalKindCategoryPressure ShadowSignalKind = "category_pressure"

	// SignalKindConfidence represents confidence in current state.
	SignalKindConfidence ShadowSignalKind = "confidence"

	// SignalKindLabelSuggestion represents a suggested label/category.
	SignalKindLabelSuggestion ShadowSignalKind = "label_suggestion"
)

// Validate checks if the signal kind is valid.
func (k ShadowSignalKind) Validate() bool {
	switch k {
	case SignalKindRegretDelta, SignalKindCategoryPressure,
		SignalKindConfidence, SignalKindLabelSuggestion:
		return true
	default:
		return false
	}
}

// AbstractCategory represents abstract categories (no identifiable info).
type AbstractCategory string

const (
	CategoryMoney   AbstractCategory = "money"
	CategoryTime    AbstractCategory = "time"
	CategoryPeople  AbstractCategory = "people"
	CategoryWork    AbstractCategory = "work"
	CategoryHome    AbstractCategory = "home"
	CategoryHealth  AbstractCategory = "health"
	CategoryFamily  AbstractCategory = "family"
	CategorySchool  AbstractCategory = "school"
	CategoryUnknown AbstractCategory = "unknown"
)

// AllCategories returns all valid abstract categories in sorted order.
// Used for deterministic ordering in hashing.
func AllCategories() []AbstractCategory {
	return []AbstractCategory{
		CategoryFamily, CategoryHealth, CategoryHome, CategoryMoney,
		CategoryPeople, CategorySchool, CategoryTime, CategoryUnknown, CategoryWork,
	}
}

// Validate checks if the category is valid.
func (c AbstractCategory) Validate() bool {
	switch c {
	case CategoryMoney, CategoryTime, CategoryPeople, CategoryWork, CategoryHome,
		CategoryHealth, CategoryFamily, CategorySchool, CategoryUnknown:
		return true
	default:
		return false
	}
}

// ShadowSignal represents a single metadata signal from shadow-mode.
//
// CRITICAL: Contains NO content, NO identifiable information.
// Only: kind, circle, item hash, abstract category, float values, and hashes.
type ShadowSignal struct {
	// Kind identifies the type of signal.
	Kind ShadowSignalKind

	// CircleID is the circle this signal relates to.
	CircleID identity.EntityID

	// ItemKeyHash is a SHA256 hash of the item key (not the item itself).
	ItemKeyHash string

	// Category is the abstract category (money, time, people, etc.)
	Category AbstractCategory

	// ValueFloat is the primary numeric value (e.g., delta, pressure level).
	// Range: -1.0 to 1.0
	ValueFloat float64

	// ConfidenceFloat is the confidence in this signal.
	// Range: 0.0 to 1.0
	ConfidenceFloat float64

	// NotesHash is a hash of any internal notes (NOT the notes themselves).
	// This ensures no content leaks through signals.
	NotesHash string

	// CreatedAt is when this signal was generated (injected clock).
	CreatedAt time.Time
}

// Validate checks if the signal is valid.
func (s *ShadowSignal) Validate() error {
	if !s.Kind.Validate() {
		return ErrInvalidSignalKind
	}
	if s.CircleID == "" {
		return ErrMissingCircleID
	}
	if s.ItemKeyHash == "" {
		return ErrMissingItemKeyHash
	}
	if !s.Category.Validate() {
		return ErrInvalidCategory
	}
	if s.ValueFloat < -1.0 || s.ValueFloat > 1.0 {
		return ErrValueOutOfRange
	}
	if s.ConfidenceFloat < 0.0 || s.ConfidenceFloat > 1.0 {
		return ErrConfidenceOutOfRange
	}
	if s.CreatedAt.IsZero() {
		return ErrMissingCreatedAt
	}
	return nil
}

// ShadowRun represents a complete shadow-mode observation run.
//
// CRITICAL: Contains only metadata. Never content or identifiable info.
type ShadowRun struct {
	// RunID uniquely identifies this run.
	RunID string

	// CircleID is the circle this run is for.
	CircleID identity.EntityID

	// InputsHash is a SHA256 hash of the abstract inputs (not the inputs themselves).
	InputsHash string

	// ModelSpec identifies the model used (name only, no API keys/secrets).
	ModelSpec string

	// Seed is the deterministic seed for reproducibility.
	Seed int64

	// Signals are the metadata signals produced by this run.
	// Max 5 signals per run.
	Signals []ShadowSignal

	// CreatedAt is when this run was created (injected clock).
	CreatedAt time.Time

	// hash is cached after first computation.
	hash string
}

// MaxSignalsPerRun is the maximum number of signals allowed per run.
const MaxSignalsPerRun = 5

// Validate checks if the run is valid.
func (r *ShadowRun) Validate() error {
	if r.RunID == "" {
		return ErrMissingRunID
	}
	if r.CircleID == "" {
		return ErrMissingCircleID
	}
	if r.InputsHash == "" {
		return ErrMissingInputsHash
	}
	if r.ModelSpec == "" {
		return ErrMissingModelSpec
	}
	if len(r.Signals) > MaxSignalsPerRun {
		return ErrTooManySignals
	}
	if r.CreatedAt.IsZero() {
		return ErrMissingCreatedAt
	}

	for i := range r.Signals {
		if err := r.Signals[i].Validate(); err != nil {
			return err
		}
	}

	return nil
}

// Error types for validation.
type shadowError string

func (e shadowError) Error() string { return string(e) }

const (
	ErrInvalidSignalKind    shadowError = "invalid signal kind"
	ErrMissingCircleID      shadowError = "missing circle ID"
	ErrMissingItemKeyHash   shadowError = "missing item key hash"
	ErrInvalidCategory      shadowError = "invalid category"
	ErrValueOutOfRange      shadowError = "value out of range [-1.0, 1.0]"
	ErrConfidenceOutOfRange shadowError = "confidence out of range [0.0, 1.0]"
	ErrMissingCreatedAt     shadowError = "missing created at timestamp"
	ErrMissingRunID         shadowError = "missing run ID"
	ErrMissingInputsHash    shadowError = "missing inputs hash"
	ErrMissingModelSpec     shadowError = "missing model spec"
	ErrTooManySignals       shadowError = "too many signals (max 5)"
	ErrInvalidHorizon       shadowError = "invalid horizon"
	ErrInvalidMagnitude     shadowError = "invalid magnitude"
	ErrInvalidConfidence    shadowError = "invalid confidence bucket"
	ErrInvalidSuggestionTyp shadowError = "invalid suggestion type"
	ErrMissingReceiptID     shadowError = "missing receipt ID"
	ErrMissingWindowBucket  shadowError = "missing window bucket"
	ErrTooManySuggestions   shadowError = "too many suggestions (max 5)"
)

// =============================================================================
// Phase 19.2: LLM Shadow Mode Contract Types
// =============================================================================

// Horizon indicates when something is relevant.
// CRITICAL: Abstract bucket only - never specific dates.
type Horizon string

const (
	HorizonNow     Horizon = "now"     // Needs attention today
	HorizonSoon    Horizon = "soon"    // Within a few days
	HorizonLater   Horizon = "later"   // Within a week or two
	HorizonSomeday Horizon = "someday" // Eventually, no urgency
)

// Validate checks if the horizon is valid.
func (h Horizon) Validate() bool {
	switch h {
	case HorizonNow, HorizonSoon, HorizonLater, HorizonSomeday:
		return true
	default:
		return false
	}
}

// MagnitudeBucket indicates relative quantity without exposing raw counts.
// CRITICAL: Abstract bucket only - never specific numbers.
type MagnitudeBucket string

const (
	MagnitudeNothing MagnitudeBucket = "nothing" // Zero items
	MagnitudeAFew    MagnitudeBucket = "a_few"   // 1-3 items
	MagnitudeSeveral MagnitudeBucket = "several" // 4+ items
)

// Validate checks if the magnitude is valid.
func (m MagnitudeBucket) Validate() bool {
	switch m {
	case MagnitudeNothing, MagnitudeAFew, MagnitudeSeveral:
		return true
	default:
		return false
	}
}

// MagnitudeFromCount converts a raw count to a magnitude bucket.
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

// ConfidenceBucket indicates model confidence without exposing raw scores.
type ConfidenceBucket string

const (
	ConfidenceLow  ConfidenceBucket = "low"  // 0.0 - 0.33
	ConfidenceMed  ConfidenceBucket = "med"  // 0.33 - 0.66
	ConfidenceHigh ConfidenceBucket = "high" // 0.66 - 1.0
)

// Validate checks if the confidence bucket is valid.
func (c ConfidenceBucket) Validate() bool {
	switch c {
	case ConfidenceLow, ConfidenceMed, ConfidenceHigh:
		return true
	default:
		return false
	}
}

// ConfidenceFromFloat converts a raw confidence to a bucket.
func ConfidenceFromFloat(f float64) ConfidenceBucket {
	switch {
	case f < 0.33:
		return ConfidenceLow
	case f < 0.66:
		return ConfidenceMed
	default:
		return ConfidenceHigh
	}
}

// SuggestionType indicates what the shadow model thinks might be appropriate.
// CRITICAL: This is LOGGED ONLY. It does NOT affect actual behavior.
type SuggestionType string

const (
	// SuggestHold means the model thinks this should remain held.
	SuggestHold SuggestionType = "hold"

	// SuggestSurfaceCandidate means the model thinks this could surface.
	// CRITICAL: This does NOT actually surface anything.
	SuggestSurfaceCandidate SuggestionType = "surface_candidate"

	// SuggestDraftCandidate means the model thinks a draft might help.
	// CRITICAL: This does NOT create any draft.
	SuggestDraftCandidate SuggestionType = "draft_candidate"
)

// Validate checks if the suggestion type is valid.
func (s SuggestionType) Validate() bool {
	switch s {
	case SuggestHold, SuggestSurfaceCandidate, SuggestDraftCandidate:
		return true
	default:
		return false
	}
}

// ShadowSuggestion represents a single abstract suggestion from shadow analysis.
//
// CRITICAL: Contains NO content, NO identifiable information.
// Only abstract categories and buckets.
type ShadowSuggestion struct {
	// Category is the abstract category (money, time, people, etc.)
	Category AbstractCategory

	// Horizon indicates when this might be relevant.
	Horizon Horizon

	// Magnitude indicates how much is present.
	Magnitude MagnitudeBucket

	// Confidence indicates model confidence.
	Confidence ConfidenceBucket

	// SuggestionType indicates what the model thinks might be appropriate.
	// CRITICAL: This is for observation ONLY. Does NOT affect behavior.
	SuggestionType SuggestionType

	// ItemKeyHash is a SHA256 hash of the related item (not the item itself).
	ItemKeyHash string
}

// Validate checks if the suggestion is valid.
func (s *ShadowSuggestion) Validate() error {
	if !s.Category.Validate() {
		return ErrInvalidCategory
	}
	if !s.Horizon.Validate() {
		return ErrInvalidHorizon
	}
	if !s.Magnitude.Validate() {
		return ErrInvalidMagnitude
	}
	if !s.Confidence.Validate() {
		return ErrInvalidConfidence
	}
	if !s.SuggestionType.Validate() {
		return ErrInvalidSuggestionTyp
	}
	return nil
}

// MaxSuggestionsPerReceipt is the maximum suggestions allowed per receipt.
const MaxSuggestionsPerReceipt = 5

// ShadowReceipt represents a privacy-safe receipt of shadow analysis.
//
// Phase 19.2: LLM Shadow Mode Contract
// Phase 19.3: Extended with Provenance and WhyGeneric
//
// CRITICAL: Contains ONLY abstract data.
// CRITICAL: No raw text, emails, amounts, dates, or identifiable info.
// CRITICAL: This is for AUDIT/OBSERVATION only. Does NOT affect behavior.
type ShadowReceipt struct {
	// ReceiptID uniquely identifies this receipt.
	ReceiptID string

	// CircleID is the circle this receipt is for.
	CircleID identity.EntityID

	// WindowBucket is the time bucket (e.g., "2024-01-15" day bucket).
	// Uses injected clock, bucketed to day.
	WindowBucket string

	// InputDigestHash is a SHA256 hash of the abstract input digest.
	// This allows replay verification without storing inputs.
	InputDigestHash string

	// Suggestions are the abstract suggestions from the analysis.
	// Max 5 suggestions per receipt.
	Suggestions []ShadowSuggestion

	// ModelSpec identifies the model used (e.g., "stub", "deterministic-v1").
	ModelSpec string

	// CreatedAt is when this receipt was created (injected clock).
	CreatedAt time.Time

	// Phase 19.3: Provenance for auditability
	// Provenance captures provider and request metadata.
	Provenance Provenance

	// Phase 19.3: WhyGeneric is a short generic rationale from the model.
	// CRITICAL: Must be validated to contain no identifiable information.
	// Max 140 characters. May be empty for stub provider.
	WhyGeneric string

	// hash is cached after first computation.
	hash string
}

// MaxWhyGenericLength is the maximum length for WhyGeneric field.
const MaxWhyGenericLength = 140

// Validate checks if the receipt is valid.
func (r *ShadowReceipt) Validate() error {
	if r.ReceiptID == "" {
		return ErrMissingReceiptID
	}
	if r.CircleID == "" {
		return ErrMissingCircleID
	}
	if r.WindowBucket == "" {
		return ErrMissingWindowBucket
	}
	if r.InputDigestHash == "" {
		return ErrMissingInputsHash
	}
	if r.ModelSpec == "" {
		return ErrMissingModelSpec
	}
	if len(r.Suggestions) > MaxSuggestionsPerReceipt {
		return ErrTooManySuggestions
	}
	if r.CreatedAt.IsZero() {
		return ErrMissingCreatedAt
	}

	for i := range r.Suggestions {
		if err := r.Suggestions[i].Validate(); err != nil {
			return err
		}
	}

	// Phase 19.3: Validate provenance
	if err := r.Provenance.Validate(); err != nil {
		return err
	}

	// Phase 19.3: Validate WhyGeneric length
	if len(r.WhyGeneric) > MaxWhyGenericLength {
		return ErrWhyGenericTooLong
	}

	return nil
}

// ShadowInputDigest contains abstract, pre-bucketed data for shadow analysis.
//
// CRITICAL: This struct contains ONLY abstract data derived from safe sources.
// All counts are already bucketed. No raw content is ever present.
type ShadowInputDigest struct {
	// CircleID is the circle being analyzed.
	CircleID identity.EntityID

	// ObligationCountByCategory maps category => magnitude bucket.
	ObligationCountByCategory map[AbstractCategory]MagnitudeBucket

	// HeldCountByCategory maps category => magnitude bucket of held items.
	HeldCountByCategory map[AbstractCategory]MagnitudeBucket

	// SurfaceCandidateCount is the magnitude bucket of surface candidates.
	SurfaceCandidateCount MagnitudeBucket

	// DraftCandidateCount is the magnitude bucket of draft candidates.
	DraftCandidateCount MagnitudeBucket

	// TriggersSeenBucket indicates if any triggers were seen.
	TriggersSeen bool

	// MirrorBucket indicates the mirror summary state.
	MirrorBucket MagnitudeBucket
}
