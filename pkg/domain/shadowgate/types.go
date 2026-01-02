// Package shadowgate provides types for Shadow Gating + Promotion Candidates.
//
// Phase 19.5: Shadow Gating + Promotion Candidates (NO behavior change)
//
// CRITICAL INVARIANTS:
//   - Shadow still does not affect behavior
//   - No canon thresholds/policies changed
//   - No obligation rules changed
//   - No interruption logic changed
//   - No drafts generated from shadow
//   - No execution boundaries touched
//
// This package produces "promotion candidates" from Shadow Diff + Calibration votes
// in a deterministic, privacy-safe way. Candidates are viewable in the web UI,
// and users can "propose promotion" (record intent only) but behavior is NOT changed.
//
// Reference: docs/ADR/ADR-0046-phase19-5-shadow-gating-and-promotion-candidates.md
package shadowgate

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"time"

	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/shadowdiff"
	"quantumlife/pkg/domain/shadowllm"
)

// =============================================================================
// Enums
// =============================================================================

// CandidateOrigin indicates which diff class the candidate came from.
type CandidateOrigin string

const (
	// OriginShadowOnly means shadow suggested this but canon didn't.
	OriginShadowOnly CandidateOrigin = "shadow_only"

	// OriginCanonOnly means canon surfaced this but shadow didn't.
	OriginCanonOnly CandidateOrigin = "canon_only"

	// OriginConflict means canon and shadow disagreed on this item.
	OriginConflict CandidateOrigin = "conflict"
)

// Validate checks if the origin is valid.
func (o CandidateOrigin) Validate() bool {
	switch o {
	case OriginShadowOnly, OriginCanonOnly, OriginConflict:
		return true
	default:
		return false
	}
}

// OriginFromNovelty converts a shadowdiff.Novelty to CandidateOrigin.
func OriginFromNovelty(n shadowdiff.Novelty, hasConflict bool) CandidateOrigin {
	if hasConflict {
		return OriginConflict
	}
	switch n {
	case shadowdiff.NoveltyShadowOnly:
		return OriginShadowOnly
	case shadowdiff.NoveltyCanonOnly:
		return OriginCanonOnly
	default:
		// NoveltyNone with conflict handled above
		return OriginConflict
	}
}

// UsefulnessBucket indicates usefulness based on vote percentages.
type UsefulnessBucket string

const (
	// UsefulnessUnknown means no votes yet.
	UsefulnessUnknown UsefulnessBucket = "unknown"

	// UsefulnessLow means <25% useful votes.
	UsefulnessLow UsefulnessBucket = "low"

	// UsefulnessMedium means 25-75% useful votes.
	UsefulnessMedium UsefulnessBucket = "medium"

	// UsefulnessHigh means >75% useful votes.
	UsefulnessHigh UsefulnessBucket = "high"
)

// Validate checks if the bucket is valid.
func (u UsefulnessBucket) Validate() bool {
	switch u {
	case UsefulnessUnknown, UsefulnessLow, UsefulnessMedium, UsefulnessHigh:
		return true
	default:
		return false
	}
}

// UsefulnessBucketFromPct computes bucket from percentage.
func UsefulnessBucketFromPct(pct int) UsefulnessBucket {
	if pct < 0 {
		return UsefulnessUnknown
	}
	if pct < 25 {
		return UsefulnessLow
	}
	if pct <= 75 {
		return UsefulnessMedium
	}
	return UsefulnessHigh
}

// VoteConfidenceBucket indicates confidence based on vote volume.
type VoteConfidenceBucket string

const (
	// VoteConfidenceUnknown means 0 votes.
	VoteConfidenceUnknown VoteConfidenceBucket = "unknown"

	// VoteConfidenceLow means 1-2 votes.
	VoteConfidenceLow VoteConfidenceBucket = "low"

	// VoteConfidenceMedium means 3-5 votes.
	VoteConfidenceMedium VoteConfidenceBucket = "medium"

	// VoteConfidenceHigh means 6+ votes.
	VoteConfidenceHigh VoteConfidenceBucket = "high"
)

// Validate checks if the bucket is valid.
func (v VoteConfidenceBucket) Validate() bool {
	switch v {
	case VoteConfidenceUnknown, VoteConfidenceLow, VoteConfidenceMedium, VoteConfidenceHigh:
		return true
	default:
		return false
	}
}

