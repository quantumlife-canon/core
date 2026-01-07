// Package truelayer provides bounded TrueLayer sync for Phase 31.3b.
//
// Phase 31.3b: Real TrueLayer Sync (Accounts + Transactions → Finance Mirror + Commerce Observer)
// Reference: docs/ADR/ADR-0066-phase31-3b-truelayer-real-sync.md
//
// CRITICAL INVARIANTS:
//   - NO goroutines
//   - NO time.Now() - clock injection only
//   - Deterministic output: same inputs + clock = same hashes/receipts
//   - Bounded sync: max 25 accounts, max 25 transactions per account, 7-day window
//   - NO retries - single attempt, fail gracefully
//   - NEVER log secrets (access token, refresh token)
//   - Privacy: only classification fields extracted, no amounts/merchants/timestamps stored
package truelayer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"

	"quantumlife/pkg/domain/financemirror"
)

// Bounded sync limits (Phase 31.3b)
const (
	// MaxAccounts is the maximum number of accounts to fetch.
	MaxAccounts = 25

	// MaxTransactionsPerAccount is the maximum transactions per account.
	MaxTransactionsPerAccount = 25

	// SyncWindowDays is the number of days of transaction history.
	SyncWindowDays = 7

	// ResponseSizeLimit is the maximum response size in bytes (1MB).
	ResponseSizeLimit = 1024 * 1024
)

// SyncService provides bounded TrueLayer sync with clock injection.
// CRITICAL: No goroutines. No time.Now(). Deterministic output.
type SyncService struct {
	client *Client
	clock  func() time.Time
}

// SyncServiceConfig configures the sync service.
type SyncServiceConfig struct {
	// Client is the TrueLayer API client (required).
	Client *Client

	// Clock provides the current time (required for determinism).
	Clock func() time.Time
}

// NewSyncService creates a new sync service.
func NewSyncService(cfg SyncServiceConfig) *SyncService {
	return &SyncService{
		client: cfg.Client,
		clock:  cfg.Clock,
	}
}

// SyncInput contains input for a sync operation.
type SyncInput struct {
	// CircleID identifies the circle being synced.
	CircleID string

	// AccessToken is the OAuth access token.
	// SENSITIVE: Never log this value.
	AccessToken string
}

// SyncOutput contains the result of a sync operation.
// Privacy-preserving: only magnitude buckets and hashes.
type SyncOutput struct {
	// Success indicates whether the sync succeeded.
	Success bool

	// FailReason is set if Success is false (no PII).
	FailReason string

	// AccountsCount is the raw count (converted to magnitude in receipt).
	AccountsCount int

	// TransactionsCount is the raw count (converted to magnitude in receipt).
	TransactionsCount int

	// TransactionData contains classified transaction data for commerce observer.
	// CRITICAL: Only classification fields - no amounts, merchants, or raw timestamps.
	TransactionData []TransactionClassification

	// EvidenceTokens for receipt hashing (abstract only).
	EvidenceTokens []string

	// SyncTime is when the sync occurred.
	SyncTime time.Time
}

// TransactionClassification contains only the fields needed for classification.
// CRITICAL: No amounts, merchant names, or raw timestamps.
type TransactionClassification struct {
	// TransactionID is the bank transaction ID (will be hashed, never stored raw).
	TransactionID string

	// ProviderCategory is the bank-assigned category (e.g., "FOOD_AND_DRINK").
	ProviderCategory string

	// ProviderCategoryID is the MCC code or similar (e.g., "5812").
	ProviderCategoryID string

	// PaymentChannel is the payment type (e.g., "debit", "credit").
	PaymentChannel string
}

