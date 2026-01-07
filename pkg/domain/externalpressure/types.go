// Package externalpressure provides domain types for Phase 31.4: External Pressure Circles.
//
// External Pressure Circles are DERIVED from commerce observations. They represent
// abstract "external forces" that may influence a sovereign circle through intersections.
// They are NOT user-manageable, NOT surfaced by name, and MUST NOT create execution capability.
//
// CRITICAL INVARIANTS:
//   - NO raw merchant strings, NO vendor identifiers, NO amounts, NO timestamps
//   - Only category hints, magnitude buckets, and horizon buckets
//   - Derived circles CANNOT approve, CANNOT execute, CANNOT receive drafts
//   - Hash-only persistence; deterministic: same inputs => same hashes
//   - No goroutines. No time.Now() - clock injection only.
//
// Reference: docs/ADR/ADR-0067-phase31-4-external-pressure-circles.md
package externalpressure

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// CircleKind distinguishes sovereign circles from derived external circles.
type CircleKind string

const (
	// CircleKindSovereign represents a sovereign circle (me, family, work, etc.).
	// These are user-owned and have full execution capability.
	CircleKindSovereign CircleKind = "sovereign"

	// CircleKindExternalDerived represents a derived external pressure circle.
	// These are NEVER user-manageable and CANNOT execute.
	CircleKindExternalDerived CircleKind = "external_derived"
)

// Validate checks if the circle kind is valid.
func (k CircleKind) Validate() error {
	switch k {
	case CircleKindSovereign, CircleKindExternalDerived:
		return nil
	default:
		return fmt.Errorf("invalid circle kind: %s", k)
	}
}

// SourceKind represents the origin of commerce data.
type SourceKind string

const (
	// SourceGmailReceipt indicates observation from Gmail receipt classification (Phase 31.1).
	SourceGmailReceipt SourceKind = "gmail_receipt"
	// SourceFinanceTrueLayer indicates observation from TrueLayer transaction sync (Phase 31.2/31.3b).
	SourceFinanceTrueLayer SourceKind = "finance_truelayer"
)

// AllSourceKinds returns all source kinds in deterministic order.
func AllSourceKinds() []SourceKind {
	return []SourceKind{
		SourceGmailReceipt,
		SourceFinanceTrueLayer,
	}
}

// Validate checks if the source kind is valid.
func (s SourceKind) Validate() error {
	switch s {
	case SourceGmailReceipt, SourceFinanceTrueLayer:
		return nil
	default:
		return fmt.Errorf("invalid source kind: %s", s)
	}
}

// DisplayText returns abstract human-readable text for the source.
// CRITICAL: Never expose provider names in user-facing text.
func (s SourceKind) DisplayText() string {
	switch s {
	case SourceGmailReceipt:
		return "email"
	case SourceFinanceTrueLayer:
		return "bank"
	default:
		return "unknown"
	}
}

// PressureCategory represents an abstract category of external pressure.
// CRITICAL: These are abstract buckets, NOT merchant identifiers.
type PressureCategory string

const (
	// PressureCategoryDelivery represents delivery-related pressure.
	PressureCategoryDelivery PressureCategory = "delivery"
	// PressureCategoryTransport represents transport-related pressure.
	PressureCategoryTransport PressureCategory = "transport"
	// PressureCategoryRetail represents retail-related pressure.
	PressureCategoryRetail PressureCategory = "retail"
	// PressureCategorySubscription represents subscription-related pressure.
	PressureCategorySubscription PressureCategory = "subscription"
	// PressureCategoryOther represents uncategorized pressure.
	PressureCategoryOther PressureCategory = "other"
)

// AllPressureCategories returns all categories in deterministic order.
func AllPressureCategories() []PressureCategory {
	return []PressureCategory{
		PressureCategoryDelivery,
		PressureCategoryTransport,
		PressureCategoryRetail,
		PressureCategorySubscription,
		PressureCategoryOther,
	}
}

// Validate checks if the pressure category is valid.
func (c PressureCategory) Validate() error {
	switch c {
	case PressureCategoryDelivery, PressureCategoryTransport, PressureCategoryRetail,
		PressureCategorySubscription, PressureCategoryOther:
		return nil
	default:
		return fmt.Errorf("invalid pressure category: %s", c)
	}
}

