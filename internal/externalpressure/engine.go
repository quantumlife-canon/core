// Package externalpressure provides the external pressure engine for Phase 31.4.
//
// This engine transforms commerce observations into abstract pressure maps
// that attach to sovereign circles through derived external circles.
//
// CRITICAL INVARIANTS:
//   - NO raw merchant strings, NO vendor identifiers, NO amounts, NO timestamps
//   - Only category hints, magnitude buckets, and horizon buckets
//   - Derived circles CANNOT approve, CANNOT execute, CANNOT receive drafts
//   - Hash-only persistence; deterministic: same inputs => same hashes
//   - No goroutines. No time.Now() - clock injection only.
//   - stdlib only.
//
// Reference: docs/ADR/ADR-0067-phase31-4-external-pressure-circles.md
package externalpressure

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"time"

	"quantumlife/pkg/domain/externalpressure"
)

// Engine orchestrates external pressure computation.
// CRITICAL: No goroutines. No time.Now() - clock injection only.
type Engine struct {
	clock func() time.Time
}

// NewEngine creates a new external pressure engine.
func NewEngine(clock func() time.Time) *Engine {
	return &Engine{
		clock: clock,
	}
}

// ComputePressureMap computes the pressure map from inputs.
// Returns nil if no meaningful pressure detected.
//
// CRITICAL: Outputs contain ONLY abstract buckets, never raw data.
func (e *Engine) ComputePressureMap(inputs *externalpressure.PressureInputs) *externalpressure.PressureMapSnapshot {
	if inputs == nil {
		return nil
	}

	if len(inputs.Observations) == 0 {
		return nil
	}

	if inputs.SovereignCircleIDHash == "" {
		return nil
	}

	if inputs.PeriodKey == "" {
		return nil
	}

	// Group observations by category
	categoryGroups := make(map[externalpressure.PressureCategory][]externalpressure.ObservationInput)
	for _, obs := range inputs.Observations {
		categoryGroups[obs.Category] = append(categoryGroups[obs.Category], obs)
	}

	// Build pressure items
	items := make([]externalpressure.PressureItem, 0, len(categoryGroups))
	for cat, obsList := range categoryGroups {
		item := e.buildPressureItem(cat, obsList)
		items = append(items, item)
	}

	// Sort items by category for determinism
	sort.Slice(items, func(i, j int) bool {
		return string(items[i].Category) < string(items[j].Category)
	})

	// If too many items, select deterministically by lowest hash
	if len(items) > externalpressure.MaxPressureItems {
		items = selectByLowestHash(items, externalpressure.MaxPressureItems)
	}

	snapshot := &externalpressure.PressureMapSnapshot{
		SovereignCircleIDHash: inputs.SovereignCircleIDHash,
		PeriodKey:             inputs.PeriodKey,
		Items:                 items,
	}

	snapshot.StatusHash = snapshot.ComputeHash()
	return snapshot
}

// buildPressureItem builds a pressure item from observations in a category.
func (e *Engine) buildPressureItem(category externalpressure.PressureCategory, observations []externalpressure.ObservationInput) externalpressure.PressureItem {
	// Collect unique sources
	sourceSet := make(map[externalpressure.SourceKind]bool)
	var evidenceHashes []string

	for _, obs := range observations {
		sourceSet[obs.Source] = true
		evidenceHashes = append(evidenceHashes, obs.EvidenceHash)
	}

	// Convert to sorted slice
	sources := make([]externalpressure.SourceKind, 0, len(sourceSet))
	for s := range sourceSet {
		sources = append(sources, s)
	}
	sort.Slice(sources, func(i, j int) bool {
		return string(sources[i]) < string(sources[j])
	})

	// Compute magnitude from count
	magnitude := externalpressure.ToPressureMagnitude(len(observations))

	// Default horizon is unknown (we don't have horizon data from commerce observations)
	horizon := externalpressure.PressureHorizonUnknown

	// Compute evidence hash from all observation hashes
	sort.Strings(evidenceHashes)
	evidenceHash := computeEvidenceHash(category, sources, evidenceHashes)

	return externalpressure.PressureItem{
		Category:           category,
		Magnitude:          magnitude,
		Horizon:            horizon,
		SourceKindsPresent: sources,
		EvidenceHash:       evidenceHash,
	}
}

