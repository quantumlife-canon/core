// Package demo_phase2_obligations demonstrates obligation extraction and DailyView.
//
// This demo shows:
// - Extracting obligations from mock events (email, calendar, finance)
// - Computing NeedsYou status deterministically
// - Displaying per-circle summaries
// - Listing obligations with due dates, regret scores, and reasons
//
// CRITICAL: This is READ-ONLY. No notifications, no writes, no background execution.
// CRITICAL: Uses injected clock for determinism.
//
// Reference: docs/ADR/ADR-0019-phase2-obligation-extraction.md
package demo_phase2_obligations

import (
	"fmt"
	"strings"
	"time"

	"quantumlife/internal/obligations"
	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/events"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/view"
)

// DemoConfig configures the demo.
type DemoConfig struct {
	// Clock to use (injected for determinism)
	Clock clock.Clock

	// Scenario controls which mock data to generate
	Scenario Scenario

	// Verbose enables detailed output
	Verbose bool
}

// Scenario defines the demo scenario.
type Scenario string

const (
	// ScenarioNeedsYou generates high-regret obligations (NeedsYou = true)
	ScenarioNeedsYou Scenario = "needs_you"

	// ScenarioNothingNeedsYou generates only low-priority items (NeedsYou = false)
	ScenarioNothingNeedsYou Scenario = "nothing_needs_you"

	// ScenarioMixed generates a mix (demonstrates thresholds)
	ScenarioMixed Scenario = "mixed"
)

// DefaultConfig returns sensible defaults.
func DefaultConfig() DemoConfig {
	return DemoConfig{
		Clock:    clock.NewFixed(time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC)),
		Scenario: ScenarioMixed,
		Verbose:  true,
	}
}

// Result contains demo output.
type Result struct {
	// Banner is the headline output
	Banner string

	// NeedsYou is the computed state
	NeedsYou bool

	// NeedsYouReasons explains why NeedsYou is true
	NeedsYouReasons []string

	// CircleSummaries shows per-circle stats
	CircleSummaries []CircleSummary

	// Obligations are all extracted obligations (sorted by priority)
	Obligations []ObligationSummary

	// Hash is the deterministic hash of the view
	Hash string
}

// CircleSummary summarizes a circle's obligations.
type CircleSummary struct {
	CircleID         string
	CircleName       string
	TotalObligations int
	HighRegretCount  int
	TodayCount       int
	TopReasons       []string
}

// ObligationSummary summarizes a single obligation.
type ObligationSummary struct {
	ID          string
	CircleID    string
	Type        string
	Reason      string
	Horizon     string
	Severity    string
	RegretScore float64
	DueBy       string
}

// Run executes the demo.
func Run(config DemoConfig) (*Result, error) {
	now := config.Clock.Now()

	// Create event store and populate with mock data
	eventStore := events.NewInMemoryEventStore()
	circleIDs := populateMockEvents(eventStore, config.Scenario, now)

	// Create extraction engine with mock identity repo
	engineConfig := obligations.DefaultConfig()
	engine := obligations.NewEngine(engineConfig, config.Clock, &mockIdentityRepo{})

	// Extract obligations
	extractResult := engine.Extract(eventStore, circleIDs)

	// Build DailyView
	viewConfig := view.DefaultNeedsYouConfig()
	builder := view.NewDailyViewBuilder(now, viewConfig)

	// Add circles
	circleNames := map[identity.EntityID]string{
		"circle-work":    "Work",
		"circle-family":  "Family",
		"circle-finance": "Finance",
	}
	for _, circleID := range circleIDs {
		builder.AddCircle(circleID, circleNames[circleID])
	}

	// Set obligations
	builder.SetObligations(extractResult.Obligations)

	// Build the view
	dailyView := builder.Build()

	// Build result
	result := &Result{
		Banner:          generateBanner(dailyView, now),
		NeedsYou:        dailyView.NeedsYou,
		NeedsYouReasons: dailyView.NeedsYouReasons,
		Hash:            dailyView.Hash,
	}

	// Build circle summaries
	for _, circleID := range circleIDs {
		summary := dailyView.GetCircleSummary(circleID)
		if summary == nil {
			continue
		}
		result.CircleSummaries = append(result.CircleSummaries, CircleSummary{
			CircleID:         string(circleID),
			CircleName:       summary.CircleName,
			TotalObligations: summary.ObligationCount,
			HighRegretCount:  summary.HighRegretCount,
			TodayCount:       summary.TodayHorizonCount,
			TopReasons:       summary.TopReasons,
		})
	}

	// Build obligation summaries
	for _, oblig := range dailyView.Obligations {
		dueStr := ""
		if oblig.DueBy != nil {
			dueStr = oblig.DueBy.Format("2006-01-02 15:04")
		}
		result.Obligations = append(result.Obligations, ObligationSummary{
			ID:          oblig.ID,
			CircleID:    string(oblig.CircleID),
			Type:        string(oblig.Type),
			Reason:      oblig.Reason,
			Horizon:     string(oblig.Horizon),
			Severity:    string(oblig.Severity),
			RegretScore: oblig.RegretScore,
			DueBy:       dueStr,
		})
	}

	// Print if verbose
	if config.Verbose {
		printResult(result)
	}

	return result, nil
}

