// Package shadowllm provides the ShadowModel interface for shadow-mode observation.
//
// CRITICAL: ShadowModel implementations MUST NOT:
// - Make network calls (no net/http imports)
// - Use goroutines
// - Call time.Now() (use injected clock)
// - Store or return any identifiable information
// - Store or return any content (email bodies, subjects, names, amounts)
//
// ShadowModel implementations MUST:
// - Be deterministic (same inputs + seed + clock => same outputs)
// - Use only abstract inputs (buckets, counts, hashes)
// - Return only metadata (scores, deltas, confidence values)
package shadowllm

import (
	"time"

	"quantumlife/pkg/domain/identity"
)

// ShadowModel is the interface for shadow-mode observation.
//
// Implementations observe the state of a circle AFTER the main loop
// and emit metadata-only signals. They NEVER affect the main loop.
type ShadowModel interface {
	// Observe runs shadow-mode observation and returns a ShadowRun.
	//
	// CRITICAL: This method must be deterministic.
	// Given the same ShadowContext (including seed and clock),
	// it must return a ShadowRun with the same Hash().
	//
	// CRITICAL: This method must NOT make network calls.
	// CRITICAL: This method must NOT spawn goroutines.
	Observe(ctx ShadowContext) (ShadowRun, error)

	// Name returns the model name (e.g., "stub", "deterministic-v1").
	Name() string
}

// ShadowContext provides abstract inputs to the shadow model.
//
// CRITICAL: ShadowContext contains ONLY abstract data.
// FORBIDDEN: raw email subjects, bodies, vendor names, amounts,
//
//	timestamps that could identify events, personal identifiers.
//
// This is the ONLY input to shadow-mode. If it's not here, the model can't see it.
type ShadowContext struct {
	// CircleID is the circle being observed.
	CircleID identity.EntityID

	// InputsHash is a precomputed hash of the abstract inputs.
	// This allows replay verification without storing inputs.
	InputsHash string

	// Seed is the deterministic seed for this run.
	// Same seed + same inputs => same outputs.
	Seed int64

	// Clock provides the current time (injected, never time.Now()).
	Clock func() time.Time

	// AbstractInputs contains ONLY abstract, non-identifiable data.
	// All values are buckets, counts, or hashes - never raw content.
	AbstractInputs AbstractInputs
}

// AbstractInputs contains only abstract, aggregated data.
//
// CRITICAL: This struct must NEVER contain:
// - Email subjects, bodies, or sender names
// - Vendor/merchant names
// - Amounts (use bucketed ranges instead)
// - Timestamps that could identify specific events
// - Any personally identifiable information
type AbstractInputs struct {
	// ObligationCountByCategory maps category => count.
	// Example: {"money": 3, "time": 5}
	ObligationCountByCategory map[AbstractCategory]int

	// HeldCountByCategory maps category => count of held items.
	HeldCountByCategory map[AbstractCategory]int

	// SurfacedCountByCategory maps category => count of surfaced items.
	SurfacedCountByCategory map[AbstractCategory]int

	// HorizonBuckets maps horizon (day/week/month) => count of items.
	// Example: {"day": 2, "week": 5, "month": 10}
	HorizonBuckets map[string]int

	// TriggerKindCounts maps trigger kind => count.
	// Example: {"email": 10, "calendar": 3}
	TriggerKindCounts map[string]int

	// CategoryPressure maps category => current pressure (0.0-1.0).
	// This is computed by the main loop, not raw data.
	CategoryPressure map[AbstractCategory]float64

	// AverageRegret is the average regret score (0.0-1.0).
	AverageRegret float64

	// TotalObligationCount is the total number of obligations.
	TotalObligationCount int

	// TotalHeldCount is the total number of held items.
	TotalHeldCount int

	// TotalSurfacedCount is the total number of surfaced items.
	TotalSurfacedCount int
}

// CanonicalString returns the pipe-delimited canonical representation.
// Used for hashing and deterministic comparison.
func (a *AbstractInputs) CanonicalString() string {
	// Build deterministic string representation
	// Categories are sorted alphabetically
	categories := []AbstractCategory{
		CategoryHome, CategoryMoney, CategoryPeople, CategoryTime, CategoryWork,
	}

	var parts []string
	parts = append(parts, "ABSTRACT_INPUTS|v1")

	// Obligation counts by category
	for _, cat := range categories {
		count := a.ObligationCountByCategory[cat]
		parts = append(parts, string(cat)+":obl:"+itoa(count))
	}

	// Held counts by category
	for _, cat := range categories {
		count := a.HeldCountByCategory[cat]
		parts = append(parts, string(cat)+":held:"+itoa(count))
	}

	// Surfaced counts by category
	for _, cat := range categories {
		count := a.SurfacedCountByCategory[cat]
		parts = append(parts, string(cat)+":surf:"+itoa(count))
	}

	// Horizon buckets (sorted)
	horizons := []string{"day", "month", "week"}
	for _, h := range horizons {
		count := a.HorizonBuckets[h]
		parts = append(parts, "horizon:"+h+":"+itoa(count))
	}

	// Trigger kind counts (sorted)
	triggers := []string{"calendar", "email", "finance"}
	for _, t := range triggers {
		count := a.TriggerKindCounts[t]
		parts = append(parts, "trigger:"+t+":"+itoa(count))
	}

	// Category pressure
	for _, cat := range categories {
		pressure := a.CategoryPressure[cat]
		parts = append(parts, string(cat)+":pressure:"+formatFloat(pressure))
	}

	// Totals
	parts = append(parts, "avg_regret:"+formatFloat(a.AverageRegret))
	parts = append(parts, "total_obl:"+itoa(a.TotalObligationCount))
	parts = append(parts, "total_held:"+itoa(a.TotalHeldCount))
	parts = append(parts, "total_surf:"+itoa(a.TotalSurfacedCount))

	return joinPipe(parts)
}

// Hash returns the SHA256 hash of the canonical string.
func (a *AbstractInputs) Hash() string {
	return computeHash(a.CanonicalString())
}

// Validate checks if the context is valid.
func (ctx *ShadowContext) Validate() error {
	if ctx.CircleID == "" {
		return ErrMissingCircleID
	}
	if ctx.InputsHash == "" {
		return ErrMissingInputsHash
	}
	if ctx.Clock == nil {
		return ErrMissingClock
	}
	return nil
}

// ErrMissingClock is returned when Clock is nil.
const ErrMissingClock shadowError = "missing clock function"

// itoa converts int to string without strconv import in this file.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// joinPipe joins strings with pipe separator.
func joinPipe(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += "|" + parts[i]
	}
	return result
}
