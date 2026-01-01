// Package interruptions implements the interruption engine for Phase 3.
//
// The engine transforms obligations into prioritized interruptions,
// applying dedup and quota logic to prevent "spammy repeats".
//
// CRITICAL: Uses injected clock, never time.Now().
// CRITICAL: Synchronous processing, no goroutines.
// CRITICAL: Deterministic: same inputs + clock = same outputs.
// CRITICAL: Read-only. This engine only classifies, never acts.
//
// Reference: docs/ADR/ADR-0020-phase3-interruptions-and-digest.md
package interruptions

import (
	"time"

	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/interrupt"
	"quantumlife/pkg/domain/obligation"
	"quantumlife/pkg/domain/view"
)

// Config holds engine configuration.
type Config struct {
	// Regret base scores by circle type
	CircleRegretBase map[string]int

	// Time urgency boosts
	DueWithin24hBoost int
	DueWithin7dBoost  int

	// Action needed boost
	ActionNeededBoost int

	// Level thresholds
	UrgentThreshold  int // Regret >= this AND due within 24h
	NotifyThreshold  int // Regret >= this AND due within 48h
	QueuedThreshold  int // Regret >= this
	AmbientThreshold int // Regret >= this

	// Default expiry for items without due date
	DefaultExpiryDays int
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		CircleRegretBase: map[string]int{
			"finance": 30,
			"family":  25,
			"work":    15,
			"health":  20,
			"home":    10,
		},
		DueWithin24hBoost: 30,
		DueWithin7dBoost:  15,
		ActionNeededBoost: 15,
		UrgentThreshold:   90,
		NotifyThreshold:   75,
		QueuedThreshold:   50,
		AmbientThreshold:  25,
		DefaultExpiryDays: 7,
	}
}

// Engine processes obligations into interruptions.
type Engine struct {
	config        Config
	clk           clock.Clock
	dedupStore    DedupStore
	quotaEnforcer *QuotaEnforcer
}

// NewEngine creates a new interruption engine.
func NewEngine(config Config, clk clock.Clock, dedupStore DedupStore, quotaStore QuotaStore) *Engine {
	quotaConfig := DefaultQuotaConfig()
	return &Engine{
		config:        config,
		clk:           clk,
		dedupStore:    dedupStore,
		quotaEnforcer: NewQuotaEnforcer(quotaConfig, quotaStore),
	}
}

// ProcessResult contains engine output.
type ProcessResult struct {
	Interruptions []*interrupt.Interruption
	Report        *interrupt.DecisionReport
	Hash          string
}

// Process transforms obligations into prioritized interruptions.
func (e *Engine) Process(dailyView *view.DailyView, obligations []*obligation.Obligation) ProcessResult {
	now := e.clk.Now()
	report := interrupt.NewDecisionReport()

	// Step 1: Transform obligations to interruptions
	var interruptions []*interrupt.Interruption
	for _, oblig := range obligations {
		intr := e.obligationToInterruption(oblig, now)
		interruptions = append(interruptions, intr)
	}
	report.TotalProcessed = len(interruptions)

	// Step 2: Apply dedup
	interruptions, dedupDropped := Dedup(interruptions, e.dedupStore)
	report.DedupDropped = dedupDropped

	// Step 3: Apply quota
	interruptions, quotaDowngraded := e.quotaEnforcer.Apply(interruptions, now)
	report.QuotaDowngraded = quotaDowngraded

	// Step 4: Sort by priority
	interrupt.SortInterruptions(interruptions)

	// Step 5: Count by level
	report.CountByLevel = interrupt.CountByLevel(interruptions)

	// Step 6: Build circle summaries
	e.buildCircleSummaries(interruptions, report)

	// Step 7: Compute hash
	hash := interrupt.ComputeInterruptionsHash(interruptions)

	return ProcessResult{
		Interruptions: interruptions,
		Report:        report,
		Hash:          hash,
	}
}

