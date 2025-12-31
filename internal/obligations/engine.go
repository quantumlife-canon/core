// Package obligations implements the obligation extraction engine.
//
// The engine extracts obligations from canonical events using deterministic rules.
// No ML, no external dependencies, no persistence.
//
// CRITICAL: Uses injected clock, never time.Now().
// CRITICAL: Synchronous processing, no goroutines.
// CRITICAL: Same events + same clock = same obligations.
//
// Reference: docs/ADR/ADR-0019-phase2-obligation-extraction.md
package obligations

import (
	"fmt"
	"strings"
	"time"

	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/events"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/obligation"
)

// Config holds engine configuration with sensible defaults.
type Config struct {
	// Regret thresholds by circle type (default: 0.3 for most)
	CircleRegretThresholds map[string]float64

	// Finance thresholds
	LowBalanceThresholdMinor int64   // Below this triggers review (default: 50000 = £500)
	LargeTransactionMinor    int64   // Above this triggers review (default: 50000 = £500)
	LowBalanceRegret         float64 // Regret score for low balance (default: 0.7)

	// Email thresholds
	StaleEmailDays     int     // Days before unread email becomes followup (default: 7)
	ImportantRegret    float64 // Regret for important emails (default: 0.6)
	ActionNeededRegret float64 // Regret for action-needed emails (default: 0.7)

	// Calendar thresholds
	UpcomingEventHours int     // Hours before event triggers obligation (default: 24)
	UnrespondedRegret  float64 // Regret for unresponded invites (default: 0.6)
	ConflictRegret     float64 // Regret for calendar conflicts (default: 0.8)

	// High-priority sender domains (increase regret)
	HighPriorityDomains []string
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		CircleRegretThresholds: map[string]float64{
			"work":    0.3,
			"family":  0.4,
			"finance": 0.5,
			"health":  0.4,
			"home":    0.5,
		},
		LowBalanceThresholdMinor: 50000, // £500
		LargeTransactionMinor:    50000, // £500
		LowBalanceRegret:         0.7,
		StaleEmailDays:           7,
		ImportantRegret:          0.6,
		ActionNeededRegret:       0.7,
		UpcomingEventHours:       24,
		UnrespondedRegret:        0.6,
		ConflictRegret:           0.8,
		HighPriorityDomains: []string{
			"company.com", "bank.co.uk", "hmrc.gov.uk",
			"school.edu", "nhs.uk",
		},
	}
}

// IdentityRepository provides identity lookups.
// Minimal interface to avoid importing internal packages.
type IdentityRepository interface {
	// GetByID retrieves an entity by ID.
	GetByID(id identity.EntityID) (identity.Entity, error)
	// IsHighPriority returns true if the entity is marked high-priority.
	IsHighPriority(id identity.EntityID) bool
}

// Engine extracts obligations from events.
type Engine struct {
	config       Config
	clk          clock.Clock
	identityRepo IdentityRepository
}

// NewEngine creates a new extraction engine.
func NewEngine(config Config, clk clock.Clock, identityRepo IdentityRepository) *Engine {
	return &Engine{
		config:       config,
		clk:          clk,
		identityRepo: identityRepo,
	}
}

// ExtractResult holds extraction results.
type ExtractResult struct {
	Obligations []*obligation.Obligation
	Hash        string // Deterministic hash of all obligations
}

// Extract processes all events and extracts obligations.
// Events are processed synchronously in a single pass.
func (e *Engine) Extract(eventStore events.EventStore, circleIDs []identity.EntityID) ExtractResult {
	now := e.clk.Now()
	var allObligations []*obligation.Obligation

	for _, circleID := range circleIDs {
		// Process emails
		emailType := events.EventTypeEmailMessage
		emails, _ := eventStore.GetByCircle(circleID, &emailType, 0)
		for _, evt := range emails {
			if email, ok := evt.(*events.EmailMessageEvent); ok {
				obligs := e.extractFromEmail(email, circleID, now)
				allObligations = append(allObligations, obligs...)
			}
		}

		// Process calendar events
		calType := events.EventTypeCalendarEvent
		calEvents, _ := eventStore.GetByCircle(circleID, &calType, 0)
		for _, evt := range calEvents {
			if calEvt, ok := evt.(*events.CalendarEventEvent); ok {
				obligs := e.extractFromCalendar(calEvt, circleID, now, calEvents)
				allObligations = append(allObligations, obligs...)
			}
		}

		// Process finance - balances
		balType := events.EventTypeBalance
		balances, _ := eventStore.GetByCircle(circleID, &balType, 0)
		for _, evt := range balances {
			if bal, ok := evt.(*events.BalanceEvent); ok {
				obligs := e.extractFromBalance(bal, circleID, now)
				allObligations = append(allObligations, obligs...)
			}
		}

		// Process finance - transactions
		txType := events.EventTypeTransaction
		transactions, _ := eventStore.GetByCircle(circleID, &txType, 0)
		for _, evt := range transactions {
			if tx, ok := evt.(*events.TransactionEvent); ok {
				obligs := e.extractFromTransaction(tx, circleID, now)
				allObligations = append(allObligations, obligs...)
			}
		}
	}

	// Sort deterministically
	obligation.SortObligations(allObligations)

	// Compute hash
	hash := obligation.ComputeObligationsHash(allObligations)

	return ExtractResult{
		Obligations: allObligations,
		Hash:        hash,
	}
}

