// Package rulepack provides types for Rule Pack Export.
//
// Phase 19.6: Rule Pack Export (Promotion Pipeline)
//
// CRITICAL INVARIANTS:
//   - RulePack does NOT apply itself
//   - No policy mutation
//   - No behavior change
//   - No raw identifiers (emails, URLs, vendor names, currency)
//   - Deterministic: same inputs + clock => same hashes
//   - Pipe-delimited canonical format (no JSON)
//
// This package turns PromotionIntents into exportable, deterministic
// RulePack artifacts for human review.
//
// Reference: docs/ADR/ADR-0047-phase19-6-rulepack-export.md
package rulepack

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"time"

	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/shadowgate"
	"quantumlife/pkg/domain/shadowllm"
)

// =============================================================================
// Constants
// =============================================================================

// ExportFormatVersion is the current export format version.
const ExportFormatVersion = "v1"

// Gating thresholds for including candidates in a pack.
// These are documented constants per the spec.
const (
	// MinUsefulnessBucket is the minimum usefulness required.
	MinUsefulnessBucket = shadowgate.UsefulnessMedium

	// MinVoteCount is the minimum number of votes required.
	MinVoteCount = 3

	// MinVoteConfidenceBucket is the minimum vote confidence required.
	MinVoteConfidenceBucket = shadowgate.VoteConfidenceMedium
)

// =============================================================================
// Enums
// =============================================================================

// ChangeKind indicates the type of rule change proposed.
type ChangeKind string

const (
	// ChangeBiasAdjust suggests adjusting the bias for a trigger/category.
	ChangeBiasAdjust ChangeKind = "bias_adjust"

	// ChangeThresholdAdjust suggests adjusting a threshold.
	ChangeThresholdAdjust ChangeKind = "threshold_adjust"

	// ChangeSuppressSuggest suggests suppressing certain items.
	ChangeSuppressSuggest ChangeKind = "suppress_suggest"
)

// Validate checks if the change kind is valid.
func (c ChangeKind) Validate() bool {
	switch c {
	case ChangeBiasAdjust, ChangeThresholdAdjust, ChangeSuppressSuggest:
		return true
	default:
		return false
	}
}

// AllChangeKinds returns all valid change kinds.
func AllChangeKinds() []ChangeKind {
	return []ChangeKind{ChangeBiasAdjust, ChangeThresholdAdjust, ChangeSuppressSuggest}
}

// TargetScope indicates what the change targets.
type TargetScope string

const (
	// ScopeCircle targets a specific circle.
	ScopeCircle TargetScope = "circle"

	// ScopeTrigger targets a specific trigger type.
	ScopeTrigger TargetScope = "trigger"

	// ScopeCategory targets a category.
	ScopeCategory TargetScope = "category"

	// ScopeItemKey targets a specific item key (hash only).
	ScopeItemKey TargetScope = "itemkey"

	// ScopeUnknown is the default when scope cannot be determined.
	ScopeUnknown TargetScope = "unknown"
)

// Validate checks if the target scope is valid.
func (t TargetScope) Validate() bool {
	switch t {
	case ScopeCircle, ScopeTrigger, ScopeCategory, ScopeItemKey, ScopeUnknown:
		return true
	default:
		return false
	}
}

// SuggestedDelta indicates the magnitude of suggested change.
type SuggestedDelta string

const (
	// DeltaNone means no change suggested.
	DeltaNone SuggestedDelta = "delta_none"

	// DeltaSmall suggests a small adjustment.
	DeltaSmall SuggestedDelta = "delta_small"

	// DeltaMedium suggests a medium adjustment.
	DeltaMedium SuggestedDelta = "delta_medium"

	// DeltaLarge suggests a large adjustment.
	DeltaLarge SuggestedDelta = "delta_large"
)

// Validate checks if the delta is valid.
func (d SuggestedDelta) Validate() bool {
	switch d {
	case DeltaNone, DeltaSmall, DeltaMedium, DeltaLarge:
		return true
	default:
		return false
	}
}

