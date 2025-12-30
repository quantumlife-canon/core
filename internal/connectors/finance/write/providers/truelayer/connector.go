// Package truelayer provides TrueLayer payment connector for v9 Slice 3.
//
// CRITICAL: This is the FIRST slice where money may actually move.
// It must be minimal, constrained, auditable, interruptible, and boring.
//
// Subordinate to:
// - docs/CANON_ADDENDUM_V9_FINANCIAL_EXECUTION.md
// - docs/TECHNICAL_SPLIT_V9_EXECUTION.md
package truelayer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"quantumlife/internal/connectors/finance/write"
	"quantumlife/pkg/events"
)

// Connector implements write.WriteConnector for TrueLayer.
//
// CRITICAL: This is the ONLY provider in v9 Slice 3.
// Money CAN move through this connector.
type Connector struct {
	mu sync.RWMutex

	// Configuration
	clientID     string
	clientSecret string
	signingKey   string // For request signing
	environment  string
	httpClient   *http.Client
	config       write.WriteConfig

	// URLs
	authURL     string
	paymentsURL string

	// State
	accessToken      string
	tokenExpiry      time.Time
	payeeRegistry    *write.PayeeRegistry
	abortedEnvelopes map[string]bool
	auditEmitter     func(event events.Event)
	idGenerator      func() string
}

// ConnectorConfig configures the TrueLayer write connector.
type ConnectorConfig struct {
	// ClientID is the TrueLayer client ID.
	ClientID string

	// ClientSecret is the TrueLayer client secret.
	// SENSITIVE: Never log this value.
	ClientSecret string

	// SigningKey is the private key for request signing.
	// SENSITIVE: Never log this value.
	SigningKey string

	// Environment is "sandbox" or "live".
	// Defaults to "sandbox" for safety.
	Environment string

	// HTTPClient is an optional custom HTTP client.
	HTTPClient *http.Client

	// Config is the write configuration.
	Config write.WriteConfig

	// AuditEmitter emits audit events.
	AuditEmitter func(event events.Event)

	// IDGenerator generates unique IDs.
	IDGenerator func() string
}

// NewConnector creates a new TrueLayer write connector.
//
// CRITICAL: Defaults to sandbox mode for safety.
func NewConnector(cfg ConnectorConfig) (*Connector, error) {
	// Default to sandbox for safety
	env := strings.ToLower(cfg.Environment)
	if env == "" {
		env = "sandbox"
	}

	// Determine URLs
	var authURL, paymentsURL string
	switch env {
	case "live", "production":
		authURL = LiveAuthURL
		paymentsURL = LivePaymentsURL
	default:
		authURL = SandboxAuthURL
		paymentsURL = SandboxPaymentsURL
	}

	// Create HTTP client
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	// Apply default config
	config := cfg.Config
	if config.CapCents == 0 {
		config.CapCents = write.DefaultCapCents
	}
	if len(config.AllowedCurrencies) == 0 {
		config.AllowedCurrencies = []string{"GBP"}
	}
	if config.ForcedPauseDuration == 0 {
		config.ForcedPauseDuration = 2 * time.Second
	}

	// Set up payee registry with sandbox payees
	payeeRegistry := write.NewPayeeRegistry()
	for _, payee := range write.SandboxPayees() {
		payeeRegistry.Register(payee)
	}

	return &Connector{
		clientID:         cfg.ClientID,
		clientSecret:     cfg.ClientSecret,
		signingKey:       cfg.SigningKey,
		environment:      env,
		httpClient:       httpClient,
		config:           config,
		authURL:          authURL,
		paymentsURL:      paymentsURL,
		payeeRegistry:    payeeRegistry,
		abortedEnvelopes: make(map[string]bool),
		auditEmitter:     cfg.AuditEmitter,
		idGenerator:      cfg.IDGenerator,
	}, nil
}

// Provider returns the provider name.
func (c *Connector) Provider() string {
	return "truelayer"
}

