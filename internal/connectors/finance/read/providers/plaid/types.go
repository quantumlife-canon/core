package plaid

import (
	"time"
)

// Plaid API request/response types.
// Only READ fields are included — no payment or transfer-related fields exist.

// AccountsGetRequest is the request for /accounts/get.
type AccountsGetRequest struct {
	AccessToken string `json:"access_token"`
}

// AccountsGetResponse is the response from /accounts/get.
type AccountsGetResponse struct {
	Accounts  []PlaidAccount `json:"accounts"`
	Item      PlaidItem      `json:"item"`
	RequestID string         `json:"request_id"`
}

// PlaidAccount represents an account from Plaid.
// CRITICAL: Only read fields are mapped. No payment or transfer fields exist.
type PlaidAccount struct {
	AccountID    string        `json:"account_id"`
	Balances     PlaidBalances `json:"balances"`
	Mask         string        `json:"mask"`
	Name         string        `json:"name"`
	OfficialName string        `json:"official_name"`
	Type         string        `json:"type"`    // depository, credit, loan, investment, other
	Subtype      string        `json:"subtype"` // checking, savings, credit card, etc.
}

// PlaidBalances represents account balances.
type PlaidBalances struct {
	Available              *float64 `json:"available"`
	Current                *float64 `json:"current"`
	Limit                  *float64 `json:"limit"`
	IsoCurrencyCode        string   `json:"iso_currency_code"`
	UnofficialCurrencyCode string   `json:"unofficial_currency_code"`
}

// PlaidItem represents a Plaid Item (a connection to a financial institution).
type PlaidItem struct {
	ItemID                string   `json:"item_id"`
	InstitutionID         string   `json:"institution_id"`
	AvailableProducts     []string `json:"available_products"`
	BilledProducts        []string `json:"billed_products"`
	ConsentExpirationTime *string  `json:"consent_expiration_time"`
}

// TransactionsGetRequest is the request for /transactions/get.
type TransactionsGetRequest struct {
	AccessToken string                  `json:"access_token"`
	StartDate   string                  `json:"start_date"` // YYYY-MM-DD
	EndDate     string                  `json:"end_date"`   // YYYY-MM-DD
	Options     *TransactionsGetOptions `json:"options,omitempty"`
}

// TransactionsGetOptions contains optional parameters for transactions.
type TransactionsGetOptions struct {
	AccountIDs                         []string `json:"account_ids,omitempty"`
	Count                              int      `json:"count,omitempty"`
	Offset                             int      `json:"offset,omitempty"`
	IncludePersonalFinanceCategoryBeta bool     `json:"include_personal_finance_category_beta,omitempty"`
}

// TransactionsGetResponse is the response from /transactions/get.
type TransactionsGetResponse struct {
	Accounts          []PlaidAccount     `json:"accounts"`
	Transactions      []PlaidTransaction `json:"transactions"`
	TotalTransactions int                `json:"total_transactions"`
	Item              PlaidItem          `json:"item"`
	RequestID         string             `json:"request_id"`
}

// PlaidTransaction represents a transaction from Plaid.
// CRITICAL: Only read fields are mapped. No payment or transfer fields exist.
type PlaidTransaction struct {
	TransactionID           string                   `json:"transaction_id"`
	AccountID               string                   `json:"account_id"`
	Amount                  float64                  `json:"amount"`
	IsoCurrencyCode         string                   `json:"iso_currency_code"`
	UnofficialCurrencyCode  string                   `json:"unofficial_currency_code"`
	Date                    string                   `json:"date"` // YYYY-MM-DD
	Name                    string                   `json:"name"`
	MerchantName            string                   `json:"merchant_name"`
	Pending                 bool                     `json:"pending"`
	PendingTransactionID    string                   `json:"pending_transaction_id"`
	Category                []string                 `json:"category"`
	CategoryID              string                   `json:"category_id"`
	PaymentChannel          string                   `json:"payment_channel"` // online, in store, other
	PersonalFinanceCategory *PersonalFinanceCategory `json:"personal_finance_category,omitempty"`
	Location                *TransactionLocation     `json:"location,omitempty"`
}

// PersonalFinanceCategory contains Plaid's personal finance categorization.
type PersonalFinanceCategory struct {
	Primary  string `json:"primary"`
	Detailed string `json:"detailed"`
}