// extractFromEmail applies email rules.
func (e *Engine) extractFromEmail(email *events.EmailMessageEvent, circleID identity.EntityID, now time.Time) []*obligation.Obligation {
	var result []*obligation.Obligation

	// Skip read emails for most rules
	if email.IsRead {
		return result
	}

	// Skip automated/transactional emails for action obligations
	if email.IsAutomated && !email.IsTransactional {
		return result
	}

	// Check for action-needed indicators
	hasActionCue := hasEmailActionCue(email.Subject, email.BodyPreview)
	isImportant := email.IsImportant || email.IsStarred
	isHighPrioritySender := e.isHighPrioritySender(email.SenderDomain)

	// Parse due date from subject/body
	dueResult := obligation.ParseDueDate(email.Subject+" "+email.BodyPreview, now)

	// Rule 1: Unread + action cue -> reply/review obligation
	if hasActionCue {
		oblig := obligation.NewObligation(
			circleID,
			email.EventID(),
			"email",
			obligation.ObligationReview,
			email.OccurredAt(),
		)

		regret := e.config.ActionNeededRegret
		if isHighPrioritySender {
			regret += 0.15
		}

		oblig.WithScoring(regret, 0.85).
			WithReason("Email requires action").
			WithEvidence(obligation.EvidenceKeySubject, email.Subject).
			WithEvidence(obligation.EvidenceKeySender, email.From.Address).
			WithSeverity(obligation.SeverityHigh)

		if dueResult.Found {
			oblig.WithDueBy(dueResult.DueDate, now)
			oblig.WithEvidence(obligation.EvidenceKeyDueDate, dueResult.DueDate.Format("2006-01-02"))
		}

		result = append(result, oblig)
		return result // Don't create duplicate obligations
	}

	// Rule 2: Unread + important flag -> review obligation
	if isImportant {
		oblig := obligation.NewObligation(
			circleID,
			email.EventID(),
			"email",
			obligation.ObligationReview,
			email.OccurredAt(),
		)

		regret := e.config.ImportantRegret
		if isHighPrioritySender {
			regret += 0.1
		}

		oblig.WithScoring(regret, 0.75).
			WithReason("Important email awaiting review").
			WithEvidence(obligation.EvidenceKeySubject, email.Subject).
			WithEvidence(obligation.EvidenceKeySender, email.From.Address).
			WithSeverity(obligation.SeverityMedium)

		if dueResult.Found {
			oblig.WithDueBy(dueResult.DueDate, now)
		}

		result = append(result, oblig)
		return result
	}

	// Rule 3: Unread + transactional (invoice, receipt) -> review obligation
	if email.IsTransactional && hasInvoiceCue(email.Subject) {
		oblig := obligation.NewObligation(
			circleID,
			email.EventID(),
			"email",
			obligation.ObligationPay,
			email.OccurredAt(),
		)

		oblig.WithScoring(0.65, 0.80).
			WithReason("Invoice or payment notification").
			WithEvidence(obligation.EvidenceKeySubject, email.Subject).
			WithEvidence(obligation.EvidenceKeySender, email.From.Address).
			WithSeverity(obligation.SeverityMedium)

		if dueResult.Found {
			oblig.WithDueBy(dueResult.DueDate, now)
		}

		result = append(result, oblig)
		return result
	}

	// Rule 4: Stale unread email -> followup (low priority)
	emailAge := now.Sub(email.OccurredAt())
	staleThreshold := time.Duration(e.config.StaleEmailDays) * 24 * time.Hour
	if emailAge > staleThreshold && isHighPrioritySender {
		oblig := obligation.NewObligation(
			circleID,
			email.EventID(),
			"email",
			obligation.ObligationFollowup,
			email.OccurredAt(),
		)

		oblig.WithScoring(0.35, 0.60).
			WithReason("Stale unread email from important sender").
			WithEvidence(obligation.EvidenceKeySubject, email.Subject).
			WithEvidence(obligation.EvidenceKeySender, email.From.Address).
			WithSeverity(obligation.SeverityLow)

		result = append(result, oblig)
	}

	return result
}

