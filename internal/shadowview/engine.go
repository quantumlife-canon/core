// Package shadowview provides the shadow receipt viewer for Phase 21 and Phase 27.
//
// Phase 21: Unified Onboarding + Shadow Receipt Viewer
// Phase 27: Real Shadow Receipt (Primary Proof of Intelligence, Zero Pressure)
//
// CRITICAL INVARIANTS:
//   - Deterministic projection from existing receipts
//   - No goroutines. No time.Now().
//   - Stdlib only.
//   - Shows ONLY abstract buckets and hashes
//   - Shadow remains observation-only (Phase 27)
//   - Shadow never alters runtime behavior (Phase 27)
//
// Reference: docs/ADR/ADR-0051-phase21-onboarding-modes-shadow-receipt-viewer.md
// Reference: docs/ADR/ADR-0058-phase27-real-shadow-receipt-primary-proof.md
package shadowview

import (
	"sort"
	"time"

	"quantumlife/pkg/domain/shadowllm"
	domainshadowview "quantumlife/pkg/domain/shadowview"
)

// Engine builds shadow receipt page views.
//
// CRITICAL: Engine does NOT store state.
// CRITICAL: Engine does NOT spawn goroutines.
// CRITICAL: Engine uses clock injection for determinism.
type Engine struct {
	clock func() time.Time
}

// NewEngine creates a new shadow view engine.
func NewEngine(clock func() time.Time) *Engine {
	return &Engine{
		clock: clock,
	}
}

// BuildPageInput contains the inputs needed to build the receipt page.
type BuildPageInput struct {
	// Receipt is the shadow receipt to display (may be nil).
	Receipt *shadowllm.ShadowReceipt

	// HasGmailConnection indicates if Gmail is connected.
	HasGmailConnection bool

	// CalibrationAgreement is the calibration agreement bucket (if Phase 19.4+).
	// Empty string if no calibration.
	CalibrationAgreement string

	// CalibrationVote is the usefulness vote (if recorded).
	CalibrationVote string
}

// BuildPage creates the shadow receipt page view.
//
// CRITICAL: Deterministic projection - same input => same output.
// CRITICAL: Shows ONLY abstract buckets and hashes.
func (e *Engine) BuildPage(input BuildPageInput) ShadowReceiptPage {
	page := ShadowReceiptPage{
		HasReceipt: input.Receipt != nil,
	}

	// Source section
	page.Source = e.buildSourceSection(input.HasGmailConnection)

	if input.Receipt == nil {
		// No receipt - return minimal page
		page.Observation = ObservationSection{
			Magnitude: MagnitudeDisplayNothing,
			Statement: "No observations yet.",
		}
		page.Confidence = ConfidenceSection{
			Bucket:    "low",
			Statement: "Observation only.",
		}
		page.Restraint = e.buildRestraintSection()
		page.Calibration = e.buildCalibrationSection("", "")
		page.TrustAnchor = TrustAnchorSection{
			PeriodLabel: "today",
			ReceiptHash: "",
			Statement:   "No receipt recorded yet.",
		}
		return page
	}

	// Build from receipt
	page.Observation = e.buildObservationSection(input.Receipt)
	page.Confidence = e.buildConfidenceSection(input.Receipt)
	page.Restraint = e.buildRestraintSection()
	page.Calibration = e.buildCalibrationSection(input.CalibrationAgreement, input.CalibrationVote)
	page.TrustAnchor = e.buildTrustAnchorSection(input.Receipt)
	page.ReceiptHash = input.Receipt.Hash()

	return page
}

// buildSourceSection creates the source section.
func (e *Engine) buildSourceSection(hasGmail bool) SourceSection {
	if hasGmail {
		return SourceSection{
			Statement:   "Connected: email (read-only)",
			IsConnected: true,
		}
	}
	return SourceSection{
		Statement:   "No sources connected",
		IsConnected: false,
	}
}