// selectByLowestHash selects items with the lowest hashes deterministically.
func selectByLowestHash(items []externalpressure.PressureItem, maxItems int) []externalpressure.PressureItem {
	if len(items) <= maxItems {
		return items
	}

	// Sort by hash
	sorted := make([]externalpressure.PressureItem, len(items))
	copy(sorted, items)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ComputeHash() < sorted[j].ComputeHash()
	})

	return sorted[:maxItems]
}

// computeEvidenceHash computes a deterministic hash from pressure item components.
func computeEvidenceHash(category externalpressure.PressureCategory, sources []externalpressure.SourceKind, evidenceHashes []string) string {
	var builder []byte
	builder = append(builder, []byte("PRESSURE_EVIDENCE|v1|")...)
	builder = append(builder, []byte(string(category))...)

	for _, s := range sources {
		builder = append(builder, '|')
		builder = append(builder, []byte(string(s))...)
	}

	for _, h := range evidenceHashes {
		builder = append(builder, '|')
		builder = append(builder, []byte(h)...)
	}

	hash := sha256.Sum256(builder)
	return hex.EncodeToString(hash[:16])
}

// DeriveExternalCircle derives an external circle from a pressure item.
// Returns the external circle representing external pressure in this category.
//
// CRITICAL: External circles CANNOT approve, CANNOT execute, CANNOT receive drafts.
func (e *Engine) DeriveExternalCircle(
	sovereignCircleIDHash string,
	category externalpressure.PressureCategory,
	source externalpressure.SourceKind,
	periodKey string,
	evidenceHash string,
) *externalpressure.ExternalDerivedCircle {
	if sovereignCircleIDHash == "" || periodKey == "" || evidenceHash == "" {
		return nil
	}

	circleIDHash := externalpressure.ComputeExternalCircleID(source, category, sovereignCircleIDHash)

	return &externalpressure.ExternalDerivedCircle{
		CircleIDHash:  circleIDHash,
		SourceKind:    source,
		CategoryHint:  category,
		CreatedPeriod: periodKey,
		EvidenceHash:  evidenceHash,
	}
}

// DeriveExternalCirclesFromSnapshot derives all external circles from a pressure map snapshot.
func (e *Engine) DeriveExternalCirclesFromSnapshot(snapshot *externalpressure.PressureMapSnapshot) []*externalpressure.ExternalDerivedCircle {
	if snapshot == nil || len(snapshot.Items) == 0 {
		return nil
	}

	var circles []*externalpressure.ExternalDerivedCircle

	for _, item := range snapshot.Items {
		// Derive a circle for each source in the item
		for _, source := range item.SourceKindsPresent {
			circle := e.DeriveExternalCircle(
				snapshot.SovereignCircleIDHash,
				item.Category,
				source,
				snapshot.PeriodKey,
				item.EvidenceHash,
			)
			if circle != nil {
				circles = append(circles, circle)
			}
		}
	}

	// Sort by circle ID hash for determinism
	sort.Slice(circles, func(i, j int) bool {
		return circles[i].CircleIDHash < circles[j].CircleIDHash
	})

	return circles
}

// BuildProofPage builds the pressure proof page from a snapshot.
// Returns nil if no items (silence is success).
func (e *Engine) BuildProofPage(snapshot *externalpressure.PressureMapSnapshot) *externalpressure.PressureProofPage {
	return externalpressure.NewPressureProofPage(snapshot)
}

// ShouldShowPressureCue determines if the pressure cue should be shown.
// Respects single whisper rule.
func (e *Engine) ShouldShowPressureCue(snapshot *externalpressure.PressureMapSnapshot, otherCueActive bool) bool {
	// Single whisper rule
	if otherCueActive {
		return false
	}

	// Must have pressure items
	if snapshot == nil || len(snapshot.Items) == 0 {
		return false
	}

	return true
}

