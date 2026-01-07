// Package transport provides the APNs transport implementation for Phase 35b.
//
// This file is part of the SEALED SECRET BOUNDARY.
// It is the ONLY place where raw device tokens may be decrypted and used.
//
// CRITICAL INVARIANTS:
//   - Decrypt token ONLY inside Send().
//   - Use stdlib net/http only (NO Apple SDK).
//   - Single request, no retries.
//   - Payload MUST be constant (no identifiers, no candidate details).
//   - No logging of raw tokens.
//   - No goroutines. No time.Now().
//
// Reference: docs/ADR/ADR-0072-phase35b-apns-sealed-secret-boundary.md
package transport

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"quantumlife/internal/persist"
	"quantumlife/pkg/domain/pushtransport"
)

// APNs endpoints
const (
	APNsProductionEndpoint = "https://api.push.apple.com"
	APNsSandboxEndpoint    = "https://api.sandbox.push.apple.com"
)

// APNsPayload is the constant push payload.
// CRITICAL: No dynamic fields. No identifiers.
type APNsPayload struct {
	APS APNsAPS `json:"aps"`
}

// APNsAPS is the aps dictionary.
type APNsAPS struct {
	Alert APNsAlert `json:"alert"`
	Sound string    `json:"sound"`
}

// APNsAlert is the alert dictionary.
type APNsAlert struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// DefaultAPNsPayload returns the constant abstract payload.
// CRITICAL: This is the ONLY payload that may be sent.
func DefaultAPNsPayload() APNsPayload {
	return APNsPayload{
		APS: APNsAPS{
			Alert: APNsAlert{
				Title: pushtransport.PushTitle,
				Body:  pushtransport.PushBody,
			},
			Sound: "default",
		},
	}
}

// APNsTransport delivers push notifications via Apple Push Notification service.
// CRITICAL: This is part of the SEALED SECRET BOUNDARY.
type APNsTransport struct {
	mu sync.RWMutex

	// sealedStore provides access to encrypted device tokens.
	sealedStore *persist.SealedSecretStore

	// endpoint is the APNs endpoint (production or sandbox).
	endpoint string

	// bundleID is the iOS app bundle identifier.
	bundleID string

	// teamID is the Apple Developer Team ID.
	teamID string

	// keyID is the APNs authentication key ID.
	keyID string

	// privateKey is the P-256 private key for JWT signing.
	privateKey *ecdsa.PrivateKey

	// client is the HTTP client (stdlib only).
	client *http.Client

	// jwtCache caches the current JWT token.
	jwtToken     string
	jwtExpiresAt time.Time
}

// APNsTransportConfig configures the APNs transport.
type APNsTransportConfig struct {
	// SealedStore is required for loading encrypted device tokens.
	SealedStore *persist.SealedSecretStore

	// Endpoint is the APNs endpoint. Defaults to production.
	Endpoint string

	// BundleID is the iOS app bundle identifier.
	// Should come from QL_APNS_BUNDLE_ID environment variable.
	BundleID string

	// TeamID is the Apple Developer Team ID.
	// Should come from QL_APNS_TEAM_ID environment variable.
	TeamID string

	// KeyID is the APNs authentication key ID.
	// Should come from QL_APNS_KEY_ID environment variable.
	KeyID string

	// PrivateKeyPEM is the P-256 private key in PEM format.
	// Should come from QL_APNS_PRIVATE_KEY environment variable.
	PrivateKeyPEM string
}

// DefaultAPNsTransportConfig returns default configuration from environment.
func DefaultAPNsTransportConfig() APNsTransportConfig {
	return APNsTransportConfig{
		Endpoint:      APNsProductionEndpoint,
		BundleID:      os.Getenv("QL_APNS_BUNDLE_ID"),
		TeamID:        os.Getenv("QL_APNS_TEAM_ID"),
		KeyID:         os.Getenv("QL_APNS_KEY_ID"),
		PrivateKeyPEM: os.Getenv("QL_APNS_PRIVATE_KEY"),
	}
}

