// Package preflearn provides deterministic preference learning from feedback.
//
// Phase 14: Circle Policies + Preference Learning (Deterministic)
//
// The engine consumes feedback records and produces policy/suppression updates
// using rule-based logic. No ML or probabilistic methods are used.
//
// Learning Rules:
// 1. "unnecessary" feedback: increase thresholds or add suppressions
// 2. "helpful" feedback: decrease thresholds (with floors)
// 3. Repeated "unnecessary" on same trigger => add suppression rule
//
// All updates are deterministic and auditable.
//
// Reference: docs/ADR/ADR-0030-phase14-policy-learning.md
package preflearn

import (
	"fmt"
	"strings"
	"time"

	"quantumlife/pkg/domain/feedback"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/policy"
	"quantumlife/pkg/domain/suppress"
)

// Config contains learning engine configuration.
type Config struct {
	// ThresholdIncrement is how much to increase thresholds on "unnecessary" (default 5).
	ThresholdIncrement int

	// ThresholdDecrement is how much to decrease thresholds on "helpful" (default 3).
	ThresholdDecrement int

	// ThresholdFloor is the minimum threshold value (default 5).
	ThresholdFloor int

	// ThresholdCeiling is the maximum threshold value (default 95).
	ThresholdCeiling int

	// RepeatedUnnecessaryCount triggers suppression (default 2).
	RepeatedUnnecessaryCount int

	// RepeatedUnnecessaryWindow is the time window for counting (default 7 days).
	RepeatedUnnecessaryWindow time.Duration

	// SuppressionTTL is the default suppression duration (default 30 days).
	SuppressionTTL time.Duration

	// BiasIncrement is how much to adjust trigger bias (default 5).
	BiasIncrement int

	// BiasFloor is the minimum bias value (default -50).
	BiasFloor int

	// BiasCeiling is the maximum bias value (default 50).
	BiasCeiling int
}

// DefaultConfig returns default learning configuration.
func DefaultConfig() Config {
	return Config{
		ThresholdIncrement:        5,
		ThresholdDecrement:        3,
		ThresholdFloor:            5,
		ThresholdCeiling:          95,
		RepeatedUnnecessaryCount:  2,
		RepeatedUnnecessaryWindow: 7 * 24 * time.Hour,
		SuppressionTTL:            30 * 24 * time.Hour,
		BiasIncrement:             5,
		BiasFloor:                 -50,
		BiasCeiling:               50,
	}
}

// Engine applies feedback to policy and suppression state.
type Engine struct {
	config Config
}

// NewEngine creates a new preference learning engine.
func NewEngine(config Config) *Engine {
	return &Engine{config: config}
}

// InterruptContext provides context about an interruption for learning.
type InterruptContext struct {
	InterruptID string
	CircleID    string
	Trigger     string
	PersonID    string // Optional: from identity graph
	VendorID    string // Optional: extracted vendor
	DedupKey    string // Dedup key for specific item suppression
}

// ApplyResult contains the results of applying feedback.
type ApplyResult struct {
	// PolicyChanged indicates if the policy was updated.
	PolicyChanged bool

	// NewPolicy is the updated policy (if changed).
	NewPolicy *policy.PolicySet

	// SuppressAdded contains new suppression rules.
	SuppressAdded []suppress.SuppressionRule

	// SuppressRemoved contains IDs of removed suppression rules.
	SuppressRemoved []string

	// BeforePolicyHash is the policy hash before changes.
	BeforePolicyHash string

	// AfterPolicyHash is the policy hash after changes.
	AfterPolicyHash string

	// BeforeSuppressHash is the suppression hash before changes.
	BeforeSuppressHash string

	// AfterSuppressHash is the suppression hash after changes.
	AfterSuppressHash string

	// Decisions describes what was decided and why.
	Decisions []DecisionRecord
}

// DecisionRecord describes a learning decision.
type DecisionRecord struct {
	FeedbackID string
	Action     string
	Reason     string
	Details    string
}

// CanonicalString returns a deterministic representation.
func (d DecisionRecord) CanonicalString() string {
	return fmt.Sprintf("decision|%s|%s|%s|%s",
		d.FeedbackID, d.Action, d.Reason, d.Details)
}

// FeedbackHistory provides access to historical feedback for learning decisions.
type FeedbackHistory interface {
	// GetRecentByCircleAndTrigger returns recent unnecessary feedback for a circle/trigger.
	GetRecentByCircleAndTrigger(circleID identity.EntityID, trigger string, since time.Time) []feedback.FeedbackRecord
}