// DeltaFromUsefulness derives delta from usefulness bucket.
func DeltaFromUsefulness(u shadowgate.UsefulnessBucket) SuggestedDelta {
	switch u {
	case shadowgate.UsefulnessHigh:
		return DeltaLarge
	case shadowgate.UsefulnessMedium:
		return DeltaMedium
	case shadowgate.UsefulnessLow:
		return DeltaSmall
	default:
		return DeltaNone
	}
}

// NoveltyBucket abstracts the novelty type.
type NoveltyBucket string

const (
	NoveltyNone       NoveltyBucket = "none"
	NoveltyShadowOnly NoveltyBucket = "shadow_only"
	NoveltyCanonOnly  NoveltyBucket = "canon_only"
)

// AgreementBucket abstracts the agreement type.
type AgreementBucket string

const (
	AgreementMatch    AgreementBucket = "match"
	AgreementSofter   AgreementBucket = "softer"
	AgreementEarlier  AgreementBucket = "earlier"
	AgreementLater    AgreementBucket = "later"
	AgreementConflict AgreementBucket = "conflict"
)

// AckKind indicates how a pack was acknowledged.
type AckKind string

const (
	AckViewed    AckKind = "viewed"
	AckExported  AckKind = "exported"
	AckDismissed AckKind = "dismissed"
)

// Validate checks if the ack kind is valid.
func (a AckKind) Validate() bool {
	switch a {
	case AckViewed, AckExported, AckDismissed:
		return true
	default:
		return false
	}
}

// =============================================================================
// RuleChange
// =============================================================================

// RuleChange represents a single proposed change derived from a PromotionIntent.
//
// CRITICAL: Contains NO raw identifiers. Only hashes and buckets.
type RuleChange struct {
	// ChangeID uniquely identifies this change within a pack.
	ChangeID string

	// CandidateHash references the source candidate (privacy-safe).
	CandidateHash string

	// IntentHash references the source promotion intent.
	IntentHash string

	// CircleID is the target circle (may be empty for global).
	CircleID identity.EntityID

	// ChangeKind indicates the type of change.
	ChangeKind ChangeKind

	// TargetScope indicates what is being changed.
	TargetScope TargetScope

	// TargetHash is a hash of the target (for itemkey scope).
	// NEVER contains raw identifiers.
	TargetHash string

	// Category is the abstract category.
	Category shadowllm.AbstractCategory

	// SuggestedDelta is the magnitude of suggested change.
	SuggestedDelta SuggestedDelta

	// Evidence buckets (all abstract, no raw data)
	UsefulnessBucket     shadowgate.UsefulnessBucket
	VoteConfidenceBucket shadowgate.VoteConfidenceBucket
	NoveltyBucket        NoveltyBucket
	AgreementBucket      AgreementBucket
}

// CanonicalString returns the pipe-delimited canonical representation.
func (r *RuleChange) CanonicalString() string {
	return "RULE_CHANGE|v1|" +
		r.ChangeID + "|" +
		r.CandidateHash + "|" +
		r.IntentHash + "|" +
		string(r.CircleID) + "|" +
		string(r.ChangeKind) + "|" +
		string(r.TargetScope) + "|" +
		r.TargetHash + "|" +
		string(r.Category) + "|" +
		string(r.SuggestedDelta) + "|" +
		string(r.UsefulnessBucket) + "|" +
		string(r.VoteConfidenceBucket) + "|" +
		string(r.NoveltyBucket) + "|" +
		string(r.AgreementBucket)
}

// ComputeHash computes the SHA256 hash of the change.
func (r *RuleChange) ComputeHash() string {
	h := sha256.Sum256([]byte(r.CanonicalString()))
	return hex.EncodeToString(h[:])
}

// ComputeID computes a stable ID for this change.
func (r *RuleChange) ComputeID() string {
	idStr := "CHANGE_ID|" +
		r.CandidateHash + "|" +
		r.IntentHash + "|" +
		string(r.ChangeKind)
	h := sha256.Sum256([]byte(idStr))
	return hex.EncodeToString(h[:16])
}

