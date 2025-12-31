// Package demo_readonly_mirror tests the read-only ingestion and view snapshot system.
//
// This demo verifies:
// - Ingestion from mock adapters (Gmail, Calendar, Finance)
// - Deterministic view snapshot hashes
// - Per-circle counts and "Nothing Needs You" status
// - No writes to external systems (read-only only)
//
// GUARDRAIL: This package does NOT spawn goroutines. All operations are synchronous.
//
// Reference: docs/QUANTUMLIFE_CONSTITUTION_V1.md
package demo_readonly_mirror

import (
	"fmt"
	"testing"
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

// TestReadOnlyMirror_SatishConfig tests ingestion with a realistic Satish configuration.
//
// Configuration:
// - 20 emails (15 work, 5 personal)
// - 5 calendar events (3 work, 2 family)
// - 3 financial transactions + 1 balance
// - 3 circles: Work, Family, Finance
func TestReadOnlyMirror_SatishConfig(t *testing.T) {
	// Use fixed clock for deterministic hashes
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)

	// Create stores
	eventStore := events.NewInMemoryEventStore()
	viewStore := view.NewInMemoryViewStore()
	identityRepo := identity.NewInMemoryRepository()

	// Create identity generator
	idGen := identity.NewGenerator()

	// Create owner (Satish)
	owner := idGen.PersonFromEmail("satish@example.com", fixedTime)
	if err := identityRepo.Store(owner); err != nil {
		t.Fatalf("Failed to store owner: %v", err)
	}

	// Create circles
	workCircle := idGen.CircleFromName(owner.ID(), "Work", fixedTime)
	familyCircle := idGen.CircleFromName(owner.ID(), "Family", fixedTime)
	financeCircle := idGen.CircleFromName(owner.ID(), "Finance", fixedTime)

	for _, circle := range []*identity.Circle{workCircle, familyCircle, financeCircle} {
		if err := identityRepo.Store(circle); err != nil {
			t.Fatalf("Failed to store circle %s: %v", circle.Name, err)
		}
	}

	// Create mock adapters
	gmailAdapter := createSatishGmailAdapter(clk, workCircle.ID(), familyCircle.ID())
	gcalAdapter := createSatishCalendarAdapter(clk, workCircle.ID(), familyCircle.ID())
	financeAdapter := createSatishFinanceAdapter(clk, financeCircle.ID())

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
			{CalendarID: "work-cal", CalendarName: "Work", AccountEmail: "satish.work@company.com", Provider: "google", CircleID: workCircle.ID()},
			{CalendarID: "family-cal", CalendarName: "Family", AccountEmail: "satish.personal@gmail.com", Provider: "google", CircleID: familyCircle.ID()},
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
	result, err := runner.Run(config)
	if err != nil {
		t.Fatalf("Ingestion failed: %v", err)
	}

	// Assert event counts
	t.Run("EventCounts", func(t *testing.T) {
		if result.EmailEventsIngested != 20 {
			t.Errorf("Expected 20 email events, got %d", result.EmailEventsIngested)
		}
		if result.CalendarEventsIngested != 5 {
			t.Errorf("Expected 5 calendar events, got %d", result.CalendarEventsIngested)
		}
		// 3 transactions + 1 balance = 4 finance events
		if result.FinanceEventsIngested != 4 {
			t.Errorf("Expected 4 finance events, got %d", result.FinanceEventsIngested)
		}
		if result.TotalEventsIngested != 29 {
			t.Errorf("Expected 29 total events, got %d", result.TotalEventsIngested)
		}
	})

	// Assert view snapshots
	t.Run("ViewSnapshots", func(t *testing.T) {
		if result.ViewSnapshotsCreated != 3 {
			t.Errorf("Expected 3 view snapshots, got %d", result.ViewSnapshotsCreated)
		}
	})

	// Assert per-circle results
	t.Run("WorkCircle", func(t *testing.T) {
		workResult := findCircleResult(result.CircleResults, "Work")
		if workResult == nil {
			t.Fatal("Work circle result not found")
		}

		// 15 work emails, 8 unread, 3 important
		if workResult.UnreadEmails != 8 {
			t.Errorf("Work: expected 8 unread emails, got %d", workResult.UnreadEmails)
		}
		if workResult.ImportantEmails != 3 {
			t.Errorf("Work: expected 3 important emails, got %d", workResult.ImportantEmails)
		}

		// 3 work events, 1 today
		if workResult.UpcomingEvents != 3 {
			t.Errorf("Work: expected 3 upcoming events, got %d", workResult.UpcomingEvents)
		}
		if workResult.TodayEvents != 1 {
			t.Errorf("Work: expected 1 today event, got %d", workResult.TodayEvents)
		}

		// Work should need attention (has unread + today event)
		if workResult.NothingNeedsYou {
			t.Error("Work: should NOT be 'Nothing Needs You'")
		}

		// Verify hash is deterministic
		if workResult.ViewHash == "" {
			t.Error("Work: view hash should not be empty")
		}
	})

	t.Run("FamilyCircle", func(t *testing.T) {
		familyResult := findCircleResult(result.CircleResults, "Family")
		if familyResult == nil {
			t.Fatal("Family circle result not found")
		}

		// 5 personal emails, 2 unread, 1 important
		if familyResult.UnreadEmails != 2 {
			t.Errorf("Family: expected 2 unread emails, got %d", familyResult.UnreadEmails)
		}
		if familyResult.ImportantEmails != 1 {
			t.Errorf("Family: expected 1 important email, got %d", familyResult.ImportantEmails)
		}

		// 2 family events, 1 today
		if familyResult.UpcomingEvents != 2 {
			t.Errorf("Family: expected 2 upcoming events, got %d", familyResult.UpcomingEvents)
		}
	})

	t.Run("FinanceCircle", func(t *testing.T) {
		financeResult := findCircleResult(result.CircleResults, "Finance")
		if financeResult == nil {
			t.Fatal("Finance circle result not found")
		}

		// 3 transactions, 1 pending
		if financeResult.PendingTx != 1 {
			t.Errorf("Finance: expected 1 pending tx, got %d", financeResult.PendingTx)
		}
		if financeResult.NewTx != 3 {
			t.Errorf("Finance: expected 3 new tx, got %d", financeResult.NewTx)
		}

		// Balance: £2,500.00
		if financeResult.TotalBalanceMinor != 250000 {
			t.Errorf("Finance: expected balance 250000, got %d", financeResult.TotalBalanceMinor)
		}
		if financeResult.BalanceCurrency != "GBP" {
			t.Errorf("Finance: expected currency GBP, got %s", financeResult.BalanceCurrency)
		}
	})

	// Test deterministic hashes - running again with same data should produce same hashes
	t.Run("DeterministicHashes", func(t *testing.T) {
		// Create fresh stores
		eventStore2 := events.NewInMemoryEventStore()
		viewStore2 := view.NewInMemoryViewStore()

		// Same clock, same time
		clk2 := clock.NewFixed(fixedTime)

		runner2 := ingestion.NewRunner(clk2, eventStore2, viewStore2, identityRepo)
		runner2.SetEmailAdapter(createSatishGmailAdapter(clk2, workCircle.ID(), familyCircle.ID()))
		runner2.SetCalendarAdapter(createSatishCalendarAdapter(clk2, workCircle.ID(), familyCircle.ID()))
		runner2.SetFinanceAdapter(createSatishFinanceAdapter(clk2, financeCircle.ID()))

		result2, err := runner2.Run(config)
		if err != nil {
			t.Fatalf("Second ingestion failed: %v", err)
		}

		// Compare hashes
		for i, cr := range result.CircleResults {
			if cr.ViewHash != result2.CircleResults[i].ViewHash {
				t.Errorf("Circle %s: hash mismatch\n  run1: %s\n  run2: %s",
					cr.CircleName, cr.ViewHash, result2.CircleResults[i].ViewHash)
			}
		}
	})
}