// buildObservationSection creates the observation section from a receipt.
func (e *Engine) buildObservationSection(receipt *shadowllm.ShadowReceipt) ObservationSection {
	// Extract unique categories from suggestions
	categorySet := make(map[shadowllm.AbstractCategory]bool)
	var overallMagnitude shadowllm.MagnitudeBucket = shadowllm.MagnitudeNothing
	var overallHorizon shadowllm.Horizon = shadowllm.HorizonSomeday

	for _, sug := range receipt.Suggestions {
		categorySet[sug.Category] = true

		// Track highest magnitude
		if sug.Magnitude == shadowllm.MagnitudeSeveral {
			overallMagnitude = shadowllm.MagnitudeSeveral
		} else if sug.Magnitude == shadowllm.MagnitudeAFew && overallMagnitude != shadowllm.MagnitudeSeveral {
			overallMagnitude = shadowllm.MagnitudeAFew
		}

		// Track most urgent horizon
		if sug.Horizon == shadowllm.HorizonNow {
			overallHorizon = shadowllm.HorizonNow
		} else if sug.Horizon == shadowllm.HorizonSoon && overallHorizon != shadowllm.HorizonNow {
			overallHorizon = shadowllm.HorizonSoon
		} else if sug.Horizon == shadowllm.HorizonLater && overallHorizon == shadowllm.HorizonSomeday {
			overallHorizon = shadowllm.HorizonLater
		}
	}

	// Sort categories for determinism
	var categories []string
	for cat := range categorySet {
		categories = append(categories, CategoryDisplayText(cat))
	}
	sort.Strings(categories)

	// Build statement
	var statement string
	if len(receipt.Suggestions) == 0 {
		statement = "No patterns observed."
	} else if overallMagnitude == shadowllm.MagnitudeAFew {
		statement = "A few patterns observed."
	} else if overallMagnitude == shadowllm.MagnitudeSeveral {
		statement = "Several patterns observed."
	} else {
		statement = "Minimal activity observed."
	}

	return ObservationSection{
		Magnitude:  MagnitudeDisplayText(overallMagnitude),
		Categories: categories,
		Horizon:    HorizonDisplayText(overallHorizon),
		Statement:  statement,
	}
}

// buildConfidenceSection creates the confidence section.
func (e *Engine) buildConfidenceSection(receipt *shadowllm.ShadowReceipt) ConfidenceSection {
	// Find highest confidence from suggestions
	var highestConf shadowllm.ConfidenceBucket = shadowllm.ConfidenceLow
	for _, sug := range receipt.Suggestions {
		if sug.Confidence == shadowllm.ConfidenceHigh {
			highestConf = shadowllm.ConfidenceHigh
			break
		} else if sug.Confidence == shadowllm.ConfidenceMed && highestConf != shadowllm.ConfidenceHigh {
			highestConf = shadowllm.ConfidenceMed
		}
	}

	return ConfidenceSection{
		Bucket:    ConfidenceDisplayText(highestConf),
		Statement: "Observation only.",
	}
}

// buildRestraintSection creates the restraint section.
// CRITICAL: These are ALWAYS true - shadow mode never executes.
func (e *Engine) buildRestraintSection() RestraintSection {
	return RestraintSection{
		NoActionsTaken:      true,
		NoDraftsCreated:     true,
		NoNotificationsSent: true,
		NoRulesPromoted:     true,
		Statements: []string{
			"No actions taken.",
			"No drafts created.",
			"No notifications sent.",
			"No rules promoted.",
		},
	}
}

// buildCalibrationSection creates the calibration section.
func (e *Engine) buildCalibrationSection(agreement, vote string) CalibrationSection {
	if agreement == "" && vote == "" {
		return CalibrationSection{
			HasCalibration: false,
			Statement:      "No calibration recorded.",
		}
	}

	section := CalibrationSection{
		HasCalibration:  true,
		AgreementBucket: agreement,
		VoteUsefulness:  vote,
	}

	if agreement != "" {
		section.Statement = "Calibration: " + agreement
	} else {
		section.Statement = "Calibration in progress."
	}

	return section
}