// Validate checks if the rule change is valid.
func (r *RuleChange) Validate() error {
	if r.CandidateHash == "" {
		return ErrMissingCandidateHash
	}
	if r.IntentHash == "" {
		return ErrMissingIntentHash
	}
	if !r.ChangeKind.Validate() {
		return ErrInvalidChangeKind
	}
	if !r.TargetScope.Validate() {
		return ErrInvalidTargetScope
	}
	if !r.SuggestedDelta.Validate() {
		return ErrInvalidDelta
	}
	return nil
}

// =============================================================================
// RuleChange Sorting
// =============================================================================

// RuleChangeList is a sortable list of changes.
type RuleChangeList []RuleChange

func (cl RuleChangeList) Len() int      { return len(cl) }
func (cl RuleChangeList) Swap(i, j int) { cl[i], cl[j] = cl[j], cl[i] }

// Less sorts by: CircleID, CandidateHash, ChangeKind, TargetScope, TargetHash
func (cl RuleChangeList) Less(i, j int) bool {
	// Primary: CircleID
	if cl[i].CircleID != cl[j].CircleID {
		return cl[i].CircleID < cl[j].CircleID
	}
	// Secondary: CandidateHash
	if cl[i].CandidateHash != cl[j].CandidateHash {
		return cl[i].CandidateHash < cl[j].CandidateHash
	}
	// Tertiary: ChangeKind
	if cl[i].ChangeKind != cl[j].ChangeKind {
		return cl[i].ChangeKind < cl[j].ChangeKind
	}
	// Quaternary: TargetScope
	if cl[i].TargetScope != cl[j].TargetScope {
		return cl[i].TargetScope < cl[j].TargetScope
	}
	// Final: TargetHash
	return cl[i].TargetHash < cl[j].TargetHash
}

// SortRuleChanges sorts changes in deterministic order.
func SortRuleChanges(changes []RuleChange) {
	sort.Sort(RuleChangeList(changes))
}

// =============================================================================
// RulePack
// =============================================================================

// RulePack is an exportable collection of proposed rule changes.
//
// CRITICAL: Does NOT apply itself. No policy mutation. No behavior change.
type RulePack struct {
	// PackID uniquely identifies this pack.
	PackID string

	// PackHash is the SHA256 hash of the full pack content.
	PackHash string

	// PeriodKey is the time bucket (YYYY-MM-DD).
	PeriodKey string

	// CircleID is the target circle (empty string means "all").
	CircleID identity.EntityID

	// CreatedAtBucket is the time bucket string (not raw timestamp).
	CreatedAtBucket string

	// ExportFormatVersion is the format version.
	ExportFormatVersion string

	// Changes are the proposed rule changes (sorted deterministically).
	Changes []RuleChange

	// CreatedAt is the exact creation time (for internal use only, not exported).
	CreatedAt time.Time
}

// CanonicalString returns the pipe-delimited canonical representation.
func (p *RulePack) CanonicalString() string {
	// Build changes hash
	changesHash := p.computeChangesHash()

	return "RULE_PACK|" + p.ExportFormatVersion + "|" +
		p.PackID + "|" +
		p.PeriodKey + "|" +
		string(p.CircleID) + "|" +
		p.CreatedAtBucket + "|" +
		itoa(len(p.Changes)) + "|" +
		changesHash
}

// computeChangesHash computes a hash of all changes.
func (p *RulePack) computeChangesHash() string {
	if len(p.Changes) == 0 {
		return "empty"
	}
	var combined string
	for _, c := range p.Changes {
		combined += c.CanonicalString() + "\n"
	}
	h := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(h[:16])
}

// ComputeHash computes the SHA256 hash of the pack.
func (p *RulePack) ComputeHash() string {
	h := sha256.Sum256([]byte(p.CanonicalString()))
	return hex.EncodeToString(h[:])
}

// ComputeID computes a stable ID for this pack.
func (p *RulePack) ComputeID() string {
	idStr := "PACK_ID|" +
		p.PeriodKey + "|" +
		string(p.CircleID) + "|" +
		p.CreatedAtBucket
	h := sha256.Sum256([]byte(idStr))
	return hex.EncodeToString(h[:16])
}

