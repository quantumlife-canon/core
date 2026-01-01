// Package ingestion provides multi-account ingestion for QuantumLife.
//
// CRITICAL: This package is READ-ONLY. It ingests data from adapters,
// stores canonical events, and builds view snapshots. It NEVER writes
// to external systems.
//
// GUARDRAIL: This package does NOT spawn goroutines. All operations
// are synchronous and deterministic. For each circle, integrations are
// processed in stable sorted order.
//
// Reference: docs/ADR/ADR-0026-phase11-multicircle-real-loop.md
package ingestion

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/config"
	"quantumlife/pkg/domain/events"
	"quantumlife/pkg/domain/identity"
)

// MultiRunner performs synchronous ingestion across multiple circles.
// CRITICAL: No goroutines. All operations are synchronous.
type MultiRunner struct {
	clock      clock.Clock
	eventStore events.EventStore

	// Adapters per provider (keyed by provider name)
	emailAdapters    map[string]EmailAdapter
	calendarAdapters map[string]CalendarAdapter
	financeAdapters  map[string]FinanceAdapter
}

// NewMultiRunner creates a new multi-account ingestion runner.
func NewMultiRunner(clk clock.Clock, eventStore events.EventStore) *MultiRunner {
	return &MultiRunner{
		clock:            clk,
		eventStore:       eventStore,
		emailAdapters:    make(map[string]EmailAdapter),
		calendarAdapters: make(map[string]CalendarAdapter),
		financeAdapters:  make(map[string]FinanceAdapter),
	}
}

// RegisterEmailAdapter registers an email adapter for a provider.
func (r *MultiRunner) RegisterEmailAdapter(provider string, adapter EmailAdapter) {
	r.emailAdapters[provider] = adapter
}

// RegisterCalendarAdapter registers a calendar adapter for a provider.
func (r *MultiRunner) RegisterCalendarAdapter(provider string, adapter CalendarAdapter) {
	r.calendarAdapters[provider] = adapter
}

// RegisterFinanceAdapter registers a finance adapter for a provider.
func (r *MultiRunner) RegisterFinanceAdapter(provider string, adapter FinanceAdapter) {
	r.financeAdapters[provider] = adapter
}

// MultiRunOptions configures a multi-circle ingestion run.
type MultiRunOptions struct {
	// EmailLookbackHours is how many hours back to fetch emails.
	EmailLookbackHours int

	// CalendarLookbackDays is how many days back to fetch calendar events.
	CalendarLookbackDays int

	// CalendarLookaheadDays is how many days ahead to fetch calendar events.
	CalendarLookaheadDays int

	// FinanceLookbackDays is how many days back to fetch transactions.
	FinanceLookbackDays int

	// MaxEmailsPerAccount is the maximum emails to fetch per account.
	MaxEmailsPerAccount int

	// MaxEventsPerCalendar is the maximum events to fetch per calendar.
	MaxEventsPerCalendar int

	// CircleID limits ingestion to a specific circle (empty = all circles).
	CircleID identity.EntityID
}

// DefaultMultiRunOptions returns sensible defaults.
func DefaultMultiRunOptions() MultiRunOptions {
	return MultiRunOptions{
		EmailLookbackHours:    72, // 3 days
		CalendarLookbackDays:  1,  // yesterday
		CalendarLookaheadDays: 14, // 2 weeks
		FinanceLookbackDays:   30, // 1 month
		MaxEmailsPerAccount:   100,
		MaxEventsPerCalendar:  100,
	}
}

// MultiRunResult contains results from a multi-circle ingestion run.
type MultiRunResult struct {
	// StartTime is when the run started.
	StartTime time.Time

	// EndTime is when the run completed.
	EndTime time.Time

	// Duration is the total duration of the run.
	Duration time.Duration

	// CircleReceipts contains per-circle sync receipts.
	CircleReceipts []CircleSyncReceipt

	// TotalEmailsFetched is the total emails fetched across all circles.
	TotalEmailsFetched int

	// TotalCalendarEventsFetched is the total calendar events fetched.
	TotalCalendarEventsFetched int

	// TotalFinanceEventsFetched is the total finance events fetched.
	TotalFinanceEventsFetched int

	// Warnings contains non-fatal warnings.
	Warnings []string

	// Hash is a deterministic hash of the run result.
	Hash string
}