// buildTrustAnchorSection creates the trust anchor section.
func (e *Engine) buildTrustAnchorSection(receipt *shadowllm.ShadowReceipt) TrustAnchorSection {
	return TrustAnchorSection{
		PeriodLabel: receipt.WindowBucket,
		ReceiptHash: receipt.Hash(),
		Statement:   "Append-only. This proof cannot be edited.",
	}
}

// EmptyPage returns a page for when no receipts exist.
func (e *Engine) EmptyPage(hasGmailConnection bool) ShadowReceiptPage {
	return e.BuildPage(BuildPageInput{
		Receipt:            nil,
		HasGmailConnection: hasGmailConnection,
	})
}

// ReceiptCue represents the whisper cue for the shadow receipt proof page.
//
// CRITICAL: Shows ONLY abstract statement and link text.
// CRITICAL: Follows single whisper rule - at most ONE cue on /today.
type ReceiptCue struct {
	// Available indicates if the cue should be shown.
	Available bool

	// CueText is the subtle text hinting at proof availability.
	CueText string

	// LinkText is the text for the link to /shadow/receipt.
	LinkText string

	// ReceiptHash for dismissal tracking.
	ReceiptHash string
}

// BuildCueInput contains the inputs needed to build a receipt cue.
type BuildCueInput struct {
	// Receipt is the shadow receipt (may be nil).
	Receipt *shadowllm.ShadowReceipt

	// IsDismissed indicates if the cue was dismissed for current period.
	IsDismissed bool

	// OtherCueActive indicates if another whisper cue is already active.
	// CRITICAL: Single whisper rule - only ONE cue per page.
	OtherCueActive bool
}

// BuildCue determines if and what receipt cue to show on /today.
//
// Cue shows when:
// - Receipt exists for current period
// - Not dismissed for current period
// - No other whisper cue is active (single whisper rule)
//
// CRITICAL: Deterministic - same input => same output.
func (e *Engine) BuildCue(input BuildCueInput) ReceiptCue {
	// No cue if no receipt
	if input.Receipt == nil {
		return ReceiptCue{Available: false}
	}

	// No cue if dismissed
	if input.IsDismissed {
		return ReceiptCue{Available: false}
	}

	// No cue if another whisper is active (single whisper rule)
	if input.OtherCueActive {
		return ReceiptCue{Available: false}
	}

	// Build the cue
	return ReceiptCue{
		Available:   true,
		CueText:     "Proof of observation recorded.",
		LinkText:    "View shadow receipt",
		ReceiptHash: input.Receipt.Hash(),
	}
}

// =============================================================================
// Phase 27: Primary Proof of Intelligence
// =============================================================================

// BuildPrimaryPageInput contains inputs for the Phase 27 primary proof page.
type BuildPrimaryPageInput struct {
	// Receipt is the shadow receipt (may be nil).
	Receipt *shadowllm.ShadowReceipt

	// ProviderKind is the configured shadow provider kind.
	// Use empty string or "none" if not configured.
	ProviderKind string

	// HasVoted indicates if user already voted on this receipt.
	HasVoted bool

	// IsDismissed indicates if the receipt cue was dismissed.
	IsDismissed bool
}