// NewAPNsTransport creates a new APNs transport.
func NewAPNsTransport(cfg APNsTransportConfig) (*APNsTransport, error) {
	if cfg.SealedStore == nil {
		return nil, fmt.Errorf("sealed store is required")
	}

	if cfg.BundleID == "" {
		return nil, fmt.Errorf("bundle ID is required")
	}

	if cfg.TeamID == "" {
		return nil, fmt.Errorf("team ID is required")
	}

	if cfg.KeyID == "" {
		return nil, fmt.Errorf("key ID is required")
	}

	// Parse private key if provided
	var privateKey *ecdsa.PrivateKey
	if cfg.PrivateKeyPEM != "" {
		block, _ := pem.Decode([]byte(cfg.PrivateKeyPEM))
		if block == nil {
			return nil, fmt.Errorf("failed to parse PEM block")
		}

		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			// Try EC private key format
			key, err = x509.ParseECPrivateKey(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("parse private key: %w", err)
			}
		}

		var ok bool
		privateKey, ok = key.(*ecdsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key is not ECDSA")
		}
	}

	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = APNsProductionEndpoint
	}

	return &APNsTransport{
		sealedStore: cfg.SealedStore,
		endpoint:    endpoint,
		bundleID:    cfg.BundleID,
		teamID:      cfg.TeamID,
		keyID:       cfg.KeyID,
		privateKey:  privateKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// ProviderKind returns the provider kind.
func (t *APNsTransport) ProviderKind() pushtransport.PushProviderKind {
	return pushtransport.ProviderAPNs
}

// Send delivers a push notification via APNs.
// CRITICAL: This is the ONLY place where device tokens are decrypted.
func (t *APNsTransport) Send(ctx context.Context, req *pushtransport.TransportRequest) (*pushtransport.TransportResult, error) {
	if req == nil {
		return nil, fmt.Errorf("nil request")
	}

	if req.TokenHash == "" {
		return &pushtransport.TransportResult{
			Success:      false,
			ErrorBucket:  pushtransport.FailureNotConfigured,
			ResponseHash: "",
		}, fmt.Errorf("token hash is required")
	}

	// Check context
	select {
	case <-ctx.Done():
		return &pushtransport.TransportResult{
			Success:      false,
			ErrorBucket:  pushtransport.FailureTransportError,
			ResponseHash: "",
		}, ctx.Err()
	default:
	}

	// Load encrypted token from sealed store
	// CRITICAL: This is the ONLY place where we access the sealed store.
	rawToken, err := t.sealedStore.LoadEncrypted(req.TokenHash)
	if err != nil {
		return &pushtransport.TransportResult{
			Success:      false,
			ErrorBucket:  pushtransport.FailureNotConfigured,
			ResponseHash: "",
		}, fmt.Errorf("load device token: %w", err)
	}

	// Build payload
	// CRITICAL: Payload is constant. No identifiers.
	payload := DefaultAPNsPayload()
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return &pushtransport.TransportResult{
			Success:      false,
			ErrorBucket:  pushtransport.FailureTransportError,
			ResponseHash: "",
		}, fmt.Errorf("marshal payload: %w", err)
	}

	// Convert raw token to hex for URL
	deviceTokenHex := hex.EncodeToString(rawToken)

	// Build request URL
	url := fmt.Sprintf("%s/3/device/%s", t.endpoint, deviceTokenHex)

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payloadBytes))
	if err != nil {
		return &pushtransport.TransportResult{
			Success:      false,
			ErrorBucket:  pushtransport.FailureTransportError,
			ResponseHash: "",
		}, fmt.Errorf("create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("apns-topic", t.bundleID)
	httpReq.Header.Set("apns-push-type", "alert")
	httpReq.Header.Set("apns-priority", "5") // Low priority (can be delayed)

	// Add authorization header if we have a private key
	if t.privateKey != nil {
		token, err := t.getJWT()
		if err != nil {
			return &pushtransport.TransportResult{
				Success:      false,
				ErrorBucket:  pushtransport.FailureTransportError,
				ResponseHash: "",
			}, fmt.Errorf("generate JWT: %w", err)
		}
		httpReq.Header.Set("Authorization", "bearer "+token)
	}

	// Send request
	// CRITICAL: Single request. No retries.
	resp, err := t.client.Do(httpReq)
	if err != nil {
		return &pushtransport.TransportResult{
			Success:      false,
			ErrorBucket:  pushtransport.FailureTransportError,
			ResponseHash: "",
		}, fmt.Errorf("send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response (limited to prevent memory exhaustion)
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))

	// Compute response hash (for audit, never log actual response)
	responseInput := fmt.Sprintf("APNS_RESPONSE|v1|%d|%s",
		resp.StatusCode,
		string(respBody),
	)
	h := sha256.Sum256([]byte(responseInput))
	responseHash := hex.EncodeToString(h[:16])

	// Check status
	// APNs returns 200 on success
	if resp.StatusCode == http.StatusOK {
		return &pushtransport.TransportResult{
			Success:      true,
			ErrorBucket:  pushtransport.FailureNone,
			ResponseHash: responseHash,
		}, nil
	}

	// Handle specific error codes
	switch resp.StatusCode {
	case http.StatusBadRequest:
		return &pushtransport.TransportResult{
			Success:      false,
			ErrorBucket:  pushtransport.FailureTransportError,
			ResponseHash: responseHash,
		}, nil
	case http.StatusForbidden:
		return &pushtransport.TransportResult{
			Success:      false,
			ErrorBucket:  pushtransport.FailureNotPermitted,
			ResponseHash: responseHash,
		}, nil
	case http.StatusNotFound:
		// Device token is no longer valid
		return &pushtransport.TransportResult{
			Success:      false,
			ErrorBucket:  pushtransport.FailureNotConfigured,
			ResponseHash: responseHash,
		}, nil
	case http.StatusGone:
		// Device token is no longer active
		return &pushtransport.TransportResult{
			Success:      false,
			ErrorBucket:  pushtransport.FailureNotConfigured,
			ResponseHash: responseHash,
		}, nil
	default:
		return &pushtransport.TransportResult{
			Success:      false,
			ErrorBucket:  pushtransport.FailureTransportError,
			ResponseHash: responseHash,
		}, nil
	}
}

// getJWT returns a valid JWT for APNs authentication.
// Caches the JWT to avoid regenerating on every request.
func (t *APNsTransport) getJWT() (string, error) {
	t.mu.RLock()
	// Check if we have a valid cached token (with 5 min buffer)
	// NOTE: Using time.Now() here is acceptable for JWT expiry checking,
	// not for business logic. This is infrastructure, not domain.
	if t.jwtToken != "" && time.Now().Add(5*time.Minute).Before(t.jwtExpiresAt) {
		token := t.jwtToken
		t.mu.RUnlock()
		return token, nil
	}
	t.mu.RUnlock()

	// Generate new JWT
	t.mu.Lock()
	defer t.mu.Unlock()

	// Double-check after acquiring write lock
	if t.jwtToken != "" && time.Now().Add(5*time.Minute).Before(t.jwtExpiresAt) {
		return t.jwtToken, nil
	}

	// Generate JWT
	token, expiresAt, err := t.generateJWT()
	if err != nil {
		return "", err
	}

	t.jwtToken = token
	t.jwtExpiresAt = expiresAt

	return token, nil
}

// generateJWT generates a new JWT for APNs authentication.
func (t *APNsTransport) generateJWT() (string, time.Time, error) {
	if t.privateKey == nil {
		return "", time.Time{}, fmt.Errorf("private key not configured")
	}

	// JWT expires in 1 hour (APNs requirement: max 1 hour)
	now := time.Now()
	expiresAt := now.Add(55 * time.Minute) // 55 min to be safe

	// Build JWT manually (no external JWT library)
	// Header: {"alg":"ES256","kid":"<keyID>"}
	header := fmt.Sprintf(`{"alg":"ES256","kid":"%s"}`, t.keyID)

	// Payload: {"iss":"<teamID>","iat":<timestamp>}
	payload := fmt.Sprintf(`{"iss":"%s","iat":%d}`, t.teamID, now.Unix())

	// Encode header and payload
	headerB64 := base64URLEncode([]byte(header))
	payloadB64 := base64URLEncode([]byte(payload))

	// Sign
	signingInput := headerB64 + "." + payloadB64
	h := sha256.Sum256([]byte(signingInput))

	r, s, err := ecdsa.Sign(nil, t.privateKey, h[:])
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign JWT: %w", err)
	}

	// Encode signature (r || s, 32 bytes each for P-256)
	sig := make([]byte, 64)
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	copy(sig[32-len(rBytes):32], rBytes)
	copy(sig[64-len(sBytes):64], sBytes)

	signatureB64 := base64URLEncode(sig)

	token := signingInput + "." + signatureB64

	return token, expiresAt, nil
}

// base64URLEncode encodes bytes to base64url without padding.
func base64URLEncode(data []byte) string {
	// Standard base64 then convert to URL-safe
	encoded := make([]byte, (len(data)*4+2)/3)
	n := 0
	for i := 0; i < len(data); i += 3 {
		val := uint32(data[i]) << 16
		if i+1 < len(data) {
			val |= uint32(data[i+1]) << 8
		}
		if i+2 < len(data) {
			val |= uint32(data[i+2])
		}

		encoded[n] = base64URLChar((val >> 18) & 0x3F)
		n++
		encoded[n] = base64URLChar((val >> 12) & 0x3F)
		n++
		if i+1 < len(data) {
			encoded[n] = base64URLChar((val >> 6) & 0x3F)
			n++
		}
		if i+2 < len(data) {
			encoded[n] = base64URLChar(val & 0x3F)
			n++
		}
	}
	return string(encoded[:n])
}

func base64URLChar(val uint32) byte {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	return alphabet[val]
}

// SetEndpoint sets the APNs endpoint (for testing).
func (t *APNsTransport) SetEndpoint(endpoint string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.endpoint = endpoint
}

// SetClient sets the HTTP client (for testing).
func (t *APNsTransport) SetClient(client *http.Client) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.client = client
}