// PeriodFromTime converts a time to a period key (daily bucket).
// Format: "YYYY-MM-DD"
func PeriodFromTime(t time.Time) string {
	year := t.Year()
	month := int(t.Month())
	day := t.Day()

	return formatDayPeriod(year, month, day)
}

// formatDayPeriod formats a date as a period key.
func formatDayPeriod(year, month, day int) string {
	// Manual formatting (stdlib only, no fmt import needed)
	yearStr := formatInt(year, 4)
	monthStr := formatInt(month, 2)
	dayStr := formatInt(day, 2)
	return yearStr + "-" + monthStr + "-" + dayStr
}

// formatInt formats an integer with leading zeros.
func formatInt(n, width int) string {
	result := ""
	for i := width - 1; i >= 0; i-- {
		divisor := 1
		for j := 0; j < i; j++ {
			divisor *= 10
		}
		digit := (n / divisor) % 10
		result += string(rune('0' + digit))
	}
	return result
}

// ComputeSovereignCircleIDHash computes a hash for a sovereign circle ID.
func ComputeSovereignCircleIDHash(circleID string) string {
	h := sha256.Sum256([]byte("sovereign|" + circleID))
	return hex.EncodeToString(h[:16])
}

// ConvertCommerceObservations converts commerce observer observations to pressure inputs.
// This is the bridge between Phase 31 commerce observations and Phase 31.4 pressure.
func ConvertCommerceObservations(
	sovereignCircleIDHash string,
	periodKey string,
	observations []CommerceObservationData,
) *externalpressure.PressureInputs {
	if len(observations) == 0 {
		return nil
	}

	inputs := &externalpressure.PressureInputs{
		SovereignCircleIDHash: sovereignCircleIDHash,
		PeriodKey:             periodKey,
		Observations:          make([]externalpressure.ObservationInput, 0, len(observations)),
	}

	for _, obs := range observations {
		input := externalpressure.ObservationInput{
			Source:       mapSourceKind(obs.Source),
			Category:     externalpressure.MapCommerceCategoryToPressure(obs.Category),
			EvidenceHash: obs.EvidenceHash,
		}
		inputs.Observations = append(inputs.Observations, input)
	}

	return inputs
}

// CommerceObservationData represents commerce observation data from Phase 31.
type CommerceObservationData struct {
	Source       string
	Category     string
	EvidenceHash string
}

// mapSourceKind maps commerce observer source kind to pressure source kind.
func mapSourceKind(source string) externalpressure.SourceKind {
	switch source {
	case "gmail_receipt":
		return externalpressure.SourceGmailReceipt
	case "finance_truelayer":
		return externalpressure.SourceFinanceTrueLayer
	default:
		return externalpressure.SourceGmailReceipt
	}
}

// ValidateForbiddenPatterns checks if any forbidden patterns appear in strings.
// Returns true if forbidden patterns are detected.
// CRITICAL: Use this to guard against merchant strings leaking through.
func ValidateForbiddenPatterns(values ...string) bool {
	forbidden := []string{
		// Common delivery services (abstract away)
		"deliveroo", "uber", "doordash", "grubhub", "postmates",
		"just eat", "justeat",
		// Common transport services
		"lyft", "bolt",
		// Common retail
		"amazon", "walmart", "target", "tesco", "sainsbury",
		"asda", "morrisons", "aldi", "lidl",
		// Payment providers
		"paypal", "venmo", "stripe",
		// Subscriptions
		"netflix", "spotify", "disney", "hulu", "hbo",
		// Email patterns
		"@", "http://", "https://",
		// Amounts
		"£", "$", "€", "¥",
	}

	for _, val := range values {
		lowerVal := toLower(val)
		for _, f := range forbidden {
			if containsIgnoreCase(lowerVal, f) {
				return true
			}
		}
	}

	return false
}

// toLower converts a string to lowercase (stdlib only).
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

// containsIgnoreCase checks if haystack contains needle (case-insensitive).
func containsIgnoreCase(haystack, needle string) bool {
	if len(needle) > len(haystack) {
		return false
	}
	for i := 0; i <= len(haystack)-len(needle); i++ {
		match := true
		for j := 0; j < len(needle); j++ {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