// generateBanner creates the headline banner.
func generateBanner(dv *view.DailyView, now time.Time) string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString("================================================================================\n")
	sb.WriteString("             QUANTUMLIFE PHASE 2: OBLIGATION EXTRACTION DEMO\n")
	sb.WriteString("================================================================================\n\n")

	sb.WriteString(fmt.Sprintf("  Date: %s\n", now.Format("Monday, 2 January 2006")))
	sb.WriteString(fmt.Sprintf("  Time: %s\n", now.Format("15:04 MST")))
	sb.WriteString("\n")

	if dv.NothingNeedsYou() {
		sb.WriteString("  ╔═══════════════════════════════════════════════════════════════════════════╗\n")
		sb.WriteString("  ║                                                                           ║\n")
		sb.WriteString("  ║                        NOTHING NEEDS YOU                                  ║\n")
		sb.WriteString("  ║                                                                           ║\n")
		sb.WriteString("  ║        All obligations are within normal thresholds.                      ║\n")
		sb.WriteString("  ║        Enjoy your day with peace of mind.                                 ║\n")
		sb.WriteString("  ║                                                                           ║\n")
		sb.WriteString("  ╚═══════════════════════════════════════════════════════════════════════════╝\n")
	} else {
		sb.WriteString("  ╔═══════════════════════════════════════════════════════════════════════════╗\n")
		sb.WriteString("  ║                                                                           ║\n")
		sb.WriteString("  ║                           NEEDS YOU                                       ║\n")
		sb.WriteString("  ║                                                                           ║\n")
		for i, reason := range dv.NeedsYouReasons {
			if i >= 3 {
				break
			}
			// Truncate reason to fit in box
			r := reason
			if len(r) > 65 {
				r = r[:62] + "..."
			}
			line := fmt.Sprintf("  ║  %d. %-68s ║\n", i+1, r)
			sb.WriteString(line)
		}
		sb.WriteString("  ║                                                                           ║\n")
		sb.WriteString("  ╚═══════════════════════════════════════════════════════════════════════════╝\n")
	}

	sb.WriteString("\n")
	sb.WriteString("  This is READ-ONLY. No actions taken. No notifications sent.\n")
	sb.WriteString("  View hash: " + dv.Hash[:16] + "...\n")
	sb.WriteString("\n")
	sb.WriteString("================================================================================\n")

	return sb.String()
}

