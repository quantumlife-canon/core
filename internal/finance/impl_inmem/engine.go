// Package impl_inmem provides an in-memory implementation of the financial read runtime.
//
// CRITICAL: This is READ-ONLY by design. No execution capability exists.
//
// Reference: docs/TECHNOLOGY_SELECTION_V8_FINANCIAL_READ.md
package impl_inmem

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"quantumlife/internal/connectors/finance/read"
	"quantumlife/internal/finance"
	"quantumlife/internal/finance/categorize"
	"quantumlife/internal/finance/normalize"
	"quantumlife/internal/finance/propose"
	"quantumlife/internal/finance/reconcile"
	"quantumlife/pkg/primitives"
	finprimitives "quantumlife/pkg/primitives/finance"
)

// Engine implements finance.Reader with in-memory storage.
type Engine struct {
	mu sync.RWMutex

	// Connector for reading financial data
	connector read.ReadConnector

	// Categorizer for transaction categorization
	categorizer *categorize.Categorizer

	// Proposal generator
	proposalGen *propose.Generator

	// v8.4: Normalizer registry and reconciliation engine
	normalizers     *normalize.Registry
	reconcileEngine reconcile.Engine

	// In-memory storage
	snapshots    map[string]*finprimitives.AccountSnapshot    // snapshotID -> snapshot
	transactions map[string][]finprimitives.TransactionRecord // ownerID -> transactions
	observations map[string][]finprimitives.FinancialObservation
	proposals    map[string][]finprimitives.FinancialProposal
	dismissals   map[string]*dismissalRecord // fingerprint -> record

	// v8.4: Canonical ID tracking for deduplication
	seenCanonicalIDs map[string]bool // canonicalID -> true

	// Configuration
	clockFunc func() time.Time
	idCounter int
	auditFunc func(eventType string, metadata map[string]string) // Optional audit callback
}

type dismissalRecord struct {
	Fingerprint string
	DismissedAt time.Time
	DismissedBy string
}

// EngineConfig configures the engine.
type EngineConfig struct {
	Connector read.ReadConnector
	ClockFunc func() time.Time
	AuditFunc func(eventType string, metadata map[string]string)
}

// NewEngine creates a new financial read engine.
func NewEngine(config EngineConfig) *Engine {
	clockFunc := config.ClockFunc
	if clockFunc == nil {
		clockFunc = time.Now
	}

	e := &Engine{
		connector:        config.Connector,
		categorizer:      categorize.NewCategorizer(),
		normalizers:      normalize.DefaultRegistry(),
		reconcileEngine:  reconcile.NewEngine(),
		clockFunc:        clockFunc,
		auditFunc:        config.AuditFunc,
		snapshots:        make(map[string]*finprimitives.AccountSnapshot),
		transactions:     make(map[string][]finprimitives.TransactionRecord),
		observations:     make(map[string][]finprimitives.FinancialObservation),
		proposals:        make(map[string][]finprimitives.FinancialProposal),
		dismissals:       make(map[string]*dismissalRecord),
		seenCanonicalIDs: make(map[string]bool),
	}

	// Create dismissal store for proposal generator
	dismissalStore := &inMemoryDismissalStore{engine: e}

	// Create proposal generator
	e.proposalGen = propose.NewGenerator(
		propose.DefaultConfig(),
		dismissalStore,
		clockFunc,
		e.generateID,
	)

	return e
}

