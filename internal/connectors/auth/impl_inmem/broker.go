// Package impl_inmem provides in-memory implementation of the token broker.
// This file implements the TokenBroker interface.
//
// CRITICAL: This is for demo/testing only. Production requires:
// - Persistent token storage (Postgres + Key Vault)
// - Proper OAuth HTTP callback server
// - Token refresh handling
//
// Reference: docs/TECHNICAL_SPLIT_V1.md §3.5 Action Execution Layer
package impl_inmem

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"quantumlife/internal/connectors/auth"
	"quantumlife/pkg/primitives"
)

// Broker implements the auth.TokenBroker interface.
type Broker struct {
	config          auth.Config
	store           *TokenStore
	persistentStore *TokenStoreWithPersistence
	scopeMapper     *ScopeMapper
	authorityCheck  auth.AuthorityChecker
	httpClient      *http.Client
	clockFunc       func() time.Time
}

// BrokerOption configures a Broker.
type BrokerOption func(*Broker)

// WithHTTPClient sets a custom HTTP client (for testing).
func WithHTTPClient(client *http.Client) BrokerOption {
	return func(b *Broker) {
		b.httpClient = client
	}
}

// WithClock sets a custom clock function (for testing).
func WithClock(clockFunc func() time.Time) BrokerOption {
	return func(b *Broker) {
		b.clockFunc = clockFunc
	}
}

// NewBroker creates a new token broker.
func NewBroker(config auth.Config, authorityChecker auth.AuthorityChecker, opts ...BrokerOption) *Broker {
	b := &Broker{
		config:         config,
		store:          NewTokenStore(config.TokenEncryptionKey),
		scopeMapper:    NewScopeMapper(),
		authorityCheck: authorityChecker,
		httpClient:     &http.Client{Timeout: 30 * time.Second},
		clockFunc:      time.Now,
	}

	for _, opt := range opts {
		opt(b)
	}

	return b
}

// NewBrokerWithPersistence creates a broker with optional file persistence.
// If TOKEN_ENC_KEY is set, tokens are persisted to ~/.quantumlife/broker_store.json.
// This allows tokens to survive across CLI invocations.
func NewBrokerWithPersistence(config auth.Config, authorityChecker auth.AuthorityChecker, opts ...BrokerOption) (*Broker, error) {
	persistStore, err := NewTokenStoreWithPersistence(config.TokenEncryptionKey)
	if err != nil {
		return nil, err
	}

	b := &Broker{
		config:          config,
		store:           persistStore.TokenStore,
		persistentStore: persistStore,
		scopeMapper:     NewScopeMapper(),
		authorityCheck:  authorityChecker,
		httpClient:      &http.Client{Timeout: 30 * time.Second},
		clockFunc:       time.Now,
	}

	for _, opt := range opts {
		opt(b)
	}

	return b, nil
}

// IsPersistenceEnabled returns true if broker store persistence is enabled.
func (b *Broker) IsPersistenceEnabled() bool {
	return b.persistentStore != nil && b.persistentStore.IsPersistenceEnabled()
}

