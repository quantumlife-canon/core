// Package financemirror provides domain types for Phase 29: Finance Mirror Proof.
//
// This phase implements read-only TrueLayer connect + finance mirror proof page.
// After sync: a calm proof page showing "Seen, quietly." with abstract buckets only.
//
// CRITICAL INVARIANTS:
//   - NO raw account data, NO amounts, NO merchants, NO bank names
//   - Only magnitude buckets and category buckets
//   - Deterministic outputs: sorted inputs, canonical strings, SHA256 hashing
//   - Privacy guard blocks any tokens containing identifiable data
//   - No goroutines. No time.Now() - clock injection only.
//
// Reference: docs/ADR/ADR-0060-phase29-truelayer-readonly-finance-mirror.md
package financemirror

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"
)

// MagnitudeBucket represents an abstract count bucket.
// Never stores raw counts - only these abstract categories.
type MagnitudeBucket string

const (
	// MagnitudeNothing indicates zero items.
	MagnitudeNothing MagnitudeBucket = "nothing"
	// MagnitudeAFew indicates 1-3 items.
	MagnitudeAFew MagnitudeBucket = "a_few"
	// MagnitudeSeveral indicates 4-10 items.
	MagnitudeSeveral MagnitudeBucket = "several"
	// MagnitudeMany indicates 11+ items.
	MagnitudeMany MagnitudeBucket = "many"
)

// ToMagnitudeBucket converts a raw count to a magnitude bucket.
// This is the ONLY place where raw counts are used.
func ToMagnitudeBucket(count int) MagnitudeBucket {
	switch {
	case count == 0:
		return MagnitudeNothing
	case count <= 3:
		return MagnitudeAFew
	case count <= 10:
		return MagnitudeSeveral
	default:
		return MagnitudeMany
	}
}

// DisplayText returns calm, human-readable text for the bucket.
func (m MagnitudeBucket) DisplayText() string {
	switch m {
	case MagnitudeNothing:
		return "nothing"
	case MagnitudeAFew:
		return "a few"
	case MagnitudeSeveral:
		return "several"
	case MagnitudeMany:
		return "many"
	default:
		return "unknown"
	}
}

// CategoryBucket represents an abstract financial category.
// These are the only categories shown - never raw merchant names.
type CategoryBucket string

const (
	// CategoryLiquidity represents available funds.
	CategoryLiquidity CategoryBucket = "liquidity"
	// CategoryObligations represents recurring commitments.
	CategoryObligations CategoryBucket = "obligations"
	// CategoryUpcomingPressure represents near-term outflows.
	CategoryUpcomingPressure CategoryBucket = "upcoming_pressure"
	// CategorySpendPattern represents recent spending pattern.
	CategorySpendPattern CategoryBucket = "spend_pattern"
)

// AllCategories returns all category buckets in deterministic order.
func AllCategories() []CategoryBucket {
	return []CategoryBucket{
		CategoryLiquidity,
		CategoryObligations,
		CategoryUpcomingPressure,
		CategorySpendPattern,
	}
}

// DisplayText returns calm, human-readable text for the category.
func (c CategoryBucket) DisplayText() string {
	switch c {
	case CategoryLiquidity:
		return "Funds available"
	case CategoryObligations:
		return "Recurring commitments"
	case CategoryUpcomingPressure:
		return "Near-term outflows"
	case CategorySpendPattern:
		return "Recent activity"
	default:
		return "Unknown"
	}
}

// CategorySignal represents a single category observation.
// Contains only abstract buckets - never raw amounts.
type CategorySignal struct {
	Category  CategoryBucket
	Magnitude MagnitudeBucket
	// Trend is optional: "stable", "increasing", "decreasing", or empty
	Trend string
}

// CanonicalString returns the canonical string representation.
func (c *CategorySignal) CanonicalString() string {
	return fmt.Sprintf("v1|category_signal|%s|%s|%s",
		c.Category, c.Magnitude, c.Trend)
}

// TimeBucket floors a timestamp to 5-minute intervals for privacy.
func TimeBucket(t time.Time) time.Time {
	return t.Truncate(5 * time.Minute)
}