// VoteConfidenceBucketFromCount computes bucket from vote count.
func VoteConfidenceBucketFromCount(count int) VoteConfidenceBucket {
	if count == 0 {
		return VoteConfidenceUnknown
	}
	if count <= 2 {
		return VoteConfidenceLow
	}
	if count <= 5 {
		return VoteConfidenceMedium
	}
	return VoteConfidenceHigh
}

// NoteCode is the enum for promotion intent note codes.
type NoteCode string

const (
	// NotePromoteRule indicates intent to promote to a rule.
	NotePromoteRule NoteCode = "promote_rule"

	// NoteNeedsMoreVotes indicates more votes needed before deciding.
	NoteNeedsMoreVotes NoteCode = "needs_more_votes"

	// NoteIgnoreForNow indicates this should be ignored temporarily.
	NoteIgnoreForNow NoteCode = "ignore_for_now"
)

// Validate checks if the note code is valid.
func (n NoteCode) Validate() bool {
	switch n {
	case NotePromoteRule, NoteNeedsMoreVotes, NoteIgnoreForNow:
		return true
	default:
		return false
	}
}

// IsValid is an alias for Validate.
func (n NoteCode) IsValid() bool {
	return n.Validate()
}

// AllNoteCodes returns all valid note codes.
func AllNoteCodes() []NoteCode {
	return []NoteCode{NotePromoteRule, NoteNeedsMoreVotes, NoteIgnoreForNow}
}

// =============================================================================
// Candidate
// =============================================================================

// Candidate represents a potential promotion candidate derived from shadow diffs.
//
// CRITICAL: WhyGeneric must be privacy-safe (no names, emails, URLs, amounts).
type Candidate struct {
	// ID uniquely identifies this candidate (SHA256 of canonical string).
	ID string

	// Hash is the SHA256 hash of the full candidate state.
	Hash string

	// PeriodKey is the time bucket (YYYY-MM-DD).
	PeriodKey string

	// CircleID is the circle this candidate is for.
	CircleID identity.EntityID

	// Origin indicates which diff class this came from.
	Origin CandidateOrigin

	// Category is the abstract category (money, time, people, etc.).
	Category shadowllm.AbstractCategory

	// HorizonBucket indicates when this is relevant.
	HorizonBucket shadowllm.Horizon

	// MagnitudeBucket indicates relative quantity.
	MagnitudeBucket shadowllm.MagnitudeBucket

	// WhyGeneric is a privacy-safe reason string.
	// MUST NOT contain names, emails, URLs, amounts, vendors, timestamps.
	WhyGeneric string

	// UsefulnessPct is the usefulness percentage (0-100).
	UsefulnessPct int

	// UsefulnessBucket is derived from UsefulnessPct.
	UsefulnessBucket UsefulnessBucket

	// VoteConfidenceBucket is derived from total vote count.
	VoteConfidenceBucket VoteConfidenceBucket

	// VotesUseful is the count of "useful" votes.
	VotesUseful int

	// VotesUnnecessary is the count of "unnecessary" votes.
	VotesUnnecessary int

	// FirstSeenBucket is when this candidate was first seen (day bucket).
	FirstSeenBucket string

	// LastSeenBucket is when this candidate was last seen (day bucket).
	LastSeenBucket string

	// CreatedAt is when this candidate record was created.
	CreatedAt time.Time
}

// CanonicalString returns the pipe-delimited canonical representation.
// Used for deterministic hashing.
func (c *Candidate) CanonicalString() string {
	return "SHADOW_CANDIDATE|v1|" +
		c.PeriodKey + "|" +
		string(c.CircleID) + "|" +
		string(c.Origin) + "|" +
		string(c.Category) + "|" +
		string(c.HorizonBucket) + "|" +
		string(c.MagnitudeBucket) + "|" +
		c.WhyGeneric + "|" +
		itoa(c.UsefulnessPct) + "|" +
		string(c.UsefulnessBucket) + "|" +
		string(c.VoteConfidenceBucket) + "|" +
		itoa(c.VotesUseful) + "|" +
		itoa(c.VotesUnnecessary) + "|" +
		c.FirstSeenBucket + "|" +
		c.LastSeenBucket + "|" +
		c.CreatedAt.UTC().Format(time.RFC3339)
}

// ComputeHash computes and returns the SHA256 hash of the canonical string.
func (c *Candidate) ComputeHash() string {
	h := sha256.Sum256([]byte(c.CanonicalString()))
	return hex.EncodeToString(h[:])
}