// printResult prints the full result to stdout.
func printResult(r *Result) {
	fmt.Print(r.Banner)

	// Circle summaries
	fmt.Println("\n=== Circle Summaries ===")
	for _, c := range r.CircleSummaries {
		fmt.Printf("\n  [%s] %s\n", c.CircleID, c.CircleName)
		fmt.Printf("    Total: %d | High Regret: %d | Due Today: %d\n",
			c.TotalObligations, c.HighRegretCount, c.TodayCount)
		if len(c.TopReasons) > 0 {
			fmt.Println("    Top reasons:")
			for _, reason := range c.TopReasons {
				fmt.Printf("      - %s\n", truncate(reason, 60))
			}
		}
	}

	// Obligations list
	fmt.Println("\n=== Obligations (sorted by priority) ===")
	if len(r.Obligations) == 0 {
		fmt.Println("  No obligations extracted.")
	}
	for i, o := range r.Obligations {
		fmt.Printf("\n  %d. [%s] %s\n", i+1, o.Type, o.Reason)
		fmt.Printf("     ID: %s | Circle: %s\n", o.ID[:12]+"...", o.CircleID)
		fmt.Printf("     Horizon: %s | Severity: %s | Regret: %.2f\n",
			o.Horizon, o.Severity, o.RegretScore)
		if o.DueBy != "" {
			fmt.Printf("     Due: %s\n", o.DueBy)
		}
	}

	fmt.Println("\n================================================================================")
	fmt.Printf("  Determinism check: View hash = %s\n", r.Hash[:24]+"...")
	fmt.Println("================================================================================")
}

// populateMockEvents creates mock events based on scenario.
func populateMockEvents(store *events.InMemoryEventStore, scenario Scenario, now time.Time) []identity.EntityID {
	circleWork := identity.EntityID("circle-work")
	circleFamily := identity.EntityID("circle-family")
	circleFinance := identity.EntityID("circle-finance")

	switch scenario {
	case ScenarioNeedsYou:
		populateNeedsYouEvents(store, now, circleWork, circleFamily, circleFinance)
	case ScenarioNothingNeedsYou:
		populateNothingNeedsYouEvents(store, now, circleWork, circleFamily, circleFinance)
	default: // ScenarioMixed
		populateMixedEvents(store, now, circleWork, circleFamily, circleFinance)
	}

	return []identity.EntityID{circleWork, circleFamily, circleFinance}
}

// populateNeedsYouEvents creates high-regret scenarios.
func populateNeedsYouEvents(store *events.InMemoryEventStore, now time.Time, work, family, finance identity.EntityID) {
	// Work: Urgent email from boss
	urgentEmail := events.NewEmailMessageEvent("gmail", "msg-001", "user@work.com", now, now.Add(-2*time.Hour))
	urgentEmail.Circle = work
	urgentEmail.Subject = "URGENT: Action required by EOD"
	urgentEmail.BodyPreview = "Please review and approve the budget proposal. This needs to be done today."
	urgentEmail.From = events.EmailAddress{Address: "boss@company.com", Name: "Jane Boss"}
	urgentEmail.IsRead = false
	urgentEmail.IsImportant = true
	urgentEmail.SenderDomain = "company.com"
	store.Store(urgentEmail)

	// Work: Unresponded calendar invite
	meetingInvite := events.NewCalendarEventEvent("google", "cal-work", "evt-001", "user@work.com", now, now)
	meetingInvite.Circle = work
	meetingInvite.Title = "Quarterly Review Meeting"
	meetingInvite.StartTime = now.Add(4 * time.Hour)
	meetingInvite.EndTime = now.Add(5 * time.Hour)
	meetingInvite.MyResponseStatus = events.RSVPNeedsAction
	meetingInvite.AttendeeCount = 10
	store.Store(meetingInvite)

	// Family: School event needing decision
	schoolEvent := events.NewCalendarEventEvent("google", "cal-family", "evt-002", "user@family.com", now, now)
	schoolEvent.Circle = family
	schoolEvent.Title = "Parent-Teacher Conference"
	schoolEvent.StartTime = now.Add(6 * time.Hour)
	schoolEvent.EndTime = now.Add(7 * time.Hour)
	schoolEvent.MyResponseStatus = events.RSVPNeedsAction
	store.Store(schoolEvent)

	// Finance: Low balance alert
	lowBalance := events.NewBalanceEvent("truelayer", "acc-001", now, now)
	lowBalance.Circle = finance
	lowBalance.AccountType = "CHECKING"
	lowBalance.Institution = "Barclays"
	lowBalance.AvailableMinor = 25000 // £250 - below threshold
	lowBalance.CurrentMinor = 27500
	lowBalance.Currency = "GBP"
	store.Store(lowBalance)
}