// PeriodBucket returns the day bucket string (YYYY-MM-DD).
func PeriodBucket(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

// FinanceSyncReceipt represents a sync operation receipt.
//
// CRITICAL: Contains NO raw data, NO identifiable info.
// Only: circle_id, magnitude_buckets, time_bucket, evidence_hash.
type FinanceSyncReceipt struct {
	// ReceiptID uniquely identifies this receipt (deterministic hash).
	ReceiptID string

	// CircleID identifies the circle this sync was for.
	CircleID string

	// Provider identifies the sync source (e.g., "truelayer").
	Provider string

	// TimeBucket is the floored sync time (5-minute granularity).
	TimeBucket time.Time

	// PeriodBucket is the day bucket (YYYY-MM-DD).
	PeriodBucket string

	// AccountsMagnitude is the abstract count of accounts seen.
	AccountsMagnitude MagnitudeBucket

	// TransactionsMagnitude is the abstract count of transactions seen.
	TransactionsMagnitude MagnitudeBucket

	// EvidenceHash is a deterministic hash of what was seen.
	// Computed from sorted, abstract tokens only.
	EvidenceHash string

	// Success indicates if the sync completed successfully.
	Success bool

	// FailReason is set only if Success is false.
	// Contains generic reason, never raw error messages with PII.
	FailReason string

	// StatusHash is the overall receipt hash.
	StatusHash string
}

// NewFinanceSyncReceipt creates a new finance sync receipt.
// The receipt_id and hash are computed deterministically.
func NewFinanceSyncReceipt(
	circleID string,
	provider string,
	syncTime time.Time,
	accountsCount int,
	transactionsCount int,
	evidenceTokens []string,
	success bool,
	failReason string,
) *FinanceSyncReceipt {
	timeBucket := TimeBucket(syncTime)
	periodBucket := PeriodBucket(syncTime)

	// Convert counts to magnitude buckets
	accountsMagnitude := ToMagnitudeBucket(accountsCount)
	transactionsMagnitude := ToMagnitudeBucket(transactionsCount)

	// Compute evidence hash from sorted tokens
	evidenceHash := computeEvidenceHash(evidenceTokens)

	r := &FinanceSyncReceipt{
		CircleID:              circleID,
		Provider:              provider,
		TimeBucket:            timeBucket,
		PeriodBucket:          periodBucket,
		AccountsMagnitude:     accountsMagnitude,
		TransactionsMagnitude: transactionsMagnitude,
		EvidenceHash:          evidenceHash,
		Success:               success,
		FailReason:            failReason,
	}

	// Compute receipt ID and status hash
	r.ReceiptID = r.computeReceiptID()
	r.StatusHash = r.computeStatusHash()

	return r
}

// computeEvidenceHash computes a deterministic hash from sorted evidence tokens.
func computeEvidenceHash(tokens []string) string {
	if len(tokens) == 0 {
		return "empty"
	}

	// Sort tokens for determinism
	sorted := make([]string, len(tokens))
	copy(sorted, tokens)
	sort.Strings(sorted)

	// Create canonical string
	canonical := fmt.Sprintf("EVIDENCE|v1|%d", len(sorted))
	for _, t := range sorted {
		canonical += "|" + t
	}

	h := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(h[:16]) // 32 hex chars
}

// computeReceiptID generates a deterministic receipt ID.
func (r *FinanceSyncReceipt) computeReceiptID() string {
	canonical := fmt.Sprintf("FINANCE_SYNC_RECEIPT_ID|v1|%s|%s|%s|%d",
		r.CircleID, r.Provider, r.PeriodBucket, r.TimeBucket.Unix())
	h := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(h[:8]) // 16 hex chars
}

// computeStatusHash generates a deterministic hash for the receipt.
func (r *FinanceSyncReceipt) computeStatusHash() string {
	successStr := "false"
	if r.Success {
		successStr = "true"
	}
	canonical := fmt.Sprintf("FINANCE_SYNC_RECEIPT|v1|%s|%s|%s|%s|%s|%s|%s|%s|%s",
		r.ReceiptID, r.CircleID, r.Provider, r.PeriodBucket,
		r.AccountsMagnitude, r.TransactionsMagnitude,
		r.EvidenceHash, successStr, r.FailReason)
	h := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(h[:])
}

// CanonicalString returns the canonical string representation.
func (r *FinanceSyncReceipt) CanonicalString() string {
	successStr := "false"
	if r.Success {
		successStr = "true"
	}
	return fmt.Sprintf("v1|finance_sync_receipt|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s",
		r.ReceiptID, r.CircleID, r.Provider, r.PeriodBucket,
		r.AccountsMagnitude, r.TransactionsMagnitude,
		r.EvidenceHash, successStr, r.FailReason, r.StatusHash)
}

// Validate checks the receipt is valid.
func (r *FinanceSyncReceipt) Validate() error {
	if r.ReceiptID == "" {
		return fmt.Errorf("missing receipt_id")
	}
	if r.CircleID == "" {
		return fmt.Errorf("missing circle_id")
	}
	if r.Provider == "" {
		return fmt.Errorf("missing provider")
	}
	if r.PeriodBucket == "" {
		return fmt.Errorf("missing period_bucket")
	}
	if r.TimeBucket.IsZero() {
		return fmt.Errorf("missing time_bucket")
	}
	return nil
}

// FinanceMirrorPage represents the finance mirror proof page.
//
// CRITICAL: Contains NO raw data, NO identifiable info.
// Only: title, calm line, category signals, reassurance, hash.
type FinanceMirrorPage struct {
	// Title is the page title.
	Title string

	// CalmLine is the single calm message based on what was seen.
	CalmLine string

	// Categories contains up to 3 category signals (deterministic selection).
	Categories []CategorySignal

	// Reassurance is the privacy reassurance message.
	Reassurance string

	// LastSyncBucket is the abstract time of last sync.
	LastSyncBucket string

	// Connected indicates if finance is connected.
	Connected bool

	// StatusHash is a deterministic hash of the page content.
	StatusHash string
}

// CalmLines are the possible calm messages based on magnitude.
var CalmLines = map[MagnitudeBucket]string{
	MagnitudeNothing: "Nothing to see here. That's fine.",
	MagnitudeAFew:    "A few things passed through. Quietly noted.",
	MagnitudeSeveral: "Several things observed. All held.",
	MagnitudeMany:    "Many things seen. Everything held quietly.",
}

// DefaultReassurance is the standard privacy reassurance.
const DefaultReassurance = "Nothing was stored. Only the fact that it was seen."

// NewFinanceMirrorPage creates a new finance mirror page.
func NewFinanceMirrorPage(
	connected bool,
	lastSyncTime time.Time,
	overallMagnitude MagnitudeBucket,
	categories []CategorySignal,
) *FinanceMirrorPage {
	var lastSyncBucket string
	if !lastSyncTime.IsZero() {
		bucket := TimeBucket(lastSyncTime)
		lastSyncBucket = bucket.Format("Jan 2 15:04")
	} else {
		lastSyncBucket = "never"
	}

	// Select calm line based on magnitude
	calmLine, ok := CalmLines[overallMagnitude]
	if !ok {
		calmLine = CalmLines[MagnitudeNothing]
	}

	// Limit to 3 categories (deterministic: use first 3 after sorting)
	sortedCategories := make([]CategorySignal, len(categories))
	copy(sortedCategories, categories)
	sort.Slice(sortedCategories, func(i, j int) bool {
		return string(sortedCategories[i].Category) < string(sortedCategories[j].Category)
	})
	if len(sortedCategories) > 3 {
		sortedCategories = sortedCategories[:3]
	}

	page := &FinanceMirrorPage{
		Title:          "Seen, quietly.",
		CalmLine:       calmLine,
		Categories:     sortedCategories,
		Reassurance:    DefaultReassurance,
		LastSyncBucket: lastSyncBucket,
		Connected:      connected,
	}

	page.StatusHash = page.computeStatusHash()
	return page
}

// computeStatusHash computes a deterministic hash of the page.
func (p *FinanceMirrorPage) computeStatusHash() string {
	canonical := p.CanonicalString()
	h := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(h[:16]) // 32 hex chars
}

// CanonicalString returns the canonical string representation.
func (p *FinanceMirrorPage) CanonicalString() string {
	connectedStr := "false"
	if p.Connected {
		connectedStr = "true"
	}

	// Build categories string
	catStrs := make([]string, len(p.Categories))
	for i, c := range p.Categories {
		catStrs[i] = c.CanonicalString()
	}
	sort.Strings(catStrs)

	catsCanonical := ""
	for _, s := range catStrs {
		catsCanonical += "|" + s
	}

	return fmt.Sprintf("v1|finance_mirror_page|%s|%s|%s|%s%s",
		p.Title, p.CalmLine, p.LastSyncBucket, connectedStr, catsCanonical)
}

// FinanceMirrorAck represents an acknowledgment of viewing the mirror page.
type FinanceMirrorAck struct {
	// CircleID identifies the circle.
	CircleID string

	// PeriodBucket is the day bucket (YYYY-MM-DD).
	PeriodBucket string

	// PageHash is the hash of the page that was acknowledged.
	PageHash string

	// AckHash is the deterministic hash of this ack.
	AckHash string
}

// NewFinanceMirrorAck creates a new acknowledgment.
func NewFinanceMirrorAck(circleID, periodBucket, pageHash string) *FinanceMirrorAck {
	ack := &FinanceMirrorAck{
		CircleID:     circleID,
		PeriodBucket: periodBucket,
		PageHash:     pageHash,
	}
	ack.AckHash = ack.computeAckHash()
	return ack
}

// computeAckHash computes the deterministic ack hash.
func (a *FinanceMirrorAck) computeAckHash() string {
	canonical := fmt.Sprintf("FINANCE_MIRROR_ACK|v1|%s|%s|%s",
		a.CircleID, a.PeriodBucket, a.PageHash)
	h := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(h[:16])
}

// CanonicalString returns the canonical string representation.
func (a *FinanceMirrorAck) CanonicalString() string {
	return fmt.Sprintf("v1|finance_mirror_ack|%s|%s|%s|%s",
		a.CircleID, a.PeriodBucket, a.PageHash, a.AckHash)
}

// FinanceMirrorCue represents the whisper cue for finance mirror.
type FinanceMirrorCue struct {
	Available bool
	CueText   string // "Your finances were seen — quietly."
	LinkText  string // "view proof"
	CueHash   string
}

// NewFinanceMirrorCue creates a cue for display.
func NewFinanceMirrorCue(available bool) *FinanceMirrorCue {
	if !available {
		return &FinanceMirrorCue{Available: false}
	}
	cue := &FinanceMirrorCue{
		Available: true,
		CueText:   "Your finances were seen — quietly.",
		LinkText:  "view proof",
	}
	cue.CueHash = hashString(fmt.Sprintf("v1|cue|%s|%s", cue.CueText, cue.LinkText))
	return cue
}

// hashString computes a SHA256 hash of the input string.
func hashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:16])
}
