package obligations

import (
	"testing"
	"time"

	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/events"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/obligation"
)

// mockIdentityRepo implements IdentityRepository for tests.
type mockIdentityRepo struct{}

func (m *mockIdentityRepo) GetByID(id identity.EntityID) (identity.Entity, error) {
	return nil, nil
}

func (m *mockIdentityRepo) IsHighPriority(id identity.EntityID) bool {
	return false
}

func TestEngineExtractDeterminism(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	config := DefaultConfig()
	repo := &mockIdentityRepo{}

	// Create event store with test data
	store := createTestEventStore(fixedTime)
	circleIDs := []identity.EntityID{"circle-work", "circle-family"}

	// Run extraction twice
	engine := NewEngine(config, clk, repo)
	result1 := engine.Extract(store, circleIDs)
	result2 := engine.Extract(store, circleIDs)

	// Hash must be identical
	if result1.Hash != result2.Hash {
		t.Errorf("Hash mismatch: %s vs %s", result1.Hash, result2.Hash)
	}

	// Same number of obligations
	if len(result1.Obligations) != len(result2.Obligations) {
		t.Errorf("Obligation count mismatch: %d vs %d",
			len(result1.Obligations), len(result2.Obligations))
	}

	// Same IDs in same order
	for i := range result1.Obligations {
		if result1.Obligations[i].ID != result2.Obligations[i].ID {
			t.Errorf("Obligation[%d] ID mismatch", i)
		}
	}
}

func TestEngineExtractFromEmail(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	config := DefaultConfig()
	repo := &mockIdentityRepo{}
	engine := NewEngine(config, clk, repo)

	tests := []struct {
		name         string
		setupEmail   func() *events.EmailMessageEvent
		expectOblig  bool
		expectedType obligation.ObligationType
		minRegret    float64
	}{
		{
			name: "action required email",
			setupEmail: func() *events.EmailMessageEvent {
				email := events.NewEmailMessageEvent("gmail", "msg-001", "user@work.com", fixedTime, fixedTime.Add(-1*time.Hour))
				email.Circle = "circle-work"
				email.Subject = "Action required: Review budget"
				email.BodyPreview = "Please review and approve by Friday"
				email.From = events.EmailAddress{Address: "boss@company.com"}
				email.IsRead = false
				email.SenderDomain = "company.com"
				return email
			},
			expectOblig:  true,
			expectedType: obligation.ObligationReview,
			minRegret:    0.7,
		},
		{
			name: "important unread email",
			setupEmail: func() *events.EmailMessageEvent {
				email := events.NewEmailMessageEvent("gmail", "msg-002", "user@work.com", fixedTime, fixedTime.Add(-2*time.Hour))
				email.Circle = "circle-work"
				email.Subject = "Q1 Planning"
				email.From = events.EmailAddress{Address: "manager@company.com"}
				email.IsRead = false
				email.IsImportant = true
				email.SenderDomain = "company.com"
				return email
			},
			expectOblig:  true,
			expectedType: obligation.ObligationReview,
			minRegret:    0.5,
		},
		{
			name: "read email - no obligation",
			setupEmail: func() *events.EmailMessageEvent {
				email := events.NewEmailMessageEvent("gmail", "msg-003", "user@work.com", fixedTime, fixedTime.Add(-1*time.Hour))
				email.Circle = "circle-work"
				email.Subject = "FYI: Newsletter"
				email.IsRead = true
				return email
			},
			expectOblig: false,
		},
		{
			name: "automated non-transactional - no obligation",
			setupEmail: func() *events.EmailMessageEvent {
				email := events.NewEmailMessageEvent("gmail", "msg-004", "user@work.com", fixedTime, fixedTime.Add(-1*time.Hour))
				email.Circle = "circle-work"
				email.Subject = "Weekly digest"
				email.IsRead = false
				email.IsAutomated = true
				email.IsTransactional = false
				return email
			},
			expectOblig: false,
		},
		{
			name: "invoice email",
			setupEmail: func() *events.EmailMessageEvent {
				email := events.NewEmailMessageEvent("gmail", "msg-005", "user@family.com", fixedTime, fixedTime.Add(-1*time.Hour))
				email.Circle = "circle-family"
				email.Subject = "Invoice #4521 - Payment due"
				email.IsRead = false
				email.IsTransactional = true
				return email
			},
			expectOblig:  true,
			expectedType: obligation.ObligationPay,
			minRegret:    0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := events.NewInMemoryEventStore()
			email := tt.setupEmail()
			store.Store(email)

			result := engine.Extract(store, []identity.EntityID{email.Circle})

			if tt.expectOblig {
				if len(result.Obligations) == 0 {
					t.Error("Expected obligation but got none")
					return
				}
				oblig := result.Obligations[0]
				if oblig.Type != tt.expectedType {
					t.Errorf("Type = %s, want %s", oblig.Type, tt.expectedType)
				}
				if oblig.RegretScore < tt.minRegret {
					t.Errorf("RegretScore = %.2f, want >= %.2f", oblig.RegretScore, tt.minRegret)
				}
			} else {
				if len(result.Obligations) > 0 {
					t.Errorf("Expected no obligations, got %d", len(result.Obligations))
				}
			}
		})
	}
}