// ApplyFeedback processes feedback records and produces updates.
func (e *Engine) ApplyFeedback(
	records []feedback.FeedbackRecord,
	contexts map[string]InterruptContext, // FeedbackID -> context
	currentPolicy *policy.PolicySet,
	currentSuppress *suppress.SuppressionSet,
	history FeedbackHistory,
	now time.Time,
) (*ApplyResult, error) {
	result := &ApplyResult{
		BeforePolicyHash:   currentPolicy.Hash,
		BeforeSuppressHash: currentSuppress.Hash,
		Decisions:          []DecisionRecord{},
	}

	// Track policy changes
	policyChanges := make(map[string]policy.CirclePolicy)
	triggerChanges := make(map[string]policy.TriggerPolicy)

	// Track suppression changes
	var newRules []suppress.SuppressionRule

	// Process each feedback record deterministically (already sorted by store)
	for _, fr := range records {
		ctx, hasCtx := contexts[fr.FeedbackID]
		circleID := string(fr.CircleID)

		switch fr.Signal {
		case feedback.SignalUnnecessary:
			decision := e.handleUnnecessary(fr, ctx, hasCtx, currentPolicy, currentSuppress, history, now, circleID, policyChanges, triggerChanges, &newRules)
			result.Decisions = append(result.Decisions, decision)

		case feedback.SignalHelpful:
			decision := e.handleHelpful(fr, ctx, hasCtx, currentPolicy, circleID, policyChanges, triggerChanges)
			result.Decisions = append(result.Decisions, decision)
		}
	}

	// Build new policy if changes occurred
	if len(policyChanges) > 0 || len(triggerChanges) > 0 {
		result.PolicyChanged = true
		newPS := &policy.PolicySet{
			Version:    currentPolicy.Version + 1,
			CapturedAt: now,
			Circles:    make(map[string]policy.CirclePolicy),
			Triggers:   make(map[string]policy.TriggerPolicy),
		}

		// Copy existing
		for k, v := range currentPolicy.Circles {
			newPS.Circles[k] = v
		}
		for k, v := range currentPolicy.Triggers {
			newPS.Triggers[k] = v
		}

		// Apply changes
		for k, v := range policyChanges {
			newPS.Circles[k] = v
		}
		for k, v := range triggerChanges {
			newPS.Triggers[k] = v
		}

		newPS.ComputeHash()
		result.NewPolicy = newPS
		result.AfterPolicyHash = newPS.Hash
	} else {
		result.AfterPolicyHash = currentPolicy.Hash
	}

	// Add suppression rules
	result.SuppressAdded = newRules

	// Compute after suppress hash
	if len(newRules) > 0 {
		// Create temporary set to compute hash
		tempSet := &suppress.SuppressionSet{
			Version: currentSuppress.Version + len(newRules),
			Rules:   append([]suppress.SuppressionRule{}, currentSuppress.Rules...),
		}
		for _, r := range newRules {
			tempSet.Rules = append(tempSet.Rules, r)
		}
		tempSet.ComputeHash()
		result.AfterSuppressHash = tempSet.Hash
	} else {
		result.AfterSuppressHash = currentSuppress.Hash
	}

	return result, nil
}

// handleUnnecessary processes unnecessary feedback.
func (e *Engine) handleUnnecessary(
	fr feedback.FeedbackRecord,
	ctx InterruptContext,
	hasCtx bool,
	currentPolicy *policy.PolicySet,
	currentSuppress *suppress.SuppressionSet,
	history FeedbackHistory,
	now time.Time,
	circleID string,
	policyChanges map[string]policy.CirclePolicy,
	triggerChanges map[string]policy.TriggerPolicy,
	newRules *[]suppress.SuppressionRule,
) DecisionRecord {
	decision := DecisionRecord{
		FeedbackID: fr.FeedbackID,
	}

	if !hasCtx {
		// No context - just bump threshold
		decision.Action = "threshold_increase"
		decision.Reason = "no_context"
		e.bumpCircleThreshold(circleID, currentPolicy, policyChanges)
		decision.Details = fmt.Sprintf("circle:%s threshold+%d", circleID, e.config.ThresholdIncrement)
		return decision
	}

	// Check for repeated unnecessary feedback on this trigger
	if ctx.Trigger != "" && history != nil {
		since := now.Add(-e.config.RepeatedUnnecessaryWindow)
		recentFeedback := history.GetRecentByCircleAndTrigger(fr.CircleID, ctx.Trigger, since)

		if len(recentFeedback) >= e.config.RepeatedUnnecessaryCount-1 {
			// Add suppression rule
			expires := now.Add(e.config.SuppressionTTL)

			// Prefer person/vendor scope if available
			var scope suppress.Scope
			var key string
			var reason string

			if ctx.PersonID != "" {
				scope = suppress.ScopePerson
				key = ctx.PersonID
				reason = fmt.Sprintf("repeated_unnecessary:person:%s", ctx.PersonID)
			} else if ctx.VendorID != "" {
				scope = suppress.ScopeVendor
				key = ctx.VendorID
				reason = fmt.Sprintf("repeated_unnecessary:vendor:%s", ctx.VendorID)
			} else {
				scope = suppress.ScopeTrigger
				key = ctx.Trigger
				reason = fmt.Sprintf("repeated_unnecessary:trigger:%s", ctx.Trigger)
			}

			// Check if rule already exists
			existingRule := currentSuppress.FindMatch(now, circleID, scope, key)
			if existingRule == nil {
				rule := suppress.NewSuppressionRule(
					circleID,
					scope,
					key,
					now,
					&expires,
					reason,
					suppress.SourceFeedback,
				)
				*newRules = append(*newRules, rule)

				decision.Action = "suppression_add"
				decision.Reason = "repeated_unnecessary"
				decision.Details = fmt.Sprintf("circle:%s scope:%s key:%s ttl:30d", circleID, scope, key)
				return decision
			}
		}
	}

	// Apply trigger bias if trigger known
	if ctx.Trigger != "" {
		e.adjustTriggerBias(ctx.Trigger, -e.config.BiasIncrement, currentPolicy, triggerChanges)
		decision.Action = "trigger_bias_decrease"
		decision.Reason = "unnecessary_feedback"
		decision.Details = fmt.Sprintf("trigger:%s bias-%d", ctx.Trigger, e.config.BiasIncrement)
		return decision
	}

	// Fall back to threshold bump
	decision.Action = "threshold_increase"
	decision.Reason = "unnecessary_feedback"
	e.bumpCircleThreshold(circleID, currentPolicy, policyChanges)
	decision.Details = fmt.Sprintf("circle:%s threshold+%d", circleID, e.config.ThresholdIncrement)
	return decision
}

