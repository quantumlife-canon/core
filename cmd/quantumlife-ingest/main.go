// Command quantumlife-ingest performs a single synchronous ingestion run.
//
// CRITICAL: This command runs once and exits. It does NOT spawn background
// processes or polling loops. For continuous ingestion, use a scheduler
// (e.g., cron) to invoke this command periodically.
//
// Usage:
//
//	go run ./cmd/quantumlife-ingest
//	# or
//	make ingest-once
//
// Reference: docs/TECHNICAL_ARCHITECTURE_V1.md
package main

import (
	"fmt"
	"os"
	"time"

	"quantumlife/internal/ingestion"
	"quantumlife/internal/integrations/finance_read"
	"quantumlife/internal/integrations/gcal_read"
	"quantumlife/internal/integrations/gmail_read"
	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/events"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/view"
)

func main() {
	fmt.Println("QuantumLife Ingestion Runner")
	fmt.Println("============================")
	fmt.Println()

	// Use real clock at entry point
	clk := clock.NewReal()

	// Create stores
	eventStore := events.NewInMemoryEventStore()
	viewStore := view.NewInMemoryViewStore()
	identityRepo := identity.NewInMemoryRepository()

	// Create identity generator
	idGen := identity.NewGenerator()

	// Create owner (Satish)
	owner := idGen.PersonFromEmail("satish@example.com", clk.Now())
	if err := identityRepo.Store(owner); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to store owner: %v\n", err)
		os.Exit(1)
	}

	// Create circles for Satish
	workCircle := idGen.CircleFromName(owner.ID(), "Work", clk.Now())
	familyCircle := idGen.CircleFromName(owner.ID(), "Family", clk.Now())
	financeCircle := idGen.CircleFromName(owner.ID(), "Finance", clk.Now())

	if err := identityRepo.Store(workCircle); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to store work circle: %v\n", err)
	}
	if err := identityRepo.Store(familyCircle); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to store family circle: %v\n", err)
	}
	if err := identityRepo.Store(financeCircle); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to store finance circle: %v\n", err)
	}

	// Create mock adapters with sample data
	gmailAdapter := createMockGmailAdapter(clk, workCircle.ID(), familyCircle.ID())
	gcalAdapter := createMockCalendarAdapter(clk, workCircle.ID(), familyCircle.ID())
	financeAdapter := createMockFinanceAdapter(clk, financeCircle.ID())

	// Create runner
	runner := ingestion.NewRunner(clk, eventStore, viewStore, identityRepo)
	runner.SetEmailAdapter(gmailAdapter)
	runner.SetCalendarAdapter(gcalAdapter)
	runner.SetFinanceAdapter(financeAdapter)

	// Build config
	config := &ingestion.Config{
		OwnerID:   owner.ID(),
		OwnerName: "Satish",
		EmailAccounts: []ingestion.EmailAccountConfig{
			{Email: "satish.work@company.com", Provider: "gmail", CircleID: workCircle.ID()},
			{Email: "satish.personal@gmail.com", Provider: "gmail", CircleID: familyCircle.ID()},
		},
		CalendarAccounts: []ingestion.CalendarAccountConfig{
			{CalendarID: "work-calendar", CalendarName: "Work", AccountEmail: "satish.work@company.com", Provider: "google", CircleID: workCircle.ID()},
			{CalendarID: "family-calendar", CalendarName: "Family", AccountEmail: "satish.personal@gmail.com", Provider: "google", CircleID: familyCircle.ID()},
		},
		FinanceAccounts: []ingestion.FinanceAccountConfig{
			{AccountID: "barclays-checking", Institution: "Barclays", MaskedNumber: "****1234", Currency: "GBP", CircleID: financeCircle.ID()},
		},
		Circles: []ingestion.CircleConfig{
			{ID: workCircle.ID(), Name: "Work"},
			{ID: familyCircle.ID(), Name: "Family"},
			{ID: financeCircle.ID(), Name: "Finance"},
		},
	}

	// Run ingestion
	fmt.Println("Running ingestion...")
	result, err := runner.Run(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ingestion failed: %v\n", err)
		os.Exit(1)
	}

	// Print report
	printReport(result)
}