// TestReadOnlyMirror_NothingNeedsYou tests the "Nothing Needs You" state.
func TestReadOnlyMirror_NothingNeedsYou(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)

	eventStore := events.NewInMemoryEventStore()
	viewStore := view.NewInMemoryViewStore()
	identityRepo := identity.NewInMemoryRepository()

	idGen := identity.NewGenerator()
	owner := idGen.PersonFromEmail("zen@example.com", fixedTime)
	if err := identityRepo.Store(owner); err != nil {
		t.Fatalf("Failed to store owner: %v", err)
	}

	zenCircle := idGen.CircleFromName(owner.ID(), "Zen", fixedTime)
	if err := identityRepo.Store(zenCircle); err != nil {
		t.Fatalf("Failed to store circle: %v", err)
	}

	// Create adapter with all read emails (nothing needing action)
	gmailAdapter := gmail_read.NewMockAdapter(clk)

	// Add 5 emails - all read, none important
	for i := 0; i < 5; i++ {
		gmailAdapter.AddMockMessage(&gmail_read.MockMessage{
			MessageID:    fmt.Sprintf("zen-msg-%d", i),
			AccountEmail: "zen@example.com",
			From:         events.EmailAddress{Address: "friend@example.com"},
			To:           []events.EmailAddress{{Address: "zen@example.com"}},
			Subject:      "Friendly chat",
			SentAt:       fixedTime.Add(-time.Duration(i) * time.Hour),
			IsRead:       true, // All read
			IsImportant:  false,
			CircleID:     zenCircle.ID(),
		})
	}

	runner := ingestion.NewRunner(clk, eventStore, viewStore, identityRepo)
	runner.SetEmailAdapter(gmailAdapter)

	config := &ingestion.Config{
		OwnerID:   owner.ID(),
		OwnerName: "Zen Master",
		EmailAccounts: []ingestion.EmailAccountConfig{
			{Email: "zen@example.com", Provider: "gmail", CircleID: zenCircle.ID()},
		},
		Circles: []ingestion.CircleConfig{
			{ID: zenCircle.ID(), Name: "Zen"},
		},
	}

	result, err := runner.Run(config)
	if err != nil {
		t.Fatalf("Ingestion failed: %v", err)
	}

	zenResult := findCircleResult(result.CircleResults, "Zen")
	if zenResult == nil {
		t.Fatal("Zen circle result not found")
	}

	// All emails read, no events, no finance - should be "Nothing Needs You"
	if !zenResult.NothingNeedsYou {
		t.Errorf("Zen circle should be 'Nothing Needs You' but has:\n"+
			"  unread: %d, important: %d, today: %d, pending: %d",
			zenResult.UnreadEmails, zenResult.ImportantEmails,
			zenResult.TodayEvents, zenResult.PendingTx)
	}
}