// Sync fetches and normalizes financial data.
// v8.4: Includes canonical ID computation and reconciliation.
func (e *Engine) Sync(ctx context.Context, env primitives.ExecutionEnvelope, req finance.SyncRequest) (*finance.SyncResult, error) {
	// CRITICAL: Validate envelope for finance read
	if err := read.ValidateEnvelopeForFinanceRead(&env); err != nil {
		e.emitAudit("finance.mode.rejected", map[string]string{
			"trace_id": env.TraceID,
			"reason":   err.Error(),
		})
		return nil, err
	}

	e.emitAudit("finance.read.started", map[string]string{
		"trace_id":  env.TraceID,
		"circle_id": req.CircleID,
	})

	now := e.clockFunc()

	// Calculate window
	windowDays := req.WindowDays
	if windowDays == 0 {
		windowDays = 90
	}
	startDate := now.AddDate(0, 0, -windowDays)

	// Fetch accounts
	accountsReceipt, err := e.connector.ListAccounts(ctx, env, read.ListAccountsRequest{
		IncludeBalances: req.IncludeBalances,
	})
	if err != nil {
		e.emitAudit("finance.provider.unavailable", map[string]string{
			"trace_id": env.TraceID,
			"error":    err.Error(),
		})
		return nil, fmt.Errorf("failed to list accounts: %w", err)
	}

	// Fetch transactions
	txReceipt, err := e.connector.ListTransactions(ctx, env, read.ListTransactionsRequest{
		StartDate: startDate,
		EndDate:   now,
	})
	if err != nil {
		e.emitAudit("finance.provider.unavailable", map[string]string{
			"trace_id": env.TraceID,
			"error":    err.Error(),
		})
		return nil, fmt.Errorf("failed to list transactions: %w", err)
	}

	// v8.4: Normalize with canonical ID computation
	snapshot := e.normalizeAccounts(req.CircleID, accountsReceipt, env.TraceID, now)
	transactions := e.normalizeTransactions(req.CircleID, txReceipt, env.TraceID, now)

	// v8.4: Reconcile to deduplicate and merge pending→posted
	reconcileCtx := reconcile.ReconcileContext{
		OwnerType:         "circle",
		OwnerID:           req.CircleID,
		TraceID:           env.TraceID,
		ReconcilerVersion: "v8.4",
	}

	// Convert transactions to normalize.NormalizedTransactionResult for reconciliation
	normalizedTxns := e.convertToNormalizedResults(transactions)

	txnResult, err := e.reconcileEngine.ReconcileTransactions(reconcileCtx, normalizedTxns)
	if err != nil {
		e.emitAudit("finance.reconcile.failed", map[string]string{
			"trace_id": env.TraceID,
			"error":    err.Error(),
		})
		// Continue with unreconciled data
		txnResult = nil
	}

	// Apply reconciliation results
	var reconciledTransactions []finprimitives.TransactionRecord
	var duplicatesRemoved, pendingMerged int

	if txnResult != nil {
		for _, rt := range txnResult.Transactions {
			reconciledTransactions = append(reconciledTransactions, rt.Transaction)
		}
		duplicatesRemoved = txnResult.Report.DuplicatesRemoved
		pendingMerged = txnResult.Report.PendingMerged

		// v8.4 audit event for reconciliation (counts only, no amounts)
		e.emitAudit("finance.reconciled", map[string]string{
			"trace_id":           env.TraceID,
			"input_count":        fmt.Sprintf("%d", txnResult.Report.InputCount),
			"output_count":       fmt.Sprintf("%d", txnResult.Report.OutputCount),
			"duplicates_removed": fmt.Sprintf("%d", duplicatesRemoved),
			"pending_merged":     fmt.Sprintf("%d", pendingMerged),
		})
	} else {
		reconciledTransactions = transactions
	}

	e.mu.Lock()
	e.snapshots[snapshot.SnapshotID] = snapshot
	e.transactions[req.CircleID] = reconciledTransactions
	e.mu.Unlock()

	e.emitAudit("finance.sync.completed", map[string]string{
		"trace_id":          env.TraceID,
		"snapshot_id":       snapshot.SnapshotID,
		"account_count":     fmt.Sprintf("%d", len(snapshot.Accounts)),
		"transaction_count": fmt.Sprintf("%d", len(reconciledTransactions)),
	})

	freshness := finprimitives.FreshnessCurrent
	if accountsReceipt.Partial || txReceipt.Partial {
		freshness = finprimitives.FreshnessPartial
	}

	return &finance.SyncResult{
		SnapshotID:         snapshot.SnapshotID,
		TransactionBatchID: e.generateID(),
		AccountCount:       len(snapshot.Accounts),
		TransactionCount:   len(reconciledTransactions),
		Freshness:          freshness,
		PartialReason:      accountsReceipt.PartialReason,
		SyncedAt:           now,
		TraceID:            env.TraceID,
	}, nil
}