// populateNothingNeedsYouEvents creates only low-priority items.
func populateNothingNeedsYouEvents(store *events.InMemoryEventStore, now time.Time, work, family, finance identity.EntityID) {
	// Work: Already read email (no obligation)
	readEmail := events.NewEmailMessageEvent("gmail", "msg-010", "user@work.com", now, now.Add(-24*time.Hour))
	readEmail.Circle = work
	readEmail.Subject = "Weekly Newsletter"
	readEmail.BodyPreview = "This week's updates from the team..."
	readEmail.From = events.EmailAddress{Address: "newsletter@company.com"}
	readEmail.IsRead = true
	readEmail.SenderDomain = "company.com"
	store.Store(readEmail)

	// Work: Accepted meeting (low priority)
	acceptedMeeting := events.NewCalendarEventEvent("google", "cal-work", "evt-010", "user@work.com", now, now)
	acceptedMeeting.Circle = work
	acceptedMeeting.Title = "Weekly Team Sync"
	acceptedMeeting.StartTime = now.Add(48 * time.Hour) // Far future
	acceptedMeeting.EndTime = now.Add(49 * time.Hour)
	acceptedMeeting.MyResponseStatus = events.RSVPAccepted
	store.Store(acceptedMeeting)

	// Finance: Healthy balance
	healthyBalance := events.NewBalanceEvent("truelayer", "acc-010", now, now)
	healthyBalance.Circle = finance
	healthyBalance.AccountType = "CHECKING"
	healthyBalance.Institution = "Barclays"
	healthyBalance.AvailableMinor = 250000 // £2500 - well above threshold
	healthyBalance.CurrentMinor = 260000
	healthyBalance.Currency = "GBP"
	store.Store(healthyBalance)

	// Family: Automated email (no obligation)
	automatedEmail := events.NewEmailMessageEvent("gmail", "msg-011", "user@family.com", now, now.Add(-1*time.Hour))
	automatedEmail.Circle = family
	automatedEmail.Subject = "Your order has shipped"
	automatedEmail.BodyPreview = "Your Amazon order is on its way..."
	automatedEmail.From = events.EmailAddress{Address: "ship-confirm@amazon.co.uk"}
	automatedEmail.IsRead = false
	automatedEmail.IsAutomated = true
	automatedEmail.SenderDomain = "amazon.co.uk"
	store.Store(automatedEmail)
}

