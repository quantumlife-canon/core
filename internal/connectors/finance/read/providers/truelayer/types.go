package truelayer

import (
	"time"
)

// TrueLayer API response types.
// Only READ fields are included — no payment or write-related fields exist.

// AccountsResponse is the TrueLayer accounts endpoint response.
type AccountsResponse struct {
	Results []TrueLayerAccount `json:"results"`
	Status  string             `json:"status"`
}

// TrueLayerAccount represents an account from TrueLayer.
// CRITICAL: Only read fields are mapped. No payment or write fields exist.
type TrueLayerAccount struct {
	AccountID       string          `json:"account_id"`
	AccountType     string          `json:"account_type"` // TRANSACTION, SAVINGS, BUSINESS_TRANSACTION, BUSINESS_SAVINGS
	DisplayName     string          `json:"display_name"`
	Currency        string          `json:"currency"`
	Provider        AccountProvider `json:"provider"`
	AccountNumber   *AccountNumber  `json:"account_number,omitempty"`
	UpdateTimestamp string          `json:"update_timestamp"`
}

// AccountProvider contains provider metadata.
type AccountProvider struct {
	ProviderID  string `json:"provider_id"`
	DisplayName string `json:"display_name"`
	LogoURI     string `json:"logo_uri,omitempty"`
}

// AccountNumber contains masked account identifiers.
type AccountNumber struct {
	IBAN     string `json:"iban,omitempty"`
	SwiftBIC string `json:"swift_bic,omitempty"`
	Number   string `json:"number,omitempty"`
	SortCode string `json:"sort_code,omitempty"`
}

// BalanceResponse is the TrueLayer balance endpoint response.
type BalanceResponse struct {
	Results []TrueLayerBalance `json:"results"`
	Status  string             `json:"status"`
}

// TrueLayerBalance represents an account balance.
type TrueLayerBalance struct {
	Currency        string  `json:"currency"`
	Available       float64 `json:"available"`
	Current         float64 `json:"current"`
	Overdraft       float64 `json:"overdraft,omitempty"`
	UpdateTimestamp string  `json:"update_timestamp"`
}

// TransactionsResponse is the TrueLayer transactions endpoint response.
type TransactionsResponse struct {
	Results []TrueLayerTransaction `json:"results"`
	Status  string                 `json:"status"`
}

// TrueLayerTransaction represents a transaction from TrueLayer.
// CRITICAL: Only read fields are mapped. No payment or write fields exist.
type TrueLayerTransaction struct {
	TransactionID             string           `json:"transaction_id"`
	Timestamp                 string           `json:"timestamp"`
	Description               string           `json:"description"`
	Amount                    float64          `json:"amount"`
	Currency                  string           `json:"currency"`
	TransactionType           string           `json:"transaction_type"` // DEBIT, CREDIT
	TransactionCategory       string           `json:"transaction_category"`
	TransactionClassification []string         `json:"transaction_classification,omitempty"`
	MerchantName              string           `json:"merchant_name,omitempty"`
	RunningBalance            *RunningBalance  `json:"running_balance,omitempty"`
	Meta                      *TransactionMeta `json:"meta,omitempty"`
}

// RunningBalance represents the balance after a transaction.
type RunningBalance struct {
	Currency string  `json:"currency"`
	Amount   float64 `json:"amount"`
}

// TransactionMeta contains additional transaction metadata.
type TransactionMeta struct {
	ProviderTransactionCategory string `json:"provider_transaction_category,omitempty"`
	ProviderReference           string `json:"provider_reference,omitempty"`
}

// TokenResponse is the OAuth token endpoint response.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// ErrorResponse is the TrueLayer error response format.
type ErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// TrueLayer OAuth scopes.
// CRITICAL: Only read scopes are allowed. Payment scopes MUST be rejected.
const (
	// ScopeAccounts allows reading account information.
	ScopeAccounts = "accounts"

	// ScopeBalance allows reading account balances.
	ScopeBalance = "balance"

	// ScopeTransactions allows reading transaction history.
	ScopeTransactions = "transactions"

	// ScopeInfo allows reading account holder info.
	ScopeInfo = "info"

	// ScopeOfflineAccess allows refresh token usage.
	ScopeOfflineAccess = "offline_access"
)

// AllowedTrueLayerScopes is the exhaustive list of permitted TrueLayer scopes.
// CRITICAL: Only read scopes. No payment or write scopes exist.
var AllowedTrueLayerScopes = []string{
	ScopeAccounts,
	ScopeBalance,
	ScopeTransactions,
	ScopeInfo,
	ScopeOfflineAccess,
}

// ForbiddenTrueLayerScopePatterns are patterns that MUST be rejected.
// CRITICAL: These prevent any payment/write capability from being requested.
var ForbiddenTrueLayerScopePatterns = []string{
	"payment",
	"payments",
	"pay",
	"transfer",
	"write",
	"initiate",
	"standing_order",
	"direct_debit",
	"beneficiar",
	"mandate",
}

// DefaultReadScopes returns the default scopes for read-only access.
func DefaultReadScopes() []string {
	return []string{
		ScopeAccounts,
		ScopeBalance,
		ScopeTransactions,
		ScopeOfflineAccess,
	}
}

// ParseTimestamp parses a TrueLayer timestamp.
func ParseTimestamp(ts string) (time.Time, error) {
	// TrueLayer uses ISO 8601 format
	return time.Parse(time.RFC3339, ts)
}