// BuildPrimaryPage creates the Phase 27 primary proof page.
//
// CRITICAL: Deterministic projection - same input => same output.
// CRITICAL: Shows ONLY abstract buckets and hashes.
// CRITICAL: This is proof, not marketing.
func (e *Engine) BuildPrimaryPage(input BuildPrimaryPageInput) domainshadowview.ShadowReceiptPrimaryPage {
	page := domainshadowview.ShadowReceiptPrimaryPage{
		HasReceipt: input.Receipt != nil,
		BackPath:   "/today",
	}

	// Determine provider info
	providerKind := mapProviderKind(input.ProviderKind)
	wasConsulted := input.Receipt != nil && providerKind != domainshadowview.ProviderNone

	// Build provider disclosure
	page.Provider = buildProviderSection(providerKind, wasConsulted)

	if input.Receipt == nil {
		// No receipt - return minimal page
		page.Evidence = domainshadowview.ShadowReceiptEvidence{
			Kind:      domainshadowview.EvidenceAskedNone,
			Statement: "No model was consulted.",
		}
		page.ModelReturn = domainshadowview.ShadowReceiptModelReturn{
			Horizon:    domainshadowview.HorizonNone,
			Magnitude:  domainshadowview.MagnitudeNothing,
			Confidence: domainshadowview.ConfidenceNA,
			Statement:  "Nothing to report.",
		}
		page.Decision = domainshadowview.ShadowReceiptDecision{
			Kind:      domainshadowview.DecisionNoModel,
			Statement: "No action was needed.",
		}
		page.Reason = domainshadowview.ShadowReceiptReason{
			Kind:      domainshadowview.ReasonNoModelConsulted,
			Statement: "Shadow mode observed nothing requiring attention.",
		}
		page.VoteEligibility = domainshadowview.ShadowReceiptVoteEligibility{
			Eligible:     false,
			AlreadyVoted: false,
			ReceiptHash:  "",
		}
		page.StatusHash = page.ComputeStatusHash()
		return page
	}

	// Build from receipt
	page.Evidence = buildEvidenceSection(wasConsulted)
	page.ModelReturn = buildModelReturnSection(input.Receipt)
	page.Decision = buildDecisionSection(input.Receipt)
	page.Reason = buildReasonSection(input.Receipt)
	page.VoteEligibility = domainshadowview.ShadowReceiptVoteEligibility{
		Eligible:     !input.HasVoted && !input.IsDismissed,
		AlreadyVoted: input.HasVoted,
		ReceiptHash:  input.Receipt.Hash(),
	}
	page.StatusHash = page.ComputeStatusHash()

	return page
}

// buildProviderSection creates the provider disclosure section.
func buildProviderSection(kind domainshadowview.ProviderKind, wasConsulted bool) domainshadowview.ShadowReceiptProvider {
	var statement string
	if !wasConsulted {
		statement = "No model was consulted."
	} else {
		switch kind {
		case domainshadowview.ProviderStub:
			statement = "A model was consulted (stub)."
		case domainshadowview.ProviderAzureOpenAIChat:
			statement = "A model was consulted (azure_openai_chat)."
		default:
			statement = "A model was consulted."
		}
	}

	return domainshadowview.ShadowReceiptProvider{
		Kind:         kind,
		WasConsulted: wasConsulted,
		Statement:    statement,
	}
}

// buildEvidenceSection creates the "what we asked" section.
func buildEvidenceSection(wasConsulted bool) domainshadowview.ShadowReceiptEvidence {
	if !wasConsulted {
		return domainshadowview.ShadowReceiptEvidence{
			Kind:      domainshadowview.EvidenceAskedNone,
			Statement: "No model was consulted.",
		}
	}
	return domainshadowview.ShadowReceiptEvidence{
		Kind:      domainshadowview.EvidenceAskedSurfaceCheck,
		Statement: "We asked whether anything should reach you today.",
	}
}

// buildModelReturnSection creates the "what the model returned" section.
func buildModelReturnSection(receipt *shadowllm.ShadowReceipt) domainshadowview.ShadowReceiptModelReturn {
	if len(receipt.Suggestions) == 0 {
		return domainshadowview.ShadowReceiptModelReturn{
			Horizon:    domainshadowview.HorizonNone,
			Magnitude:  domainshadowview.MagnitudeNothing,
			Confidence: domainshadowview.ConfidenceNA,
			Statement:  "The model found nothing noteworthy.",
		}
	}

	// Compute overall buckets from suggestions
	var horizon domainshadowview.HorizonBucket = domainshadowview.HorizonSomeday
	var magnitude domainshadowview.MagnitudeBucket = domainshadowview.MagnitudeNothing
	var confidence domainshadowview.ConfidenceBucket = domainshadowview.ConfidenceLow

	for _, sug := range receipt.Suggestions {
		// Map horizon (most urgent wins)
		h := mapShadowHorizon(sug.Horizon)
		if horizonPriority(h) > horizonPriority(horizon) {
			horizon = h
		}

		// Map magnitude (highest wins)
		m := mapShadowMagnitude(sug.Magnitude)
		if magnitudePriority(m) > magnitudePriority(magnitude) {
			magnitude = m
		}

		// Map confidence (highest wins)
		c := mapShadowConfidence(sug.Confidence)
		if confidencePriority(c) > confidencePriority(confidence) {
			confidence = c
		}
	}

	// Build statement
	statement := buildModelReturnStatement(horizon, magnitude, confidence)

	return domainshadowview.ShadowReceiptModelReturn{
		Horizon:    horizon,
		Magnitude:  magnitude,
		Confidence: confidence,
		Statement:  statement,
	}
}

