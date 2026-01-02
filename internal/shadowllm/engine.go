// Package shadowllm provides the shadow-mode engine for LLM observation.
//
// Phase 19.2: LLM Shadow Mode Contract
// Phase 19.3: Azure OpenAI Shadow Provider
//
// CRITICAL INVARIANTS:
//   - Shadow mode produces METADATA ONLY (abstract buckets, hashes) - never content
//   - Shadow mode does NOT affect behavior - observation ONLY
//   - Shadow mode is OFF by default - explicit action required
//   - No goroutines in internal/. No time.Now() - clock injection only.
//   - Stub provider: Deterministic (same inputs + same clock => identical receipt hash)
//   - Real providers: Non-deterministic OK but receipts include provenance
//   - Stdlib only. No external dependencies.
//
// Reference: docs/ADR/ADR-0043-phase19-2-shadow-mode-contract.md
// Reference: docs/ADR/ADR-0044-phase19-3-azure-openai-shadow-provider.md
package shadowllm

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"time"

	"quantumlife/internal/shadowllm/privacy"
	"quantumlife/internal/shadowllm/prompt"
	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/shadowllm"
)

// Engine orchestrates shadow-mode analysis runs.
//
// CRITICAL: Engine does NOT make network calls.
// CRITICAL: Engine does NOT spawn goroutines.
// CRITICAL: Engine does NOT modify any state - observation only.
type Engine struct {
	clock    clock.Clock
	provider shadowllm.ShadowModel
}

// NewEngine creates a new shadow-mode engine.
//
// CRITICAL: Provider must be a stub or local implementation.
// No real LLM API calls are allowed in Phase 19.2.
func NewEngine(clk clock.Clock, provider shadowllm.ShadowModel) *Engine {
	return &Engine{
		clock:    clk,
		provider: provider,
	}
}

// RunInput contains the abstract inputs for a shadow run.
//
// CRITICAL: All inputs must already be abstract/bucketed.
// No raw content is allowed.
type RunInput struct {
	// CircleID is the circle being analyzed.
	CircleID identity.EntityID

	// Digest contains the pre-bucketed abstract inputs.
	Digest shadowllm.ShadowInputDigest

	// Seed is an optional seed for deterministic runs.
	// If zero, derives from digest hash.
	Seed int64
}

// RunOutput contains the result of a shadow run.
type RunOutput struct {
	// Receipt is the privacy-safe receipt of the analysis.
	Receipt shadowllm.ShadowReceipt

	// Status indicates the outcome.
	Status RunStatus
}

// RunStatus indicates the outcome of a shadow run.
type RunStatus string

const (
	RunStatusSuccess RunStatus = "success"
	RunStatusBlocked RunStatus = "blocked"
	RunStatusFailed  RunStatus = "failed"
)

// Run performs a shadow-mode analysis.
//
// CRITICAL: This method does NOT modify any external state.
// CRITICAL: This method does NOT make network calls.
// CRITICAL: This method does NOT spawn goroutines.
// CRITICAL: Results are for OBSERVATION ONLY - they do NOT affect behavior.
func (e *Engine) Run(input RunInput) (*RunOutput, error) {
	now := e.clock.Now()

	// Validate input
	if input.CircleID == "" {
		return &RunOutput{
			Status: RunStatusFailed,
		}, shadowllm.ErrMissingCircleID
	}

	// Compute input digest hash
	inputDigestHash := input.Digest.Hash()

	// Determine seed - use provided or derive from digest
	seed := input.Seed
	if seed == 0 {
		seed = deriveSeedFromHash(inputDigestHash)
	}

	// Build shadow context for provider
	ctx := shadowllm.ShadowContext{
		CircleID:   input.CircleID,
		InputsHash: inputDigestHash,
		Seed:       seed,
		Clock:      e.clock.Now,
		AbstractInputs: shadowllm.AbstractInputs{
			ObligationCountByCategory: convertMagnitudeToInt(input.Digest.ObligationCountByCategory),
			HeldCountByCategory:       convertMagnitudeToInt(input.Digest.HeldCountByCategory),
			TotalObligationCount:      totalFromMagnitude(input.Digest.ObligationCountByCategory),
			TotalHeldCount:            totalFromMagnitude(input.Digest.HeldCountByCategory),
		},
	}

	// Run provider (stub implementation - no network calls)
	run, err := e.provider.Observe(ctx)
	if err != nil {
		return &RunOutput{
			Status: RunStatusFailed,
		}, err
	}

	// Generate receipt ID deterministically
	receiptID := generateReceiptID(input.CircleID, inputDigestHash, now)

	// Convert signals to suggestions
	suggestions := convertSignalsToSuggestions(run.Signals)

	// Build receipt with Phase 19.3 provenance
	receipt := shadowllm.ShadowReceipt{
		ReceiptID:       receiptID,
		CircleID:        input.CircleID,
		WindowBucket:    now.UTC().Format("2006-01-02"),
		InputDigestHash: inputDigestHash,
		Suggestions:     suggestions,
		ModelSpec:       e.provider.Name(),
		CreatedAt:       now,
		Provenance: shadowllm.Provenance{
			ProviderKind:          shadowllm.ProviderKindStub,
			ModelOrDeployment:     e.provider.Name(),
			RequestPolicyHash:     privacy.PolicyHash(),
			PromptTemplateVersion: prompt.TemplateVersion,
			LatencyBucket:         shadowllm.LatencyNA, // Stub has no latency
			Status:                shadowllm.ReceiptStatusSuccess,
			ErrorBucket:           "",
		},
		WhyGeneric: "", // Stub provider has no why_generic
	}

	return &RunOutput{
		Receipt: receipt,
		Status:  RunStatusSuccess,
	}, nil
}