// Prepare validates that the payment can be executed.
//
// CRITICAL: This performs ALL validation BEFORE any money moves.
func (c *Connector) Prepare(ctx context.Context, req write.PrepareRequest) (*write.PrepareResult, error) {
	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}

	result := &write.PrepareResult{
		Valid:             true,
		ValidationDetails: make([]write.ValidationDetail, 0),
		PreparedAt:        now,
	}

	// Check 1: Envelope exists and is sealed
	if req.Envelope == nil {
		result.Valid = false
		result.InvalidReason = "envelope is nil"
		result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
			Check:   "envelope_exists",
			Passed:  false,
			Details: "envelope is nil",
		})
		return result, nil
	}

	if req.Envelope.SealHash == "" {
		result.Valid = false
		result.InvalidReason = "envelope is not sealed"
		result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
			Check:   "envelope_sealed",
			Passed:  false,
			Details: "envelope missing seal hash",
		})
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
		Check:   "envelope_sealed",
		Passed:  true,
		Details: fmt.Sprintf("seal hash: %s", req.Envelope.SealHash[:16]),
	})

	// Check 2: Envelope not expired
	if now.After(req.Envelope.Expiry) {
		result.Valid = false
		result.InvalidReason = "envelope has expired"
		result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
			Check:   "envelope_not_expired",
			Passed:  false,
			Details: fmt.Sprintf("expired at %s", req.Envelope.Expiry.Format(time.RFC3339)),
		})
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
		Check:   "envelope_not_expired",
		Passed:  true,
		Details: fmt.Sprintf("expires at %s", req.Envelope.Expiry.Format(time.RFC3339)),
	})

	// Check 3: Envelope not revoked
	if req.Envelope.Revoked {
		result.Valid = false
		result.InvalidReason = "envelope has been revoked"
		result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
			Check:   "envelope_not_revoked",
			Passed:  false,
			Details: fmt.Sprintf("revoked by %s at %s", req.Envelope.RevokedBy, req.Envelope.RevokedAt.Format(time.RFC3339)),
		})
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
		Check:   "envelope_not_revoked",
		Passed:  true,
		Details: "no revocation",
	})

	// Check 4: Revocation window has closed (or explicitly waived)
	if !req.Envelope.RevocationWaived && now.Before(req.Envelope.RevocationWindowEnd) {
		result.Valid = false
		result.InvalidReason = "revocation window is still active"
		result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
			Check:   "revocation_window_closed",
			Passed:  false,
			Details: fmt.Sprintf("window ends at %s", req.Envelope.RevocationWindowEnd.Format(time.RFC3339)),
		})
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
		Check:   "revocation_window_closed",
		Passed:  true,
		Details: "window closed or waived",
	})

	// Check 5: Approval exists
	if req.Approval == nil {
		result.Valid = false
		result.InvalidReason = "explicit approval required"
		result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
			Check:   "approval_exists",
			Passed:  false,
			Details: "no approval artifact provided",
		})
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
		Check:   "approval_exists",
		Passed:  true,
		Details: fmt.Sprintf("artifact ID: %s", req.Approval.ArtifactID),
	})

	// Check 6: Approval not expired
	if req.Approval.IsExpired(now) {
		result.Valid = false
		result.InvalidReason = "approval has expired"
		result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
			Check:   "approval_not_expired",
			Passed:  false,
			Details: fmt.Sprintf("expired at %s", req.Approval.ExpiresAt.Format(time.RFC3339)),
		})
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
		Check:   "approval_not_expired",
		Passed:  true,
		Details: fmt.Sprintf("expires at %s", req.Approval.ExpiresAt.Format(time.RFC3339)),
	})

	// Check 7: Approval action hash matches envelope
	if req.Approval.ActionHash != req.Envelope.ActionHash {
		result.Valid = false
		result.InvalidReason = "approval action hash does not match envelope"
		result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
			Check:   "approval_hash_matches",
			Passed:  false,
			Details: "action hash mismatch",
		})
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
		Check:   "approval_hash_matches",
		Passed:  true,
		Details: fmt.Sprintf("action hash: %s", req.Envelope.ActionHash[:16]),
	})

	// Check 8: Amount does not exceed cap
	if req.Envelope.ActionSpec.AmountCents > c.config.CapCents {
		result.Valid = false
		result.InvalidReason = fmt.Sprintf("amount %d exceeds hard cap %d", req.Envelope.ActionSpec.AmountCents, c.config.CapCents)
		result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
			Check:   "amount_within_cap",
			Passed:  false,
			Details: fmt.Sprintf("amount %d > cap %d", req.Envelope.ActionSpec.AmountCents, c.config.CapCents),
		})
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
		Check:   "amount_within_cap",
		Passed:  true,
		Details: fmt.Sprintf("amount %d <= cap %d", req.Envelope.ActionSpec.AmountCents, c.config.CapCents),
	})

	// Check 9: Currency is allowed
	currencyAllowed := false
	for _, allowed := range c.config.AllowedCurrencies {
		if req.Envelope.ActionSpec.Currency == allowed {
			currencyAllowed = true
			break
		}
	}
	if !currencyAllowed {
		result.Valid = false
		result.InvalidReason = fmt.Sprintf("currency %s not allowed", req.Envelope.ActionSpec.Currency)
		result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
			Check:   "currency_allowed",
			Passed:  false,
			Details: fmt.Sprintf("currency %s not in allowed list", req.Envelope.ActionSpec.Currency),
		})
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
		Check:   "currency_allowed",
		Passed:  true,
		Details: fmt.Sprintf("currency %s is allowed", req.Envelope.ActionSpec.Currency),
	})

	// Check 10: Payee is pre-defined
	_, payeeExists := c.payeeRegistry.Get(req.PayeeID)
	if !payeeExists {
		result.Valid = false
		result.InvalidReason = "payee must be pre-defined (no free-text recipients)"
		result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
			Check:   "payee_predefined",
			Passed:  false,
			Details: fmt.Sprintf("payee '%s' not found in registry", req.PayeeID),
		})
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
		Check:   "payee_predefined",
		Passed:  true,
		Details: fmt.Sprintf("payee '%s' is registered", req.PayeeID),
	})

	// Check 11: Not aborted
	c.mu.RLock()
	aborted := c.abortedEnvelopes[req.Envelope.EnvelopeID]
	c.mu.RUnlock()
	if aborted {
		result.Valid = false
		result.InvalidReason = "execution was aborted"
		result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
			Check:   "not_aborted",
			Passed:  false,
			Details: "envelope was aborted before execution",
		})
		return result, nil
	}
	result.ValidationDetails = append(result.ValidationDetails, write.ValidationDetail{
		Check:   "not_aborted",
		Passed:  true,
		Details: "not aborted",
	})

	// Emit prepare event
	if c.auditEmitter != nil {
		c.auditEmitter(events.Event{
			ID:             c.generateID(),
			Type:           events.EventV9AdapterPrepared,
			Timestamp:      now,
			CircleID:       req.Envelope.ActorCircleID,
			IntersectionID: req.Envelope.IntersectionID,
			SubjectID:      req.Envelope.EnvelopeID,
			SubjectType:    "envelope",
			Provider:       "truelayer",
			Metadata: map[string]string{
				"amount":      fmt.Sprintf("%d", req.Envelope.ActionSpec.AmountCents),
				"currency":    req.Envelope.ActionSpec.Currency,
				"payee_id":    req.PayeeID,
				"valid":       fmt.Sprintf("%t", result.Valid),
				"action_hash": req.Envelope.ActionHash[:16],
			},
		})
	}

	return result, nil
}