// buildDecisionSection creates the "what we did" section.
func buildDecisionSection(receipt *shadowllm.ShadowReceipt) domainshadowview.ShadowReceiptDecision {
	// Shadow mode ALWAYS results in no surface
	return domainshadowview.ShadowReceiptDecision{
		Kind:      domainshadowview.DecisionNoSurface,
		Statement: "We chose not to surface anything.",
	}
}

// buildReasonSection creates the "why this didn't interrupt you" section.
func buildReasonSection(receipt *shadowllm.ShadowReceipt) domainshadowview.ShadowReceiptReason {
	// Determine primary reason based on receipt content
	if len(receipt.Suggestions) == 0 {
		return domainshadowview.ShadowReceiptReason{
			Kind:      domainshadowview.ReasonBelowThreshold,
			Statement: "No patterns reached the surfacing threshold.",
		}
	}

	// Check for surface candidates
	hasSurfaceCandidate := false
	for _, sug := range receipt.Suggestions {
		if sug.SuggestionType == shadowllm.SuggestSurfaceCandidate {
			hasSurfaceCandidate = true
			break
		}
	}

	if hasSurfaceCandidate {
		// Shadow mode found candidates but restraint won
		return domainshadowview.ShadowReceiptReason{
			Kind:      domainshadowview.ReasonShadowOnly,
			Statement: "Shadow mode is observation-only. Nothing is surfaced automatically.",
		}
	}

	// Default: default hold policy
	return domainshadowview.ShadowReceiptReason{
		Kind:      domainshadowview.ReasonDefaultHold,
		Statement: "Default hold policy kept everything quiet.",
	}
}

// buildModelReturnStatement creates a human-readable statement.
func buildModelReturnStatement(h domainshadowview.HorizonBucket, m domainshadowview.MagnitudeBucket, c domainshadowview.ConfidenceBucket) string {
	if m == domainshadowview.MagnitudeNothing {
		return "The model found nothing noteworthy."
	}

	// Build description
	var magText string
	switch m {
	case domainshadowview.MagnitudeAFew:
		magText = "A few patterns"
	case domainshadowview.MagnitudeSeveral:
		magText = "Several patterns"
	default:
		magText = "Some patterns"
	}

	var horizonText string
	switch h {
	case domainshadowview.HorizonSoon:
		horizonText = "soon"
	case domainshadowview.HorizonLater:
		horizonText = "later"
	case domainshadowview.HorizonSomeday:
		horizonText = "eventually"
	default:
		horizonText = "at some point"
	}

	var confText string
	switch c {
	case domainshadowview.ConfidenceHigh:
		confText = "with high confidence"
	case domainshadowview.ConfidenceMedium:
		confText = "with moderate confidence"
	default:
		confText = "with low confidence"
	}

	return magText + " were noted for " + horizonText + ", " + confText + "."
}

// mapProviderKind maps string provider kind to domain type.
func mapProviderKind(kind string) domainshadowview.ProviderKind {
	switch kind {
	case "stub":
		return domainshadowview.ProviderStub
	case "azure_openai", "azure_openai_chat":
		return domainshadowview.ProviderAzureOpenAIChat
	case "none", "":
		return domainshadowview.ProviderNone
	default:
		return domainshadowview.ProviderNone
	}
}

// mapShadowHorizon maps shadowllm.Horizon to domain type.
func mapShadowHorizon(h shadowllm.Horizon) domainshadowview.HorizonBucket {
	switch h {
	case shadowllm.HorizonNow, shadowllm.HorizonSoon:
		return domainshadowview.HorizonSoon
	case shadowllm.HorizonLater:
		return domainshadowview.HorizonLater
	case shadowllm.HorizonSomeday:
		return domainshadowview.HorizonSomeday
	default:
		return domainshadowview.HorizonNone
	}
}