// extractFromCalendar applies calendar rules.
func (e *Engine) extractFromCalendar(calEvt *events.CalendarEventEvent, circleID identity.EntityID, now time.Time, allCalEvents []events.CanonicalEvent) []*obligation.Obligation {
	var result []*obligation.Obligation

	// Skip cancelled events
	if calEvt.IsCancelled {
		return result
	}

	// Skip past events
	if calEvt.StartTime.Before(now) {
		return result
	}

	hoursUntil := calEvt.StartTime.Sub(now).Hours()
	threshold := float64(e.config.UpcomingEventHours)

	// Rule 1: Upcoming event not accepted -> decide obligation
	if hoursUntil <= threshold && calEvt.MyResponseStatus == events.RSVPNeedsAction {
		oblig := obligation.NewObligation(
			circleID,
			calEvt.EventID(),
			"calendar",
			obligation.ObligationDecide,
			calEvt.CapturedAt(),
		)

		regret := e.config.UnrespondedRegret
		// Increase regret as event approaches
		if hoursUntil <= 4 {
			regret += 0.2
		} else if hoursUntil <= 12 {
			regret += 0.1
		}

		oblig.WithDueBy(calEvt.StartTime, now).
			WithScoring(regret, 0.85).
			WithReason("Calendar invite awaiting response").
			WithEvidence(obligation.EvidenceKeyEventTitle, calEvt.Title).
			WithSeverity(obligation.SeverityHigh)

		result = append(result, oblig)
	}

	// Rule 2: Upcoming event (accepted) -> attend obligation
	if hoursUntil <= threshold && hoursUntil > 0 &&
		(calEvt.MyResponseStatus == events.RSVPAccepted || calEvt.MyResponseStatus == events.RSVPTentative) {

		oblig := obligation.NewObligation(
			circleID,
			calEvt.EventID(),
			"calendar",
			obligation.ObligationAttend,
			calEvt.CapturedAt(),
		)

		regret := 0.5
		if hoursUntil <= 2 {
			regret = 0.8
		} else if hoursUntil <= 6 {
			regret = 0.65
		}

		oblig.WithDueBy(calEvt.StartTime, now).
			WithScoring(regret, 0.95).
			WithReason("Upcoming event to attend").
			WithEvidence(obligation.EvidenceKeyEventTitle, calEvt.Title).
			WithSeverity(obligation.SeverityMedium).
			WithSuppressible(false) // Can't dismiss upcoming events

		result = append(result, oblig)
	}

	// Rule 3: Detect conflicts with other events
	for _, other := range allCalEvents {
		otherCal, ok := other.(*events.CalendarEventEvent)
		if !ok || otherCal.EventID() == calEvt.EventID() {
			continue
		}
		if otherCal.IsCancelled {
			continue
		}

		// Check for overlap
		if eventsOverlap(calEvt, otherCal) {
			// Only create one conflict obligation per pair (use ID ordering)
			if calEvt.EventID() > otherCal.EventID() {
				continue
			}

			oblig := obligation.NewObligation(
				circleID,
				calEvt.EventID(),
				"calendar",
				obligation.ObligationDecide,
				now,
			)

			earlierStart := calEvt.StartTime
			if otherCal.StartTime.Before(earlierStart) {
				earlierStart = otherCal.StartTime
			}

			oblig.WithDueBy(earlierStart, now).
				WithScoring(e.config.ConflictRegret, 0.90).
				WithReason("Calendar conflict detected").
				WithEvidence(obligation.EvidenceKeyEventTitle, calEvt.Title).
				WithEvidence(obligation.EvidenceKeyConflictWith, otherCal.Title).
				WithSeverity(obligation.SeverityCritical)

			result = append(result, oblig)
		}
	}

	return result
}