func TestEngineExtractFromCalendar(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	config := DefaultConfig()
	repo := &mockIdentityRepo{}
	engine := NewEngine(config, clk, repo)

	tests := []struct {
		name         string
		setupEvent   func() *events.CalendarEventEvent
		expectOblig  bool
		expectedType obligation.ObligationType
	}{
		{
			name: "unresponded invite within 24h",
			setupEvent: func() *events.CalendarEventEvent {
				evt := events.NewCalendarEventEvent("google", "cal-1", "evt-001", "user@work.com", fixedTime, fixedTime)
				evt.Circle = "circle-work"
				evt.Title = "Budget Review"
				evt.StartTime = fixedTime.Add(4 * time.Hour)
				evt.EndTime = fixedTime.Add(5 * time.Hour)
				evt.MyResponseStatus = events.RSVPNeedsAction
				return evt
			},
			expectOblig:  true,
			expectedType: obligation.ObligationDecide,
		},
		{
			name: "accepted meeting within 24h",
			setupEvent: func() *events.CalendarEventEvent {
				evt := events.NewCalendarEventEvent("google", "cal-1", "evt-002", "user@work.com", fixedTime, fixedTime)
				evt.Circle = "circle-work"
				evt.Title = "Team Standup"
				evt.StartTime = fixedTime.Add(2 * time.Hour)
				evt.EndTime = fixedTime.Add(2*time.Hour + 30*time.Minute)
				evt.MyResponseStatus = events.RSVPAccepted
				return evt
			},
			expectOblig:  true,
			expectedType: obligation.ObligationAttend,
		},
		{
			name: "cancelled event - no obligation",
			setupEvent: func() *events.CalendarEventEvent {
				evt := events.NewCalendarEventEvent("google", "cal-1", "evt-003", "user@work.com", fixedTime, fixedTime)
				evt.Circle = "circle-work"
				evt.Title = "Cancelled Meeting"
				evt.StartTime = fixedTime.Add(1 * time.Hour)
				evt.EndTime = fixedTime.Add(2 * time.Hour)
				evt.IsCancelled = true
				return evt
			},
			expectOblig: false,
		},
		{
			name: "past event - no obligation",
			setupEvent: func() *events.CalendarEventEvent {
				evt := events.NewCalendarEventEvent("google", "cal-1", "evt-004", "user@work.com", fixedTime, fixedTime)
				evt.Circle = "circle-work"
				evt.Title = "Past Meeting"
				evt.StartTime = fixedTime.Add(-2 * time.Hour)
				evt.EndTime = fixedTime.Add(-1 * time.Hour)
				evt.MyResponseStatus = events.RSVPAccepted
				return evt
			},
			expectOblig: false,
		},
		{
			name: "far future event - no obligation",
			setupEvent: func() *events.CalendarEventEvent {
				evt := events.NewCalendarEventEvent("google", "cal-1", "evt-005", "user@work.com", fixedTime, fixedTime)
				evt.Circle = "circle-work"
				evt.Title = "Conference Next Month"
				evt.StartTime = fixedTime.Add(30 * 24 * time.Hour)
				evt.EndTime = fixedTime.Add(30*24*time.Hour + 2*time.Hour)
				evt.MyResponseStatus = events.RSVPAccepted
				return evt
			},
			expectOblig: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := events.NewInMemoryEventStore()
			evt := tt.setupEvent()
			store.Store(evt)

			result := engine.Extract(store, []identity.EntityID{evt.Circle})

			if tt.expectOblig {
				if len(result.Obligations) == 0 {
					t.Error("Expected obligation but got none")
					return
				}
				oblig := result.Obligations[0]
				if oblig.Type != tt.expectedType {
					t.Errorf("Type = %s, want %s", oblig.Type, tt.expectedType)
				}
			} else {
				if len(result.Obligations) > 0 {
					t.Errorf("Expected no obligations, got %d", len(result.Obligations))
				}
			}
		})
	}
}

