// Package obligations provides Gmail-specific obligation extraction rules.
//
// Phase 18.9: Real Data Quiet Verification
// Reference: docs/ADR/ADR-0042-phase18-9-real-data-quiet-verification.md
//
// CRITICAL: Default to HOLD. Never auto-surface.
// CRITICAL: When in doubt, suppress.
// CRITICAL: Abstract only - no sender names, subjects, content.
// CRITICAL: Conservative regret scores - quiet is the goal.
//
// Gmail obligations are extracted from message metadata only.
// We read message headers, not bodies. We see patterns, not content.
package obligations

import (
	"time"

	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/obligation"
)

// GmailRestraintConfig defines conservative settings for Gmail obligations.
// All values chosen to minimize interruption and maximize quietness.
type GmailRestraintConfig struct {
	// DefaultToHold is true - all Gmail obligations start held.
	DefaultToHold bool

	// MaxDailyObligations limits how many can surface per day.
	// Even if rules match, we cap surfacing.
	MaxDailyObligations int

	// StalenessThresholdDays: ignore messages older than this.
	StalenessThresholdDays int

	// BaseRegret is the starting regret for any Gmail obligation.
	// Kept very low to prefer quietness.
	BaseRegret float64

	// HighPriorityBump increases regret for important messages.
	// Still conservative - never exceeds threshold without explicit action.
	HighPriorityBump float64

	// MaxRegret caps regret to prevent auto-surfacing.
	// Keep below surface threshold to ensure holding.
	MaxRegret float64

	// RequireExplicitAction means nothing surfaces automatically.
	RequireExplicitAction bool
}

// DefaultGmailRestraintConfig returns conservative defaults.
// These values ensure Gmail data stays quiet by default.
func DefaultGmailRestraintConfig() GmailRestraintConfig {
	return GmailRestraintConfig{
		DefaultToHold:          true,  // Always hold by default
		MaxDailyObligations:    3,     // Very limited surfacing
		StalenessThresholdDays: 3,     // Ignore old messages
		BaseRegret:             0.15,  // Very low - prefer quiet
		HighPriorityBump:       0.10,  // Conservative increase
		MaxRegret:              0.35,  // Never exceeds surface threshold
		RequireExplicitAction:  true,  // Nothing auto-surfaces
	}
}

// GmailObligationExtractor extracts obligations with restraint-first approach.
type GmailObligationExtractor struct {
	config GmailRestraintConfig
}

// NewGmailObligationExtractor creates a restraint-first Gmail extractor.
func NewGmailObligationExtractor(config GmailRestraintConfig) *GmailObligationExtractor {
	return &GmailObligationExtractor{config: config}
}

// GmailMessageMeta represents abstract message metadata.
// CRITICAL: No content, no sender names, no subjects stored.
type GmailMessageMeta struct {
	// MessageHash is a hash of the message ID (not the ID itself).
	MessageHash string

	// DomainBucket is the abstract sender domain category.
	// Values: "personal", "commercial", "automated", "unknown"
	DomainBucket string

	// ReceivedAt is when the message was received.
	ReceivedAt time.Time

	// LabelBucket abstracts Gmail labels.
	// Values: "inbox", "important", "starred", "other"
	LabelBucket string

	// IsUnread indicates if the message is unread.
	IsUnread bool

	// HasActionCue indicates if action words were detected.
	// We detect but don't store the words themselves.
	HasActionCue bool

	// CircleID is the circle this message belongs to.
	CircleID identity.EntityID
}