// extractFromBalance applies balance rules.
func (e *Engine) extractFromBalance(bal *events.BalanceEvent, circleID identity.EntityID, now time.Time) []*obligation.Obligation {
	var result []*obligation.Obligation

	// Rule: Low balance -> review obligation
	if bal.AvailableMinor < e.config.LowBalanceThresholdMinor {
		oblig := obligation.NewObligation(
			circleID,
			bal.EventID(),
			"finance",
			obligation.ObligationReview,
			bal.AsOf,
		)

		oblig.WithScoring(e.config.LowBalanceRegret, 0.95).
			WithReason("Account balance below threshold").
			WithEvidence(obligation.EvidenceKeyBalance, formatMinor(bal.AvailableMinor, bal.Currency)).
			WithEvidence(obligation.EvidenceKeyThreshold, formatMinor(e.config.LowBalanceThresholdMinor, bal.Currency)).
			WithSeverity(obligation.SeverityHigh)

		result = append(result, oblig)
	}

	return result
}

// extractFromTransaction applies transaction rules.
func (e *Engine) extractFromTransaction(tx *events.TransactionEvent, circleID identity.EntityID, now time.Time) []*obligation.Obligation {
	var result []*obligation.Obligation

	// Rule 1: Large outgoing transaction -> review obligation
	if tx.TransactionType == "DEBIT" && tx.AmountMinor >= e.config.LargeTransactionMinor {
		// Only recent transactions (last 48h)
		if now.Sub(tx.TransactionDate) > 48*time.Hour {
			return result
		}

		oblig := obligation.NewObligation(
			circleID,
			tx.EventID(),
			"finance",
			obligation.ObligationReview,
			tx.TransactionDate,
		)

		oblig.WithScoring(0.45, 0.85).
			WithReason("Large transaction to review").
			WithEvidence(obligation.EvidenceKeyMerchant, tx.MerchantName).
			WithEvidence(obligation.EvidenceKeyAmount, formatMinor(tx.AmountMinor, tx.Currency)).
			WithSeverity(obligation.SeverityLow)

		result = append(result, oblig)
	}

	// Rule 2: Pending transaction -> informational (low regret)
	if tx.TransactionStatus == "PENDING" {
		oblig := obligation.NewObligation(
			circleID,
			tx.EventID(),
			"finance",
			obligation.ObligationReview,
			tx.TransactionDate,
		)

		oblig.WithScoring(0.25, 0.90).
			WithReason("Pending transaction").
			WithEvidence(obligation.EvidenceKeyMerchant, tx.MerchantName).
			WithEvidence(obligation.EvidenceKeyAmount, formatMinor(tx.AmountMinor, tx.Currency)).
			WithSeverity(obligation.SeverityLow)

		result = append(result, oblig)
	}

	return result
}

// Helper functions

func (e *Engine) isHighPrioritySender(domain string) bool {
	for _, d := range e.config.HighPriorityDomains {
		if d == domain {
			return true
		}
	}
	return false
}

func hasEmailActionCue(subject, body string) bool {
	text := strings.ToLower(subject + " " + body)
	cues := []string{
		"action required", "action needed", "please respond",
		"response required", "response needed", "urgent",
		"asap", "immediately", "deadline", "due by",
		"please review", "approval needed", "approval required",
		"sign off", "signoff", "your input", "your feedback",
	}
	for _, cue := range cues {
		if strings.Contains(text, cue) {
			return true
		}
	}
	return false
}

func hasInvoiceCue(subject string) bool {
	lower := strings.ToLower(subject)
	cues := []string{
		"invoice", "payment due", "bill", "statement",
		"amount due", "pay now", "payment reminder",
	}
	for _, cue := range cues {
		if strings.Contains(lower, cue) {
			return true
		}
	}
	return false
}

func eventsOverlap(a, b *events.CalendarEventEvent) bool {
	// Two events overlap if one starts before the other ends
	return a.StartTime.Before(b.EndTime) && b.StartTime.Before(a.EndTime)
}

func formatMinor(amountMinor int64, currency string) string {
	major := float64(amountMinor) / 100.0
	symbol := currencySymbol(currency)
	return fmt.Sprintf("%s%.2f", symbol, major)
}

func currencySymbol(currency string) string {
	switch currency {
	case "GBP":
		return "£"
	case "USD":
		return "$"
	case "EUR":
		return "€"
	default:
		return currency + " "
	}
}