// BeginOAuth generates an OAuth authorization URL.
func (b *Broker) BeginOAuth(provider auth.ProviderID, redirectURI string, state string, scopes []string) (string, error) {
	if !auth.IsValidProvider(provider) {
		return "", auth.ErrInvalidProvider
	}

	// Validate that only read scopes are requested
	if err := b.scopeMapper.ValidateReadOnlyScopes(scopes); err != nil {
		return "", err
	}

	// Check provider configuration
	if !b.config.IsProviderConfigured(provider) {
		return "", auth.ErrProviderNotConfigured
	}

	// Map QuantumLife scopes to provider scopes
	providerScopes, err := b.scopeMapper.MapToProvider(provider, scopes)
	if err != nil {
		return "", err
	}

	// Build authorization URL
	var authURL string
	var clientID string

	switch provider {
	case auth.ProviderGoogle:
		authURL = auth.GoogleAuthURL
		clientID = b.config.Google.ClientID
	case auth.ProviderMicrosoft:
		authURL = fmt.Sprintf(auth.MicrosoftAuthURLTemplate, b.config.Microsoft.TenantID)
		clientID = b.config.Microsoft.ClientID
	case auth.ProviderTrueLayer:
		// TrueLayer uses a different auth flow
		return b.buildTrueLayerAuthURL(redirectURI, state, providerScopes)
	case auth.ProviderPlaid:
		// Plaid uses Link token flow - return instructions
		return b.buildPlaidLinkInstructions(redirectURI, state, providerScopes)
	default:
		return "", auth.ErrInvalidProvider
	}

	params := url.Values{}
	params.Set("client_id", clientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("scope", strings.Join(providerScopes, " "))
	params.Set("state", state)
	params.Set("access_type", "offline") // Google: request refresh token
	params.Set("prompt", "consent")      // Force consent to get refresh token

	return authURL + "?" + params.Encode(), nil
}

// buildTrueLayerAuthURL builds the TrueLayer authorization URL.
// CRITICAL: Only read scopes are allowed.
func (b *Broker) buildTrueLayerAuthURL(redirectURI, state string, scopes []string) (string, error) {
	// Validate scopes - reject any payment/write scopes
	for _, scope := range scopes {
		if isForbiddenTrueLayerScope(scope) {
			return "", fmt.Errorf("%w: scope '%s' is forbidden", auth.ErrWriteScopeNotAllowed, scope)
		}
	}

	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", b.config.TrueLayer.ClientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("state", state)
	params.Set("scope", strings.Join(scopes, " "))
	// Enable all UK Open Banking providers
	params.Set("providers", "uk-ob-all uk-oauth-all")

	return b.config.TrueLayer.TrueLayerAuthURL() + "/?" + params.Encode(), nil
}

// isForbiddenTrueLayerScope checks if a TrueLayer scope is forbidden.
// CRITICAL: Payment and write scopes are not allowed.
func isForbiddenTrueLayerScope(scope string) bool {
	forbiddenPatterns := []string{
		"payment", "payments", "pay", "transfer", "write",
		"initiate", "standing_order", "direct_debit", "beneficiar", "mandate",
	}
	scopeLower := strings.ToLower(scope)
	for _, pattern := range forbiddenPatterns {
		if strings.Contains(scopeLower, pattern) {
			return true
		}
	}
	return false
}

// buildPlaidLinkInstructions returns instructions for Plaid Link flow.
// CRITICAL: Only read products are allowed.
// Plaid Link requires a client-side widget; for CLI we provide manual instructions.
func (b *Broker) buildPlaidLinkInstructions(redirectURI, state string, products []string) (string, error) {
	// Validate products - reject any payment/transfer products
	for _, product := range products {
		if isForbiddenPlaidProduct(product) {
			return "", fmt.Errorf("%w: product '%s' is forbidden", auth.ErrWriteScopeNotAllowed, product)
		}
	}

	// For CLI, we return a placeholder URL with instructions
	// In a real system, you'd create a Link token via the Plaid API
	// and provide the link_token for Plaid Link initialization.
	//
	// The flow is:
	// 1. Server calls /link/token/create to get a link_token
	// 2. Client initializes Plaid Link with link_token
	// 3. Circle owner completes Link flow
	// 4. Client receives public_token
	// 5. Server exchanges public_token for access_token
	//
	// For demo/testing, we return a manual instruction URL.
	params := url.Values{}
	params.Set("state", state)
	params.Set("redirect_uri", redirectURI)
	params.Set("products", strings.Join(products, ","))
	params.Set("env", b.config.Plaid.Environment)

	// Return instructions as a "URL" - CLI will display this as instructions
	return "plaid-link://manual?" + params.Encode(), nil
}

// isForbiddenPlaidProduct checks if a Plaid product is forbidden.
// CRITICAL: Payment and transfer products are not allowed.
func isForbiddenPlaidProduct(product string) bool {
	forbiddenPatterns := []string{
		"payment", "transfer", "signal", "income",
		"employment", "deposit_switch", "standing_order",
	}
	productLower := strings.ToLower(product)
	for _, pattern := range forbiddenPatterns {
		if strings.Contains(productLower, pattern) {
			return true
		}
	}
	return false
}

// ExchangeCode exchanges an authorization code for tokens.
func (b *Broker) ExchangeCode(ctx context.Context, provider auth.ProviderID, code string, redirectURI string) (auth.TokenHandle, error) {
	if !auth.IsValidProvider(provider) {
		return auth.TokenHandle{}, auth.ErrInvalidProvider
	}

	if !b.config.IsProviderConfigured(provider) {
		return auth.TokenHandle{}, auth.ErrProviderNotConfigured
	}

	// Plaid uses a different exchange flow (public_token -> access_token)
	if provider == auth.ProviderPlaid {
		return b.exchangePlaidPublicToken(ctx, "demo-circle", code)
	}

	// Build token request for OAuth providers
	var tokenURL string
	var clientID, clientSecret string

	switch provider {
	case auth.ProviderGoogle:
		tokenURL = auth.GoogleTokenURL
		clientID = b.config.Google.ClientID
		clientSecret = b.config.Google.ClientSecret
	case auth.ProviderMicrosoft:
		tokenURL = fmt.Sprintf(auth.MicrosoftTokenURLTemplate, b.config.Microsoft.TenantID)
		clientID = b.config.Microsoft.ClientID
		clientSecret = b.config.Microsoft.ClientSecret
	case auth.ProviderTrueLayer:
		tokenURL = b.config.TrueLayer.TrueLayerTokenURL()
		clientID = b.config.TrueLayer.ClientID
		clientSecret = b.config.TrueLayer.ClientSecret
	default:
		return auth.TokenHandle{}, auth.ErrInvalidProvider
	}

	// Exchange code for tokens
	data := url.Values{}
	data.Set("code", code)
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("redirect_uri", redirectURI)
	data.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return auth.TokenHandle{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return auth.TokenHandle{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return auth.TokenHandle{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return auth.TokenHandle{}, fmt.Errorf("%w: %s", auth.ErrInvalidCode, string(body))
	}

	// Parse token response
	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return auth.TokenHandle{}, err
	}

	if tokenResp.RefreshToken == "" {
		return auth.TokenHandle{}, fmt.Errorf("no refresh token returned; ensure offline access is requested")
	}

	// Map provider scopes back to QuantumLife scopes
	providerScopes := strings.Split(tokenResp.Scope, " ")
	qlScopes := b.scopeMapper.MapFromProvider(provider, providerScopes)

	// Calculate expiry (refresh tokens usually don't have explicit expiry)
	var expiresAt time.Time
	// For demo, we don't set expiry on refresh token

	// Store the refresh token (encrypted)
	// NOTE: In a real system, we'd get the circleID from context or session
	// For demo, we use a placeholder
	circleID := "demo-circle"

	handle, err := b.store.Store(ctx, circleID, provider, tokenResp.RefreshToken, qlScopes, expiresAt)
	if err != nil {
		return auth.TokenHandle{}, err
	}

	return handle, nil
}

// ExchangeCodeForCircle exchanges an authorization code for a specific circle.
// This is the version that should be used when the circle ID is known.
func (b *Broker) ExchangeCodeForCircle(ctx context.Context, circleID string, provider auth.ProviderID, code string, redirectURI string) (auth.TokenHandle, error) {
	if !auth.IsValidProvider(provider) {
		return auth.TokenHandle{}, auth.ErrInvalidProvider
	}

	if !b.config.IsProviderConfigured(provider) {
		return auth.TokenHandle{}, auth.ErrProviderNotConfigured
	}

	// Plaid uses a different exchange flow (public_token -> access_token)
	if provider == auth.ProviderPlaid {
		return b.exchangePlaidPublicTokenForCircle(ctx, circleID, code)
	}

	// Build token request for OAuth providers
	var tokenURL string
	var clientID, clientSecret string

	switch provider {
	case auth.ProviderGoogle:
		tokenURL = auth.GoogleTokenURL
		clientID = b.config.Google.ClientID
		clientSecret = b.config.Google.ClientSecret
	case auth.ProviderMicrosoft:
		tokenURL = fmt.Sprintf(auth.MicrosoftTokenURLTemplate, b.config.Microsoft.TenantID)
		clientID = b.config.Microsoft.ClientID
		clientSecret = b.config.Microsoft.ClientSecret
	case auth.ProviderTrueLayer:
		tokenURL = b.config.TrueLayer.TrueLayerTokenURL()
		clientID = b.config.TrueLayer.ClientID
		clientSecret = b.config.TrueLayer.ClientSecret
	default:
		return auth.TokenHandle{}, auth.ErrInvalidProvider
	}

	// Exchange code for tokens
	data := url.Values{}
	data.Set("code", code)
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("redirect_uri", redirectURI)
	data.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return auth.TokenHandle{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return auth.TokenHandle{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return auth.TokenHandle{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return auth.TokenHandle{}, fmt.Errorf("%w: %s", auth.ErrInvalidCode, string(body))
	}

	// Parse token response
	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return auth.TokenHandle{}, err
	}

	if tokenResp.RefreshToken == "" {
		return auth.TokenHandle{}, fmt.Errorf("no refresh token returned; ensure offline access is requested")
	}

	// Map provider scopes back to QuantumLife scopes
	providerScopes := strings.Split(tokenResp.Scope, " ")
	qlScopes := b.scopeMapper.MapFromProvider(provider, providerScopes)

	// Store the refresh token (encrypted)
	var expiresAt time.Time
	var handle auth.TokenHandle

	// Use persistent store if available, otherwise use regular store
	if b.persistentStore != nil {
		handle, err = b.persistentStore.StoreWithPersist(circleID, provider, tokenResp.RefreshToken, qlScopes, expiresAt)
	} else {
		handle, err = b.store.Store(ctx, circleID, provider, tokenResp.RefreshToken, qlScopes, expiresAt)
	}
	if err != nil {
		return auth.TokenHandle{}, err
	}

	return handle, nil
}

// exchangePlaidPublicToken exchanges a Plaid public_token for access_token.
// CRITICAL: Only read products are allowed.
func (b *Broker) exchangePlaidPublicToken(ctx context.Context, circleID, publicToken string) (auth.TokenHandle, error) {
	return b.exchangePlaidPublicTokenForCircle(ctx, circleID, publicToken)
}

// exchangePlaidPublicTokenForCircle exchanges a Plaid public_token for a specific circle.
// CRITICAL: Only read products are allowed.
func (b *Broker) exchangePlaidPublicTokenForCircle(ctx context.Context, circleID, publicToken string) (auth.TokenHandle, error) {
	// Determine Plaid base URL based on environment
	var baseURL string
	switch strings.ToLower(b.config.Plaid.Environment) {
	case "production":
		baseURL = "https://production.plaid.com"
	case "development":
		baseURL = "https://development.plaid.com"
	default:
		baseURL = "https://sandbox.plaid.com"
	}

	// Build request body
	reqBody := map[string]interface{}{
		"client_id":    b.config.Plaid.ClientID,
		"secret":       b.config.Plaid.Secret,
		"public_token": publicToken,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return auth.TokenHandle{}, err
	}

	// Make request to Plaid
	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/item/public_token/exchange", strings.NewReader(string(bodyBytes)))
	if err != nil {
		return auth.TokenHandle{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Plaid-Version", "2020-09-14")

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return auth.TokenHandle{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return auth.TokenHandle{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return auth.TokenHandle{}, fmt.Errorf("%w: %s", auth.ErrInvalidCode, string(body))
	}

	// Parse response
	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ItemID      string `json:"item_id"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return auth.TokenHandle{}, err
	}

	if tokenResp.AccessToken == "" {
		return auth.TokenHandle{}, fmt.Errorf("no access token returned from Plaid")
	}

	// Plaid access tokens don't expire (they're long-lived)
	// Store the access token as the "refresh token" for consistency
	qlScopes := []string{"finance:read"}
	var expiresAt time.Time
	var handle auth.TokenHandle

	// Use persistent store if available
	if b.persistentStore != nil {
		handle, err = b.persistentStore.StoreWithPersist(circleID, auth.ProviderPlaid, tokenResp.AccessToken, qlScopes, expiresAt)
	} else {
		handle, err = b.store.Store(ctx, circleID, auth.ProviderPlaid, tokenResp.AccessToken, qlScopes, expiresAt)
	}
	if err != nil {
		return auth.TokenHandle{}, err
	}

	return handle, nil
}

// MintAccessToken mints an access token for an operation.
func (b *Broker) MintAccessToken(ctx context.Context, envelope primitives.ExecutionEnvelope, provider auth.ProviderID, requiredScopes []string) (auth.AccessToken, error) {
	// Validate envelope
	if err := envelope.Validate(); err != nil {
		return auth.AccessToken{}, err
	}

	// Validate that only read scopes are requested
	if err := b.scopeMapper.ValidateReadOnlyScopes(requiredScopes); err != nil {
		return auth.AccessToken{}, err
	}

	// Verify authorization proof exists and grants the required scopes
	if b.authorityCheck != nil {
		proof, err := b.authorityCheck.GetProof(ctx, envelope.AuthorizationProofID)
		if err != nil {
			return auth.AccessToken{}, fmt.Errorf("%w: %v", auth.ErrAuthorizationRequired, err)
		}

		if !proof.Authorized {
			return auth.AccessToken{}, auth.ErrAuthorizationRequired
		}

		// Check that required scopes are granted
		grantedSet := make(map[string]bool)
		for _, s := range proof.ScopesGranted {
			grantedSet[s] = true
		}
		for _, s := range requiredScopes {
			if !grantedSet[s] {
				return auth.AccessToken{}, fmt.Errorf("%w: %s", auth.ErrScopeNotGranted, s)
			}
		}
	}

	// Get stored refresh token
	refreshToken, storedToken, err := b.store.Get(ctx, envelope.ActorCircleID, provider)
	if err != nil {
		return auth.AccessToken{}, err
	}

	// Map scopes to provider format
	providerScopes, err := b.scopeMapper.MapToProvider(provider, requiredScopes)
	if err != nil {
		return auth.AccessToken{}, err
	}

	// Refresh the access token
	accessToken, expiresIn, err := b.refreshAccessToken(ctx, provider, refreshToken)
	if err != nil {
		return auth.AccessToken{}, err
	}

	// Check if we also got a new refresh token (some providers rotate them)
	// For demo, we don't handle refresh token rotation

	_ = storedToken // Used for context, could log token ID

	return auth.AccessToken{
		Token:          accessToken,
		Expiry:         b.clockFunc().Add(time.Duration(expiresIn) * time.Second),
		Provider:       provider,
		ProviderScopes: providerScopes,
	}, nil
}

// refreshAccessToken refreshes an access token using a refresh token.
func (b *Broker) refreshAccessToken(ctx context.Context, provider auth.ProviderID, refreshToken string) (string, int, error) {
	// Plaid access tokens don't expire - return as-is with long expiry
	if provider == auth.ProviderPlaid {
		// Plaid access tokens are long-lived; we use a 24-hour "expiry" for caching purposes
		return refreshToken, 86400, nil
	}

	var tokenURL string
	var clientID, clientSecret string

	switch provider {
	case auth.ProviderGoogle:
		tokenURL = auth.GoogleTokenURL
		clientID = b.config.Google.ClientID
		clientSecret = b.config.Google.ClientSecret
	case auth.ProviderMicrosoft:
		tokenURL = fmt.Sprintf(auth.MicrosoftTokenURLTemplate, b.config.Microsoft.TenantID)
		clientID = b.config.Microsoft.ClientID
		clientSecret = b.config.Microsoft.ClientSecret
	case auth.ProviderTrueLayer:
		tokenURL = b.config.TrueLayer.TrueLayerTokenURL()
		clientID = b.config.TrueLayer.ClientID
		clientSecret = b.config.TrueLayer.ClientSecret
	default:
		return "", 0, auth.ErrInvalidProvider
	}

	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("refresh_token", refreshToken)
	data.Set("grant_type", "refresh_token")

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, err
	}

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("token refresh failed: %s", string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", 0, err
	}

	return tokenResp.AccessToken, tokenResp.ExpiresIn, nil
}

// RevokeToken revokes the stored token for a circle/provider.
func (b *Broker) RevokeToken(ctx context.Context, circleID string, provider auth.ProviderID) error {
	return b.store.Delete(ctx, circleID, provider)
}

// HasToken checks if a circle has a stored token for a provider.
func (b *Broker) HasToken(ctx context.Context, circleID string, provider auth.ProviderID) (bool, error) {
	return b.store.HasToken(ctx, circleID, provider), nil
}

// StoreTokenDirectly stores a token directly (for demo/testing).
// This bypasses OAuth flow and should only be used in tests.
func (b *Broker) StoreTokenDirectly(ctx context.Context, circleID string, provider auth.ProviderID, refreshToken string, scopes []string) (auth.TokenHandle, error) {
	return b.store.Store(ctx, circleID, provider, refreshToken, scopes, time.Time{})
}

// MintReadOnlyAccessToken mints an access token for read-only operations.
// This is a simpler path than MintAccessToken for ingestion pipelines.
// CRITICAL: Only read scopes are allowed. Write scopes are rejected.
//
// Parameters:
//   - circleID: The circle that owns the token
//   - provider: The OAuth provider
//   - requiredScopes: QuantumLife scopes needed (e.g., "calendar:read", "email:read")
//
// Returns an AccessToken with the actual provider token.
func (b *Broker) MintReadOnlyAccessToken(ctx context.Context, circleID string, provider auth.ProviderID, requiredScopes []string) (auth.AccessToken, error) {
	// Validate that only read scopes are requested
	if err := b.scopeMapper.ValidateReadOnlyScopes(requiredScopes); err != nil {
		return auth.AccessToken{}, err
	}

	// Get stored refresh token
	refreshToken, storedToken, err := b.store.Get(ctx, circleID, provider)
	if err != nil {
		return auth.AccessToken{}, err
	}

	// Verify stored scopes cover the required scopes
	storedSet := make(map[string]bool)
	for _, s := range storedToken.Scopes {
		storedSet[s] = true
	}
	for _, s := range requiredScopes {
		if !storedSet[s] {
			return auth.AccessToken{}, fmt.Errorf("%w: %s not in stored scopes", auth.ErrScopeNotGranted, s)
		}
	}

	// Map scopes to provider format
	providerScopes, err := b.scopeMapper.MapToProvider(provider, requiredScopes)
	if err != nil {
		return auth.AccessToken{}, err
	}

	// Refresh the access token
	accessToken, expiresIn, err := b.refreshAccessToken(ctx, provider, refreshToken)
	if err != nil {
		return auth.AccessToken{}, err
	}

	return auth.AccessToken{
		Token:          accessToken,
		Expiry:         b.clockFunc().Add(time.Duration(expiresIn) * time.Second),
		Provider:       provider,
		ProviderScopes: providerScopes,
	}, nil
}

// GetStore returns the token store (for testing).
func (b *Broker) GetStore() *TokenStore {
	return b.store
}

// GetTokenHandle retrieves a token handle for a circle/provider without exposing secrets.
func (b *Broker) GetTokenHandle(circleID string, provider auth.ProviderID) (auth.TokenHandle, bool) {
	b.store.mu.RLock()
	defer b.store.mu.RUnlock()

	key := tokenKey(circleID, provider)
	token, ok := b.store.tokens[key]
	if !ok {
		return auth.TokenHandle{}, false
	}

	return auth.TokenHandle{
		ID:        token.ID,
		CircleID:  token.CircleID,
		Provider:  token.Provider,
		Scopes:    token.Scopes,
		CreatedAt: token.CreatedAt,
		ExpiresAt: token.ExpiresAt,
	}, true
}

// Sync persists the broker store to disk if persistence is enabled.
func (b *Broker) Sync() error {
	if b.persistentStore == nil {
		return nil
	}
	return b.persistentStore.Sync()
}

// Verify interface compliance at compile time.
var _ auth.TokenBroker = (*Broker)(nil)
