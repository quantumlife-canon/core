// Package interrupt explain.go provides explainability for interruptions.
//
// Phase 14: Circle Policies + Preference Learning (Deterministic)
//
// Explainability enables users to understand "why am I seeing this?"
// Each interruption can be explained with:
// - Policy thresholds and how the score compared
// - Trigger bias adjustments
// - Quota state (how many left today)
// - Suppression check results
// - Scoring component breakdown
//
// All explanations are deterministic and auditable.
//
// Reference: docs/ADR/ADR-0030-phase14-policy-learning.md
package interrupt

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// ExplainRecord contains explanation for why an interruption was surfaced.
type ExplainRecord struct {
	// InterruptionID identifies the interruption being explained.
	InterruptionID string

	// CircleID is the circle this interruption belongs to.
	CircleID string

	// Trigger is what caused this interruption.
	Trigger string

	// RegretScore is the computed regret score (0-100).
	RegretScore int

	// Level is the final assigned level.
	Level Level

	// Reasons is a stable-ordered list of explanation strings.
	Reasons []string

	// PolicyHash is the hash of the policy used for this decision.
	PolicyHash string

	// SuppressionHit is the rule ID if this was suppressed, nil otherwise.
	SuppressionHit *string

	// Scoring contains the breakdown of score components.
	Scoring *ScoringBreakdown

	// QuotaState describes the quota situation.
	QuotaState *QuotaState

	// Hash is the canonical hash of this explanation.
	Hash string
}

// ScoringBreakdown shows how the regret score was computed.
type ScoringBreakdown struct {
	// Base is the starting score from circle base.
	CircleBase int

	// DueBoost is added for upcoming due dates.
	DueBoost int

	// ActionBoost is added for action-required items.
	ActionBoost int

	// SeverityBoost is added for high severity items.
	SeverityBoost int

	// TriggerBias is the bias applied from trigger policy.
	TriggerBias int

	// FinalScore is the clamped final score.
	FinalScore int
}

// CanonicalString returns a deterministic representation.
func (s ScoringBreakdown) CanonicalString() string {
	return fmt.Sprintf("base:%d|due_boost:%d|action_boost:%d|severity_boost:%d|trigger_bias:%d|final:%d",
		s.CircleBase, s.DueBoost, s.ActionBoost, s.SeverityBoost, s.TriggerBias, s.FinalScore)
}

// QuotaState describes the quota situation for this interruption.
type QuotaState struct {
	// NotifyQuotaUsed is how many notify interruptions were used today.
	NotifyQuotaUsed int

	// NotifyQuotaLimit is the daily notify limit.
	NotifyQuotaLimit int

	// QueuedQuotaUsed is how many queued interruptions were used today.
	QueuedQuotaUsed int

	// QueuedQuotaLimit is the daily queued limit.
	QueuedQuotaLimit int

	// WasDowngraded indicates if this was downgraded due to quota.
	WasDowngraded bool

	// DowngradedFrom is the original level before quota downgrade.
	DowngradedFrom Level
}

// CanonicalString returns a deterministic representation.
func (q QuotaState) CanonicalString() string {
	downgrade := "none"
	if q.WasDowngraded {
		downgrade = string(q.DowngradedFrom)
	}
	return fmt.Sprintf("notify_used:%d|notify_limit:%d|queued_used:%d|queued_limit:%d|downgraded:%s",
		q.NotifyQuotaUsed, q.NotifyQuotaLimit, q.QueuedQuotaUsed, q.QueuedQuotaLimit, downgrade)
}

// NewExplainRecord creates an explanation record with computed hash.
func NewExplainRecord(
	interruptionID string,
	circleID string,
	trigger string,
	regretScore int,
	level Level,
	policyHash string,
) *ExplainRecord {
	e := &ExplainRecord{
		InterruptionID: interruptionID,
		CircleID:       circleID,
		Trigger:        trigger,
		RegretScore:    regretScore,
		Level:          level,
		PolicyHash:     policyHash,
		Reasons:        []string{},
	}
	return e
}

// AddReason adds an explanation reason.
func (e *ExplainRecord) AddReason(reason string) {
	e.Reasons = append(e.Reasons, reason)
}

// SetScoring sets the scoring breakdown.
func (e *ExplainRecord) SetScoring(scoring *ScoringBreakdown) {
	e.Scoring = scoring
}

// SetQuotaState sets the quota state.
func (e *ExplainRecord) SetQuotaState(quota *QuotaState) {
	e.QuotaState = quota
}

// SetSuppressionHit sets the suppression hit.
func (e *ExplainRecord) SetSuppressionHit(ruleID string) {
	e.SuppressionHit = &ruleID
}