// ComputeID computes a stable ID from category, origin, and why.
// This allows grouping similar candidates across periods.
func (c *Candidate) ComputeID() string {
	idStr := "CANDIDATE_ID|" +
		string(c.CircleID) + "|" +
		string(c.Origin) + "|" +
		string(c.Category) + "|" +
		string(c.HorizonBucket) + "|" +
		string(c.MagnitudeBucket) + "|" +
		c.WhyGeneric
	h := sha256.Sum256([]byte(idStr))
	return hex.EncodeToString(h[:16]) // 32 hex chars
}

// Validate checks if the candidate is valid.
func (c *Candidate) Validate() error {
	if c.PeriodKey == "" {
		return ErrMissingPeriodKey
	}
	if c.CircleID == "" {
		return ErrMissingCircleID
	}
	if !c.Origin.Validate() {
		return ErrInvalidOrigin
	}
	if !c.Category.Validate() {
		return ErrInvalidCategory
	}
	if !c.HorizonBucket.Validate() {
		return ErrInvalidHorizon
	}
	if !c.MagnitudeBucket.Validate() {
		return ErrInvalidMagnitude
	}
	if c.WhyGeneric == "" {
		return ErrMissingWhyGeneric
	}
	if c.UsefulnessPct < 0 || c.UsefulnessPct > 100 {
		return ErrInvalidUsefulnessPct
	}
	if !c.UsefulnessBucket.Validate() {
		return ErrInvalidUsefulnessBucket
	}
	if !c.VoteConfidenceBucket.Validate() {
		return ErrInvalidVoteConfidenceBucket
	}
	if c.VotesUseful < 0 {
		return ErrInvalidVoteCount
	}
	if c.VotesUnnecessary < 0 {
		return ErrInvalidVoteCount
	}
	return nil
}

// =============================================================================
// Candidate Sorting
// =============================================================================

// CandidateList is a sortable list of candidates.
type CandidateList []Candidate

// Len returns the number of candidates.
func (cl CandidateList) Len() int { return len(cl) }

// Swap swaps two candidates.
func (cl CandidateList) Swap(i, j int) { cl[i], cl[j] = cl[j], cl[i] }

// Less defines the sorting order:
// 1. UsefulnessBucket desc (high > medium > low > unknown)
// 2. Origin priority (shadow_only > canon_only > conflict)
// 3. Hash asc (deterministic tiebreaker)
func (cl CandidateList) Less(i, j int) bool {
	// Primary: Usefulness bucket (high first)
	ui := usefulnessOrder(cl[i].UsefulnessBucket)
	uj := usefulnessOrder(cl[j].UsefulnessBucket)
	if ui != uj {
		return ui > uj // Higher order = higher priority
	}

	// Secondary: Origin priority
	oi := originOrder(cl[i].Origin)
	oj := originOrder(cl[j].Origin)
	if oi != oj {
		return oi > oj // Higher order = higher priority
	}

	// Tertiary: Hash asc (deterministic tiebreaker)
	return cl[i].Hash < cl[j].Hash
}

// usefulnessOrder returns a numeric order for usefulness buckets.
func usefulnessOrder(u UsefulnessBucket) int {
	switch u {
	case UsefulnessHigh:
		return 3
	case UsefulnessMedium:
		return 2
	case UsefulnessLow:
		return 1
	default:
		return 0
	}
}

// originOrder returns a numeric order for origins.
func originOrder(o CandidateOrigin) int {
	switch o {
	case OriginShadowOnly:
		return 2
	case OriginCanonOnly:
		return 1
	default:
		return 0
	}
}

// SortCandidates sorts candidates in the standard order.
func SortCandidates(candidates []Candidate) {
	sort.Sort(CandidateList(candidates))
}

// =============================================================================
// PromotionIntent
// =============================================================================

// PromotionIntent records a user's intent to promote a candidate.
//
// CRITICAL: This does NOT change any runtime behavior.
// It only records the intent for future consideration.
type PromotionIntent struct {
	// IntentID uniquely identifies this intent.
	IntentID string

	// IntentHash is the SHA256 hash of the intent state.
	IntentHash string

	// CandidateID references the candidate being promoted.
	CandidateID string

	// CandidateHash is the hash of the candidate at intent time.
	CandidateHash string

	// PeriodKey is the time bucket (YYYY-MM-DD).
	PeriodKey string

	// NoteCode is the enum indicating the intent type.
	NoteCode NoteCode

	// CreatedBucket is a 5-minute or day bucket for when this was created.
	CreatedBucket string

	// CreatedAt is the exact creation time (for auditing).
	CreatedAt time.Time
}