// DisplayText returns calm, human-readable text for the category.
func (c PressureCategory) DisplayText() string {
	switch c {
	case PressureCategoryDelivery:
		return "Delivery"
	case PressureCategoryTransport:
		return "Transport"
	case PressureCategoryRetail:
		return "Retail"
	case PressureCategorySubscription:
		return "Subscriptions"
	case PressureCategoryOther:
		return "Other"
	default:
		return "Unknown"
	}
}

// PressureMagnitude represents how much pressure exists (abstract).
// CRITICAL: No raw counts, NO amounts.
type PressureMagnitude string

const (
	// PressureMagnitudeNothing indicates no meaningful pressure.
	PressureMagnitudeNothing PressureMagnitude = "nothing"
	// PressureMagnitudeAFew indicates a small amount of pressure (1-3).
	PressureMagnitudeAFew PressureMagnitude = "a_few"
	// PressureMagnitudeSeveral indicates moderate pressure (4+).
	PressureMagnitudeSeveral PressureMagnitude = "several"
)

// AllPressureMagnitudes returns all magnitudes in deterministic order.
func AllPressureMagnitudes() []PressureMagnitude {
	return []PressureMagnitude{
		PressureMagnitudeNothing,
		PressureMagnitudeAFew,
		PressureMagnitudeSeveral,
	}
}

// Validate checks if the magnitude is valid.
func (m PressureMagnitude) Validate() error {
	switch m {
	case PressureMagnitudeNothing, PressureMagnitudeAFew, PressureMagnitudeSeveral:
		return nil
	default:
		return fmt.Errorf("invalid pressure magnitude: %s", m)
	}
}

// DisplayText returns calm, human-readable text for the magnitude.
func (m PressureMagnitude) DisplayText() string {
	switch m {
	case PressureMagnitudeNothing:
		return "nothing"
	case PressureMagnitudeAFew:
		return "a few"
	case PressureMagnitudeSeveral:
		return "several"
	default:
		return "unknown"
	}
}

// ToPressureMagnitude converts a raw count to a pressure magnitude bucket.
// This is the ONLY place where raw counts are used.
func ToPressureMagnitude(count int) PressureMagnitude {
	switch {
	case count == 0:
		return PressureMagnitudeNothing
	case count <= 3:
		return PressureMagnitudeAFew
	default:
		return PressureMagnitudeSeveral
	}
}

// PressureHorizon represents the time horizon of pressure (optional).
// CRITICAL: No raw timestamps.
type PressureHorizon string

const (
	// PressureHorizonSoon indicates pressure likely applies soon.
	PressureHorizonSoon PressureHorizon = "soon"
	// PressureHorizonLater indicates pressure applies later.
	PressureHorizonLater PressureHorizon = "later"
	// PressureHorizonUnknown indicates no horizon information available.
	PressureHorizonUnknown PressureHorizon = "unknown"
)

// AllPressureHorizons returns all horizons in deterministic order.
func AllPressureHorizons() []PressureHorizon {
	return []PressureHorizon{
		PressureHorizonSoon,
		PressureHorizonLater,
		PressureHorizonUnknown,
	}
}

// Validate checks if the horizon is valid.
func (h PressureHorizon) Validate() error {
	switch h {
	case PressureHorizonSoon, PressureHorizonLater, PressureHorizonUnknown:
		return nil
	default:
		return fmt.Errorf("invalid pressure horizon: %s", h)
	}
}

// DisplayText returns calm, human-readable text for the horizon.
func (h PressureHorizon) DisplayText() string {
	switch h {
	case PressureHorizonSoon:
		return "soon"
	case PressureHorizonLater:
		return "later"
	case PressureHorizonUnknown:
		return ""
	default:
		return ""
	}
}

// ExternalDerivedCircle represents a derived external pressure circle.
// CRITICAL: This is NOT user-manageable. It is derived from commerce observations.
// CRITICAL: CANNOT approve, CANNOT execute, CANNOT receive drafts.
type ExternalDerivedCircle struct {
	// CircleIDHash is the deterministic hash of the external circle key.
	// Computed from: "external|" + source_kind + "|" + category + "|" + sovereign_circle_id_hash
	CircleIDHash string

	// SourceKind indicates where this circle was derived from.
	SourceKind SourceKind

	// CategoryHint is the abstract category (never a merchant name).
	CategoryHint PressureCategory

	// CreatedPeriod is the bucketed period when this circle was first observed.
	// Format: "YYYY-MM-DD" (day bucket, not timestamp)
	CreatedPeriod string

	// EvidenceHash is a deterministic hash of the evidence that created this circle.
	EvidenceHash string
}