// CircleSyncReceipt contains sync results for a single circle.
type CircleSyncReceipt struct {
	// CircleID is the circle identifier.
	CircleID identity.EntityID

	// CircleName is the circle display name.
	CircleName string

	// FetchedAt is when the sync was performed.
	FetchedAt time.Time

	// EmailsFetched is the number of emails fetched.
	EmailsFetched int

	// CalendarEventsFetched is the number of calendar events fetched.
	CalendarEventsFetched int

	// FinanceEventsFetched is the number of finance events fetched.
	FinanceEventsFetched int

	// IntegrationResults contains per-integration results.
	IntegrationResults []IntegrationSyncResult

	// Hash is a deterministic hash of the receipt.
	Hash string
}

// IntegrationSyncResult contains sync results for a single integration.
type IntegrationSyncResult struct {
	// Type is the integration type (email, calendar, finance).
	Type string

	// Provider is the provider name (google, microsoft, plaid, etc).
	Provider string

	// AccountID is the account identifier.
	AccountID string

	// EventsFetched is the number of events fetched.
	EventsFetched int

	// Success indicates if the sync was successful.
	Success bool

	// Error contains any error message.
	Error string
}

// Run performs a synchronous multi-circle ingestion run.
// GUARDRAIL: This method does NOT spawn goroutines.
func (r *MultiRunner) Run(cfg *config.MultiCircleConfig, opts MultiRunOptions) (*MultiRunResult, error) {
	now := r.clock.Now()
	result := &MultiRunResult{
		StartTime:      now,
		CircleReceipts: make([]CircleSyncReceipt, 0, len(cfg.Circles)),
	}

	// Process circles in deterministic sorted order
	circleIDs := cfg.CircleIDs()

	for _, circleID := range circleIDs {
		// Skip if filtering to a specific circle
		if opts.CircleID != "" && circleID != opts.CircleID {
			continue
		}

		circle := cfg.GetCircle(circleID)
		if circle == nil {
			continue
		}

		receipt := r.syncCircle(circle, now, opts)
		result.CircleReceipts = append(result.CircleReceipts, receipt)

		result.TotalEmailsFetched += receipt.EmailsFetched
		result.TotalCalendarEventsFetched += receipt.CalendarEventsFetched
		result.TotalFinanceEventsFetched += receipt.FinanceEventsFetched
	}

	result.EndTime = r.clock.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.Hash = r.computeResultHash(result)

	return result, nil
}

// syncCircle syncs a single circle's integrations.
func (r *MultiRunner) syncCircle(circle *config.CircleConfig, now time.Time, opts MultiRunOptions) CircleSyncReceipt {
	receipt := CircleSyncReceipt{
		CircleID:           circle.ID,
		CircleName:         circle.Name,
		FetchedAt:          now,
		IntegrationResults: make([]IntegrationSyncResult, 0),
	}

	// Process email integrations in sorted order
	sortedEmails := r.sortEmailIntegrations(circle.EmailIntegrations)
	for _, email := range sortedEmails {
		result := r.syncEmailIntegration(circle.ID, email, now, opts)
		receipt.IntegrationResults = append(receipt.IntegrationResults, result)
		receipt.EmailsFetched += result.EventsFetched
	}

	// Process calendar integrations in sorted order
	sortedCalendars := r.sortCalendarIntegrations(circle.CalendarIntegrations)
	for _, cal := range sortedCalendars {
		result := r.syncCalendarIntegration(circle.ID, cal, now, opts)
		receipt.IntegrationResults = append(receipt.IntegrationResults, result)
		receipt.CalendarEventsFetched += result.EventsFetched
	}

	// Process finance integrations in sorted order
	sortedFinance := r.sortFinanceIntegrations(circle.FinanceIntegrations)
	for _, fin := range sortedFinance {
		result := r.syncFinanceIntegration(circle.ID, fin, now, opts)
		receipt.IntegrationResults = append(receipt.IntegrationResults, result)
		receipt.FinanceEventsFetched += result.EventsFetched
	}

	receipt.Hash = r.computeReceiptHash(&receipt)
	return receipt
}