// TransactionLocation contains location data for a transaction.
type TransactionLocation struct {
	Address     string  `json:"address"`
	City        string  `json:"city"`
	Region      string  `json:"region"`
	PostalCode  string  `json:"postal_code"`
	Country     string  `json:"country"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
	StoreNumber string  `json:"store_number"`
}

// LinkTokenCreateRequest is the request for /link/token/create.
type LinkTokenCreateRequest struct {
	ClientName   string        `json:"client_name"`
	Language     string        `json:"language"`
	CountryCodes []string      `json:"country_codes"`
	User         LinkTokenUser `json:"user"`
	Products     []string      `json:"products"`
	RedirectURI  string        `json:"redirect_uri,omitempty"`
}

// LinkTokenUser identifies the user for Link.
type LinkTokenUser struct {
	ClientUserID string `json:"client_user_id"`
}

// LinkTokenCreateResponse is the response from /link/token/create.
type LinkTokenCreateResponse struct {
	LinkToken  string    `json:"link_token"`
	Expiration time.Time `json:"expiration"`
	RequestID  string    `json:"request_id"`
}

// ItemPublicTokenExchangeRequest is the request for /item/public_token/exchange.
type ItemPublicTokenExchangeRequest struct {
	PublicToken string `json:"public_token"`
}

// ItemPublicTokenExchangeResponse is the response from /item/public_token/exchange.
type ItemPublicTokenExchangeResponse struct {
	AccessToken string `json:"access_token"`
	ItemID      string `json:"item_id"`
	RequestID   string `json:"request_id"`
}

// InstitutionsGetByIDRequest is the request for /institutions/get_by_id.
type InstitutionsGetByIDRequest struct {
	InstitutionID string   `json:"institution_id"`
	CountryCodes  []string `json:"country_codes"`
}

// InstitutionsGetByIDResponse is the response from /institutions/get_by_id.
type InstitutionsGetByIDResponse struct {
	Institution PlaidInstitution `json:"institution"`
	RequestID   string           `json:"request_id"`
}

// PlaidInstitution represents a financial institution.
type PlaidInstitution struct {
	InstitutionID string   `json:"institution_id"`
	Name          string   `json:"name"`
	Products      []string `json:"products"`
	CountryCodes  []string `json:"country_codes"`
	URL           string   `json:"url"`
	Logo          string   `json:"logo"`
	PrimaryColor  string   `json:"primary_color"`
}

// PlaidErrorResponse is the error response format from Plaid.
type PlaidErrorResponse struct {
	ErrorType      string `json:"error_type"`
	ErrorCode      string `json:"error_code"`
	ErrorMessage   string `json:"error_message"`
	DisplayMessage string `json:"display_message"`
	RequestID      string `json:"request_id"`
}

// Plaid products.
// CRITICAL: Only read products are allowed. Payment products MUST be rejected.
const (
	// ProductTransactions allows reading transaction history.
	ProductTransactions = "transactions"

	// ProductAuth allows reading account and routing numbers (read-only).
	ProductAuth = "auth"

	// ProductIdentity allows reading account holder identity.
	ProductIdentity = "identity"

	// ProductAssets allows reading asset reports.
	ProductAssets = "assets"

	// ProductInvestments allows reading investment account data.
	ProductInvestments = "investments"

	// ProductLiabilities allows reading liability account data.
	ProductLiabilities = "liabilities"
)

// AllowedPlaidProducts is the exhaustive list of permitted Plaid products.
// CRITICAL: Only read products. No payment or transfer products exist.
var AllowedPlaidProducts = []string{
	ProductTransactions,
	ProductAuth,
	ProductIdentity,
	ProductAssets,
	ProductInvestments,
	ProductLiabilities,
}

// ForbiddenPlaidProductPatterns are patterns that MUST be rejected.
// CRITICAL: These prevent any payment/transfer capability from being requested.
var ForbiddenPlaidProductPatterns = []string{
	"payment",
	"transfer",
	"signal",
	"income",         // Income verification can trigger actions
	"employment",     // Employment verification can trigger actions
	"deposit_switch", // Account switching is a write action
	"standing_order",
}

// DefaultReadProducts returns the default products for read-only access.
func DefaultReadProducts() []string {
	return []string{
		ProductTransactions,
	}
}

// IsAllowedPlaidProduct checks if a product is in the allowed list.
func IsAllowedPlaidProduct(product string) bool {
	for _, allowed := range AllowedPlaidProducts {
		if product == allowed {
			return true
		}
	}
	return false
}

// IsForbiddenPlaidProduct checks if a product matches any forbidden pattern.
func IsForbiddenPlaidProduct(product string) bool {
	for _, pattern := range ForbiddenPlaidProductPatterns {
		if containsPattern(product, pattern) {
			return true
		}
	}
	return false
}

// containsPattern checks if s contains pattern (case-insensitive).
func containsPattern(s, pattern string) bool {
	sLower := toLower(s)
	pLower := toLower(pattern)
	for i := 0; i <= len(sLower)-len(pLower); i++ {
		if sLower[i:i+len(pLower)] == pLower {
			return true
		}
	}
	return false
}

// toLower converts a string to lowercase.
func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		b[i] = c
	}
	return string(b)
}

// ParseDate parses a Plaid date string (YYYY-MM-DD).
func ParseDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}

// FormatDate formats a time as a Plaid date string (YYYY-MM-DD).
func FormatDate(t time.Time) string {
	return t.Format("2006-01-02")
}
