package plaid

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is a minimal HTTP client for Plaid read-only APIs.
// CRITICAL: This client only implements read operations.
// No payment or transfer methods exist by design.
type Client struct {
	httpClient *http.Client
	baseURL    string
	clientID   string
	// secret is SENSITIVE - never logged
	secret string
}

// ClientConfig configures the Plaid client.
type ClientConfig struct {
	// Environment is "sandbox", "development", or "production"
	Environment string

	// ClientID is the Plaid client ID.
	ClientID string

	// Secret is the Plaid secret.
	// SENSITIVE: Never log this value.
	Secret string

	// HTTPClient is an optional custom HTTP client (for testing).
	HTTPClient *http.Client

	// Timeout is the request timeout.
	Timeout time.Duration
}

// Plaid API endpoints.
const (
	// Sandbox endpoint
	sandboxBaseURL = "https://sandbox.plaid.com"

	// Development endpoint
	developmentBaseURL = "https://development.plaid.com"

	// Production endpoint
	productionBaseURL = "https://production.plaid.com"
)

// NewClient creates a new Plaid client.
// CRITICAL: Only read operations are possible.
func NewClient(config ClientConfig) (*Client, error) {
	if config.ClientID == "" || config.Secret == "" {
		return nil, ErrNotConfigured
	}

	// Determine base URL based on environment
	var baseURL string
	switch toLower(config.Environment) {
	case "production":
		baseURL = productionBaseURL
	case "development":
		baseURL = developmentBaseURL
	default:
		// Default to sandbox for safety
		baseURL = sandboxBaseURL
	}

	httpClient := config.HTTPClient
	if httpClient == nil {
		timeout := config.Timeout
		if timeout == 0 {
			timeout = 30 * time.Second
		}
		httpClient = &http.Client{Timeout: timeout}
	}

	return &Client{
		httpClient: httpClient,
		baseURL:    baseURL,
		clientID:   config.ClientID,
		secret:     config.Secret,
	}, nil
}

// CreateLinkToken creates a Link token for initializing Plaid Link.
// CRITICAL: Only read products are allowed.
func (c *Client) CreateLinkToken(ctx context.Context, req LinkTokenCreateRequest) (*LinkTokenCreateResponse, error) {
	// Validate products - reject any payment/transfer products
	for _, product := range req.Products {
		if IsForbiddenPlaidProduct(product) {
			return nil, ErrForbiddenProduct
		}
	}

	body := map[string]interface{}{
		"client_name":   req.ClientName,
		"language":      req.Language,
		"country_codes": req.CountryCodes,
		"user":          req.User,
		"products":      req.Products,
	}
	if req.RedirectURI != "" {
		body["redirect_uri"] = req.RedirectURI
	}

	return doPost[LinkTokenCreateResponse](ctx, c, "/link/token/create", body)
}

// ExchangePublicToken exchanges a public token for an access token.
func (c *Client) ExchangePublicToken(ctx context.Context, publicToken string) (*ItemPublicTokenExchangeResponse, error) {
	body := map[string]interface{}{
		"public_token": publicToken,
	}
	return doPost[ItemPublicTokenExchangeResponse](ctx, c, "/item/public_token/exchange", body)
}

// GetAccounts fetches accounts from Plaid.
// This is a READ-ONLY operation.
func (c *Client) GetAccounts(ctx context.Context, accessToken string) (*AccountsGetResponse, error) {
	body := map[string]interface{}{
		"access_token": accessToken,
	}
	return doPost[AccountsGetResponse](ctx, c, "/accounts/get", body)
}

// GetTransactions fetches transactions from Plaid.
// This is a READ-ONLY operation.
func (c *Client) GetTransactions(ctx context.Context, accessToken string, startDate, endDate time.Time, opts *TransactionsGetOptions) (*TransactionsGetResponse, error) {
	body := map[string]interface{}{
		"access_token": accessToken,
		"start_date":   FormatDate(startDate),
		"end_date":     FormatDate(endDate),
	}
	if opts != nil {
		options := map[string]interface{}{}
		if len(opts.AccountIDs) > 0 {
			options["account_ids"] = opts.AccountIDs
		}
		if opts.Count > 0 {
			options["count"] = opts.Count
		}
		if opts.Offset > 0 {
			options["offset"] = opts.Offset
		}
		if len(options) > 0 {
			body["options"] = options
		}
	}
	return doPost[TransactionsGetResponse](ctx, c, "/transactions/get", body)
}

// GetInstitution fetches institution details.
// This is a READ-ONLY operation.
func (c *Client) GetInstitution(ctx context.Context, institutionID string, countryCodes []string) (*InstitutionsGetByIDResponse, error) {
	body := map[string]interface{}{
		"institution_id": institutionID,
		"country_codes":  countryCodes,
	}
	return doPost[InstitutionsGetByIDResponse](ctx, c, "/institutions/get_by_id", body)
}

// doPost performs a POST request with JSON body and decodes the response.
func doPost[Resp any](ctx context.Context, c *Client, path string, reqBody map[string]interface{}) (*Resp, error) {
	// Add client credentials to request
	reqBody["client_id"] = c.clientID
	reqBody["secret"] = c.secret

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+path, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Plaid-Version", "2020-09-14")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result Resp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// parseError parses an error response from Plaid.
func (c *Client) parseError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	apiErr := &APIError{
		StatusCode: resp.StatusCode,
	}

	var errResp PlaidErrorResponse
	if err := json.Unmarshal(body, &errResp); err == nil {
		apiErr.ErrorType = errResp.ErrorType
		apiErr.ErrorCode = errResp.ErrorCode
		apiErr.ErrorMessage = errResp.ErrorMessage
		apiErr.RequestID = errResp.RequestID
	} else {
		apiErr.ErrorMessage = string(body)
	}

	// Map to specific errors
	switch apiErr.ErrorType {
	case "INVALID_ACCESS_TOKEN":
		return ErrInvalidToken
	case "RATE_LIMIT_EXCEEDED":
		return ErrRateLimited
	case "ITEM_ERROR":
		if apiErr.ErrorCode == "ITEM_LOGIN_REQUIRED" {
			return ErrItemLoginRequired
		}
	case "INSTITUTION_ERROR":
		if apiErr.ErrorCode == "INSTITUTION_NOT_SUPPORTED" {
			return ErrInstitutionNotSupported
		}
	}

	return apiErr
}

// SetBaseURL sets the base URL for the client (for testing).
func (c *Client) SetBaseURL(url string) {
	c.baseURL = url
}
