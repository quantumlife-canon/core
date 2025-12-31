// Package ingestion provides the read-only ingestion runner.
//
// CRITICAL: This package is READ-ONLY. It ingests data from adapters,
// stores canonical events, and builds view snapshots. It NEVER writes
// to external systems.
//
// GUARDRAIL: This package does NOT spawn goroutines. All operations
// are synchronous. Background polling belongs in cmd/* processes.
//
// Reference: docs/TECHNICAL_ARCHITECTURE_V1.md
package ingestion

import (
	"fmt"
	"time"

	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/events"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/view"
)

// Config defines the ingestion configuration.
type Config struct {
	// Owner identity
	OwnerID   identity.EntityID
	OwnerName string

	// Email accounts to ingest
	EmailAccounts []EmailAccountConfig

	// Calendar accounts to ingest
	CalendarAccounts []CalendarAccountConfig

	// Finance accounts to ingest
	FinanceAccounts []FinanceAccountConfig

	// Circle mappings
	Circles []CircleConfig
}

// EmailAccountConfig defines an email account for ingestion.
type EmailAccountConfig struct {
	Email    string
	Provider string
	CircleID identity.EntityID
}

// CalendarAccountConfig defines a calendar account for ingestion.
type CalendarAccountConfig struct {
	CalendarID   string
	CalendarName string
	AccountEmail string
	Provider     string
	CircleID     identity.EntityID
}

// FinanceAccountConfig defines a finance account for ingestion.
type FinanceAccountConfig struct {
	AccountID    string
	Institution  string
	MaskedNumber string
	Currency     string
	CircleID     identity.EntityID
}

// CircleConfig defines a circle for the owner.
type CircleConfig struct {
	ID   identity.EntityID
	Name string
}

// Runner performs a single synchronous ingestion run.
type Runner struct {
	clock        clock.Clock
	eventStore   events.EventStore
	viewStore    view.ViewStore
	identityGen  *identity.Generator
	identityRepo identity.Repository

	// Adapters (injected)
	emailAdapter    EmailAdapter
	calendarAdapter CalendarAdapter
	financeAdapter  FinanceAdapter
}

// EmailAdapter interface for email ingestion.
type EmailAdapter interface {
	FetchMessages(accountEmail string, since time.Time, limit int) ([]*events.EmailMessageEvent, error)
	FetchUnreadCount(accountEmail string) (int, error)
	Name() string
}

// CalendarAdapter interface for calendar ingestion.
type CalendarAdapter interface {
	FetchEvents(calendarID string, from, to time.Time) ([]*events.CalendarEventEvent, error)
	FetchUpcomingCount(calendarID string, days int) (int, error)
	Name() string
}

// FinanceAdapter interface for finance ingestion.
type FinanceAdapter interface {
	FetchTransactions(accountID string, since time.Time, limit int) ([]*events.TransactionEvent, error)
	FetchBalance(accountID string) (*events.BalanceEvent, error)
	FetchPendingCount(accountID string) (int, error)
	Name() string
}

// NewRunner creates a new ingestion runner.
func NewRunner(
	clk clock.Clock,
	eventStore events.EventStore,
	viewStore view.ViewStore,
	identityRepo identity.Repository,
) *Runner {
	return &Runner{
		clock:        clk,
		eventStore:   eventStore,
		viewStore:    viewStore,
		identityGen:  identity.NewGenerator(),
		identityRepo: identityRepo,
	}
}

// SetEmailAdapter sets the email adapter.
func (r *Runner) SetEmailAdapter(adapter EmailAdapter) {
	r.emailAdapter = adapter
}

// SetCalendarAdapter sets the calendar adapter.
func (r *Runner) SetCalendarAdapter(adapter CalendarAdapter) {
	r.calendarAdapter = adapter
}

// SetFinanceAdapter sets the finance adapter.
func (r *Runner) SetFinanceAdapter(adapter FinanceAdapter) {
	r.financeAdapter = adapter
}

// RunResult contains the results of an ingestion run.
type RunResult struct {
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration

	// Event counts
	EmailEventsIngested    int
	CalendarEventsIngested int
	FinanceEventsIngested  int
	TotalEventsIngested    int

	// View snapshots created
	ViewSnapshotsCreated int

	// Per-circle results
	CircleResults []CircleRunResult

	// Errors (non-fatal)
	Warnings []string
}

// CircleRunResult contains results for a single circle.
type CircleRunResult struct {
	CircleID   identity.EntityID
	CircleName string

	// Counts
	UnreadEmails      int
	ImportantEmails   int
	UpcomingEvents    int
	TodayEvents       int
	PendingTx         int
	NewTx             int
	TotalBalanceMinor int64
	BalanceCurrency   string

	// View hash
	ViewHash string

	// "Nothing Needs You" status
	NothingNeedsYou bool
}