// ComputeHash computes and sets the Hash field.
func (e *ExplainRecord) ComputeHash() string {
	canonical := e.CanonicalString()
	hash := sha256.Sum256([]byte(canonical))
	e.Hash = hex.EncodeToString(hash[:])
	return e.Hash
}

// CanonicalString returns a deterministic representation.
func (e *ExplainRecord) CanonicalString() string {
	var sb strings.Builder

	sb.WriteString("explain|")
	sb.WriteString("id:")
	sb.WriteString(e.InterruptionID)
	sb.WriteString("|circle:")
	sb.WriteString(e.CircleID)
	sb.WriteString("|trigger:")
	sb.WriteString(e.Trigger)
	sb.WriteString("|regret:")
	sb.WriteString(fmt.Sprintf("%d", e.RegretScore))
	sb.WriteString("|level:")
	sb.WriteString(string(e.Level))
	sb.WriteString("|policy_hash:")
	sb.WriteString(e.PolicyHash)

	// Suppression
	sb.WriteString("|suppressed:")
	if e.SuppressionHit != nil {
		sb.WriteString(*e.SuppressionHit)
	} else {
		sb.WriteString("none")
	}

	// Reasons (already in stable order)
	sb.WriteString("|reasons:[")
	for i, r := range e.Reasons {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(r)
	}
	sb.WriteString("]")

	// Scoring
	if e.Scoring != nil {
		sb.WriteString("|scoring:")
		sb.WriteString(e.Scoring.CanonicalString())
	}

	// Quota
	if e.QuotaState != nil {
		sb.WriteString("|quota:")
		sb.WriteString(e.QuotaState.CanonicalString())
	}

	return sb.String()
}

// FormatForUI returns a human-readable explanation.
func (e *ExplainRecord) FormatForUI() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Interruption: %s\n", e.InterruptionID))
	sb.WriteString(fmt.Sprintf("Circle: %s\n", e.CircleID))
	sb.WriteString(fmt.Sprintf("Trigger: %s\n", e.Trigger))
	sb.WriteString(fmt.Sprintf("Regret Score: %d/100\n", e.RegretScore))
	sb.WriteString(fmt.Sprintf("Level: %s\n", e.Level))
	sb.WriteString("\n")

	// Reasons
	sb.WriteString("Why this interruption:\n")
	for i, r := range e.Reasons {
		sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, r))
	}

	// Scoring breakdown
	if e.Scoring != nil {
		sb.WriteString("\nScore breakdown:\n")
		sb.WriteString(fmt.Sprintf("  Circle base: %d\n", e.Scoring.CircleBase))
		if e.Scoring.DueBoost > 0 {
			sb.WriteString(fmt.Sprintf("  Due date boost: +%d\n", e.Scoring.DueBoost))
		}
		if e.Scoring.ActionBoost > 0 {
			sb.WriteString(fmt.Sprintf("  Action required boost: +%d\n", e.Scoring.ActionBoost))
		}
		if e.Scoring.SeverityBoost > 0 {
			sb.WriteString(fmt.Sprintf("  Severity boost: +%d\n", e.Scoring.SeverityBoost))
		}
		if e.Scoring.TriggerBias != 0 {
			if e.Scoring.TriggerBias > 0 {
				sb.WriteString(fmt.Sprintf("  Trigger boost: +%d\n", e.Scoring.TriggerBias))
			} else {
				sb.WriteString(fmt.Sprintf("  Trigger reduction: %d\n", e.Scoring.TriggerBias))
			}
		}
		sb.WriteString(fmt.Sprintf("  Final score: %d\n", e.Scoring.FinalScore))
	}

	// Quota state
	if e.QuotaState != nil {
		sb.WriteString("\nQuota status:\n")
		sb.WriteString(fmt.Sprintf("  Notify: %d/%d used\n", e.QuotaState.NotifyQuotaUsed, e.QuotaState.NotifyQuotaLimit))
		sb.WriteString(fmt.Sprintf("  Queued: %d/%d used\n", e.QuotaState.QueuedQuotaUsed, e.QuotaState.QueuedQuotaLimit))
		if e.QuotaState.WasDowngraded {
			sb.WriteString(fmt.Sprintf("  Note: Downgraded from %s due to quota\n", e.QuotaState.DowngradedFrom))
		}
	}

	// Suppression
	if e.SuppressionHit != nil {
		sb.WriteString(fmt.Sprintf("\nSuppressed by rule: %s\n", *e.SuppressionHit))
	}

	return sb.String()
}