// CanonicalString returns the pipe-delimited canonical representation.
func (p *PromotionIntent) CanonicalString() string {
	return "PROMOTION_INTENT|v1|" +
		p.CandidateID + "|" +
		p.CandidateHash + "|" +
		p.PeriodKey + "|" +
		string(p.NoteCode) + "|" +
		p.CreatedBucket + "|" +
		p.CreatedAt.UTC().Format(time.RFC3339)
}

// ComputeHash computes and returns the SHA256 hash of the canonical string.
func (p *PromotionIntent) ComputeHash() string {
	h := sha256.Sum256([]byte(p.CanonicalString()))
	return hex.EncodeToString(h[:])
}

// ComputeID computes a stable ID from candidate ID and note code.
func (p *PromotionIntent) ComputeID() string {
	idStr := "INTENT_ID|" +
		p.CandidateID + "|" +
		string(p.NoteCode) + "|" +
		p.CreatedBucket
	h := sha256.Sum256([]byte(idStr))
	return hex.EncodeToString(h[:16])
}

// Validate checks if the intent is valid.
func (p *PromotionIntent) Validate() error {
	if p.CandidateID == "" {
		return ErrMissingCandidateID
	}
	if p.CandidateHash == "" {
		return ErrMissingCandidateHash
	}
	if p.PeriodKey == "" {
		return ErrMissingPeriodKey
	}
	if !p.NoteCode.Validate() {
		return ErrInvalidNoteCode
	}
	if p.CreatedBucket == "" {
		return ErrMissingCreatedBucket
	}
	return nil
}

// =============================================================================
// PromotionIntent Sorting
// =============================================================================

// PromotionIntentList is a sortable list of intents.
type PromotionIntentList []PromotionIntent

// Len returns the number of intents.
func (pl PromotionIntentList) Len() int { return len(pl) }

// Swap swaps two intents.
func (pl PromotionIntentList) Swap(i, j int) { pl[i], pl[j] = pl[j], pl[i] }

// Less sorts by created bucket desc, then intent hash asc.
func (pl PromotionIntentList) Less(i, j int) bool {
	if pl[i].CreatedBucket != pl[j].CreatedBucket {
		return pl[i].CreatedBucket > pl[j].CreatedBucket // Newer first
	}
	return pl[i].IntentHash < pl[j].IntentHash
}

// SortPromotionIntents sorts intents in the standard order.
func SortPromotionIntents(intents []PromotionIntent) {
	sort.Sort(PromotionIntentList(intents))
}

// =============================================================================
// Errors
// =============================================================================

type gateError string

func (e gateError) Error() string { return string(e) }

const (
	ErrMissingPeriodKey           gateError = "missing period key"
	ErrMissingCircleID            gateError = "missing circle ID"
	ErrInvalidOrigin              gateError = "invalid candidate origin"
	ErrInvalidCategory            gateError = "invalid category"
	ErrInvalidHorizon             gateError = "invalid horizon"
	ErrInvalidMagnitude           gateError = "invalid magnitude"
	ErrMissingWhyGeneric          gateError = "missing why generic"
	ErrInvalidUsefulnessPct       gateError = "usefulness pct must be 0-100"
	ErrInvalidUsefulnessBucket    gateError = "invalid usefulness bucket"
	ErrInvalidVoteConfidenceBucket gateError = "invalid vote confidence bucket"
	ErrInvalidVoteCount           gateError = "vote count must be non-negative"
	ErrMissingCandidateID         gateError = "missing candidate ID"
	ErrMissingCandidateHash       gateError = "missing candidate hash"
	ErrInvalidNoteCode            gateError = "invalid note code"
	ErrMissingCreatedBucket       gateError = "missing created bucket"
	ErrPrivacyViolation           gateError = "privacy violation in candidate"
)

// =============================================================================
// Helpers
// =============================================================================

// itoa converts int to string without fmt dependency.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

// PeriodKeyFromTime computes the period key (YYYY-MM-DD) from a time.
func PeriodKeyFromTime(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

// FiveMinuteBucket computes a 5-minute bucket from a time.
func FiveMinuteBucket(t time.Time) string {
	t = t.UTC()
	minute := (t.Minute() / 5) * 5
	return t.Format("2006-01-02T15:") + itoa(minute)
}