func TestEngineExtractCalendarConflicts(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	config := DefaultConfig()
	repo := &mockIdentityRepo{}
	engine := NewEngine(config, clk, repo)

	store := events.NewInMemoryEventStore()
	circleID := identity.EntityID("circle-work")

	// Create two overlapping events
	evt1 := events.NewCalendarEventEvent("google", "cal-1", "evt-001", "user@work.com", fixedTime, fixedTime)
	evt1.Circle = circleID
	evt1.Title = "Meeting A"
	evt1.StartTime = fixedTime.Add(2 * time.Hour)
	evt1.EndTime = fixedTime.Add(3 * time.Hour)
	evt1.MyResponseStatus = events.RSVPAccepted
	store.Store(evt1)

	evt2 := events.NewCalendarEventEvent("google", "cal-1", "evt-002", "user@work.com", fixedTime, fixedTime)
	evt2.Circle = circleID
	evt2.Title = "Meeting B"
	evt2.StartTime = fixedTime.Add(2*time.Hour + 30*time.Minute)
	evt2.EndTime = fixedTime.Add(3*time.Hour + 30*time.Minute)
	evt2.MyResponseStatus = events.RSVPAccepted
	store.Store(evt2)

	result := engine.Extract(store, []identity.EntityID{circleID})

	// Should have conflict obligation
	var hasConflict bool
	for _, oblig := range result.Obligations {
		if oblig.Type == obligation.ObligationDecide &&
			oblig.Evidence[obligation.EvidenceKeyConflictWith] != "" {
			hasConflict = true
			break
		}
	}

	if !hasConflict {
		t.Error("Expected conflict obligation for overlapping events")
	}
}

func TestEngineExtractFromBalance(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	config := DefaultConfig()
	repo := &mockIdentityRepo{}
	engine := NewEngine(config, clk, repo)

	tests := []struct {
		name           string
		availableMinor int64
		expectOblig    bool
	}{
		{"low balance triggers obligation", 25000, true},       // £250
		{"threshold balance triggers obligation", 49999, true}, // Just below £500
		{"healthy balance no obligation", 100000, false},       // £1000
		{"very healthy balance no obligation", 500000, false},  // £5000
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := events.NewInMemoryEventStore()
			circleID := identity.EntityID("circle-finance")

			bal := events.NewBalanceEvent("truelayer", "acc-001", fixedTime, fixedTime)
			bal.Circle = circleID
			bal.AccountType = "CHECKING"
			bal.AvailableMinor = tt.availableMinor
			bal.CurrentMinor = tt.availableMinor + 5000
			bal.Currency = "GBP"
			store.Store(bal)

			result := engine.Extract(store, []identity.EntityID{circleID})

			if tt.expectOblig {
				if len(result.Obligations) == 0 {
					t.Error("Expected low balance obligation")
				}
			} else {
				// Should only have no obligations for healthy balance
				hasBalanceOblig := false
				for _, o := range result.Obligations {
					if o.SourceType == "finance" {
						hasBalanceOblig = true
					}
				}
				if hasBalanceOblig {
					t.Error("Did not expect balance obligation for healthy balance")
				}
			}
		})
	}
}