// obligationToInterruption transforms a single obligation.
func (e *Engine) obligationToInterruption(oblig *obligation.Obligation, now time.Time) *interrupt.Interruption {
	// Compute regret score (0-100)
	regret := e.computeRegretScore(oblig, now)

	// Compute level from regret and horizon
	level := e.computeLevel(regret, oblig, now)

	// Compute trigger from obligation type
	trigger := e.obligationToTrigger(oblig)

	// Compute expiry
	expiresAt := e.computeExpiry(oblig, now)

	// Compute confidence (rules-based, so relatively high)
	confidence := 80
	if oblig.Confidence > 0 {
		confidence = int(oblig.Confidence * 100)
	}

	// Build summary
	summary := e.buildSummary(oblig)

	return interrupt.NewInterruption(
		oblig.CircleID,
		trigger,
		oblig.SourceEventID,
		oblig.ID,
		regret,
		confidence,
		level,
		expiresAt,
		now,
		summary,
	)
}

// computeRegretScore calculates regret (0-100).
func (e *Engine) computeRegretScore(oblig *obligation.Obligation, now time.Time) int {
	score := 0

	// Base by circle type
	circleType := circleTypeFromID(oblig.CircleID)
	if base, ok := e.config.CircleRegretBase[circleType]; ok {
		score += base
	} else {
		score += 15 // Default
	}

	// Time urgency boost
	if oblig.DueBy != nil {
		until := oblig.DueBy.Sub(now)
		switch {
		case until <= 24*time.Hour:
			score += e.config.DueWithin24hBoost
		case until <= 7*24*time.Hour:
			score += e.config.DueWithin7dBoost
		}
	}

	// Action needed indicators
	if hasActionIndicator(oblig) {
		score += e.config.ActionNeededBoost
	}

	// Add obligation's own regret (scaled)
	score += int(oblig.RegretScore * 30)

	// Severity boost
	switch oblig.Severity {
	case obligation.SeverityCritical:
		score += 20
	case obligation.SeverityHigh:
		score += 10
	}

	// Cap at 100
	if score > 100 {
		score = 100
	}

	return score
}

// computeLevel determines interruption level.
func (e *Engine) computeLevel(regret int, oblig *obligation.Obligation, now time.Time) interrupt.Level {
	var hoursUntilDue float64 = -1
	if oblig.DueBy != nil {
		hoursUntilDue = oblig.DueBy.Sub(now).Hours()
	}

	// Urgent: Regret >= 90 AND due within 24h
	if regret >= e.config.UrgentThreshold && hoursUntilDue >= 0 && hoursUntilDue <= 24 {
		return interrupt.LevelUrgent
	}

	// Notify: Regret >= 75 AND due within 48h
	if regret >= e.config.NotifyThreshold && hoursUntilDue >= 0 && hoursUntilDue <= 48 {
		return interrupt.LevelNotify
	}

	// Queued: Regret >= 50
	if regret >= e.config.QueuedThreshold {
		return interrupt.LevelQueued
	}

	// Ambient: Regret >= 25
	if regret >= e.config.AmbientThreshold {
		return interrupt.LevelAmbient
	}

	// Silent: else
	return interrupt.LevelSilent
}

// obligationToTrigger maps obligation type to trigger.
func (e *Engine) obligationToTrigger(oblig *obligation.Obligation) interrupt.Trigger {
	// Handle commerce source first (Phase 8)
	if oblig.SourceType == "commerce" {
		return e.commerceToTrigger(oblig)
	}

	switch oblig.Type {
	case obligation.ObligationReply, obligation.ObligationReview:
		if oblig.SourceType == "email" {
			return interrupt.TriggerEmailActionNeeded
		}
		if oblig.SourceType == "finance" {
			if oblig.Evidence[obligation.EvidenceKeyBalance] != "" {
				return interrupt.TriggerFinanceLowBalance
			}
			return interrupt.TriggerFinanceLargeTxn
		}
	case obligation.ObligationAttend:
		return interrupt.TriggerCalendarUpcoming
	case obligation.ObligationDecide:
		if oblig.Evidence[obligation.EvidenceKeyConflictWith] != "" {
			return interrupt.TriggerCalendarConflict
		}
		return interrupt.TriggerCalendarInvitePending
	case obligation.ObligationPay:
		return interrupt.TriggerFinancePending
	}

	// Check for due soon
	if oblig.Horizon == obligation.HorizonToday || oblig.Horizon == obligation.Horizon24h {
		return interrupt.TriggerObligationDueSoon
	}

	return interrupt.TriggerUnknown
}