// Observe generates observations from financial data.
func (e *Engine) Observe(ctx context.Context, env primitives.ExecutionEnvelope, req finance.ObserveRequest) (*finance.ObservationsResult, error) {
	// CRITICAL: Validate envelope
	if err := read.ValidateEnvelopeForFinanceRead(&env); err != nil {
		return nil, err
	}

	now := e.clockFunc()
	windowDays := req.WindowDays
	if windowDays == 0 {
		windowDays = 30
	}
	windowStart := now.AddDate(0, 0, -windowDays)

	e.mu.RLock()
	transactions := e.transactions[req.OwnerID]
	e.mu.RUnlock()

	// Filter transactions to window
	var windowTx []finprimitives.TransactionRecord
	for _, tx := range transactions {
		if tx.Date.After(windowStart) && tx.Date.Before(now) {
			windowTx = append(windowTx, tx)
		}
	}

	// Generate observations
	observations := e.generateObservations(req.OwnerType, req.OwnerID, windowTx, windowStart, now, env.TraceID)

	e.mu.Lock()
	e.observations[req.OwnerID] = observations
	e.mu.Unlock()

	for _, obs := range observations {
		e.emitAudit("finance.observation.created", map[string]string{
			"trace_id":       env.TraceID,
			"observation_id": obs.ObservationID,
			"type":           string(obs.Type),
		})
	}

	return &finance.ObservationsResult{
		Observations:        observations,
		SuppressedCount:     0,
		AnalysisWindowStart: windowStart,
		AnalysisWindowEnd:   now,
		TraceID:             env.TraceID,
	}, nil
}

// Propose generates proposals from observations.
func (e *Engine) Propose(ctx context.Context, env primitives.ExecutionEnvelope, req finance.ProposeRequest) (*finance.ProposalsResult, error) {
	// CRITICAL: Validate envelope
	if err := read.ValidateEnvelopeForFinanceRead(&env); err != nil {
		return nil, err
	}

	e.mu.RLock()
	observations := e.observations[req.OwnerID]
	e.mu.RUnlock()

	// Generate proposals
	batch := e.proposalGen.Generate(req.OwnerType, req.OwnerID, observations, env.TraceID)

	// Store proposals
	e.mu.Lock()
	e.proposals[req.OwnerID] = batch.Proposals
	e.mu.Unlock()

	// Emit audit events
	for _, p := range batch.Proposals {
		e.emitAudit("finance.proposal.generated", map[string]string{
			"trace_id":    env.TraceID,
			"proposal_id": p.ProposalID,
			"type":        string(p.Type),
		})
	}

	if batch.SilenceApplied {
		e.emitAudit("finance.proposal.suppressed", map[string]string{
			"trace_id": env.TraceID,
			"reason":   batch.SilenceReason,
		})
	}

	return &finance.ProposalsResult{
		Proposals:       batch.Proposals,
		SuppressedCount: batch.SuppressedCount,
		SilenceApplied:  batch.SilenceApplied,
		SilenceReason:   batch.SilenceReason,
		TraceID:         env.TraceID,
	}, nil
}

// Dismiss marks a proposal as dismissed.
func (e *Engine) Dismiss(ctx context.Context, env primitives.ExecutionEnvelope, req finance.DismissRequest) error {
	// CRITICAL: Validate envelope
	if err := read.ValidateEnvelopeForFinanceRead(&env); err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// Find the proposal
	for ownerID, proposals := range e.proposals {
		for i, p := range proposals {
			if p.ProposalID == req.ProposalID {
				// Mark as dismissed
				now := e.clockFunc()
				e.proposals[ownerID][i].Status = finprimitives.StatusDismissed
				e.proposals[ownerID][i].DismissedAt = &now
				e.proposals[ownerID][i].DismissedBy = req.DismissedBy

				// Record dismissal for suppression
				e.dismissals[p.Fingerprint] = &dismissalRecord{
					Fingerprint: p.Fingerprint,
					DismissedAt: now,
					DismissedBy: req.DismissedBy,
				}

				e.emitAudit("finance.dismissal.recorded", map[string]string{
					"trace_id":    env.TraceID,
					"proposal_id": req.ProposalID,
					"fingerprint": p.Fingerprint,
				})

				return nil
			}
		}
	}

	return fmt.Errorf("proposal not found: %s", req.ProposalID)
}