// ExtractFromMessages applies restraint-first rules to Gmail messages.
// Returns obligations with conservative regret scores.
func (e *GmailObligationExtractor) ExtractFromMessages(
	messages []GmailMessageMeta,
	now time.Time,
) []*obligation.Obligation {
	var result []*obligation.Obligation

	// Staleness threshold
	staleThreshold := now.Add(-time.Duration(e.config.StalenessThresholdDays) * 24 * time.Hour)

	for _, msg := range messages {
		// Skip old messages - don't create obligations for stale data
		if msg.ReceivedAt.Before(staleThreshold) {
			continue
		}

		// Skip read messages - already handled
		if !msg.IsUnread {
			continue
		}

		// Skip automated/commercial unless action cue detected
		if (msg.DomainBucket == "automated" || msg.DomainBucket == "commercial") && !msg.HasActionCue {
			continue
		}

		// Create obligation with conservative regret
		oblig := e.createRestrainedObligation(msg, now)
		if oblig != nil {
			result = append(result, oblig)
		}
	}

	// Apply daily cap
	if len(result) > e.config.MaxDailyObligations {
		result = result[:e.config.MaxDailyObligations]
	}

	// Sort deterministically
	obligation.SortObligations(result)

	return result
}

// createRestrainedObligation creates an obligation with conservative scoring.
func (e *GmailObligationExtractor) createRestrainedObligation(
	msg GmailMessageMeta,
	now time.Time,
) *obligation.Obligation {
	oblig := obligation.NewObligation(
		msg.CircleID,
		msg.MessageHash, // Hash of message ID, not the ID itself
		"gmail",
		obligation.ObligationReview,
		msg.ReceivedAt,
	)

	// Start with base regret (very low)
	regret := e.config.BaseRegret

	// Small bump for important/starred
	if msg.LabelBucket == "important" || msg.LabelBucket == "starred" {
		regret += e.config.HighPriorityBump
	}

	// Small bump for action cues
	if msg.HasActionCue {
		regret += e.config.HighPriorityBump
	}

	// Cap regret to ensure holding
	if regret > e.config.MaxRegret {
		regret = e.config.MaxRegret
	}

	// Abstract reason - never expose specifics
	reason := "Email noticed"
	if msg.LabelBucket == "important" {
		reason = "Important email noticed"
	}

	oblig.WithScoring(regret, 0.70). // Low confidence = more likely to hold
						WithReason(reason).
						WithEvidence("domain_bucket", msg.DomainBucket).
						WithEvidence("label_bucket", msg.LabelBucket).
						WithSeverity(obligation.SeverityLow). // Always low severity
						WithSuppressible(true)                // Always suppressible

	// If RequireExplicitAction is true, mark as requiring human review
	if e.config.RequireExplicitAction {
		oblig.WithEvidence("requires_explicit_action", "true")
	}

	return oblig
}

// ShouldHold determines if a Gmail obligation should be held.
// Phase 18.9: Default answer is YES.
func (e *GmailObligationExtractor) ShouldHold(oblig *obligation.Obligation) bool {
	// If DefaultToHold is true, always hold
	if e.config.DefaultToHold {
		return true
	}

	// If RequireExplicitAction, always hold
	if e.config.RequireExplicitAction {
		return true
	}

	// If regret is below surface threshold, hold
	if oblig.RegretScore < 0.5 {
		return true
	}

	// Default: hold
	return true
}

// GmailRestraintPolicy defines the policy for Gmail obligations.
// This is the source of truth for Gmail quietness rules.
type GmailRestraintPolicy struct {
	// NeverAutoSurface prevents any Gmail obligation from auto-surfacing.
	NeverAutoSurface bool

	// RequireUserAction means user must explicitly request surfacing.
	RequireUserAction bool

	// HoldByDefault keeps all Gmail obligations in held state.
	HoldByDefault bool

	// AbstractOnly prevents storing any identifiable information.
	AbstractOnly bool
}

// DefaultGmailRestraintPolicy returns the Phase 18.9 restraint policy.
func DefaultGmailRestraintPolicy() GmailRestraintPolicy {
	return GmailRestraintPolicy{
		NeverAutoSurface:  true,
		RequireUserAction: true,
		HoldByDefault:     true,
		AbstractOnly:      true,
	}
}

// Validate checks if the policy maintains quietness guarantees.
func (p GmailRestraintPolicy) Validate() bool {
	// All must be true for Phase 18.9 compliance
	return p.NeverAutoSurface &&
		p.RequireUserAction &&
		p.HoldByDefault &&
		p.AbstractOnly
}