// commerceToTrigger maps commerce obligations to triggers.
func (e *Engine) commerceToTrigger(oblig *obligation.Obligation) interrupt.Trigger {
	switch oblig.Type {
	case obligation.ObligationPay:
		return interrupt.TriggerCommerceInvoiceDue
	case obligation.ObligationFollowup:
		// Check for shipment vs refund
		if oblig.Evidence["tracking_id"] != "" || oblig.Evidence["status"] != "" {
			return interrupt.TriggerCommerceShipmentPending
		}
		return interrupt.TriggerCommerceRefundPending
	case obligation.ObligationReview:
		// Could be subscription renewal or large order
		if vendor := oblig.Evidence["vendor"]; vendor != "" {
			// Check reason for subscription keyword
			if containsSubscription(oblig.Reason) {
				return interrupt.TriggerCommerceSubscriptionRenewed
			}
		}
		return interrupt.TriggerObligationDueSoon
	}

	return interrupt.TriggerUnknown
}

// containsSubscription checks if reason mentions subscription.
func containsSubscription(reason string) bool {
	lowerReason := reason
	for i := 0; i < len(lowerReason); i++ {
		if lowerReason[i] >= 'A' && lowerReason[i] <= 'Z' {
			lowerReason = lowerReason[:i] + string(lowerReason[i]+32) + lowerReason[i+1:]
		}
	}
	return len(lowerReason) > 0 && (contains(lowerReason, "subscription") ||
		contains(lowerReason, "renewal") ||
		contains(lowerReason, "renewed"))
}

// contains is a simple substring check.
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// computeExpiry determines when this interruption expires.
func (e *Engine) computeExpiry(oblig *obligation.Obligation, now time.Time) time.Time {
	if oblig.DueBy != nil {
		return *oblig.DueBy
	}
	return now.AddDate(0, 0, e.config.DefaultExpiryDays)
}

// buildSummary creates a human-readable summary.
func (e *Engine) buildSummary(oblig *obligation.Obligation) string {
	if oblig.Reason != "" {
		return oblig.Reason
	}

	// Build from type
	switch oblig.Type {
	case obligation.ObligationReply:
		return "Reply needed"
	case obligation.ObligationAttend:
		if title := oblig.Evidence[obligation.EvidenceKeyEventTitle]; title != "" {
			return "Event: " + truncate(title, 40)
		}
		return "Event to attend"
	case obligation.ObligationPay:
		return "Payment attention needed"
	case obligation.ObligationReview:
		if subject := oblig.Evidence[obligation.EvidenceKeySubject]; subject != "" {
			return "Review: " + truncate(subject, 40)
		}
		return "Item to review"
	case obligation.ObligationDecide:
		return "Decision needed"
	case obligation.ObligationFollowup:
		return "Follow up needed"
	}

	return "Attention needed"
}

// buildCircleSummaries populates circle summaries in the report.
func (e *Engine) buildCircleSummaries(interruptions []*interrupt.Interruption, report *interrupt.DecisionReport) {
	for _, intr := range interruptions {
		summary, ok := report.CircleSummaries[intr.CircleID]
		if !ok {
			summary = &interrupt.CircleDecisionSummary{
				CircleID: intr.CircleID,
			}
			report.CircleSummaries[intr.CircleID] = summary
		}

		summary.Total++
		switch intr.Level {
		case interrupt.LevelUrgent:
			summary.Urgent++
		case interrupt.LevelNotify:
			summary.Notify++
		case interrupt.LevelQueued:
			summary.Queued++
		case interrupt.LevelAmbient:
			summary.Ambient++
		case interrupt.LevelSilent:
			summary.Silent++
		}

		// Track triggers (up to 3)
		if len(summary.TopTriggers) < 3 {
			found := false
			for _, t := range summary.TopTriggers {
				if t == intr.Trigger {
					found = true
					break
				}
			}
			if !found {
				summary.TopTriggers = append(summary.TopTriggers, intr.Trigger)
			}
		}
	}
}

// hasActionIndicator checks if obligation has action-needed markers.
func hasActionIndicator(oblig *obligation.Obligation) bool {
	// Check obligation type
	switch oblig.Type {
	case obligation.ObligationReply, obligation.ObligationPay, obligation.ObligationDecide:
		return true
	}

	// Check severity
	if oblig.Severity == obligation.SeverityHigh || oblig.Severity == obligation.SeverityCritical {
		return true
	}

	return false
}

// truncate truncates a string to maxLen.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
