// Package commerceobserver provides domain types for Phase 31: Commerce Observers.
//
// Commerce Observers are NOT finance. They are NOT budgeting. They are NOT insights.
// They are long-horizon behavioral signals that MAY matter someday, but usually do not.
//
// CRITICAL INVARIANTS:
//   - NO amounts, NO merchant names, NO timestamps, NO items
//   - Only category buckets, frequency buckets, stability buckets
//   - Deterministic outputs: sorted inputs, canonical strings, SHA256 hashing
//   - Default outcome: NOTHING SHOWN
//   - No goroutines. No time.Now() - clock injection only.
//
// This phase is OBSERVATION ONLY. Commerce is observed. Nothing else.
//
// Reference: docs/ADR/ADR-0062-phase31-commerce-observers.md
package commerceobserver

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// CategoryBucket represents an abstract commerce category.
// These are the only categories shown - never raw merchant names.
type CategoryBucket string

const (
	// CategoryFoodDelivery represents food delivery services.
	CategoryFoodDelivery CategoryBucket = "food_delivery"
	// CategoryTransport represents transportation services.
	CategoryTransport CategoryBucket = "transport"
	// CategoryRetail represents retail purchases.
	CategoryRetail CategoryBucket = "retail"
	// CategorySubscriptions represents recurring subscriptions.
	CategorySubscriptions CategoryBucket = "subscriptions"
	// CategoryUtilities represents utility payments.
	CategoryUtilities CategoryBucket = "utilities"
	// CategoryOther represents uncategorized items.
	CategoryOther CategoryBucket = "other"
)

// AllCategoryBuckets returns all category buckets in deterministic order.
func AllCategoryBuckets() []CategoryBucket {
	return []CategoryBucket{
		CategoryFoodDelivery,
		CategoryTransport,
		CategoryRetail,
		CategorySubscriptions,
		CategoryUtilities,
		CategoryOther,
	}
}

// Validate checks if the category bucket is valid.
func (c CategoryBucket) Validate() error {
	switch c {
	case CategoryFoodDelivery, CategoryTransport, CategoryRetail,
		CategorySubscriptions, CategoryUtilities, CategoryOther:
		return nil
	default:
		return fmt.Errorf("invalid category bucket: %s", c)
	}
}

// DisplayText returns calm, human-readable text for the category.
func (c CategoryBucket) DisplayText() string {
	switch c {
	case CategoryFoodDelivery:
		return "Food delivery"
	case CategoryTransport:
		return "Transport"
	case CategoryRetail:
		return "Retail"
	case CategorySubscriptions:
		return "Subscriptions"
	case CategoryUtilities:
		return "Utilities"
	case CategoryOther:
		return "Other"
	default:
		return "Unknown"
	}
}

// FrequencyBucket represents how often activity occurs in a category.
type FrequencyBucket string

const (
	// FrequencyRare indicates infrequent activity (< 2 per month).
	FrequencyRare FrequencyBucket = "rare"
	// FrequencyOccasional indicates occasional activity (2-8 per month).
	FrequencyOccasional FrequencyBucket = "occasional"
	// FrequencyFrequent indicates frequent activity (> 8 per month).
	FrequencyFrequent FrequencyBucket = "frequent"
)

// AllFrequencyBuckets returns all frequency buckets in deterministic order.
func AllFrequencyBuckets() []FrequencyBucket {
	return []FrequencyBucket{
		FrequencyRare,
		FrequencyOccasional,
		FrequencyFrequent,
	}
}

// ToFrequencyBucket converts a raw count to a frequency bucket.
// This is the ONLY place where raw counts are used.
func ToFrequencyBucket(countPerMonth int) FrequencyBucket {
	switch {
	case countPerMonth < 2:
		return FrequencyRare
	case countPerMonth <= 8:
		return FrequencyOccasional
	default:
		return FrequencyFrequent
	}
}

// Validate checks if the frequency bucket is valid.
func (f FrequencyBucket) Validate() error {
	switch f {
	case FrequencyRare, FrequencyOccasional, FrequencyFrequent:
		return nil
	default:
		return fmt.Errorf("invalid frequency bucket: %s", f)
	}
}

