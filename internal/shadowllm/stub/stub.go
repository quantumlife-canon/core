// Package stub provides a deterministic stub implementation of ShadowModel.
//
// Phase 19: LLM Shadow-Mode Contract
//
// CRITICAL: This stub does NOT make network calls.
// CRITICAL: This stub does NOT use goroutines.
// CRITICAL: This stub is DETERMINISTIC - same inputs + seed => same outputs.
// CRITICAL: This stub outputs METADATA ONLY - never content strings.
//
// The stub uses SHA256(seed + inputs hash) to generate deterministic pseudo-random
// float values for signals. This ensures reproducibility without any real LLM.
//
// Reference: docs/ADR/ADR-0043-phase19-shadow-mode-contract.md
package stub

import (
	"crypto/sha256"
	"encoding/binary"

	"quantumlife/pkg/domain/shadowllm"
)

// StubModel is a deterministic stub implementation of ShadowModel.
// It generates reproducible signals based on seed and inputs hash.
type StubModel struct {
	name string
}

// NewStubModel creates a new deterministic stub model.
func NewStubModel() *StubModel {
	return &StubModel{
		name: "stub",
	}
}

// Name returns the model name.
func (m *StubModel) Name() string {
	return m.name
}

// Observe generates deterministic signals based on the context.
//
// CRITICAL: This method is deterministic.
// Same seed + same inputs hash => same output signals.
//
// CRITICAL: This method does NOT make network calls.
// CRITICAL: This method does NOT spawn goroutines.
func (m *StubModel) Observe(ctx shadowllm.ShadowContext) (shadowllm.ShadowRun, error) {
	if err := ctx.Validate(); err != nil {
		return shadowllm.ShadowRun{}, err
	}

	now := ctx.Clock()

	// Generate deterministic run ID from seed + inputs hash
	runID := generateRunID(ctx.Seed, ctx.InputsHash)

	// Generate signals based on abstract inputs
	signals := m.generateSignals(ctx)

	run := shadowllm.ShadowRun{
		RunID:      runID,
		CircleID:   ctx.CircleID,
		InputsHash: ctx.InputsHash,
		ModelSpec:  m.name,
		Seed:       ctx.Seed,
		Signals:    signals,
		CreatedAt:  now,
	}

	return run, nil
}

// generateSignals creates deterministic signals from the context.
// Always returns 1-3 suggestions to enable diff flow.
// Max 5 signals per run.
func (m *StubModel) generateSignals(ctx shadowllm.ShadowContext) []shadowllm.ShadowSignal {
	var signals []shadowllm.ShadowSignal

	// Determine how many signals to generate (1-3 based on seed)
	numSignals := selectSignalCount(ctx.Seed, ctx.InputsHash)

	// Categories to potentially emit signals for
	categories := []shadowllm.AbstractCategory{
		shadowllm.CategoryMoney,
		shadowllm.CategoryTime,
		shadowllm.CategoryPeople,
		shadowllm.CategoryWork,
		shadowllm.CategoryHome,
	}

	signalCount := 0

	for _, cat := range categories {
		if signalCount >= numSignals {
			break
		}
		if signalCount >= shadowllm.MaxSignalsPerRun {
			break
		}

		// Generate deterministic values based on seed, category, and inputs hash
		valueFloat := generateDeterministicFloat(ctx.Seed, string(cat), ctx.InputsHash, "value")
		confidenceFloat := generateDeterministicFloat(ctx.Seed, string(cat), ctx.InputsHash, "confidence")

		// Normalize to valid ranges
		valueFloat = normalizeToRange(valueFloat, -1.0, 1.0)
		confidenceFloat = normalizeToRange(confidenceFloat, 0.0, 1.0)

		// Generate item key hash (deterministic)
		itemKeyHash := generateItemKeyHash(ctx.Seed, string(cat), ctx.InputsHash)

		// Determine signal kind based on deterministic selection
		kind := selectSignalKind(ctx.Seed, string(cat))

		signal := shadowllm.ShadowSignal{
			Kind:            kind,
			CircleID:        ctx.CircleID,
			ItemKeyHash:     itemKeyHash,
			Category:        cat,
			ValueFloat:      valueFloat,
			ConfidenceFloat: confidenceFloat,
			NotesHash:       shadowllm.HashNotes(""), // Empty notes hash
			CreatedAt:       ctx.Clock(),
		}

		signals = append(signals, signal)
		signalCount++
	}

	return signals
}

// selectSignalCount deterministically selects how many signals to generate (1-3).
func selectSignalCount(seed int64, inputsHash string) int {
	h := sha256.New()
	seedBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(seedBytes, uint64(seed))
	h.Write(seedBytes)
	h.Write([]byte(inputsHash))
	h.Write([]byte("SIGNAL_COUNT"))

	sum := h.Sum(nil)
	// Generate 1-3 signals based on hash
	return 1 + int(sum[0])%3
}

// generateRunID creates a deterministic run ID from seed and inputs hash.
func generateRunID(seed int64, inputsHash string) string {
	h := sha256.New()
	seedBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(seedBytes, uint64(seed))
	h.Write(seedBytes)
	h.Write([]byte(inputsHash))
	h.Write([]byte("RUN_ID"))

	sum := h.Sum(nil)
	return bytesToHex(sum[:16]) // Use first 16 bytes for run ID
}

// generateDeterministicFloat generates a deterministic float from seed and category.
func generateDeterministicFloat(seed int64, category, inputsHash, purpose string) float64 {
	h := sha256.New()
	seedBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(seedBytes, uint64(seed))
	h.Write(seedBytes)
	h.Write([]byte(category))
	h.Write([]byte(inputsHash))
	h.Write([]byte(purpose))

	sum := h.Sum(nil)
	// Use first 8 bytes as uint64, then normalize to [0, 1)
	val := binary.BigEndian.Uint64(sum[:8])
	return float64(val) / float64(^uint64(0))
}

// normalizeToRange normalizes a [0, 1) value to [min, max].
func normalizeToRange(val, min, max float64) float64 {
	return min + val*(max-min)
}

// generateItemKeyHash creates a deterministic item key hash.
func generateItemKeyHash(seed int64, category, inputsHash string) string {
	h := sha256.New()
	seedBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(seedBytes, uint64(seed))
	h.Write(seedBytes)
	h.Write([]byte(category))
	h.Write([]byte(inputsHash))
	h.Write([]byte("ITEM_KEY"))

	return bytesToHex(h.Sum(nil))
}

// selectSignalKind deterministically selects a signal kind.
func selectSignalKind(seed int64, category string) shadowllm.ShadowSignalKind {
	h := sha256.New()
	seedBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(seedBytes, uint64(seed))
	h.Write(seedBytes)
	h.Write([]byte(category))
	h.Write([]byte("SIGNAL_KIND"))

	sum := h.Sum(nil)
	index := int(sum[0]) % 4

	kinds := []shadowllm.ShadowSignalKind{
		shadowllm.SignalKindRegretDelta,
		shadowllm.SignalKindCategoryPressure,
		shadowllm.SignalKindConfidence,
		shadowllm.SignalKindLabelSuggestion,
	}

	return kinds[index]
}

// bytesToHex converts bytes to hex string.
func bytesToHex(b []byte) string {
	const hexChars = "0123456789abcdef"
	result := make([]byte, len(b)*2)
	for i, v := range b {
		result[i*2] = hexChars[v>>4]
		result[i*2+1] = hexChars[v&0x0f]
	}
	return string(result)
}

// Verify interface compliance at compile time.
var _ shadowllm.ShadowModel = (*StubModel)(nil)