func TestEngineExtractFromTransaction(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	config := DefaultConfig()
	repo := &mockIdentityRepo{}
	engine := NewEngine(config, clk, repo)

	tests := []struct {
		name        string
		setupTx     func() *events.TransactionEvent
		expectOblig bool
	}{
		{
			name: "large recent debit",
			setupTx: func() *events.TransactionEvent {
				tx := events.NewTransactionEvent("truelayer", "acc-001", "tx-001", fixedTime, fixedTime.Add(-2*time.Hour))
				tx.Circle = "circle-finance"
				tx.TransactionType = "DEBIT"
				tx.TransactionStatus = "POSTED"
				tx.AmountMinor = 75000 // £750
				tx.Currency = "GBP"
				tx.MerchantName = "John Lewis"
				tx.TransactionDate = fixedTime.Add(-2 * time.Hour)
				return tx
			},
			expectOblig: true,
		},
		{
			name: "small debit - no obligation",
			setupTx: func() *events.TransactionEvent {
				tx := events.NewTransactionEvent("truelayer", "acc-001", "tx-002", fixedTime, fixedTime.Add(-1*time.Hour))
				tx.Circle = "circle-finance"
				tx.TransactionType = "DEBIT"
				tx.TransactionStatus = "POSTED"
				tx.AmountMinor = 1500 // £15
				tx.Currency = "GBP"
				tx.MerchantName = "Cafe"
				tx.TransactionDate = fixedTime.Add(-1 * time.Hour)
				return tx
			},
			expectOblig: false,
		},
		{
			name: "pending transaction",
			setupTx: func() *events.TransactionEvent {
				tx := events.NewTransactionEvent("truelayer", "acc-001", "tx-003", fixedTime, fixedTime.Add(-30*time.Minute))
				tx.Circle = "circle-finance"
				tx.TransactionType = "DEBIT"
				tx.TransactionStatus = "PENDING"
				tx.AmountMinor = 2500 // £25
				tx.Currency = "GBP"
				tx.MerchantName = "Tesco"
				tx.TransactionDate = fixedTime.Add(-30 * time.Minute)
				return tx
			},
			expectOblig: true,
		},
		{
			name: "old large debit - no obligation",
			setupTx: func() *events.TransactionEvent {
				tx := events.NewTransactionEvent("truelayer", "acc-001", "tx-004", fixedTime, fixedTime.Add(-72*time.Hour))
				tx.Circle = "circle-finance"
				tx.TransactionType = "DEBIT"
				tx.TransactionStatus = "POSTED"
				tx.AmountMinor = 100000 // £1000
				tx.Currency = "GBP"
				tx.MerchantName = "Old Purchase"
				tx.TransactionDate = fixedTime.Add(-72 * time.Hour) // 3 days ago
				return tx
			},
			expectOblig: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := events.NewInMemoryEventStore()
			tx := tt.setupTx()
			store.Store(tx)

			result := engine.Extract(store, []identity.EntityID{tx.Circle})

			if tt.expectOblig {
				if len(result.Obligations) == 0 {
					t.Error("Expected transaction obligation")
				}
			} else {
				if len(result.Obligations) > 0 {
					t.Errorf("Did not expect obligation, got %d", len(result.Obligations))
				}
			}
		})
	}
}

func TestEngineObligationsSorted(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)
	config := DefaultConfig()
	repo := &mockIdentityRepo{}
	engine := NewEngine(config, clk, repo)

	store := createTestEventStore(fixedTime)
	circleIDs := []identity.EntityID{"circle-work", "circle-family", "circle-finance"}

	result := engine.Extract(store, circleIDs)

	// Verify sorted by horizon, then regret
	for i := 1; i < len(result.Obligations); i++ {
		prev := result.Obligations[i-1]
		curr := result.Obligations[i]

		prevHorizon := obligation.HorizonOrder(prev.Horizon)
		currHorizon := obligation.HorizonOrder(curr.Horizon)

		if prevHorizon > currHorizon {
			t.Errorf("Obligation[%d] has worse horizon than [%d]: %s vs %s",
				i-1, i, prev.Horizon, curr.Horizon)
		}

		if prevHorizon == currHorizon && prev.RegretScore < curr.RegretScore {
			t.Errorf("Obligation[%d] has lower regret than [%d] within same horizon",
				i-1, i)
		}
	}
}

func createTestEventStore(now time.Time) *events.InMemoryEventStore {
	store := events.NewInMemoryEventStore()

	// Work email
	email := events.NewEmailMessageEvent("gmail", "msg-100", "user@work.com", now, now.Add(-1*time.Hour))
	email.Circle = "circle-work"
	email.Subject = "Action required: Budget approval"
	email.From = events.EmailAddress{Address: "cfo@company.com"}
	email.IsRead = false
	email.SenderDomain = "company.com"
	store.Store(email)

	// Work calendar
	meeting := events.NewCalendarEventEvent("google", "cal-1", "evt-100", "user@work.com", now, now)
	meeting.Circle = "circle-work"
	meeting.Title = "Team Sync"
	meeting.StartTime = now.Add(3 * time.Hour)
	meeting.EndTime = now.Add(4 * time.Hour)
	meeting.MyResponseStatus = events.RSVPAccepted
	store.Store(meeting)

	// Family email
	familyEmail := events.NewEmailMessageEvent("gmail", "msg-200", "user@family.com", now, now.Add(-2*time.Hour))
	familyEmail.Circle = "circle-family"
	familyEmail.Subject = "Invoice - Payment due by Friday"
	familyEmail.From = events.EmailAddress{Address: "billing@utility.co.uk"}
	familyEmail.IsRead = false
	familyEmail.IsTransactional = true
	store.Store(familyEmail)

	// Finance balance
	balance := events.NewBalanceEvent("truelayer", "acc-300", now, now)
	balance.Circle = "circle-finance"
	balance.AvailableMinor = 45000 // £450 - below threshold
	balance.CurrentMinor = 47500
	balance.Currency = "GBP"
	store.Store(balance)

	return store
}