func createMockGmailAdapter(clk clock.Clock, workCircleID, familyCircleID identity.EntityID) *gmail_read.MockAdapter {
	adapter := gmail_read.NewMockAdapter(clk)
	now := clk.Now()

	// Work emails
	adapter.AddMockMessage(&gmail_read.MockMessage{
		MessageID:    "msg-work-1",
		AccountEmail: "satish.work@company.com",
		From:         events.EmailAddress{Address: "manager@company.com", Name: "Manager"},
		To:           []events.EmailAddress{{Address: "satish.work@company.com", Name: "Satish"}},
		Subject:      "Q1 Planning Meeting",
		BodyPreview:  "Hi Satish, Let's discuss the Q1 roadmap...",
		SentAt:       now.Add(-2 * time.Hour),
		IsRead:       false,
		IsImportant:  true,
		CircleID:     workCircleID,
	})

	adapter.AddMockMessage(&gmail_read.MockMessage{
		MessageID:    "msg-work-2",
		AccountEmail: "satish.work@company.com",
		From:         events.EmailAddress{Address: "hr@company.com", Name: "HR"},
		To:           []events.EmailAddress{{Address: "satish.work@company.com", Name: "Satish"}},
		Subject:      "Benefits Update",
		BodyPreview:  "Annual benefits enrollment is now open...",
		SentAt:       now.Add(-24 * time.Hour),
		IsRead:       true,
		IsImportant:  false,
		CircleID:     workCircleID,
	})

	// Personal emails
	adapter.AddMockMessage(&gmail_read.MockMessage{
		MessageID:       "msg-personal-1",
		AccountEmail:    "satish.personal@gmail.com",
		From:            events.EmailAddress{Address: "school@example.edu", Name: "School"},
		To:              []events.EmailAddress{{Address: "satish.personal@gmail.com", Name: "Satish"}},
		Subject:         "Parent-Teacher Meeting Reminder",
		BodyPreview:     "Reminder: Parent-teacher meeting is scheduled for...",
		SentAt:          now.Add(-6 * time.Hour),
		IsRead:          false,
		IsImportant:     true,
		IsTransactional: true,
		CircleID:        familyCircleID,
	})

	return adapter
}

func createMockCalendarAdapter(clk clock.Clock, workCircleID, familyCircleID identity.EntityID) *gcal_read.MockAdapter {
	adapter := gcal_read.NewMockAdapter(clk)
	now := clk.Now()

	// Work events
	adapter.AddMockEvent(&gcal_read.MockCalendarEvent{
		CalendarID:   "work-calendar",
		CalendarName: "Work",
		AccountEmail: "satish.work@company.com",
		EventUID:     "event-work-1",
		Title:        "Team Standup",
		StartTime:    now.Add(2 * time.Hour),
		EndTime:      now.Add(2*time.Hour + 30*time.Minute),
		Timezone:     "Europe/London",
		IsBusy:       true,
		CircleID:     workCircleID,
	})

	adapter.AddMockEvent(&gcal_read.MockCalendarEvent{
		CalendarID:   "work-calendar",
		CalendarName: "Work",
		AccountEmail: "satish.work@company.com",
		EventUID:     "event-work-2",
		Title:        "1:1 with Manager",
		StartTime:    now.AddDate(0, 0, 2),
		EndTime:      now.AddDate(0, 0, 2).Add(1 * time.Hour),
		Timezone:     "Europe/London",
		IsBusy:       true,
		CircleID:     workCircleID,
	})

	// Family events
	adapter.AddMockEvent(&gcal_read.MockCalendarEvent{
		CalendarID:   "family-calendar",
		CalendarName: "Family",
		AccountEmail: "satish.personal@gmail.com",
		EventUID:     "event-family-1",
		Title:        "School Pickup",
		StartTime:    now.Add(5 * time.Hour),
		EndTime:      now.Add(5*time.Hour + 30*time.Minute),
		Timezone:     "Europe/London",
		IsBusy:       true,
		CircleID:     familyCircleID,
	})

	return adapter
}