// deriveSeedFromHash derives a deterministic seed from a hash string.
func deriveSeedFromHash(hash string) int64 {
	if len(hash) < 16 {
		return 0
	}
	bytes, err := hex.DecodeString(hash[:16])
	if err != nil || len(bytes) < 8 {
		return 0
	}
	return int64(binary.BigEndian.Uint64(bytes))
}

// generateReceiptID creates a deterministic receipt ID.
func generateReceiptID(circleID identity.EntityID, inputHash string, t time.Time) string {
	h := sha256.New()
	h.Write([]byte("RECEIPT_ID|"))
	h.Write([]byte(circleID))
	h.Write([]byte("|"))
	h.Write([]byte(inputHash))
	h.Write([]byte("|"))
	h.Write([]byte(t.UTC().Format(time.RFC3339Nano)))
	sum := h.Sum(nil)
	return hex.EncodeToString(sum[:16]) // 32 hex chars
}

// convertMagnitudeToInt converts magnitude buckets to int counts for legacy context.
func convertMagnitudeToInt(m map[shadowllm.AbstractCategory]shadowllm.MagnitudeBucket) map[shadowllm.AbstractCategory]int {
	result := make(map[shadowllm.AbstractCategory]int)
	for cat, mag := range m {
		switch mag {
		case shadowllm.MagnitudeNothing:
			result[cat] = 0
		case shadowllm.MagnitudeAFew:
			result[cat] = 2 // midpoint of 1-3
		case shadowllm.MagnitudeSeveral:
			result[cat] = 5 // arbitrary "several"
		}
	}
	return result
}

// totalFromMagnitude computes an approximate total from magnitude buckets.
func totalFromMagnitude(m map[shadowllm.AbstractCategory]shadowllm.MagnitudeBucket) int {
	total := 0
	for _, mag := range m {
		switch mag {
		case shadowllm.MagnitudeAFew:
			total += 2
		case shadowllm.MagnitudeSeveral:
			total += 5
		}
	}
	return total
}

// convertSignalsToSuggestions converts legacy signals to Phase 19.2 suggestions.
func convertSignalsToSuggestions(signals []shadowllm.ShadowSignal) []shadowllm.ShadowSuggestion {
	if len(signals) == 0 {
		return nil
	}

	suggestions := make([]shadowllm.ShadowSuggestion, 0, len(signals))
	for _, sig := range signals {
		sug := shadowllm.ShadowSuggestion{
			Category:       sig.Category,
			Horizon:        horizonFromValue(sig.ValueFloat),
			Magnitude:      magnitudeFromConfidence(sig.ConfidenceFloat),
			Confidence:     shadowllm.ConfidenceFromFloat(sig.ConfidenceFloat),
			SuggestionType: suggestionTypeFromKind(sig.Kind),
			ItemKeyHash:    sig.ItemKeyHash,
		}
		suggestions = append(suggestions, sug)
	}

	// Limit to max suggestions
	if len(suggestions) > shadowllm.MaxSuggestionsPerReceipt {
		suggestions = suggestions[:shadowllm.MaxSuggestionsPerReceipt]
	}

	return suggestions
}

// horizonFromValue maps a value float to a horizon bucket.
func horizonFromValue(v float64) shadowllm.Horizon {
	switch {
	case v >= 0.5:
		return shadowllm.HorizonNow
	case v >= 0.0:
		return shadowllm.HorizonSoon
	case v >= -0.5:
		return shadowllm.HorizonLater
	default:
		return shadowllm.HorizonSomeday
	}
}

// magnitudeFromConfidence maps confidence to a magnitude estimate.
func magnitudeFromConfidence(c float64) shadowllm.MagnitudeBucket {
	switch {
	case c >= 0.66:
		return shadowllm.MagnitudeSeveral
	case c >= 0.33:
		return shadowllm.MagnitudeAFew
	default:
		return shadowllm.MagnitudeNothing
	}
}

// suggestionTypeFromKind maps signal kind to suggestion type.
func suggestionTypeFromKind(k shadowllm.ShadowSignalKind) shadowllm.SuggestionType {
	switch k {
	case shadowllm.SignalKindCategoryPressure:
		return shadowllm.SuggestSurfaceCandidate
	case shadowllm.SignalKindLabelSuggestion:
		return shadowllm.SuggestDraftCandidate
	default:
		return shadowllm.SuggestHold
	}
}