// syncEmailIntegration syncs a single email integration.
func (r *MultiRunner) syncEmailIntegration(
	circleID identity.EntityID,
	integration config.EmailIntegration,
	now time.Time,
	opts MultiRunOptions,
) IntegrationSyncResult {
	result := IntegrationSyncResult{
		Type:      "email",
		Provider:  integration.Provider,
		AccountID: integration.Identifier,
	}

	adapter, ok := r.emailAdapters[integration.Provider]
	if !ok {
		result.Error = fmt.Sprintf("no adapter for provider: %s", integration.Provider)
		return result
	}

	since := now.Add(-time.Duration(opts.EmailLookbackHours) * time.Hour)
	messages, err := adapter.FetchMessages(integration.Identifier, since, opts.MaxEmailsPerAccount)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	// Sort messages for deterministic processing
	sortEmailMessages(messages)

	for _, msg := range messages {
		// Set circle ID on the event
		msg.SetCircleID(circleID)

		if err := r.eventStore.Store(msg); err != nil {
			if err != events.ErrEventExists {
				// Log warning but continue
				continue
			}
		}
		result.EventsFetched++
	}

	result.Success = true
	return result
}

// syncCalendarIntegration syncs a single calendar integration.
func (r *MultiRunner) syncCalendarIntegration(
	circleID identity.EntityID,
	integration config.CalendarIntegration,
	now time.Time,
	opts MultiRunOptions,
) IntegrationSyncResult {
	result := IntegrationSyncResult{
		Type:      "calendar",
		Provider:  integration.Provider,
		AccountID: integration.CalendarID,
	}

	adapter, ok := r.calendarAdapters[integration.Provider]
	if !ok {
		result.Error = fmt.Sprintf("no adapter for provider: %s", integration.Provider)
		return result
	}

	from := now.AddDate(0, 0, -opts.CalendarLookbackDays)
	to := now.AddDate(0, 0, opts.CalendarLookaheadDays)

	calEvents, err := adapter.FetchEvents(integration.CalendarID, from, to)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	// Sort events for deterministic processing
	sortCalendarEvents(calEvents)

	for _, evt := range calEvents {
		// Set circle ID on the event
		evt.SetCircleID(circleID)

		if err := r.eventStore.Store(evt); err != nil {
			if err != events.ErrEventExists {
				continue
			}
		}
		result.EventsFetched++
	}

	result.Success = true
	return result
}

// syncFinanceIntegration syncs a single finance integration.
func (r *MultiRunner) syncFinanceIntegration(
	circleID identity.EntityID,
	integration config.FinanceIntegration,
	now time.Time,
	opts MultiRunOptions,
) IntegrationSyncResult {
	result := IntegrationSyncResult{
		Type:      "finance",
		Provider:  integration.Provider,
		AccountID: integration.Identifier,
	}

	adapter, ok := r.financeAdapters[integration.Provider]
	if !ok {
		result.Error = fmt.Sprintf("no adapter for provider: %s", integration.Provider)
		return result
	}

	since := now.AddDate(0, 0, -opts.FinanceLookbackDays)
	txEvents, err := adapter.FetchTransactions(integration.Identifier, since, 100)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	// Sort transactions for deterministic processing
	sortTransactionEvents(txEvents)

	for _, tx := range txEvents {
		// Set circle ID on the event
		tx.SetCircleID(circleID)

		if err := r.eventStore.Store(tx); err != nil {
			if err != events.ErrEventExists {
				continue
			}
		}
		result.EventsFetched++
	}

	// Also fetch balance
	balanceEvent, err := adapter.FetchBalance(integration.Identifier)
	if err == nil && balanceEvent != nil {
		balanceEvent.SetCircleID(circleID)
		if err := r.eventStore.Store(balanceEvent); err == nil || err == events.ErrEventExists {
			result.EventsFetched++
		}
	}

	result.Success = true
	return result
}