// Validate checks if the pack is valid.
func (p *RulePack) Validate() error {
	if p.PeriodKey == "" {
		return ErrMissingPeriodKey
	}
	if p.CreatedAtBucket == "" {
		return ErrMissingCreatedAtBucket
	}
	if p.ExportFormatVersion == "" {
		return ErrMissingFormatVersion
	}
	for i := range p.Changes {
		if err := p.Changes[i].Validate(); err != nil {
			return err
		}
	}
	return nil
}

// ChangeCount returns the number of changes.
func (p *RulePack) ChangeCount() int {
	return len(p.Changes)
}

// ChangeMagnitude returns a magnitude bucket for the change count.
func (p *RulePack) ChangeMagnitude() shadowllm.MagnitudeBucket {
	count := len(p.Changes)
	if count == 0 {
		return shadowllm.MagnitudeNothing
	}
	if count <= 3 {
		return shadowllm.MagnitudeAFew
	}
	// 4+ is "several"
	return shadowllm.MagnitudeSeveral
}

// =============================================================================
// RulePack Sorting
// =============================================================================

// RulePackList is a sortable list of packs.
type RulePackList []RulePack

func (pl RulePackList) Len() int      { return len(pl) }
func (pl RulePackList) Swap(i, j int) { pl[i], pl[j] = pl[j], pl[i] }

// Less sorts by CreatedAtBucket desc (most recent first), then PackHash.
func (pl RulePackList) Less(i, j int) bool {
	if pl[i].CreatedAtBucket != pl[j].CreatedAtBucket {
		return pl[i].CreatedAtBucket > pl[j].CreatedAtBucket // Desc
	}
	return pl[i].PackHash < pl[j].PackHash
}

// SortRulePacks sorts packs in deterministic order (most recent first).
func SortRulePacks(packs []RulePack) {
	sort.Sort(RulePackList(packs))
}

// =============================================================================
// PackAck
// =============================================================================

// PackAck records an acknowledgment of a pack.
type PackAck struct {
	AckID         string
	AckHash       string
	PackID        string
	PackHash      string
	AckKind       AckKind
	CreatedBucket string
	CreatedAt     time.Time
}

// CanonicalString returns the pipe-delimited canonical representation.
func (a *PackAck) CanonicalString() string {
	return "PACK_ACK|v1|" +
		a.AckID + "|" +
		a.PackID + "|" +
		a.PackHash + "|" +
		string(a.AckKind) + "|" +
		a.CreatedBucket
}

// ComputeHash computes the SHA256 hash of the ack.
func (a *PackAck) ComputeHash() string {
	h := sha256.Sum256([]byte(a.CanonicalString()))
	return hex.EncodeToString(h[:])
}

// ComputeID computes a stable ID for this ack.
func (a *PackAck) ComputeID() string {
	idStr := "ACK_ID|" +
		a.PackID + "|" +
		string(a.AckKind) + "|" +
		a.CreatedBucket
	h := sha256.Sum256([]byte(idStr))
	return hex.EncodeToString(h[:16])
}

// Validate checks if the ack is valid.
func (a *PackAck) Validate() error {
	if a.PackID == "" {
		return ErrMissingPackID
	}
	if a.PackHash == "" {
		return ErrMissingPackHash
	}
	if !a.AckKind.Validate() {
		return ErrInvalidAckKind
	}
	if a.CreatedBucket == "" {
		return ErrMissingCreatedBucket
	}
	return nil
}

// =============================================================================
// Errors
// =============================================================================

type packError string

func (e packError) Error() string { return string(e) }

const (
	ErrMissingPeriodKey       packError = "missing period key"
	ErrMissingCreatedAtBucket packError = "missing created at bucket"
	ErrMissingFormatVersion   packError = "missing format version"
	ErrMissingCandidateHash   packError = "missing candidate hash"
	ErrMissingIntentHash      packError = "missing intent hash"
	ErrInvalidChangeKind      packError = "invalid change kind"
	ErrInvalidTargetScope     packError = "invalid target scope"
	ErrInvalidDelta           packError = "invalid delta"
	ErrMissingPackID          packError = "missing pack ID"
	ErrMissingPackHash        packError = "missing pack hash"
	ErrInvalidAckKind         packError = "invalid ack kind"
	ErrMissingCreatedBucket   packError = "missing created bucket"
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