// TestReadOnlyMirror_EmptyCircle tests behavior with no data.
func TestReadOnlyMirror_EmptyCircle(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)

	eventStore := events.NewInMemoryEventStore()
	viewStore := view.NewInMemoryViewStore()
	identityRepo := identity.NewInMemoryRepository()

	idGen := identity.NewGenerator()
	owner := idGen.PersonFromEmail("empty@example.com", fixedTime)
	if err := identityRepo.Store(owner); err != nil {
		t.Fatalf("Failed to store owner: %v", err)
	}

	emptyCircle := idGen.CircleFromName(owner.ID(), "Empty", fixedTime)
	if err := identityRepo.Store(emptyCircle); err != nil {
		t.Fatalf("Failed to store circle: %v", err)
	}

	runner := ingestion.NewRunner(clk, eventStore, viewStore, identityRepo)

	config := &ingestion.Config{
		OwnerID:   owner.ID(),
		OwnerName: "Empty User",
		Circles: []ingestion.CircleConfig{
			{ID: emptyCircle.ID(), Name: "Empty"},
		},
	}

	result, err := runner.Run(config)
	if err != nil {
		t.Fatalf("Ingestion failed: %v", err)
	}

	emptyResult := findCircleResult(result.CircleResults, "Empty")
	if emptyResult == nil {
		t.Fatal("Empty circle result not found")
	}

	// Empty circle should be "Nothing Needs You"
	if !emptyResult.NothingNeedsYou {
		t.Error("Empty circle should be 'Nothing Needs You'")
	}

	// All counts should be zero
	if emptyResult.UnreadEmails != 0 || emptyResult.ImportantEmails != 0 ||
		emptyResult.UpcomingEvents != 0 || emptyResult.TodayEvents != 0 ||
		emptyResult.PendingTx != 0 || emptyResult.NewTx != 0 {
		t.Error("Empty circle should have all zero counts")
	}
}