// DisplayText returns calm, human-readable text for the frequency.
func (f FrequencyBucket) DisplayText() string {
	switch f {
	case FrequencyRare:
		return "rarely"
	case FrequencyOccasional:
		return "occasionally"
	case FrequencyFrequent:
		return "frequently"
	default:
		return "unknown"
	}
}

// StabilityBucket represents how consistent activity is in a category.
type StabilityBucket string

const (
	// StabilityStable indicates consistent pattern over time.
	StabilityStable StabilityBucket = "stable"
	// StabilityDrifting indicates gradual change over time.
	StabilityDrifting StabilityBucket = "drifting"
	// StabilityVolatile indicates high variance over time.
	StabilityVolatile StabilityBucket = "volatile"
)

// AllStabilityBuckets returns all stability buckets in deterministic order.
func AllStabilityBuckets() []StabilityBucket {
	return []StabilityBucket{
		StabilityStable,
		StabilityDrifting,
		StabilityVolatile,
	}
}

// Validate checks if the stability bucket is valid.
func (s StabilityBucket) Validate() error {
	switch s {
	case StabilityStable, StabilityDrifting, StabilityVolatile:
		return nil
	default:
		return fmt.Errorf("invalid stability bucket: %s", s)
	}
}

// DisplayText returns calm, human-readable text for the stability.
func (s StabilityBucket) DisplayText() string {
	switch s {
	case StabilityStable:
		return "holding steady"
	case StabilityDrifting:
		return "shifting"
	case StabilityVolatile:
		return "varying"
	default:
		return "unknown"
	}
}

// CommerceObservation represents a single category observation.
//
// CRITICAL: Contains NO raw data, NO identifiable info.
// Only: category bucket, frequency bucket, stability bucket, period, evidence hash.
type CommerceObservation struct {
	// Category is the abstract category bucket.
	Category CategoryBucket

	// Frequency is how often activity occurs.
	Frequency FrequencyBucket

	// Stability is how consistent the pattern is.
	Stability StabilityBucket

	// Period is the observation period (e.g., "2024-W03" or "2024-01").
	Period string

	// EvidenceHash is a deterministic hash of what was observed.
	// Computed from abstract tokens only - never raw data.
	EvidenceHash string
}

// CanonicalString returns the pipe-delimited, version-prefixed canonical form.
func (o *CommerceObservation) CanonicalString() string {
	return fmt.Sprintf("COMMERCE_OBS|v1|%s|%s|%s|%s|%s",
		o.Category, o.Frequency, o.Stability, o.Period, o.EvidenceHash)
}

// ComputeHash computes a deterministic hash of the observation.
func (o *CommerceObservation) ComputeHash() string {
	h := sha256.Sum256([]byte(o.CanonicalString()))
	return hex.EncodeToString(h[:16]) // 32 hex chars
}

// Validate checks if the observation is valid.
func (o *CommerceObservation) Validate() error {
	if err := o.Category.Validate(); err != nil {
		return err
	}
	if err := o.Frequency.Validate(); err != nil {
		return err
	}
	if err := o.Stability.Validate(); err != nil {
		return err
	}
	if o.Period == "" {
		return fmt.Errorf("missing period")
	}
	if o.EvidenceHash == "" {
		return fmt.Errorf("missing evidence_hash")
	}
	return nil
}

// CommerceMirrorPage represents the commerce mirror proof page.
//
// CRITICAL: Contains NO raw data, NO identifiable info.
// Only: title, calm lines, category buckets, status hash.
// NO buttons, NO actions - back link only.
type CommerceMirrorPage struct {
	// Title is the page title. Always "Seen, quietly."
	Title string

	// Lines contains 1-2 calm sentences only.
	Lines []string

	// Buckets contains up to 3 category buckets (deterministic selection).
	Buckets []CategoryBucket

	// StatusHash is a deterministic hash of the page content.
	StatusHash string
}

// MaxBuckets is the maximum number of category buckets shown.
const MaxBuckets = 3

// MaxLines is the maximum number of calm lines.
const MaxLines = 2