// populateMixedEvents creates a realistic mix of obligations.
func populateMixedEvents(store *events.InMemoryEventStore, now time.Time, work, family, finance identity.EntityID) {
	// === WORK CIRCLE ===

	// High priority: Unread important email
	importantEmail := events.NewEmailMessageEvent("gmail", "msg-100", "user@work.com", now, now.Add(-3*time.Hour))
	importantEmail.Circle = work
	importantEmail.Subject = "Approval needed: Q1 Budget Review"
	importantEmail.BodyPreview = "Please review and approve the attached budget by Friday."
	importantEmail.From = events.EmailAddress{Address: "cfo@company.com", Name: "Sarah CFO"}
	importantEmail.IsRead = false
	importantEmail.IsImportant = true
	importantEmail.SenderDomain = "company.com"
	store.Store(importantEmail)

	// Medium priority: Unread but not urgent
	normalEmail := events.NewEmailMessageEvent("gmail", "msg-101", "user@work.com", now, now.Add(-1*time.Hour))
	normalEmail.Circle = work
	normalEmail.Subject = "Team lunch next week?"
	normalEmail.BodyPreview = "Hey, want to grab lunch next Thursday?"
	normalEmail.From = events.EmailAddress{Address: "colleague@company.com", Name: "Mike"}
	normalEmail.IsRead = false
	normalEmail.SenderDomain = "company.com"
	store.Store(normalEmail)

	// Calendar: Upcoming meeting (accepted)
	upcomingMeeting := events.NewCalendarEventEvent("google", "cal-work", "evt-100", "user@work.com", now, now)
	upcomingMeeting.Circle = work
	upcomingMeeting.Title = "Design Review"
	upcomingMeeting.StartTime = now.Add(2 * time.Hour)
	upcomingMeeting.EndTime = now.Add(3 * time.Hour)
	upcomingMeeting.MyResponseStatus = events.RSVPAccepted
	store.Store(upcomingMeeting)

	// === FAMILY CIRCLE ===

	// Invoice email
	invoiceEmail := events.NewEmailMessageEvent("gmail", "msg-200", "user@family.com", now, now.Add(-2*time.Hour))
	invoiceEmail.Circle = family
	invoiceEmail.Subject = "Invoice #4521 - Payment due: 20 Jan"
	invoiceEmail.BodyPreview = "Your invoice is attached. Payment due by 20 January 2025."
	invoiceEmail.From = events.EmailAddress{Address: "billing@utility.co.uk"}
	invoiceEmail.IsRead = false
	invoiceEmail.IsTransactional = true
	invoiceEmail.SenderDomain = "utility.co.uk"
	store.Store(invoiceEmail)

	// Calendar conflict
	familyEvent1 := events.NewCalendarEventEvent("google", "cal-family", "evt-200", "user@family.com", now, now)
	familyEvent1.Circle = family
	familyEvent1.Title = "Kids Football Practice"
	familyEvent1.StartTime = now.Add(26 * time.Hour)
	familyEvent1.EndTime = now.Add(27 * time.Hour)
	familyEvent1.MyResponseStatus = events.RSVPAccepted
	store.Store(familyEvent1)

	familyEvent2 := events.NewCalendarEventEvent("google", "cal-family", "evt-201", "user@family.com", now, now)
	familyEvent2.Circle = family
	familyEvent2.Title = "Dentist Appointment"
	familyEvent2.StartTime = now.Add(26*time.Hour + 30*time.Minute)
	familyEvent2.EndTime = now.Add(27*time.Hour + 30*time.Minute)
	familyEvent2.MyResponseStatus = events.RSVPAccepted
	store.Store(familyEvent2)

	// === FINANCE CIRCLE ===

	// Healthy balance (no alert)
	balance := events.NewBalanceEvent("truelayer", "acc-300", now, now)
	balance.Circle = finance
	balance.AccountType = "CHECKING"
	balance.Institution = "Barclays"
	balance.AvailableMinor = 150000 // £1500
	balance.CurrentMinor = 155000
	balance.Currency = "GBP"
	store.Store(balance)

	// Large recent transaction
	largeTx := events.NewTransactionEvent("truelayer", "acc-300", "tx-300", now, now.Add(-6*time.Hour))
	largeTx.Circle = finance
	largeTx.TransactionType = "DEBIT"
	largeTx.TransactionStatus = "POSTED"
	largeTx.AmountMinor = 75000 // £750
	largeTx.Currency = "GBP"
	largeTx.MerchantName = "John Lewis"
	largeTx.MerchantNameRaw = "JOHN LEWIS OXFORD ST"
	largeTx.TransactionDate = now.Add(-6 * time.Hour)
	store.Store(largeTx)

	// Pending transaction
	pendingTx := events.NewTransactionEvent("truelayer", "acc-300", "tx-301", now, now.Add(-1*time.Hour))
	pendingTx.Circle = finance
	pendingTx.TransactionType = "DEBIT"
	pendingTx.TransactionStatus = "PENDING"
	pendingTx.AmountMinor = 2500 // £25
	pendingTx.Currency = "GBP"
	pendingTx.MerchantName = "Tesco"
	pendingTx.MerchantNameRaw = "TESCO STORES 4521"
	pendingTx.TransactionDate = now.Add(-1 * time.Hour)
	store.Store(pendingTx)
}

// mockIdentityRepo implements IdentityRepository for demo purposes.
type mockIdentityRepo struct{}

func (m *mockIdentityRepo) GetByID(id identity.EntityID) (identity.Entity, error) {
	return nil, nil
}

func (m *mockIdentityRepo) IsHighPriority(id identity.EntityID) bool {
	return false
}

// truncate truncates a string to maxLen.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