// Helper functions

func findCircleResult(results []ingestion.CircleRunResult, name string) *ingestion.CircleRunResult {
	for i := range results {
		if results[i].CircleName == name {
			return &results[i]
		}
	}
	return nil
}

func createSatishGmailAdapter(clk clock.Clock, workCircleID, familyCircleID identity.EntityID) *gmail_read.MockAdapter {
	adapter := gmail_read.NewMockAdapter(clk)
	now := clk.Now()

	// 15 work emails (8 unread, 3 important)
	workEmails := []struct {
		id        string
		from      string
		subject   string
		isRead    bool
		important bool
		hoursAgo  int
	}{
		{"work-1", "manager@company.com", "Q1 Planning", false, true, 2},
		{"work-2", "hr@company.com", "Benefits Update", true, false, 24},
		{"work-3", "team@company.com", "Sprint Review", false, false, 4},
		{"work-4", "ceo@company.com", "Company Update", false, true, 8},
		{"work-5", "it@company.com", "Password Reset", true, false, 48},
		{"work-6", "recruiting@company.com", "Interview Schedule", false, false, 12},
		{"work-7", "manager@company.com", "1:1 Notes", true, false, 72},
		{"work-8", "finance@company.com", "Expense Report", false, false, 6},
		{"work-9", "security@company.com", "Security Alert", false, true, 1},
		{"work-10", "team@company.com", "Code Review", true, false, 36},
		{"work-11", "ops@company.com", "Deployment Notice", true, false, 24},
		{"work-12", "manager@company.com", "Project Update", false, false, 18},
		{"work-13", "legal@company.com", "Policy Update", true, false, 96},
		{"work-14", "team@company.com", "Meeting Notes", false, false, 10},
		{"work-15", "support@company.com", "Customer Feedback", true, false, 48},
	}

	for _, e := range workEmails {
		adapter.AddMockMessage(&gmail_read.MockMessage{
			MessageID:    e.id,
			AccountEmail: "satish.work@company.com",
			From:         events.EmailAddress{Address: e.from},
			To:           []events.EmailAddress{{Address: "satish.work@company.com"}},
			Subject:      e.subject,
			SentAt:       now.Add(-time.Duration(e.hoursAgo) * time.Hour),
			IsRead:       e.isRead,
			IsImportant:  e.important,
			CircleID:     workCircleID,
		})
	}

	// 5 personal emails (2 unread, 1 important)
	personalEmails := []struct {
		id        string
		from      string
		subject   string
		isRead    bool
		important bool
		hoursAgo  int
	}{
		{"personal-1", "school@edu.org", "Parent Meeting", false, true, 6},
		{"personal-2", "amazon@email.com", "Order Shipped", true, false, 24},
		{"personal-3", "friend@gmail.com", "Weekend Plans", false, false, 12},
		{"personal-4", "bank@notify.com", "Statement Ready", true, false, 48},
		{"personal-5", "doctor@clinic.com", "Appointment Confirmed", true, false, 72},
	}

	for _, e := range personalEmails {
		adapter.AddMockMessage(&gmail_read.MockMessage{
			MessageID:    e.id,
			AccountEmail: "satish.personal@gmail.com",
			From:         events.EmailAddress{Address: e.from},
			To:           []events.EmailAddress{{Address: "satish.personal@gmail.com"}},
			Subject:      e.subject,
			SentAt:       now.Add(-time.Duration(e.hoursAgo) * time.Hour),
			IsRead:       e.isRead,
			IsImportant:  e.important,
			CircleID:     familyCircleID,
		})
	}

	return adapter
}