// DefaultTitle is the standard mirror page title.
const DefaultTitle = "Seen, quietly."

// CalmLines are the possible calm messages.
var CalmLines = []string{
	"Some routines appear to be holding steady.",
	"A few patterns were noticed — nothing urgent.",
	"Quiet observation continues.",
}

// NewCommerceMirrorPage creates a new commerce mirror page.
// Returns nil if no observations (silence is success).
func NewCommerceMirrorPage(observations []CommerceObservation) *CommerceMirrorPage {
	if len(observations) == 0 {
		return nil
	}

	// Collect unique categories deterministically
	catSet := make(map[CategoryBucket]bool)
	for _, obs := range observations {
		catSet[obs.Category] = true
	}

	// Convert to sorted slice
	cats := make([]CategoryBucket, 0, len(catSet))
	for cat := range catSet {
		cats = append(cats, cat)
	}
	sort.Slice(cats, func(i, j int) bool {
		return string(cats[i]) < string(cats[j])
	})

	// Limit to MaxBuckets
	if len(cats) > MaxBuckets {
		cats = cats[:MaxBuckets]
	}

	// Select calm lines based on observation characteristics
	lines := selectCalmLines(observations)

	page := &CommerceMirrorPage{
		Title:   DefaultTitle,
		Lines:   lines,
		Buckets: cats,
	}

	page.StatusHash = page.ComputeHash()
	return page
}

// selectCalmLines selects appropriate calm lines based on observations.
// Deterministic: same observations always produce same lines.
func selectCalmLines(observations []CommerceObservation) []string {
	if len(observations) == 0 {
		return nil
	}

	// Check for stability patterns
	hasStable := false
	for _, obs := range observations {
		if obs.Stability == StabilityStable {
			hasStable = true
			break
		}
	}

	if hasStable {
		return []string{CalmLines[0]} // "Some routines appear to be holding steady."
	}

	if len(observations) <= 2 {
		return []string{CalmLines[1]} // "A few patterns were noticed — nothing urgent."
	}

	return []string{CalmLines[2]} // "Quiet observation continues."
}

// CanonicalString returns the pipe-delimited, version-prefixed canonical form.
func (p *CommerceMirrorPage) CanonicalString() string {
	var b strings.Builder
	b.WriteString("COMMERCE_MIRROR|v1|")
	b.WriteString(p.Title)
	b.WriteString("|")

	// Lines
	for i, line := range p.Lines {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(line)
	}
	b.WriteString("|")

	// Buckets (already sorted)
	for i, bucket := range p.Buckets {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(string(bucket))
	}

	return b.String()
}

// ComputeHash computes a deterministic hash of the page.
func (p *CommerceMirrorPage) ComputeHash() string {
	h := sha256.Sum256([]byte(p.CanonicalString()))
	return hex.EncodeToString(h[:16]) // 32 hex chars
}

// Validate checks if the page is valid.
func (p *CommerceMirrorPage) Validate() error {
	if p.Title == "" {
		return fmt.Errorf("missing title")
	}
	if len(p.Lines) == 0 {
		return fmt.Errorf("missing lines")
	}
	if len(p.Lines) > MaxLines {
		return fmt.Errorf("too many lines: %d > %d", len(p.Lines), MaxLines)
	}
	if len(p.Buckets) > MaxBuckets {
		return fmt.Errorf("too many buckets: %d > %d", len(p.Buckets), MaxBuckets)
	}
	for _, bucket := range p.Buckets {
		if err := bucket.Validate(); err != nil {
			return err
		}
	}
	if p.StatusHash == "" {
		return fmt.Errorf("missing status_hash")
	}
	return nil
}

// CommerceCue represents the whisper cue for commerce mirror.
type CommerceCue struct {
	// Available indicates if the cue should be shown.
	Available bool

	// CueText is the subtle cue text.
	CueText string

	// LinkText is the link text.
	LinkText string

	// CueHash is a deterministic hash of the cue.
	CueHash string
}

// DefaultCueText is the standard cue text.
const DefaultCueText = "We noticed a few routines — quietly."