// GetSnapshot retrieves the latest financial snapshot.
func (e *Engine) GetSnapshot(ctx context.Context, env primitives.ExecutionEnvelope, ownerType, ownerID string) (*finprimitives.AccountSnapshot, error) {
	if err := read.ValidateEnvelopeForFinanceRead(&env); err != nil {
		return nil, err
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	// Find latest snapshot for owner
	var latest *finprimitives.AccountSnapshot
	for _, s := range e.snapshots {
		if s.OwnerID == ownerID && s.OwnerType == ownerType {
			if latest == nil || s.CreatedAt.After(latest.CreatedAt) {
				latest = s
			}
		}
	}

	if latest == nil {
		return nil, fmt.Errorf("no snapshot found for %s/%s", ownerType, ownerID)
	}

	return latest, nil
}

// GetTransactions retrieves transactions within a window.
func (e *Engine) GetTransactions(ctx context.Context, env primitives.ExecutionEnvelope, req finance.GetTransactionsRequest) (*finprimitives.TransactionBatch, error) {
	if err := read.ValidateEnvelopeForFinanceRead(&env); err != nil {
		return nil, err
	}

	e.mu.RLock()
	allTx := e.transactions[req.OwnerID]
	e.mu.RUnlock()

	var filtered []finprimitives.TransactionRecord
	for _, tx := range allTx {
		if tx.Date.Before(req.WindowStart) || tx.Date.After(req.WindowEnd) {
			continue
		}
		if len(req.Categories) > 0 && !contains(req.Categories, tx.Category) {
			continue
		}
		if len(req.AccountIDs) > 0 && !contains(req.AccountIDs, tx.AccountID) {
			continue
		}
		filtered = append(filtered, tx)
	}

	return &finprimitives.TransactionBatch{
		BatchID:          e.generateID(),
		OwnerType:        req.OwnerType,
		OwnerID:          req.OwnerID,
		Transactions:     filtered,
		WindowStart:      req.WindowStart,
		WindowEnd:        req.WindowEnd,
		TransactionCount: len(filtered),
		CreatedAt:        e.clockFunc(),
		TraceID:          env.TraceID,
	}, nil
}

// GetProposals retrieves active proposals.
func (e *Engine) GetProposals(ctx context.Context, env primitives.ExecutionEnvelope, ownerType, ownerID string) ([]finprimitives.FinancialProposal, error) {
	if err := read.ValidateEnvelopeForFinanceRead(&env); err != nil {
		return nil, err
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	proposals := e.proposals[ownerID]
	var active []finprimitives.FinancialProposal
	for _, p := range proposals {
		if p.Status == finprimitives.StatusActive {
			active = append(active, p)
		}
	}

	return active, nil
}

// normalizeAccounts converts provider accounts to canonical form.
func (e *Engine) normalizeAccounts(circleID string, receipt *read.AccountsReceipt, traceID string, now time.Time) *finprimitives.AccountSnapshot {
	snapshot := &finprimitives.AccountSnapshot{
		SnapshotID:        e.generateID(),
		OwnerType:         "circle",
		OwnerID:           circleID,
		SourceProvider:    receipt.ProviderID,
		Currency:          "USD",
		FetchedAt:         receipt.FetchedAt,
		CreatedAt:         now,
		SchemaVersion:     "1.0",
		NormalizerVersion: "1.0",
		TraceID:           traceID,
		Freshness:         finprimitives.FreshnessCurrent,
	}

	if receipt.Partial {
		snapshot.Freshness = finprimitives.FreshnessPartial
		snapshot.PartialReason = receipt.PartialReason
	}

	for _, acc := range receipt.Accounts {
		normalized := finprimitives.NormalizedAccount{
			AccountID:         e.generateID(),
			ProviderAccountID: acc.AccountID,
			DisplayName:       acc.Name,
			AccountType:       mapAccountType(acc.Type),
			Mask:              acc.Mask,
			Currency:          "USD",
			InstitutionName:   acc.InstitutionName,
		}

		if acc.Balance != nil {
			normalized.BalanceCents = acc.Balance.CurrentCents
			normalized.AvailableCents = acc.Balance.AvailableCents
			normalized.Currency = acc.Balance.Currency
			normalized.BalanceAsOf = acc.Balance.AsOf
			snapshot.TotalBalanceCents += acc.Balance.CurrentCents
		}

		snapshot.Accounts = append(snapshot.Accounts, normalized)
	}

	e.emitAudit("finance.normalized", map[string]string{
		"trace_id":      traceID,
		"snapshot_id":   snapshot.SnapshotID,
		"account_count": fmt.Sprintf("%d", len(snapshot.Accounts)),
	})

	return snapshot
}

// normalizeTransactions converts provider transactions to canonical form.
func (e *Engine) normalizeTransactions(circleID string, receipt *read.TransactionsReceipt, traceID string, now time.Time) []finprimitives.TransactionRecord {
	var records []finprimitives.TransactionRecord

	for _, tx := range receipt.Transactions {
		// Categorize
		catResult := e.categorizer.Categorize(tx.MerchantName, tx.Name)

		record := finprimitives.TransactionRecord{
			RecordID:              e.generateID(),
			OwnerType:             "circle",
			OwnerID:               circleID,
			SourceProvider:        receipt.ProviderID,
			ProviderTransactionID: tx.TransactionID,
			AccountID:             tx.AccountID,
			Date:                  tx.Date,
			AmountCents:           tx.AmountCents,
			Currency:              tx.Currency,
			Description:           tx.Name,
			MerchantName:          tx.MerchantName,
			Category:              catResult.Category,
			CategoryID:            catResult.CategoryID,
			Categorization:        catResult,
			Pending:               tx.Pending,
			PaymentChannel:        tx.PaymentChannel,
			CreatedAt:             now,
			SchemaVersion:         "1.0",
			NormalizerVersion:     "1.0",
			TraceID:               traceID,
		}

		if tx.PostedDate != nil {
			record.PostedDate = tx.PostedDate
		}

		records = append(records, record)
	}

	return records
}

// generateObservations creates observations from transactions.
func (e *Engine) generateObservations(ownerType, ownerID string, transactions []finprimitives.TransactionRecord, windowStart, windowEnd time.Time, traceID string) []finprimitives.FinancialObservation {
	var observations []finprimitives.FinancialObservation
	now := e.clockFunc()

	// Calculate category totals
	categoryTotals := make(map[string]int64)
	for _, tx := range transactions {
		if tx.AmountCents < 0 {
			categoryTotals[tx.Category] += -tx.AmountCents
		}
	}

	// Generate observations for significant categories
	for category, total := range categoryTotals {
		if total > 10000 { // $100 threshold
			obs := finprimitives.FinancialObservation{
				ObservationID: e.generateID(),
				OwnerType:     ownerType,
				OwnerID:       ownerID,
				Type:          finprimitives.ObservationCategoryShift,
				Title:         fmt.Sprintf("%s spending this period", category),
				Description:   fmt.Sprintf("%s spending totaled $%.2f during this period.", category, float64(total)/100),
				Basis:         []string{fmt.Sprintf("Total from %d transactions", len(transactions))},
				Assumptions:   []string{"Based on available transaction data"},
				Limitations:   []string{"May not include all accounts"},
				Category:      category,
				WindowStart:   windowStart,
				WindowEnd:     windowEnd,
				Severity:      finprimitives.SeverityInfo,
				CreatedAt:     now,
				ExpiresAt:     now.AddDate(0, 0, 30),
				SchemaVersion: "1.0",
				TraceID:       traceID,
			}

			numVal := total
			obs.NumericValue = &numVal
			obs.Fingerprint = computeObservationFingerprint(obs)

			if total > 50000 { // $500
				obs.Severity = finprimitives.SeverityNotable
			}
			if total > 100000 { // $1000
				obs.Severity = finprimitives.SeveritySignificant
			}

			observations = append(observations, obs)
		}
	}

	return observations
}

// generateID creates a unique ID.
func (e *Engine) generateID() string {
	e.mu.Lock()
	e.idCounter++
	id := e.idCounter
	e.mu.Unlock()

	data := fmt.Sprintf("fin-%d-%d", id, time.Now().UnixNano())
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("fin-%s", hex.EncodeToString(hash[:8]))
}

// emitAudit emits an audit event.
func (e *Engine) emitAudit(eventType string, metadata map[string]string) {
	if e.auditFunc != nil {
		e.auditFunc(eventType, metadata)
	}
}

// convertToNormalizedResults converts transactions to normalize.NormalizedTransactionResult for reconciliation.
// v8.4: Computes canonical IDs and match keys for deduplication and pending→posted merging.
func (e *Engine) convertToNormalizedResults(transactions []finprimitives.TransactionRecord) []normalize.NormalizedTransactionResult {
	results := make([]normalize.NormalizedTransactionResult, 0, len(transactions))

	for _, tx := range transactions {
		// Compute canonical transaction ID
		canonicalID := finprimitives.CanonicalTransactionID(finprimitives.TransactionIdentityInput{
			Provider:              tx.SourceProvider,
			ProviderAccountID:     tx.AccountID,
			ProviderTransactionID: tx.ProviderTransactionID,
			Date:                  tx.Date,
			AmountMinorUnits:      tx.AmountCents,
			Currency:              tx.Currency,
			MerchantNormalized:    finprimitives.NormalizeMerchant(tx.MerchantName),
		})

		// Compute match key for pending→posted matching
		matchKey := finprimitives.TransactionMatchKey(finprimitives.TransactionMatchInput{
			CanonicalAccountID: tx.AccountID,
			AmountMinorUnits:   tx.AmountCents,
			Currency:           tx.Currency,
			MerchantNormalized: finprimitives.NormalizeMerchant(tx.MerchantName),
		})

		results = append(results, normalize.NormalizedTransactionResult{
			Transaction:           tx,
			CanonicalID:           canonicalID,
			MatchKey:              matchKey,
			ProviderTransactionID: tx.ProviderTransactionID,
			IsPending:             tx.Pending,
		})
	}

	return results
}

// Helper functions

func mapAccountType(t read.AccountType) finprimitives.NormalizedAccountType {
	switch t {
	case read.AccountTypeChecking, read.AccountTypeSavings:
		return finprimitives.AccountTypeDepository
	case read.AccountTypeCredit:
		return finprimitives.AccountTypeCredit
	case read.AccountTypeLoan:
		return finprimitives.AccountTypeLoan
	case read.AccountTypeInvestment:
		return finprimitives.AccountTypeInvestment
	default:
		return finprimitives.AccountTypeOther
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func computeObservationFingerprint(obs finprimitives.FinancialObservation) string {
	data := fmt.Sprintf("obs:%s:%s:%s:%s", obs.Type, obs.OwnerType, obs.OwnerID, obs.Category)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16])
}

// inMemoryDismissalStore implements propose.DismissalStore.
type inMemoryDismissalStore struct {
	engine        *Engine
	lastGenerated map[string]time.Time
	mu            sync.RWMutex
}

func (s *inMemoryDismissalStore) IsDismissed(fingerprint string) bool {
	s.engine.mu.RLock()
	defer s.engine.mu.RUnlock()
	_, exists := s.engine.dismissals[fingerprint]
	return exists
}

func (s *inMemoryDismissalStore) GetLastGenerated(fingerprint string) *time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.lastGenerated == nil {
		return nil
	}
	if t, ok := s.lastGenerated[fingerprint]; ok {
		return &t
	}
	return nil
}

func (s *inMemoryDismissalStore) RecordGenerated(fingerprint string, at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastGenerated == nil {
		s.lastGenerated = make(map[string]time.Time)
	}
	s.lastGenerated[fingerprint] = at
}

// Verify interface compliance.
var _ finance.Reader = (*Engine)(nil)
var _ propose.DismissalStore = (*inMemoryDismissalStore)(nil)