// Execute creates the payment with TrueLayer.
//
// CRITICAL: This is the ONLY method that can move money.
// NO RETRIES. Failures require new approval.
func (c *Connector) Execute(ctx context.Context, req write.ExecuteRequest) (*write.PaymentReceipt, error) {
	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}

	// Final pre-execution checks
	c.mu.RLock()
	aborted := c.abortedEnvelopes[req.Envelope.EnvelopeID]
	c.mu.RUnlock()
	if aborted {
		return nil, write.ErrExecutionAborted
	}

	// Get payee
	payee, ok := c.payeeRegistry.Get(req.PayeeID)
	if !ok {
		return nil, write.ErrInvalidPayee
	}

	// Check credentials
	if c.clientID == "" || c.clientSecret == "" {
		// Emit blocked event for missing credentials
		if c.auditEmitter != nil {
			c.auditEmitter(events.Event{
				ID:             c.generateID(),
				Type:           events.EventV9AdapterBlocked,
				Timestamp:      now,
				CircleID:       req.Envelope.ActorCircleID,
				IntersectionID: req.Envelope.IntersectionID,
				SubjectID:      req.Envelope.EnvelopeID,
				SubjectType:    "envelope",
				Provider:       "truelayer",
				Metadata: map[string]string{
					"reason":      "provider credentials not configured",
					"money_moved": "false",
				},
			})
		}
		return nil, write.ErrProviderNotConfigured
	}

	// Emit invocation event
	if c.auditEmitter != nil {
		c.auditEmitter(events.Event{
			ID:             c.generateID(),
			Type:           events.EventV9AdapterInvoked,
			Timestamp:      now,
			CircleID:       req.Envelope.ActorCircleID,
			IntersectionID: req.Envelope.IntersectionID,
			SubjectID:      req.Envelope.EnvelopeID,
			SubjectType:    "envelope",
			Provider:       "truelayer",
			Metadata: map[string]string{
				"amount":          fmt.Sprintf("%d", req.Envelope.ActionSpec.AmountCents),
				"currency":        req.Envelope.ActionSpec.Currency,
				"payee_id":        req.PayeeID,
				"idempotency_key": req.IdempotencyKey,
			},
		})
	}

	// FORCED PAUSE - intentional friction
	// This gives time for abort if needed
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(c.config.ForcedPauseDuration):
		// Continue after pause
	}

	// Check abort again after pause
	c.mu.RLock()
	aborted = c.abortedEnvelopes[req.Envelope.EnvelopeID]
	c.mu.RUnlock()
	if aborted {
		return nil, write.ErrExecutionAborted
	}

	// Get access token
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	// Build payment request
	paymentReq := &PaymentRequest{
		AmountInMinor: req.Envelope.ActionSpec.AmountCents,
		Currency:      req.Envelope.ActionSpec.Currency,
		PaymentMethod: PaymentMethod{
			Type: "bank_transfer",
			ProviderSelection: &ProviderSelection{
				Type: "user_selected",
				Filter: &ProviderFilter{
					Countries:      []string{"GB"},
					ReleaseChannel: "general_availability",
				},
			},
			Beneficiary: &Beneficiary{
				Type:              "external_account",
				AccountHolderName: payee.Name,
				Reference:         fmt.Sprintf("QL-%s", req.Envelope.EnvelopeID[:8]),
				AccountIdentifier: SandboxBeneficiary().AccountIdentifier,
			},
		},
		User: PaymentUser{
			ID:   req.Envelope.ActorCircleID,
			Name: "QuantumLife User",
		},
		Metadata: map[string]string{
			"envelope_id":     req.Envelope.EnvelopeID,
			"action_hash":     req.Envelope.ActionHash[:16],
			"approval_id":     req.Approval.ArtifactID,
			"idempotency_key": req.IdempotencyKey,
		},
	}

	// Create payment
	paymentResp, err := c.createPayment(ctx, token, paymentReq, req.IdempotencyKey)
	if err != nil {
		// Emit failure event
		if c.auditEmitter != nil {
			c.auditEmitter(events.Event{
				ID:             c.generateID(),
				Type:           events.EventV9PaymentFailed,
				Timestamp:      time.Now(),
				CircleID:       req.Envelope.ActorCircleID,
				IntersectionID: req.Envelope.IntersectionID,
				SubjectID:      req.Envelope.EnvelopeID,
				SubjectType:    "envelope",
				Provider:       "truelayer",
				Metadata: map[string]string{
					"error":       err.Error(),
					"money_moved": "false",
				},
			})
		}
		return nil, fmt.Errorf("payment creation failed: %w", err)
	}

	// Emit success event
	if c.auditEmitter != nil {
		c.auditEmitter(events.Event{
			ID:             c.generateID(),
			Type:           events.EventV9PaymentCreated,
			Timestamp:      time.Now(),
			CircleID:       req.Envelope.ActorCircleID,
			IntersectionID: req.Envelope.IntersectionID,
			SubjectID:      paymentResp.ID,
			SubjectType:    "payment",
			Provider:       "truelayer",
			Metadata: map[string]string{
				"envelope_id":  req.Envelope.EnvelopeID,
				"amount":       fmt.Sprintf("%d", req.Envelope.ActionSpec.AmountCents),
				"currency":     req.Envelope.ActionSpec.Currency,
				"status":       paymentResp.Status,
				"provider_ref": paymentResp.ID,
			},
		})
	}

	// Build receipt
	receipt := &write.PaymentReceipt{
		ReceiptID:   c.generateID(),
		EnvelopeID:  req.Envelope.EnvelopeID,
		ProviderRef: paymentResp.ID,
		Status:      mapPaymentStatus(paymentResp.Status),
		AmountCents: req.Envelope.ActionSpec.AmountCents,
		Currency:    req.Envelope.ActionSpec.Currency,
		PayeeID:     req.PayeeID,
		CreatedAt:   paymentResp.CreatedAt,
		CompletedAt: time.Now(),
		ProviderMetadata: map[string]string{
			"payment_id":     paymentResp.ID,
			"resource_token": paymentResp.ResourceToken,
			"user_id":        paymentResp.User.ID,
		},
	}

	return receipt, nil
}

