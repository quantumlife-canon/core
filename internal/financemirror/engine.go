// Package financemirror provides the finance mirror proof engine.
//
// Phase 29: TrueLayer Read-Only Connect (UK Sandbox) + Finance Mirror Proof
// Reference: docs/ADR/ADR-0060-phase29-truelayer-readonly-finance-mirror.md
//
// CRITICAL INVARIANTS:
//   - NO raw account data, NO amounts, NO merchants, NO bank names
//   - Only magnitude buckets and category buckets
//   - Deterministic outputs: sorted inputs, canonical strings, SHA256 hashing
//   - Privacy guard blocks any tokens containing identifiable data
//   - No goroutines. No time.Now() - clock injection only.
//   - stdlib only (net/http, crypto/*, encoding/*).
package financemirror

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"quantumlife/internal/connectors/finance/read"
	"quantumlife/internal/persist"
	"quantumlife/pkg/domain/financemirror"
	"quantumlife/pkg/primitives"
)

// SyncLimits defines the bounded sync parameters.
const (
	MaxAccountsToFetch     = 25
	MaxTransactionsToFetch = 25
	MaxSyncDays            = 7
)

// Engine orchestrates finance mirror operations.
// CRITICAL: No goroutines. No time.Now() - clock injection only.
type Engine struct {
	clock              func() time.Time
	financeMirrorStore *persist.FinanceMirrorStore
	syncReceiptStore   *persist.SyncReceiptStore
}

// NewEngine creates a new finance mirror engine.
func NewEngine(
	clock func() time.Time,
	financeMirrorStore *persist.FinanceMirrorStore,
	syncReceiptStore *persist.SyncReceiptStore,
) *Engine {
	return &Engine{
		clock:              clock,
		financeMirrorStore: financeMirrorStore,
		syncReceiptStore:   syncReceiptStore,
	}
}

// SyncInput contains the parameters for a finance sync.
type SyncInput struct {
	CircleID    string
	AccessToken string // SENSITIVE: Never log
}

// SyncOutput contains the result of a finance sync.
type SyncOutput struct {
	Success bool
	Error   string
	Receipt *financemirror.FinanceSyncReceipt
}

// Sync performs a bounded, privacy-safe sync from TrueLayer.
// CRITICAL: Transforms all data to abstract buckets. NO raw data stored.
func (e *Engine) Sync(ctx context.Context, connector read.ReadConnector, input SyncInput) *SyncOutput {
	now := e.clock()

	// Bounded time range: last 7 days
	endTime := now
	startTime := now.AddDate(0, 0, -MaxSyncDays)

	// Track what we saw (abstract only)
	var accountsCount int
	var transactionsCount int
	var evidenceTokens []string

	// Create execution envelope for read operations
	envelope := e.createReadEnvelope(input.CircleID, now)

	// Fetch accounts (bounded)
	accountsReceipt, err := connector.ListAccounts(ctx, envelope, read.ListAccountsRequest{
		IncludeBalances: true,
	})
	if err != nil {
		return &SyncOutput{
			Success: false,
			Error:   "accounts_fetch_failed",
			Receipt: financemirror.NewFinanceSyncReceipt(
				input.CircleID, "truelayer", now,
				0, 0, nil, false, "accounts_fetch_failed",
			),
		}
	}

	// Apply limit
	accounts := accountsReceipt.Accounts
	if len(accounts) > MaxAccountsToFetch {
		accounts = accounts[:MaxAccountsToFetch]
	}
	accountsCount = len(accounts)

	// Generate evidence tokens for accounts (abstract only)
	for _, acc := range accounts {
		// Create abstract evidence token from account type only
		token := hashForEvidence(fmt.Sprintf("account|%s", acc.Type))
		evidenceTokens = append(evidenceTokens, token)
	}

	// Fetch transactions (bounded)
	txReceipt, err := connector.ListTransactions(ctx, envelope, read.ListTransactionsRequest{
		StartDate: startTime,
		EndDate:   endTime,
		Limit:     MaxTransactionsToFetch,
	})
	if err != nil {
		// Partial success - we have accounts but no transactions
		transactionsCount = 0
	} else {
		txs := txReceipt.Transactions
		if len(txs) > MaxTransactionsToFetch {
			txs = txs[:MaxTransactionsToFetch]
		}
		transactionsCount = len(txs)

		// Generate evidence tokens for transactions (abstract only)
		for _, tx := range txs {
			// Privacy guard: validate no identifiable data leaks
			if err := validatePrivacy(tx); err != nil {
				continue // Skip this transaction, don't include in evidence
			}

			// Create abstract evidence token from category only
			token := hashForEvidence(fmt.Sprintf("tx|%s", abstractCategory(tx.ProviderCategory)))
			evidenceTokens = append(evidenceTokens, token)
		}
	}

	// Create receipt
	receipt := financemirror.NewFinanceSyncReceipt(
		input.CircleID, "truelayer", now,
		accountsCount, transactionsCount, evidenceTokens,
		true, "",
	)

	// Store receipt
	if e.financeMirrorStore != nil {
		_ = e.financeMirrorStore.StoreSyncReceipt(receipt)
	}

	return &SyncOutput{
		Success: true,
		Receipt: receipt,
	}
}