// DefaultLinkText is the standard link text.
const DefaultLinkText = "see"

// NewCommerceCue creates a cue for display.
func NewCommerceCue(available bool) *CommerceCue {
	if !available {
		return &CommerceCue{Available: false}
	}
	cue := &CommerceCue{
		Available: true,
		CueText:   DefaultCueText,
		LinkText:  DefaultLinkText,
	}
	cue.CueHash = hashString(fmt.Sprintf("v1|commerce_cue|%s|%s", cue.CueText, cue.LinkText))
	return cue
}

// CanonicalString returns the pipe-delimited, version-prefixed canonical form.
func (c *CommerceCue) CanonicalString() string {
	availStr := "false"
	if c.Available {
		availStr = "true"
	}
	return fmt.Sprintf("COMMERCE_CUE|v1|%s|%s|%s|%s",
		availStr, c.CueText, c.LinkText, c.CueHash)
}

// Validate checks if the cue is valid.
func (c *CommerceCue) Validate() error {
	if c.Available {
		if c.CueText == "" {
			return fmt.Errorf("missing cue_text")
		}
		if c.LinkText == "" {
			return fmt.Errorf("missing link_text")
		}
		if c.CueHash == "" {
			return fmt.Errorf("missing cue_hash")
		}
	}
	return nil
}

// CommerceInputs captures all inputs needed to compute observations.
// These are gathered from transaction metadata only - no raw data stored.
type CommerceInputs struct {
	// CircleID identifies the circle.
	CircleID string

	// Period is the observation period (e.g., "2024-W03").
	Period string

	// CategoryCounts contains raw counts per category.
	// Will be converted to frequency buckets immediately.
	CategoryCounts map[CategoryBucket]int

	// CategoryTrends contains trend direction per category.
	// Values: "stable", "increasing", "decreasing"
	CategoryTrends map[CategoryBucket]string
}

// CanonicalString returns the pipe-delimited, version-prefixed canonical form.
func (i *CommerceInputs) CanonicalString() string {
	var b strings.Builder
	b.WriteString("COMMERCE_INPUTS|v1|")
	b.WriteString(i.CircleID)
	b.WriteString("|")
	b.WriteString(i.Period)

	// Sorted categories for determinism
	cats := make([]CategoryBucket, 0, len(i.CategoryCounts))
	for cat := range i.CategoryCounts {
		cats = append(cats, cat)
	}
	sort.Slice(cats, func(a, c int) bool {
		return string(cats[a]) < string(cats[c])
	})

	for _, cat := range cats {
		freq := ToFrequencyBucket(i.CategoryCounts[cat])
		trend := i.CategoryTrends[cat]
		if trend == "" {
			trend = "stable"
		}
		b.WriteString("|")
		b.WriteString(string(cat))
		b.WriteString(":")
		b.WriteString(string(freq))
		b.WriteString(":")
		b.WriteString(trend)
	}

	return b.String()
}

// ComputeHash computes a deterministic hash of the inputs.
func (i *CommerceInputs) ComputeHash() string {
	h := sha256.Sum256([]byte(i.CanonicalString()))
	return hex.EncodeToString(h[:16])
}

// hashString computes a SHA256 hash of the input string.
func hashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:16])
}

// TrendToStability converts a trend string to a stability bucket.
func TrendToStability(trend string) StabilityBucket {
	switch trend {
	case "stable":
		return StabilityStable
	case "increasing", "decreasing":
		return StabilityDrifting
	default:
		return StabilityVolatile
	}
}

// ComputeEvidenceHash computes a deterministic hash from abstract tokens.
func ComputeEvidenceHash(tokens []string) string {
	if len(tokens) == 0 {
		return "empty"
	}

	// Sort tokens for determinism
	sorted := make([]string, len(tokens))
	copy(sorted, tokens)
	sort.Strings(sorted)

	// Create canonical string
	var b strings.Builder
	b.WriteString("COMMERCE_EVIDENCE|v1")
	for _, t := range sorted {
		b.WriteString("|")
		b.WriteString(t)
	}

	h := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(h[:16])
}