// mapShadowMagnitude maps shadowllm.MagnitudeBucket to domain type.
func mapShadowMagnitude(m shadowllm.MagnitudeBucket) domainshadowview.MagnitudeBucket {
	switch m {
	case shadowllm.MagnitudeNothing:
		return domainshadowview.MagnitudeNothing
	case shadowllm.MagnitudeAFew:
		return domainshadowview.MagnitudeAFew
	case shadowllm.MagnitudeSeveral:
		return domainshadowview.MagnitudeSeveral
	default:
		return domainshadowview.MagnitudeNothing
	}
}

// mapShadowConfidence maps shadowllm.ConfidenceBucket to domain type.
func mapShadowConfidence(c shadowllm.ConfidenceBucket) domainshadowview.ConfidenceBucket {
	switch c {
	case shadowllm.ConfidenceLow:
		return domainshadowview.ConfidenceLow
	case shadowllm.ConfidenceMed:
		return domainshadowview.ConfidenceMedium
	case shadowllm.ConfidenceHigh:
		return domainshadowview.ConfidenceHigh
	default:
		return domainshadowview.ConfidenceNA
	}
}

// horizonPriority returns priority for horizon (higher = more urgent).
func horizonPriority(h domainshadowview.HorizonBucket) int {
	switch h {
	case domainshadowview.HorizonSoon:
		return 3
	case domainshadowview.HorizonLater:
		return 2
	case domainshadowview.HorizonSomeday:
		return 1
	default:
		return 0
	}
}

// magnitudePriority returns priority for magnitude (higher = more).
func magnitudePriority(m domainshadowview.MagnitudeBucket) int {
	switch m {
	case domainshadowview.MagnitudeSeveral:
		return 3
	case domainshadowview.MagnitudeAFew:
		return 2
	case domainshadowview.MagnitudeNothing:
		return 1
	default:
		return 0
	}
}

// confidencePriority returns priority for confidence (higher = more confident).
func confidencePriority(c domainshadowview.ConfidenceBucket) int {
	switch c {
	case domainshadowview.ConfidenceHigh:
		return 3
	case domainshadowview.ConfidenceMedium:
		return 2
	case domainshadowview.ConfidenceLow:
		return 1
	default:
		return 0
	}
}

// BuildPrimaryCueInput contains inputs for the Phase 27 whisper cue.
type BuildPrimaryCueInput struct {
	// Receipt is the shadow receipt (may be nil).
	Receipt *shadowllm.ShadowReceipt

	// IsDismissed indicates if the cue was dismissed.
	IsDismissed bool

	// OtherCueActive indicates if another whisper cue is active.
	OtherCueActive bool

	// ProviderKind is the configured shadow provider.
	ProviderKind string
}

// BuildPrimaryCue builds the Phase 27 whisper cue.
//
// Phase 27 cue text: "We checked something — quietly."
// Priority order: Journey > Surface > Proof > Shadow receipt (lowest)
//
// CRITICAL: Subject to single-whisper rule.
// CRITICAL: Must be ignorable with zero cost.
func (e *Engine) BuildPrimaryCue(input BuildPrimaryCueInput) domainshadowview.ShadowReceiptCue {
	// No cue if no receipt
	if input.Receipt == nil {
		return domainshadowview.ShadowReceiptCue{Available: false}
	}

	// No cue if dismissed
	if input.IsDismissed {
		return domainshadowview.ShadowReceiptCue{Available: false}
	}

	// No cue if another whisper is active (single whisper rule)
	if input.OtherCueActive {
		return domainshadowview.ShadowReceiptCue{Available: false}
	}

	// Build the cue with Phase 27 text
	return domainshadowview.ShadowReceiptCue{
		Available:   true,
		CueText:     domainshadowview.DefaultCueText,
		LinkText:    domainshadowview.DefaultLinkText,
		ReceiptHash: input.Receipt.Hash(),
	}
}