// createReadEnvelope creates an execution envelope for read operations.
func (e *Engine) createReadEnvelope(circleID string, now time.Time) primitives.ExecutionEnvelope {
	traceID := fmt.Sprintf("phase29-sync-%s-%d", circleID, now.Unix())
	return primitives.ExecutionEnvelope{
		TraceID:              traceID,
		Mode:                 primitives.ModeSuggestOnly,
		ActorCircleID:        circleID,
		IntersectionID:       fmt.Sprintf("self:%s", circleID), // Self-intersection for own data
		ContractVersion:      "v1",
		ScopesUsed:           []string{read.ScopeFinanceRead},
		AuthorizationProofID: fmt.Sprintf("phase29-read-%s", circleID),
		IssuedAt:             now,
		ApprovedByHuman:      false, // Read-only, no approval needed
	}
}

// BuildMirrorPage builds the finance mirror proof page.
func (e *Engine) BuildMirrorPage(circleID string, connected bool, lastReceipt *financemirror.FinanceSyncReceipt) *financemirror.FinanceMirrorPage {
	var lastSyncTime time.Time
	var overallMagnitude financemirror.MagnitudeBucket
	var categories []financemirror.CategorySignal

	if lastReceipt != nil && lastReceipt.Success {
		lastSyncTime = lastReceipt.TimeBucket

		// Determine overall magnitude from transactions
		overallMagnitude = lastReceipt.TransactionsMagnitude

		// Build category signals (abstract only)
		categories = e.buildCategorySignals(lastReceipt)
	} else {
		overallMagnitude = financemirror.MagnitudeNothing
	}

	return financemirror.NewFinanceMirrorPage(
		connected, lastSyncTime, overallMagnitude, categories,
	)
}

// buildCategorySignals builds abstract category signals from a receipt.
func (e *Engine) buildCategorySignals(receipt *financemirror.FinanceSyncReceipt) []financemirror.CategorySignal {
	// Create signals based on what was seen (all abstract)
	signals := []financemirror.CategorySignal{}

	// Liquidity: based on accounts magnitude
	if receipt.AccountsMagnitude != financemirror.MagnitudeNothing {
		signals = append(signals, financemirror.CategorySignal{
			Category:  financemirror.CategoryLiquidity,
			Magnitude: receipt.AccountsMagnitude,
			Trend:     "stable", // Default to stable
		})
	}

	// Spend pattern: based on transactions magnitude
	if receipt.TransactionsMagnitude != financemirror.MagnitudeNothing {
		signals = append(signals, financemirror.CategorySignal{
			Category:  financemirror.CategorySpendPattern,
			Magnitude: receipt.TransactionsMagnitude,
			Trend:     "", // No trend info without more data
		})
	}

	return signals
}

// ShouldShowCue determines if the finance mirror cue should be shown.
func (e *Engine) ShouldShowCue(circleID string, connected bool) bool {
	if !connected {
		return false
	}

	// Show cue if connected and we have a recent sync
	if e.financeMirrorStore == nil {
		return false
	}

	receipt := e.financeMirrorStore.GetLatestSyncReceipt(circleID)
	if receipt == nil {
		return false
	}

	// Only show if sync was successful
	return receipt.Success
}