func createMockFinanceAdapter(clk clock.Clock, financeCircleID identity.EntityID) *finance_read.MockAdapter {
	adapter := finance_read.NewMockAdapter(clk)
	now := clk.Now()

	// Set balance
	adapter.SetMockBalance(&finance_read.MockBalance{
		AccountID:      "barclays-checking",
		AccountType:    "CHECKING",
		Institution:    "Barclays",
		MaskedNumber:   "****1234",
		CurrentMinor:   250000, // £2,500.00
		AvailableMinor: 245000, // £2,450.00
		Currency:       "GBP",
		AsOf:           now,
		CircleID:       financeCircleID,
	})

	// Add transactions
	adapter.AddMockTransaction(&finance_read.MockTransaction{
		AccountID:         "barclays-checking",
		TransactionID:     "tx-1",
		Institution:       "Barclays",
		MaskedNumber:      "****1234",
		TransactionType:   "DEBIT",
		TransactionKind:   "PURCHASE",
		TransactionStatus: "POSTED",
		AmountMinor:       4599, // £45.99
		Currency:          "GBP",
		MerchantName:      "Tesco",
		MerchantNameRaw:   "TESCO STORES 1234",
		MerchantCategory:  "GROCERIES",
		TransactionDate:   now.Add(-24 * time.Hour),
		CircleID:          financeCircleID,
	})

	adapter.AddMockTransaction(&finance_read.MockTransaction{
		AccountID:         "barclays-checking",
		TransactionID:     "tx-2",
		Institution:       "Barclays",
		MaskedNumber:      "****1234",
		TransactionType:   "DEBIT",
		TransactionKind:   "PURCHASE",
		TransactionStatus: "PENDING",
		AmountMinor:       1250, // £12.50
		Currency:          "GBP",
		MerchantName:      "Costa",
		MerchantNameRaw:   "COSTA COFFEE",
		MerchantCategory:  "DINING",
		TransactionDate:   now.Add(-2 * time.Hour),
		CircleID:          financeCircleID,
	})

	return adapter
}

func printReport(result *ingestion.RunResult) {
	fmt.Println()
	fmt.Println("Ingestion Complete")
	fmt.Println("==================")
	fmt.Printf("Duration: %v\n", result.Duration)
	fmt.Println()

	fmt.Println("Events Ingested:")
	fmt.Printf("  Email:    %d\n", result.EmailEventsIngested)
	fmt.Printf("  Calendar: %d\n", result.CalendarEventsIngested)
	fmt.Printf("  Finance:  %d\n", result.FinanceEventsIngested)
	fmt.Printf("  Total:    %d\n", result.TotalEventsIngested)
	fmt.Println()

	fmt.Printf("View Snapshots Created: %d\n", result.ViewSnapshotsCreated)
	fmt.Println()

	fmt.Println("Circle Summary:")
	fmt.Println("---------------")
	for _, cr := range result.CircleResults {
		fmt.Printf("\n[%s]\n", cr.CircleName)
		fmt.Printf("  Unread Emails:      %d\n", cr.UnreadEmails)
		fmt.Printf("  Important Emails:   %d\n", cr.ImportantEmails)
		fmt.Printf("  Upcoming Events:    %d\n", cr.UpcomingEvents)
		fmt.Printf("  Today's Events:     %d\n", cr.TodayEvents)
		fmt.Printf("  Pending Tx:         %d\n", cr.PendingTx)
		fmt.Printf("  New Transactions:   %d\n", cr.NewTx)
		if cr.TotalBalanceMinor > 0 {
			fmt.Printf("  Balance:            %s %.2f\n", cr.BalanceCurrency, float64(cr.TotalBalanceMinor)/100)
		}
		fmt.Printf("  View Hash:          %s...\n", cr.ViewHash[:16])
		if cr.NothingNeedsYou {
			fmt.Printf("  Status:             Nothing Needs You ✓\n")
		} else {
			fmt.Printf("  Status:             Items need attention\n")
		}
	}

	if len(result.Warnings) > 0 {
		fmt.Println()
		fmt.Println("Warnings:")
		for _, w := range result.Warnings {
			fmt.Printf("  - %s\n", w)
		}
	}

	fmt.Println()
	fmt.Println("Done.")
}