// Abort cancels execution before provider call if possible.
func (c *Connector) Abort(ctx context.Context, envelopeID string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Mark as aborted
	c.abortedEnvelopes[envelopeID] = true

	// Emit abort event
	if c.auditEmitter != nil {
		c.auditEmitter(events.Event{
			ID:          c.generateID(),
			Type:        events.EventV9ExecutionAborted,
			Timestamp:   time.Now(),
			SubjectID:   envelopeID,
			SubjectType: "envelope",
			Provider:    "truelayer",
			Metadata: map[string]string{
				"reason": "user-initiated abort",
			},
		})
	}

	return true, nil
}

// getAccessToken gets or refreshes the access token.
func (c *Connector) getAccessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if current token is still valid
	if c.accessToken != "" && time.Now().Before(c.tokenExpiry) {
		return c.accessToken, nil
	}

	// Get new token
	data := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
		"scope":         {"payments"},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.authURL+"/connect/token", strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token request failed: %s", string(body))
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	c.accessToken = tokenResp.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn-60) * time.Second)

	return c.accessToken, nil
}

// createPayment creates a payment with TrueLayer.
func (c *Connector) createPayment(ctx context.Context, token string, paymentReq *PaymentRequest, idempotencyKey string) (*PaymentResponse, error) {
	body, err := json.Marshal(paymentReq)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.paymentsURL+"/payments", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", idempotencyKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Detail != "" {
			return nil, fmt.Errorf("TrueLayer error: %s - %s", errResp.Type, errResp.Detail)
		}
		return nil, fmt.Errorf("payment request failed: %s", string(body))
	}

	var paymentResp PaymentResponse
	if err := json.NewDecoder(resp.Body).Decode(&paymentResp); err != nil {
		return nil, err
	}

	return &paymentResp, nil
}

// generateID generates a unique ID.
func (c *Connector) generateID() string {
	if c.idGenerator != nil {
		return c.idGenerator()
	}
	return fmt.Sprintf("tl_%d", time.Now().UnixNano())
}

// mapPaymentStatus maps TrueLayer status to our status.
func mapPaymentStatus(tlStatus string) write.PaymentStatus {
	switch tlStatus {
	case StatusAuthorizationRequired, StatusAuthorizing:
		return write.PaymentPending
	case StatusAuthorized, StatusExecuted:
		return write.PaymentExecuting
	case StatusSettled:
		return write.PaymentSucceeded
	case StatusFailed:
		return write.PaymentFailed
	default:
		return write.PaymentPending
	}
}

// Verify interface compliance.
var _ write.WriteConnector = (*Connector)(nil)