// GetLatestReceipt returns the latest sync receipt for a circle.
func (e *Engine) GetLatestReceipt(circleID string) *financemirror.FinanceSyncReceipt {
	if e.financeMirrorStore == nil {
		return nil
	}
	return e.financeMirrorStore.GetLatestSyncReceipt(circleID)
}

// Privacy validation patterns.
var (
	// Patterns that indicate PII
	emailPattern      = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	phonePattern      = regexp.MustCompile(`\+?[0-9]{10,15}`)
	ibanPattern       = regexp.MustCompile(`[A-Z]{2}[0-9]{2}[A-Z0-9]{4,}`)
	sortCodePattern   = regexp.MustCompile(`[0-9]{2}-[0-9]{2}-[0-9]{2}`)
	accountNumPattern = regexp.MustCompile(`[0-9]{8,}`)
	currencyPattern   = regexp.MustCompile(`[£$€][0-9]`)
)

// validatePrivacy checks that a transaction doesn't contain identifiable data.
// CRITICAL: Returns error if any PII patterns detected.
func validatePrivacy(tx read.Transaction) error {
	// Check all string fields for PII patterns
	fieldsToCheck := []string{
		tx.Name,
		tx.MerchantName,
		tx.ProviderCategory,
	}

	for _, field := range fieldsToCheck {
		if containsPII(field) {
			return fmt.Errorf("field contains potential PII")
		}
	}

	return nil
}

// containsPII checks if a string contains potential PII.
func containsPII(s string) bool {
	if s == "" {
		return false
	}

	// Check for patterns
	if emailPattern.MatchString(s) {
		return true
	}
	if phonePattern.MatchString(s) {
		return true
	}
	if ibanPattern.MatchString(s) {
		return true
	}
	if sortCodePattern.MatchString(s) {
		return true
	}
	if accountNumPattern.MatchString(s) {
		return true
	}
	if currencyPattern.MatchString(s) {
		return true
	}

	// Check for more than 2 contiguous digits (amount-like)
	digitCount := 0
	for _, r := range s {
		if r >= '0' && r <= '9' {
			digitCount++
			if digitCount > 2 {
				return true
			}
		} else {
			digitCount = 0
		}
	}

	return false
}

// abstractCategory maps a provider category to an abstract category.
func abstractCategory(providerCategory string) string {
	cat := strings.ToLower(providerCategory)

	// Map to abstract categories
	switch {
	case strings.Contains(cat, "food") || strings.Contains(cat, "restaurant") || strings.Contains(cat, "grocery"):
		return "essentials"
	case strings.Contains(cat, "transport") || strings.Contains(cat, "travel"):
		return "transport"
	case strings.Contains(cat, "entertainment") || strings.Contains(cat, "recreation"):
		return "leisure"
	case strings.Contains(cat, "bill") || strings.Contains(cat, "utility"):
		return "obligations"
	case strings.Contains(cat, "transfer"):
		return "transfers"
	default:
		return "other"
	}
}

// hashForEvidence creates a short hash for evidence tokens.
func hashForEvidence(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:8]) // 16 hex chars
}

// PrivacyGuard provides methods to validate and sanitize data.
type PrivacyGuard struct{}

// NewPrivacyGuard creates a new privacy guard.
func NewPrivacyGuard() *PrivacyGuard {
	return &PrivacyGuard{}
}

// ValidateInput checks that input doesn't contain forbidden patterns.
func (g *PrivacyGuard) ValidateInput(input string) error {
	if containsPII(input) {
		return fmt.Errorf("input contains forbidden pattern")
	}
	return nil
}

// SanitizeForEvidence creates a safe evidence token from raw data.
func (g *PrivacyGuard) SanitizeForEvidence(category string) string {
	// Only use pre-defined abstract categories
	abstract := abstractCategory(category)
	return hashForEvidence(abstract)
}

// BuildEvidenceVector creates a deterministic evidence vector from tokens.
func BuildEvidenceVector(tokens []string) string {
	if len(tokens) == 0 {
		return "empty"
	}

	// Sort for determinism
	sorted := make([]string, len(tokens))
	copy(sorted, tokens)
	sort.Strings(sorted)

	// Create canonical string
	canonical := fmt.Sprintf("EVIDENCE_VECTOR|v1|%d", len(sorted))
	for _, t := range sorted {
		canonical += "|" + t
	}

	h := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(h[:16])
}