// handleHelpful processes helpful feedback.
func (e *Engine) handleHelpful(
	fr feedback.FeedbackRecord,
	ctx InterruptContext,
	hasCtx bool,
	currentPolicy *policy.PolicySet,
	circleID string,
	policyChanges map[string]policy.CirclePolicy,
	triggerChanges map[string]policy.TriggerPolicy,
) DecisionRecord {
	decision := DecisionRecord{
		FeedbackID: fr.FeedbackID,
	}

	// Decrease threshold or increase bias
	if hasCtx && ctx.Trigger != "" {
		e.adjustTriggerBias(ctx.Trigger, e.config.BiasIncrement, currentPolicy, triggerChanges)
		decision.Action = "trigger_bias_increase"
		decision.Reason = "helpful_feedback"
		decision.Details = fmt.Sprintf("trigger:%s bias+%d", ctx.Trigger, e.config.BiasIncrement)
	} else {
		e.lowerCircleThreshold(circleID, currentPolicy, policyChanges)
		decision.Action = "threshold_decrease"
		decision.Reason = "helpful_feedback"
		decision.Details = fmt.Sprintf("circle:%s threshold-%d", circleID, e.config.ThresholdDecrement)
	}

	return decision
}

// bumpCircleThreshold increases the regret threshold for a circle.
func (e *Engine) bumpCircleThreshold(circleID string, current *policy.PolicySet, changes map[string]policy.CirclePolicy) {
	// Get existing or from changes
	cp, exists := changes[circleID]
	if !exists {
		if existing := current.GetCircle(circleID); existing != nil {
			cp = *existing
		} else {
			cp = policy.MinimalCirclePolicy(circleID)
		}
	}

	// Increase threshold
	cp.RegretThreshold = minInt(cp.RegretThreshold+e.config.ThresholdIncrement, e.config.ThresholdCeiling)

	// Ensure monotonicity
	if cp.NotifyThreshold < cp.RegretThreshold {
		cp.NotifyThreshold = cp.RegretThreshold
	}
	if cp.UrgentThreshold < cp.NotifyThreshold {
		cp.UrgentThreshold = cp.NotifyThreshold
	}

	changes[circleID] = cp
}

// lowerCircleThreshold decreases the regret threshold for a circle.
func (e *Engine) lowerCircleThreshold(circleID string, current *policy.PolicySet, changes map[string]policy.CirclePolicy) {
	// Get existing or from changes
	cp, exists := changes[circleID]
	if !exists {
		if existing := current.GetCircle(circleID); existing != nil {
			cp = *existing
		} else {
			cp = policy.MinimalCirclePolicy(circleID)
		}
	}

	// Decrease threshold
	cp.RegretThreshold = maxInt(cp.RegretThreshold-e.config.ThresholdDecrement, e.config.ThresholdFloor)

	changes[circleID] = cp
}

// adjustTriggerBias adjusts the bias for a trigger.
func (e *Engine) adjustTriggerBias(trigger string, delta int, current *policy.PolicySet, changes map[string]policy.TriggerPolicy) {
	// Get existing or from changes
	tp, exists := changes[trigger]
	if !exists {
		if existing := current.GetTrigger(trigger); existing != nil {
			tp = *existing
		} else {
			tp = policy.TriggerPolicy{Trigger: trigger}
		}
	}

	// Adjust bias within bounds
	tp.RegretBias = clampInt(tp.RegretBias+delta, e.config.BiasFloor, e.config.BiasCeiling)

	changes[trigger] = tp
}

// Helper functions
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clampInt(val, min, max int) int {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

// FormatDecisions returns a canonical string of all decisions.
func FormatDecisions(decisions []DecisionRecord) string {
	var sb strings.Builder
	for i, d := range decisions {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(d.CanonicalString())
	}
	return sb.String()
}