// ExplainBuilder helps construct explanations during interruption processing.
type ExplainBuilder struct {
	explain *ExplainRecord
}

// NewExplainBuilder creates a new explain builder.
func NewExplainBuilder(interruptionID, circleID, trigger string, policyHash string) *ExplainBuilder {
	return &ExplainBuilder{
		explain: NewExplainRecord(interruptionID, circleID, trigger, 0, LevelSilent, policyHash),
	}
}

// WithRegretScore sets the regret score.
func (b *ExplainBuilder) WithRegretScore(score int) *ExplainBuilder {
	b.explain.RegretScore = score
	return b
}

// WithLevel sets the level.
func (b *ExplainBuilder) WithLevel(level Level) *ExplainBuilder {
	b.explain.Level = level
	return b
}

// AddThresholdReason adds a threshold-based reason.
func (b *ExplainBuilder) AddThresholdReason(threshold, score int, thresholdName string) *ExplainBuilder {
	if score >= threshold {
		b.explain.AddReason(fmt.Sprintf("Score %d >= %s threshold %d", score, thresholdName, threshold))
	} else {
		b.explain.AddReason(fmt.Sprintf("Score %d < %s threshold %d", score, thresholdName, threshold))
	}
	return b
}

// AddDueReason adds a due-date-based reason.
func (b *ExplainBuilder) AddDueReason(hoursUntilDue int) *ExplainBuilder {
	if hoursUntilDue <= 24 {
		b.explain.AddReason(fmt.Sprintf("Due within %d hours", hoursUntilDue))
	} else if hoursUntilDue <= 72 {
		b.explain.AddReason(fmt.Sprintf("Due within %d days", hoursUntilDue/24))
	}
	return b
}

// AddActionReason adds an action-required reason.
func (b *ExplainBuilder) AddActionReason(action string) *ExplainBuilder {
	b.explain.AddReason(fmt.Sprintf("Action required: %s", action))
	return b
}

// AddSeverityReason adds a severity-based reason.
func (b *ExplainBuilder) AddSeverityReason(severity string) *ExplainBuilder {
	b.explain.AddReason(fmt.Sprintf("Severity: %s", severity))
	return b
}

// AddTriggerReason adds a trigger-based reason.
func (b *ExplainBuilder) AddTriggerReason(trigger string, bias int) *ExplainBuilder {
	if bias > 0 {
		b.explain.AddReason(fmt.Sprintf("Trigger '%s' has +%d priority boost", trigger, bias))
	} else if bias < 0 {
		b.explain.AddReason(fmt.Sprintf("Trigger '%s' has %d priority reduction", trigger, bias))
	}
	return b
}

// AddDedupReason adds a deduplication reason.
func (b *ExplainBuilder) AddDedupReason(isDuplicate bool) *ExplainBuilder {
	if isDuplicate {
		b.explain.AddReason("Duplicate of existing interruption - merged")
	}
	return b
}

// AddQuotaReason adds a quota-related reason.
func (b *ExplainBuilder) AddQuotaReason(quotaType string, used, limit int, wasDowngraded bool) *ExplainBuilder {
	if wasDowngraded {
		b.explain.AddReason(fmt.Sprintf("%s quota exceeded (%d/%d) - downgraded", quotaType, used, limit))
	} else {
		b.explain.AddReason(fmt.Sprintf("%s quota: %d/%d used", quotaType, used, limit))
	}
	return b
}

// AddSuppressionReason adds a suppression reason.
func (b *ExplainBuilder) AddSuppressionReason(ruleID, scope, key string) *ExplainBuilder {
	b.explain.SetSuppressionHit(ruleID)
	b.explain.AddReason(fmt.Sprintf("Suppressed by rule %s (scope: %s, key: %s)", ruleID, scope, key))
	return b
}

// AddHoursReason adds an hours-based reason.
func (b *ExplainBuilder) AddHoursReason(allowed bool, currentHour int) *ExplainBuilder {
	if !allowed {
		b.explain.AddReason(fmt.Sprintf("Outside allowed hours (current hour: %d)", currentHour))
	}
	return b
}

// SetScoring sets the scoring breakdown.
func (b *ExplainBuilder) SetScoring(scoring *ScoringBreakdown) *ExplainBuilder {
	b.explain.SetScoring(scoring)
	return b
}

// SetQuotaState sets the quota state.
func (b *ExplainBuilder) SetQuotaState(quota *QuotaState) *ExplainBuilder {
	b.explain.SetQuotaState(quota)
	return b
}

// Build finalizes and returns the explain record.
func (b *ExplainBuilder) Build() *ExplainRecord {
	b.explain.ComputeHash()
	return b.explain
}
