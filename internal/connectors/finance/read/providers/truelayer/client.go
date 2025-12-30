package truelayer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is a minimal HTTP client for TrueLayer read-only APIs.
// CRITICAL: This client only implements read operations.
// No payment or write methods exist by design.
type Client struct {
	httpClient *http.Client
	baseURL    string
	authURL    string
	tokenURL   string
	clientID   string
	// clientSecret is SENSITIVE - never logged
	clientSecret string
}

// ClientConfig configures the TrueLayer client.
type ClientConfig struct {
	// Environment is "sandbox" or "live"
	Environment string

	// ClientID is the TrueLayer client ID.
	ClientID string

	// ClientSecret is the TrueLayer client secret.
	// SENSITIVE: Never log this value.
	ClientSecret string

	// HTTPClient is an optional custom HTTP client (for testing).
	HTTPClient *http.Client

	// Timeout is the request timeout.
	Timeout time.Duration
}

// TrueLayer API endpoints.
const (
	// Sandbox endpoints
	sandboxAuthURL = "https://auth.truelayer-sandbox.com"
	sandboxAPIURL  = "https://api.truelayer-sandbox.com"

	// Production endpoints
	liveAuthURL = "https://auth.truelayer.com"
	liveAPIURL  = "https://api.truelayer.com"
)

// NewClient creates a new TrueLayer client.
// CRITICAL: Only read operations are possible.
func NewClient(config ClientConfig) (*Client, error) {
	if config.ClientID == "" || config.ClientSecret == "" {
		return nil, ErrNotConfigured
	}

	// Determine endpoints based on environment
	var baseURL, authURL string
	switch strings.ToLower(config.Environment) {
	case "live", "production":
		baseURL = liveAPIURL
		authURL = liveAuthURL
	default:
		// Default to sandbox for safety
		baseURL = sandboxAPIURL
		authURL = sandboxAuthURL
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
		httpClient:   httpClient,
		baseURL:      baseURL,
		authURL:      authURL,
		tokenURL:     authURL + "/connect/token",
		clientID:     config.ClientID,
		clientSecret: config.ClientSecret,
	}, nil
}

// AuthorizationURL generates the OAuth authorization URL.
// CRITICAL: Only read scopes are included.
func (c *Client) AuthorizationURL(redirectURI, state string, scopes []string) (string, error) {
	// Validate scopes - reject any payment/write scopes
	for _, scope := range scopes {
		if isForbiddenTrueLayerScope(scope) {
			return "", ErrForbiddenScope
		}
	}

	params := url.Values{
		"response_type": {"code"},
		"client_id":     {c.clientID},
		"redirect_uri":  {redirectURI},
		"state":         {state},
		"scope":         {strings.Join(scopes, " ")},
		// Enable all providers for Open Banking
		"providers": {"uk-ob-all uk-oauth-all"},
	}

	return c.authURL + "/?" + params.Encode(), nil
}

// ExchangeCode exchanges an authorization code for tokens.
func (c *Client) ExchangeCode(ctx context.Context, code, redirectURI string) (*TokenResponse, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
		"code":          {code},
		"redirect_uri":  {redirectURI},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	return &tokenResp, nil
}

// RefreshToken refreshes an access token.
func (c *Client) RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	data := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
		"refresh_token": {refreshToken},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token refresh failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	return &tokenResp, nil
}

// GetAccounts fetches accounts from TrueLayer.
// This is a READ-ONLY operation.
func (c *Client) GetAccounts(ctx context.Context, accessToken string) (*AccountsResponse, error) {
	return doGet[AccountsResponse](ctx, c, accessToken, "/data/v1/accounts")
}

// GetBalance fetches balance for an account.
// This is a READ-ONLY operation.
func (c *Client) GetBalance(ctx context.Context, accessToken, accountID string) (*BalanceResponse, error) {
	path := fmt.Sprintf("/data/v1/accounts/%s/balance", accountID)
	return doGet[BalanceResponse](ctx, c, accessToken, path)
}

// GetTransactions fetches transactions for an account.
// This is a READ-ONLY operation.
func (c *Client) GetTransactions(ctx context.Context, accessToken, accountID string, from, to time.Time) (*TransactionsResponse, error) {
	path := fmt.Sprintf("/data/v1/accounts/%s/transactions", accountID)

	// Add date range parameters
	params := url.Values{}
	if !from.IsZero() {
		params.Set("from", from.Format(time.RFC3339))
	}
	if !to.IsZero() {
		params.Set("to", to.Format(time.RFC3339))
	}

	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	return doGet[TransactionsResponse](ctx, c, accessToken, path)
}

// doGet performs a GET request and decodes the response.
func doGet[T any](ctx context.Context, c *Client, accessToken, path string) (*T, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// parseError parses an error response from TrueLayer.
func (c *Client) parseError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	apiErr := &APIError{
		StatusCode: resp.StatusCode,
		RequestID:  resp.Header.Get("X-Request-Id"),
	}

	var errResp ErrorResponse
	if err := json.Unmarshal(body, &errResp); err == nil {
		apiErr.ErrorType = errResp.Error
		apiErr.Message = errResp.ErrorDescription
	} else {
		apiErr.Message = string(body)
	}

	// Map to specific errors
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return ErrInvalidToken
	case http.StatusTooManyRequests:
		return ErrRateLimited
	case http.StatusForbidden:
		return ErrConsentRequired
	}

	return apiErr
}

// isForbiddenTrueLayerScope checks if a scope is forbidden.
func isForbiddenTrueLayerScope(scope string) bool {
	scopeLower := strings.ToLower(scope)
	for _, pattern := range ForbiddenTrueLayerScopePatterns {
		if strings.Contains(scopeLower, pattern) {
			return true
		}
	}
	return false
}

// IsAllowedTrueLayerScope checks if a scope is in the allowed list.
func IsAllowedTrueLayerScope(scope string) bool {
	for _, allowed := range AllowedTrueLayerScopes {
		if scope == allowed {
			return true
		}
	}
	return false
}

// SetBaseURL sets the base URL for the client (for testing).
func (c *Client) SetBaseURL(url string) {
	c.baseURL = url
}