// Sync performs a bounded sync of TrueLayer data.
// CRITICAL: No retries. Single attempt. Fail gracefully.
func (s *SyncService) Sync(ctx context.Context, input SyncInput) (*SyncOutput, error) {
	now := s.clock()

	if input.AccessToken == "" {
		return &SyncOutput{
			Success:    false,
			FailReason: "no_access_token",
			SyncTime:   now,
		}, nil
	}

	// Fetch accounts
	accountsResp, err := s.client.GetAccounts(ctx, input.AccessToken)
	if err != nil {
		return &SyncOutput{
			Success:    false,
			FailReason: "accounts_fetch_failed",
			SyncTime:   now,
		}, nil
	}

	// Extract and bound accounts
	accounts := accountsResp.Results
	if len(accounts) > MaxAccounts {
		// Sort for determinism before truncation
		sort.Slice(accounts, func(i, j int) bool {
			return accounts[i].AccountID < accounts[j].AccountID
		})
		accounts = accounts[:MaxAccounts]
	}

	// Calculate date range (7-day window)
	toDate := now
	fromDate := now.AddDate(0, 0, -SyncWindowDays)

	// Fetch transactions for each account (bounded)
	var allTxData []TransactionClassification
	var evidenceTokens []string

	for _, account := range accounts {
		txResp, err := s.client.GetTransactions(ctx, input.AccessToken, account.AccountID, fromDate, toDate)
		if err != nil {
			// Continue with partial data on individual account failure
			continue
		}

		// Extract ONLY classification fields - no amounts, merchants, or timestamps
		txData := extractTransactionClassifications(txResp.Results)

		// Apply per-account limit
		if len(txData) > MaxTransactionsPerAccount {
			txData = txData[:MaxTransactionsPerAccount]
		}

		allTxData = append(allTxData, txData...)

		// Add account type to evidence tokens (abstract only)
		if account.AccountType != "" {
			evidenceTokens = append(evidenceTokens, "account_type|"+account.AccountType)
		}
	}

	// Sort transactions deterministically by ID for consistent hashing
	sort.Slice(allTxData, func(i, j int) bool {
		return allTxData[i].TransactionID < allTxData[j].TransactionID
	})

	// Deduplicate and sort evidence tokens
	evidenceTokens = deduplicateStrings(evidenceTokens)

	return &SyncOutput{
		Success:           true,
		AccountsCount:     len(accounts),
		TransactionsCount: len(allTxData),
		TransactionData:   allTxData,
		EvidenceTokens:    evidenceTokens,
		SyncTime:          now,
	}, nil
}

// extractTransactionClassifications extracts only classification fields from transactions.
// CRITICAL: No amounts, merchant names, or raw timestamps.
func extractTransactionClassifications(txs []TrueLayerTransaction) []TransactionClassification {
	result := make([]TransactionClassification, 0, len(txs))
	for _, tx := range txs {
		classification := TransactionClassification{
			TransactionID:      tx.TransactionID,
			ProviderCategory:   tx.TransactionCategory,
			ProviderCategoryID: firstString(tx.TransactionClassification),
			PaymentChannel:     mapPaymentChannel(tx.TransactionType),
		}
		result = append(result, classification)
	}

	// Sort deterministically
	sort.Slice(result, func(i, j int) bool {
		return result[i].TransactionID < result[j].TransactionID
	})

	return result
}

// BuildSyncReceipt builds a FinanceSyncReceipt from sync output.
func BuildSyncReceipt(circleID string, output *SyncOutput) *financemirror.FinanceSyncReceipt {
	if output == nil {
		return nil
	}

	failReason := ""
	if !output.Success {
		failReason = output.FailReason
	}

	return financemirror.NewFinanceSyncReceipt(
		circleID,
		"truelayer", // provider
		output.SyncTime,
		output.AccountsCount,
		output.TransactionsCount,
		output.EvidenceTokens,
		output.Success,
		failReason,
	)
}

// ComputeSyncHash computes a deterministic hash of sync output.
func ComputeSyncHash(output *SyncOutput) string {
	if output == nil {
		return ""
	}

	// Build canonical string from abstract fields only
	canonical := fmt.Sprintf("SYNC|v1|%t|%d|%d|%s",
		output.Success,
		output.AccountsCount,
		output.TransactionsCount,
		output.SyncTime.UTC().Format(time.RFC3339),
	)

	h := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(h[:])
}

// Helper functions

func firstString(s []string) string {
	if len(s) > 0 {
		return s[0]
	}
	return ""
}

func mapPaymentChannel(txType string) string {
	// Map TrueLayer transaction types to payment channels
	switch txType {
	case "DEBIT":
		return "debit"
	case "CREDIT":
		return "credit"
	default:
		return "unknown"
	}
}

func deduplicateStrings(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	result := make([]string, 0, len(s))
	for _, item := range s {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	// Sort for determinism
	sort.Strings(result)
	return result
}