func createSatishCalendarAdapter(clk clock.Clock, workCircleID, familyCircleID identity.EntityID) *gcal_read.MockAdapter {
	adapter := gcal_read.NewMockAdapter(clk)
	now := clk.Now()

	// 3 work events (1 today)
	adapter.AddMockEvent(&gcal_read.MockCalendarEvent{
		CalendarID:   "work-cal",
		CalendarName: "Work",
		AccountEmail: "satish.work@company.com",
		EventUID:     "work-event-1",
		Title:        "Team Standup",
		StartTime:    now.Add(2 * time.Hour), // Today
		EndTime:      now.Add(2*time.Hour + 30*time.Minute),
		Timezone:     "Europe/London",
		IsBusy:       true,
		CircleID:     workCircleID,
	})

	adapter.AddMockEvent(&gcal_read.MockCalendarEvent{
		CalendarID:   "work-cal",
		CalendarName: "Work",
		AccountEmail: "satish.work@company.com",
		EventUID:     "work-event-2",
		Title:        "1:1 with Manager",
		StartTime:    now.AddDate(0, 0, 2), // In 2 days
		EndTime:      now.AddDate(0, 0, 2).Add(1 * time.Hour),
		Timezone:     "Europe/London",
		IsBusy:       true,
		CircleID:     workCircleID,
	})

	adapter.AddMockEvent(&gcal_read.MockCalendarEvent{
		CalendarID:   "work-cal",
		CalendarName: "Work",
		AccountEmail: "satish.work@company.com",
		EventUID:     "work-event-3",
		Title:        "All Hands",
		StartTime:    now.AddDate(0, 0, 5), // In 5 days
		EndTime:      now.AddDate(0, 0, 5).Add(2 * time.Hour),
		Timezone:     "Europe/London",
		IsBusy:       true,
		CircleID:     workCircleID,
	})

	// 2 family events (1 today)
	adapter.AddMockEvent(&gcal_read.MockCalendarEvent{
		CalendarID:   "family-cal",
		CalendarName: "Family",
		AccountEmail: "satish.personal@gmail.com",
		EventUID:     "family-event-1",
		Title:        "School Pickup",
		StartTime:    now.Add(5 * time.Hour), // Today
		EndTime:      now.Add(5*time.Hour + 30*time.Minute),
		Timezone:     "Europe/London",
		IsBusy:       true,
		CircleID:     familyCircleID,
	})

	adapter.AddMockEvent(&gcal_read.MockCalendarEvent{
		CalendarID:   "family-cal",
		CalendarName: "Family",
		AccountEmail: "satish.personal@gmail.com",
		EventUID:     "family-event-2",
		Title:        "Dentist Appointment",
		StartTime:    now.AddDate(0, 0, 3), // In 3 days
		EndTime:      now.AddDate(0, 0, 3).Add(1 * time.Hour),
		Timezone:     "Europe/London",
		IsBusy:       true,
		CircleID:     familyCircleID,
	})

	return adapter
}

func createSatishFinanceAdapter(clk clock.Clock, financeCircleID identity.EntityID) *finance_read.MockAdapter {
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

	// 3 transactions (1 pending)
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
		TransactionStatus: "PENDING", // Pending
		AmountMinor:       1250,      // £12.50
		Currency:          "GBP",
		MerchantName:      "Costa",
		MerchantNameRaw:   "COSTA COFFEE",
		MerchantCategory:  "DINING",
		TransactionDate:   now.Add(-2 * time.Hour),
		CircleID:          financeCircleID,
	})

	adapter.AddMockTransaction(&finance_read.MockTransaction{
		AccountID:         "barclays-checking",
		TransactionID:     "tx-3",
		Institution:       "Barclays",
		MaskedNumber:      "****1234",
		TransactionType:   "CREDIT",
		TransactionKind:   "TRANSFER",
		TransactionStatus: "POSTED",
		AmountMinor:       100000, // £1,000.00
		Currency:          "GBP",
		MerchantName:      "Salary",
		MerchantNameRaw:   "COMPANY PAYROLL",
		MerchantCategory:  "INCOME",
		TransactionDate:   now.Add(-48 * time.Hour),
		CircleID:          financeCircleID,
	})

	return adapter
}