// CanonicalString returns the pipe-delimited, version-prefixed canonical form.
func (c *ExternalDerivedCircle) CanonicalString() string {
	return fmt.Sprintf("EXT_CIRCLE|v1|%s|%s|%s|%s|%s",
		c.CircleIDHash, c.SourceKind, c.CategoryHint, c.CreatedPeriod, c.EvidenceHash)
}

// ComputeHash computes a deterministic hash of the circle.
func (c *ExternalDerivedCircle) ComputeHash() string {
	h := sha256.Sum256([]byte(c.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// Validate checks if the circle is valid.
func (c *ExternalDerivedCircle) Validate() error {
	if c.CircleIDHash == "" {
		return fmt.Errorf("missing circle_id_hash")
	}
	if err := c.SourceKind.Validate(); err != nil {
		return err
	}
	if err := c.CategoryHint.Validate(); err != nil {
		return err
	}
	if c.CreatedPeriod == "" {
		return fmt.Errorf("missing created_period")
	}
	if c.EvidenceHash == "" {
		return fmt.Errorf("missing evidence_hash")
	}
	return nil
}

// ComputeExternalCircleID computes the deterministic external circle ID hash.
// Key format: "external|" + source_kind + "|" + category + "|" + sovereign_circle_id_hash
func ComputeExternalCircleID(sourceKind SourceKind, category PressureCategory, sovereignCircleIDHash string) string {
	key := fmt.Sprintf("external|%s|%s|%s", sourceKind, category, sovereignCircleIDHash)
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:16])
}

// PressureItem represents a single pressure item in the pressure map.
// CRITICAL: Contains only abstract buckets, never raw data.
type PressureItem struct {
	// Category is the abstract pressure category.
	Category PressureCategory

	// Magnitude is the abstract pressure magnitude.
	Magnitude PressureMagnitude

	// Horizon is the optional time horizon.
	Horizon PressureHorizon

	// SourceKindsPresent indicates which sources contributed to this item.
	// Stored as sorted strings for determinism.
	SourceKindsPresent []SourceKind

	// EvidenceHash is a deterministic hash of the evidence.
	EvidenceHash string
}

// CanonicalString returns the pipe-delimited canonical form.
func (p *PressureItem) CanonicalString() string {
	// Sort source kinds for determinism
	sources := make([]string, len(p.SourceKindsPresent))
	for i, s := range p.SourceKindsPresent {
		sources[i] = string(s)
	}
	sort.Strings(sources)

	return fmt.Sprintf("PRESSURE_ITEM|v1|%s|%s|%s|%s|%s",
		p.Category, p.Magnitude, p.Horizon, strings.Join(sources, ","), p.EvidenceHash)
}

// ComputeHash computes a deterministic hash of the item.
func (p *PressureItem) ComputeHash() string {
	h := sha256.Sum256([]byte(p.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// Validate checks if the pressure item is valid.
func (p *PressureItem) Validate() error {
	if err := p.Category.Validate(); err != nil {
		return err
	}
	if err := p.Magnitude.Validate(); err != nil {
		return err
	}
	if err := p.Horizon.Validate(); err != nil {
		return err
	}
	for _, s := range p.SourceKindsPresent {
		if err := s.Validate(); err != nil {
			return err
		}
	}
	if p.EvidenceHash == "" {
		return fmt.Errorf("missing evidence_hash")
	}
	return nil
}

// MaxPressureItems is the maximum number of pressure items shown.
const MaxPressureItems = 3

// PressureMapSnapshot represents a snapshot of pressure for a sovereign circle.
// CRITICAL: Contains only abstract buckets and hashes, never raw data.
type PressureMapSnapshot struct {
	// SovereignCircleIDHash identifies the sovereign circle.
	SovereignCircleIDHash string

	// PeriodKey is the daily bucket period (format: "YYYY-MM-DD").
	PeriodKey string

	// Items contains up to MaxPressureItems pressure items.
	Items []PressureItem

	// StatusHash is a deterministic hash of the snapshot.
	StatusHash string
}

// CanonicalString returns the pipe-delimited, version-prefixed canonical form.
// Items are sorted deterministically before serialization.
func (s *PressureMapSnapshot) CanonicalString() string {
	var b strings.Builder
	b.WriteString("PRESSURE_MAP|v1|")
	b.WriteString(s.SovereignCircleIDHash)
	b.WriteString("|")
	b.WriteString(s.PeriodKey)

	// Sort items by category for determinism
	sortedItems := make([]PressureItem, len(s.Items))
	copy(sortedItems, s.Items)
	sort.Slice(sortedItems, func(i, j int) bool {
		return string(sortedItems[i].Category) < string(sortedItems[j].Category)
	})

	for _, item := range sortedItems {
		b.WriteString("|")
		b.WriteString(item.CanonicalString())
	}

	return b.String()
}

// ComputeHash computes a deterministic hash of the snapshot.
func (s *PressureMapSnapshot) ComputeHash() string {
	h := sha256.Sum256([]byte(s.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// Validate checks if the snapshot is valid.
func (s *PressureMapSnapshot) Validate() error {
	if s.SovereignCircleIDHash == "" {
		return fmt.Errorf("missing sovereign_circle_id_hash")
	}
	if s.PeriodKey == "" {
		return fmt.Errorf("missing period_key")
	}
	if len(s.Items) > MaxPressureItems {
		return fmt.Errorf("too many items: %d > %d", len(s.Items), MaxPressureItems)
	}
	for _, item := range s.Items {
		if err := item.Validate(); err != nil {
			return err
		}
	}
	if s.StatusHash == "" {
		return fmt.Errorf("missing status_hash")
	}
	return nil
}

// PressureInputs captures all inputs needed to compute the pressure map.
// These are gathered from commerce observations only - no raw data.
type PressureInputs struct {
	// SovereignCircleIDHash identifies the sovereign circle.
	SovereignCircleIDHash string

	// Observations contains commerce observations from Phase 31 store.
	Observations []ObservationInput

	// PeriodKey is the target period (format: "YYYY-MM-DD").
	PeriodKey string
}

// ObservationInput represents a commerce observation input for pressure computation.
// CRITICAL: Contains only abstract data from commerce observer store.
type ObservationInput struct {
	// Source indicates where this observation originated.
	Source SourceKind

	// Category is the abstract category bucket.
	Category PressureCategory

	// EvidenceHash is the hash of the original observation.
	EvidenceHash string
}

// CanonicalString returns the pipe-delimited canonical form.
func (i *PressureInputs) CanonicalString() string {
	var b strings.Builder
	b.WriteString("PRESSURE_INPUTS|v1|")
	b.WriteString(i.SovereignCircleIDHash)
	b.WriteString("|")
	b.WriteString(i.PeriodKey)

	// Sort observations for determinism
	sortedObs := make([]ObservationInput, len(i.Observations))
	copy(sortedObs, i.Observations)
	sort.Slice(sortedObs, func(a, c int) bool {
		if sortedObs[a].Source != sortedObs[c].Source {
			return string(sortedObs[a].Source) < string(sortedObs[c].Source)
		}
		if sortedObs[a].Category != sortedObs[c].Category {
			return string(sortedObs[a].Category) < string(sortedObs[c].Category)
		}
		return sortedObs[a].EvidenceHash < sortedObs[c].EvidenceHash
	})

	for _, obs := range sortedObs {
		b.WriteString("|")
		b.WriteString(string(obs.Source))
		b.WriteString(":")
		b.WriteString(string(obs.Category))
		b.WriteString(":")
		b.WriteString(obs.EvidenceHash)
	}

	return b.String()
}

// ComputeHash computes a deterministic hash of the inputs.
func (i *PressureInputs) ComputeHash() string {
	h := sha256.Sum256([]byte(i.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// PressureProofPage represents the pressure proof page for display.
// CRITICAL: Contains NO raw data, NO identifiable info.
// NO buttons, NO actions - back link only.
type PressureProofPage struct {
	// Title is the page title. Always "External, contained."
	Title string

	// Subtitle is the calm subtitle.
	Subtitle string

	// CategoryChips contains up to 3 category chips.
	CategoryChips []PressureCategory

	// MagnitudeText is the abstract magnitude description.
	MagnitudeText string

	// SourcesText describes sources abstractly (e.g., "email, bank").
	SourcesText string

	// StatusHash is a deterministic hash of the page.
	StatusHash string
}

// DefaultPressureTitle is the standard proof page title.
const DefaultPressureTitle = "External, contained."

// DefaultPressureSubtitle is the standard proof page subtitle.
const DefaultPressureSubtitle = "Some external pressure exists — and it's being held quietly."

// NewPressureProofPage creates a new pressure proof page from a snapshot.
// Returns nil if no items (silence is success).
func NewPressureProofPage(snapshot *PressureMapSnapshot) *PressureProofPage {
	if snapshot == nil || len(snapshot.Items) == 0 {
		return nil
	}

	// Collect unique categories
	catSet := make(map[PressureCategory]bool)
	for _, item := range snapshot.Items {
		catSet[item.Category] = true
	}

	// Convert to sorted slice
	cats := make([]PressureCategory, 0, len(catSet))
	for cat := range catSet {
		cats = append(cats, cat)
	}
	sort.Slice(cats, func(i, j int) bool {
		return string(cats[i]) < string(cats[j])
	})

	// Limit to MaxPressureItems
	if len(cats) > MaxPressureItems {
		cats = cats[:MaxPressureItems]
	}

	// Compute magnitude text from max magnitude
	maxMagnitude := PressureMagnitudeNothing
	for _, item := range snapshot.Items {
		if item.Magnitude == PressureMagnitudeSeveral {
			maxMagnitude = PressureMagnitudeSeveral
			break
		}
		if item.Magnitude == PressureMagnitudeAFew && maxMagnitude != PressureMagnitudeSeveral {
			maxMagnitude = PressureMagnitudeAFew
		}
	}

	// Collect unique sources
	sourceSet := make(map[SourceKind]bool)
	for _, item := range snapshot.Items {
		for _, s := range item.SourceKindsPresent {
			sourceSet[s] = true
		}
	}

	// Build sources text
	var sourceTexts []string
	for _, s := range AllSourceKinds() {
		if sourceSet[s] {
			sourceTexts = append(sourceTexts, s.DisplayText())
		}
	}
	sourcesText := strings.Join(sourceTexts, ", ")

	page := &PressureProofPage{
		Title:         DefaultPressureTitle,
		Subtitle:      DefaultPressureSubtitle,
		CategoryChips: cats,
		MagnitudeText: maxMagnitude.DisplayText(),
		SourcesText:   sourcesText,
	}

	page.StatusHash = page.ComputeHash()
	return page
}

// CanonicalString returns the pipe-delimited canonical form.
func (p *PressureProofPage) CanonicalString() string {
	var b strings.Builder
	b.WriteString("PRESSURE_PAGE|v1|")
	b.WriteString(p.Title)
	b.WriteString("|")
	b.WriteString(p.Subtitle)
	b.WriteString("|")

	// Categories
	for i, cat := range p.CategoryChips {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(string(cat))
	}
	b.WriteString("|")
	b.WriteString(p.MagnitudeText)
	b.WriteString("|")
	b.WriteString(p.SourcesText)

	return b.String()
}

// ComputeHash computes a deterministic hash of the page.
func (p *PressureProofPage) ComputeHash() string {
	h := sha256.Sum256([]byte(p.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// Validate checks if the page is valid.
func (p *PressureProofPage) Validate() error {
	if p.Title == "" {
		return fmt.Errorf("missing title")
	}
	if p.Subtitle == "" {
		return fmt.Errorf("missing subtitle")
	}
	if len(p.CategoryChips) > MaxPressureItems {
		return fmt.Errorf("too many category chips: %d > %d", len(p.CategoryChips), MaxPressureItems)
	}
	for _, cat := range p.CategoryChips {
		if err := cat.Validate(); err != nil {
			return err
		}
	}
	if p.StatusHash == "" {
		return fmt.Errorf("missing status_hash")
	}
	return nil
}

// MapCommerceCategoryToPressure maps commerce observer category to pressure category.
// CRITICAL: This is the ONLY mapping between the two systems.
func MapCommerceCategoryToPressure(commerceCategory string) PressureCategory {
	switch commerceCategory {
	case "food_delivery":
		return PressureCategoryDelivery
	case "transport":
		return PressureCategoryTransport
	case "retail":
		return PressureCategoryRetail
	case "subscriptions":
		return PressureCategorySubscription
	default:
		return PressureCategoryOther
	}
}

// hashString computes a SHA256 hash of the input string.
func hashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:16])
}