// Run performs a single synchronous ingestion run.
// GUARDRAIL: This method does NOT spawn goroutines.
func (r *Runner) Run(config *Config) (*RunResult, error) {
	startTime := r.clock.Now()
	result := &RunResult{
		StartTime:     startTime,
		CircleResults: make([]CircleRunResult, 0, len(config.Circles)),
	}

	// Track per-circle data
	circleData := make(map[identity.EntityID]*circleIngestionData)
	for _, c := range config.Circles {
		circleData[c.ID] = &circleIngestionData{
			circleID:   c.ID,
			circleName: c.Name,
		}
	}

	// Ingest emails
	if r.emailAdapter != nil {
		for _, acc := range config.EmailAccounts {
			emails, err := r.emailAdapter.FetchMessages(acc.Email, time.Time{}, 100)
			if err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("email fetch error for %s: %v", acc.Email, err))
				continue
			}

			for _, email := range emails {
				// Store event
				if err := r.eventStore.Store(email); err != nil {
					// Duplicate is OK
					if err != events.ErrEventExists {
						result.Warnings = append(result.Warnings, fmt.Sprintf("email store error: %v", err))
					}
					continue
				}
				result.EmailEventsIngested++

				// Update circle data
				if data, ok := circleData[acc.CircleID]; ok {
					data.emailCount++
					if !email.IsRead {
						data.unreadCount++
					}
					if email.IsImportant {
						data.importantCount++
					}
				}
			}
		}
	}

	// Ingest calendar events
	if r.calendarAdapter != nil {
		now := r.clock.Now()
		weekLater := now.AddDate(0, 0, 7)
		dayEnd := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())

		for _, acc := range config.CalendarAccounts {
			calEvents, err := r.calendarAdapter.FetchEvents(acc.CalendarID, now, weekLater)
			if err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("calendar fetch error for %s: %v", acc.CalendarID, err))
				continue
			}

			for _, calEvent := range calEvents {
				// Store event
				if err := r.eventStore.Store(calEvent); err != nil {
					if err != events.ErrEventExists {
						result.Warnings = append(result.Warnings, fmt.Sprintf("calendar store error: %v", err))
					}
					continue
				}
				result.CalendarEventsIngested++

				// Update circle data
				if data, ok := circleData[acc.CircleID]; ok {
					data.upcomingCount++
					if calEvent.StartTime.Before(dayEnd) {
						data.todayCount++
					}
				}
			}
		}
	}

	// Ingest finance data
	if r.financeAdapter != nil {
		for _, acc := range config.FinanceAccounts {
			// Fetch transactions
			txEvents, err := r.financeAdapter.FetchTransactions(acc.AccountID, time.Time{}, 50)
			if err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("finance tx fetch error for %s: %v", acc.AccountID, err))
			} else {
				for _, txEvent := range txEvents {
					if err := r.eventStore.Store(txEvent); err != nil {
						if err != events.ErrEventExists {
							result.Warnings = append(result.Warnings, fmt.Sprintf("finance tx store error: %v", err))
						}
						continue
					}
					result.FinanceEventsIngested++

					// Update circle data
					if data, ok := circleData[acc.CircleID]; ok {
						data.newTxCount++
						if txEvent.TransactionStatus == "PENDING" {
							data.pendingTxCount++
						}
					}
				}
			}

			// Fetch balance
			balanceEvent, err := r.financeAdapter.FetchBalance(acc.AccountID)
			if err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("finance balance fetch error for %s: %v", acc.AccountID, err))
			} else {
				if err := r.eventStore.Store(balanceEvent); err != nil {
					if err != events.ErrEventExists {
						result.Warnings = append(result.Warnings, fmt.Sprintf("finance balance store error: %v", err))
					}
				} else {
					result.FinanceEventsIngested++

					// Update circle data
					if data, ok := circleData[acc.CircleID]; ok {
						data.totalBalance += balanceEvent.CurrentMinor
						data.currency = balanceEvent.Currency
					}
				}
			}
		}
	}

	result.TotalEventsIngested = result.EmailEventsIngested + result.CalendarEventsIngested + result.FinanceEventsIngested

	// Build view snapshots for each circle
	captureTime := r.clock.Now()
	for _, c := range config.Circles {
		data := circleData[c.ID]

		builder := view.NewViewSnapshotBuilder(c.ID, c.Name, captureTime)
		builder.WithEmailCounts(data.unreadCount, data.importantCount)
		builder.WithCalendarCounts(data.upcomingCount, data.todayCount)
		builder.WithFinanceCounts(data.pendingTxCount, data.newTxCount, data.totalBalance, data.currency)

		totalItems := data.emailCount + data.upcomingCount + data.newTxCount
		needingAction := data.unreadCount + data.todayCount + data.pendingTxCount
		builder.WithTotalCounts(totalItems, needingAction)

		snapshot := builder.Build()

		if err := r.viewStore.Store(snapshot); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("view store error for %s: %v", c.Name, err))
		} else {
			result.ViewSnapshotsCreated++
		}

		// Add circle result
		circleResult := CircleRunResult{
			CircleID:          c.ID,
			CircleName:        c.Name,
			UnreadEmails:      data.unreadCount,
			ImportantEmails:   data.importantCount,
			UpcomingEvents:    data.upcomingCount,
			TodayEvents:       data.todayCount,
			PendingTx:         data.pendingTxCount,
			NewTx:             data.newTxCount,
			TotalBalanceMinor: data.totalBalance,
			BalanceCurrency:   data.currency,
			ViewHash:          snapshot.Hash,
			NothingNeedsYou:   snapshot.NothingNeedsYou(),
		}
		result.CircleResults = append(result.CircleResults, circleResult)
	}

	result.EndTime = r.clock.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

// circleIngestionData tracks ingestion data for a single circle.
type circleIngestionData struct {
	circleID       identity.EntityID
	circleName     string
	emailCount     int
	unreadCount    int
	importantCount int
	upcomingCount  int
	todayCount     int
	pendingTxCount int
	newTxCount     int
	totalBalance   int64
	currency       string
}