// Sorting helpers for deterministic ordering

func (r *MultiRunner) sortEmailIntegrations(integrations []config.EmailIntegration) []config.EmailIntegration {
	sorted := make([]config.EmailIntegration, len(integrations))
	copy(sorted, integrations)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Provider != sorted[j].Provider {
			return sorted[i].Provider < sorted[j].Provider
		}
		return sorted[i].Identifier < sorted[j].Identifier
	})
	return sorted
}

func (r *MultiRunner) sortCalendarIntegrations(integrations []config.CalendarIntegration) []config.CalendarIntegration {
	sorted := make([]config.CalendarIntegration, len(integrations))
	copy(sorted, integrations)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Provider != sorted[j].Provider {
			return sorted[i].Provider < sorted[j].Provider
		}
		return sorted[i].CalendarID < sorted[j].CalendarID
	})
	return sorted
}

func (r *MultiRunner) sortFinanceIntegrations(integrations []config.FinanceIntegration) []config.FinanceIntegration {
	sorted := make([]config.FinanceIntegration, len(integrations))
	copy(sorted, integrations)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Provider != sorted[j].Provider {
			return sorted[i].Provider < sorted[j].Provider
		}
		return sorted[i].Identifier < sorted[j].Identifier
	})
	return sorted
}

func sortEmailMessages(messages []*events.EmailMessageEvent) {
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].EventID() < messages[j].EventID()
	})
}

func sortCalendarEvents(calEvents []*events.CalendarEventEvent) {
	sort.Slice(calEvents, func(i, j int) bool {
		return calEvents[i].EventID() < calEvents[j].EventID()
	})
}

func sortTransactionEvents(txEvents []*events.TransactionEvent) {
	sort.Slice(txEvents, func(i, j int) bool {
		return txEvents[i].EventID() < txEvents[j].EventID()
	})
}

// Hash computation

func (r *MultiRunner) computeReceiptHash(receipt *CircleSyncReceipt) string {
	var b strings.Builder
	b.WriteString("circle:")
	b.WriteString(string(receipt.CircleID))
	b.WriteString("|name:")
	b.WriteString(receipt.CircleName)
	b.WriteString("|fetched_at:")
	b.WriteString(receipt.FetchedAt.UTC().Format(time.RFC3339))
	b.WriteString("|emails:")
	b.WriteString(itoa(receipt.EmailsFetched))
	b.WriteString("|calendar:")
	b.WriteString(itoa(receipt.CalendarEventsFetched))
	b.WriteString("|finance:")
	b.WriteString(itoa(receipt.FinanceEventsFetched))

	for _, ir := range receipt.IntegrationResults {
		b.WriteString("|integration:")
		b.WriteString(ir.Type)
		b.WriteString(":")
		b.WriteString(ir.Provider)
		b.WriteString(":")
		b.WriteString(ir.AccountID)
		b.WriteString(":")
		b.WriteString(itoa(ir.EventsFetched))
	}

	h := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(h[:])
}

func (r *MultiRunner) computeResultHash(result *MultiRunResult) string {
	var b strings.Builder
	b.WriteString("multi_run|")
	b.WriteString("start:")
	b.WriteString(result.StartTime.UTC().Format(time.RFC3339))
	b.WriteString("|emails:")
	b.WriteString(itoa(result.TotalEmailsFetched))
	b.WriteString("|calendar:")
	b.WriteString(itoa(result.TotalCalendarEventsFetched))
	b.WriteString("|finance:")
	b.WriteString(itoa(result.TotalFinanceEventsFetched))

	for _, cr := range result.CircleReceipts {
		b.WriteString("|receipt:")
		b.WriteString(cr.Hash)
	}

	h := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(h[:])
}

// itoa converts int to string without strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if negative {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}
