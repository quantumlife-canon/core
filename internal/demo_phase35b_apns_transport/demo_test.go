// Package demo_phase35b_apns_transport contains demo tests for Phase 35b.
//
// Phase 35b: APNs Push Transport with Sealed Secret Boundary
//
// CRITICAL INVARIANTS:
//   - Raw device tokens encrypted with AES-GCM.
//   - Token hash only in logs/events/storelog.
//   - Sealed secret boundary: only apns.go may decrypt tokens.
//   - Abstract payload only. No identifiers in push body.
//   - No goroutines. stdlib-only.
//
// Reference: docs/ADR/ADR-0072-phase35b-apns-sealed-secret-boundary.md
package demo_phase35b_apns_transport

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"quantumlife/internal/persist"
	"quantumlife/internal/pushtransport/transport"
	pt "quantumlife/pkg/domain/pushtransport"
)

// ═══════════════════════════════════════════════════════════════════════════
// Test 1: Encryption Key Generation
// ═══════════════════════════════════════════════════════════════════════════

func TestGenerateKey_ProducesValidKey(t *testing.T) {
	key, err := persist.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	// Key should be base64 encoded
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		t.Fatalf("key is not valid base64: %v", err)
	}

	// Key should be 32 bytes
	if len(decoded) != 32 {
		t.Errorf("expected 32 byte key, got %d bytes", len(decoded))
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 2: Sealed Secret Store Creation
// ═══════════════════════════════════════════════════════════════════════════

func TestSealedSecretStore_RequiresKey(t *testing.T) {
	cfg := persist.SealedSecretStoreConfig{
		EncryptionKeyBase64: "",
		DataDir:             t.TempDir(),
	}

	_, err := persist.NewSealedSecretStore(cfg)
	if err == nil {
		t.Error("expected error when encryption key is missing")
	}
}

func TestSealedSecretStore_ValidatesKeyLength(t *testing.T) {
	// 16 bytes (too short)
	shortKey := base64.StdEncoding.EncodeToString(make([]byte, 16))
	cfg := persist.SealedSecretStoreConfig{
		EncryptionKeyBase64: shortKey,
		DataDir:             t.TempDir(),
	}

	_, err := persist.NewSealedSecretStore(cfg)
	if err == nil {
		t.Error("expected error when encryption key is too short")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 3: Encryption/Decryption Roundtrip
// ═══════════════════════════════════════════════════════════════════════════

func TestSealedSecretStore_EncryptDecryptRoundtrip(t *testing.T) {
	store := createTestStore(t)

	plaintext := []byte("device-token-abc123-secret")

	// Encrypt
	encrypted, err := store.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Encrypted should not contain plaintext
	if bytes.Contains(encrypted, plaintext) {
		t.Error("encrypted data contains plaintext - encryption failed")
	}

	// Decrypt
	decrypted, err := store.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	// Should match original
	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("decrypted data does not match: got %s, want %s", decrypted, plaintext)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 4: Token Never Stored Raw
// ═══════════════════════════════════════════════════════════════════════════

func TestSealedSecretStore_TokenNeverStoredRaw(t *testing.T) {
	store := createTestStore(t)

	rawToken := []byte("device-token-abc123-secret-should-never-appear-raw")
	tokenHash := hashToken(rawToken)

	// Store encrypted
	err := store.StoreEncrypted(tokenHash, rawToken)
	if err != nil {
		t.Fatalf("StoreEncrypted failed: %v", err)
	}

	// Read the file directly
	dataDir := t.TempDir()
	// Recreate store with same dir to find the file
	store2 := createTestStoreWithDir(t, dataDir)
	_ = store2.StoreEncrypted(tokenHash, rawToken)

	// Read all files in data dir
	files, _ := os.ReadDir(dataDir)
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dataDir, f.Name()))
		if err != nil {
			continue
		}

		// Raw token should NOT appear in file
		if bytes.Contains(content, rawToken) {
			t.Errorf("raw token found in file %s - encryption failed", f.Name())
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 5: Store and Load by Token Hash
// ═══════════════════════════════════════════════════════════════════════════

func TestSealedSecretStore_StoreAndLoadByHash(t *testing.T) {
	store := createTestStore(t)

	rawToken := []byte("device-token-for-storage")
	tokenHash := hashToken(rawToken)

	// Store
	err := store.StoreEncrypted(tokenHash, rawToken)
	if err != nil {
		t.Fatalf("StoreEncrypted failed: %v", err)
	}

	// Load
	loaded, err := store.LoadEncrypted(tokenHash)
	if err != nil {
		t.Fatalf("LoadEncrypted failed: %v", err)
	}

	// Should match
	if !bytes.Equal(loaded, rawToken) {
		t.Errorf("loaded token does not match: got %s, want %s", loaded, rawToken)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 6: Delete by Token Hash
// ═══════════════════════════════════════════════════════════════════════════

func TestSealedSecretStore_Delete(t *testing.T) {
	store := createTestStore(t)

	rawToken := []byte("device-token-to-delete")
	tokenHash := hashToken(rawToken)

	// Store
	_ = store.StoreEncrypted(tokenHash, rawToken)

	// Verify exists
	if !store.Exists(tokenHash) {
		t.Fatal("token should exist after store")
	}

	// Delete
	err := store.DeleteEncrypted(tokenHash)
	if err != nil {
		t.Fatalf("DeleteEncrypted failed: %v", err)
	}

	// Verify gone
	if store.Exists(tokenHash) {
		t.Error("token should not exist after delete")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 7: File Permissions
// ═══════════════════════════════════════════════════════════════════════════

func TestSealedSecretStore_FilePermissions(t *testing.T) {
	dataDir := t.TempDir()
	store := createTestStoreWithDir(t, dataDir)

	rawToken := []byte("device-token-with-permissions")
	tokenHash := hashToken(rawToken)

	_ = store.StoreEncrypted(tokenHash, rawToken)

	// Find the file
	files, _ := os.ReadDir(dataDir)
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".sealed") {
			info, err := f.Info()
			if err != nil {
				t.Fatalf("failed to get file info: %v", err)
			}

			perm := info.Mode().Perm()
			if perm != 0600 {
				t.Errorf("expected permissions 0600, got %o", perm)
			}
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 8: APNs Transport Provider Kind
// ═══════════════════════════════════════════════════════════════════════════

func TestAPNsTransport_ProviderKind(t *testing.T) {
	store := createTestStore(t)
	apns := createTestAPNsTransport(t, store)

	if apns.ProviderKind() != pt.ProviderAPNs {
		t.Errorf("expected APNs provider kind, got %s", apns.ProviderKind())
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 9: APNs Transport Requires Token Hash
// ═══════════════════════════════════════════════════════════════════════════

func TestAPNsTransport_RequiresTokenHash(t *testing.T) {
	store := createTestStore(t)
	apns := createTestAPNsTransport(t, store)

	req := &pt.TransportRequest{
		ProviderKind: pt.ProviderAPNs,
		TokenHash:    "", // Missing
		Payload:      pt.DefaultTransportPayload("status-hash"),
		AttemptID:    "attempt-id",
	}

	ctx := context.Background()
	result, err := apns.Send(ctx, req)

	if err == nil {
		t.Error("expected error when token hash is missing")
	}

	if result.Success {
		t.Error("expected failure when token hash is missing")
	}

	if result.ErrorBucket != pt.FailureNotConfigured {
		t.Errorf("expected not_configured error bucket, got %s", result.ErrorBucket)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 10: APNs Transport Uses Sealed Store
// ═══════════════════════════════════════════════════════════════════════════

func TestAPNsTransport_UsesSealedStore(t *testing.T) {
	store := createTestStore(t)

	// Store a device token
	rawToken := []byte("device-token-for-apns-test")
	tokenHash := hashToken(rawToken)
	_ = store.StoreEncrypted(tokenHash, rawToken)

	// Create mock APNs server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the device token is in the URL
		if !strings.Contains(r.URL.Path, hex.EncodeToString(rawToken)) {
			t.Errorf("request URL should contain device token hex: %s", r.URL.Path)
		}

		// Verify payload is abstract
		body, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		_ = json.Unmarshal(body, &payload)

		aps := payload["aps"].(map[string]interface{})
		alert := aps["alert"].(map[string]interface{})

		if alert["title"] != pt.PushTitle {
			t.Errorf("expected abstract title, got %s", alert["title"])
		}

		if alert["body"] != pt.PushBody {
			t.Errorf("expected abstract body, got %s", alert["body"])
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create APNs transport with mock endpoint
	apns := createTestAPNsTransport(t, store)
	apns.SetEndpoint(server.URL)
	apns.SetClient(server.Client())

	req := &pt.TransportRequest{
		ProviderKind: pt.ProviderAPNs,
		TokenHash:    tokenHash,
		Payload:      pt.DefaultTransportPayload("status-hash"),
		AttemptID:    "attempt-id",
	}

	ctx := context.Background()
	result, err := apns.Send(ctx, req)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if !result.Success {
		t.Error("expected success")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 11: APNs Transport Handles Missing Token
// ═══════════════════════════════════════════════════════════════════════════

func TestAPNsTransport_HandlesMissingToken(t *testing.T) {
	store := createTestStore(t)
	apns := createTestAPNsTransport(t, store)

	req := &pt.TransportRequest{
		ProviderKind: pt.ProviderAPNs,
		TokenHash:    "nonexistent-token-hash",
		Payload:      pt.DefaultTransportPayload("status-hash"),
		AttemptID:    "attempt-id",
	}

	ctx := context.Background()
	result, err := apns.Send(ctx, req)

	if err == nil {
		t.Error("expected error for missing token")
	}

	if result.Success {
		t.Error("expected failure for missing token")
	}

	if result.ErrorBucket != pt.FailureNotConfigured {
		t.Errorf("expected not_configured error bucket, got %s", result.ErrorBucket)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 12: APNs Payload Is Constant
// ═══════════════════════════════════════════════════════════════════════════

func TestAPNsPayload_IsConstant(t *testing.T) {
	payload := transport.DefaultAPNsPayload()

	if payload.APS.Alert.Title != pt.PushTitle {
		t.Errorf("title should be constant %s, got %s", pt.PushTitle, payload.APS.Alert.Title)
	}

	if payload.APS.Alert.Body != pt.PushBody {
		t.Errorf("body should be constant %s, got %s", pt.PushBody, payload.APS.Alert.Body)
	}

	// Check for forbidden patterns
	payloadBytes, _ := json.Marshal(payload)
	payloadStr := string(payloadBytes)

	forbiddenPatterns := []string{"@", "merchant", "amount", "sender", "recipient", "subject"}
	for _, p := range forbiddenPatterns {
		if strings.Contains(payloadStr, p) {
			t.Errorf("payload contains forbidden pattern: %s", p)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 13: APNs Transport No Retries
// ═══════════════════════════════════════════════════════════════════════════

func TestAPNsTransport_NoRetries(t *testing.T) {
	store := createTestStore(t)

	rawToken := []byte("device-token-retry-test")
	tokenHash := hashToken(rawToken)
	_ = store.StoreEncrypted(tokenHash, rawToken)

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	apns := createTestAPNsTransport(t, store)
	apns.SetEndpoint(server.URL)
	apns.SetClient(server.Client())

	req := &pt.TransportRequest{
		ProviderKind: pt.ProviderAPNs,
		TokenHash:    tokenHash,
		Payload:      pt.DefaultTransportPayload("status-hash"),
		AttemptID:    "attempt-id",
	}

	ctx := context.Background()
	_, _ = apns.Send(ctx, req)

	// Should only have made ONE request (no retries)
	if requestCount != 1 {
		t.Errorf("expected exactly 1 request (no retries), got %d", requestCount)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 14: APNs Transport Handles 410 Gone
// ═══════════════════════════════════════════════════════════════════════════

func TestAPNsTransport_Handles410Gone(t *testing.T) {
	store := createTestStore(t)

	rawToken := []byte("device-token-gone")
	tokenHash := hashToken(rawToken)
	_ = store.StoreEncrypted(tokenHash, rawToken)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGone) // Device token no longer active
	}))
	defer server.Close()

	apns := createTestAPNsTransport(t, store)
	apns.SetEndpoint(server.URL)
	apns.SetClient(server.Client())

	req := &pt.TransportRequest{
		ProviderKind: pt.ProviderAPNs,
		TokenHash:    tokenHash,
		Payload:      pt.DefaultTransportPayload("status-hash"),
		AttemptID:    "attempt-id",
	}

	ctx := context.Background()
	result, _ := apns.Send(ctx, req)

	if result.Success {
		t.Error("expected failure for 410 Gone")
	}

	if result.ErrorBucket != pt.FailureNotConfigured {
		t.Errorf("expected not_configured for inactive token, got %s", result.ErrorBucket)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 15: Token Hash Determinism
// ═══════════════════════════════════════════════════════════════════════════

func TestTokenHash_Determinism(t *testing.T) {
	rawToken := []byte("device-token-determinism-test")

	hash1 := hashToken(rawToken)
	hash2 := hashToken(rawToken)

	if hash1 != hash2 {
		t.Errorf("token hashing is not deterministic: %s != %s", hash1, hash2)
	}

	// Hash should be 64 hex chars
	if len(hash1) != 64 {
		t.Errorf("expected 64 char hash, got %d", len(hash1))
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 16: Different Tokens Different Hashes
// ═══════════════════════════════════════════════════════════════════════════

func TestTokenHash_DifferentTokensDifferentHashes(t *testing.T) {
	token1 := []byte("device-token-1")
	token2 := []byte("device-token-2")

	hash1 := hashToken(token1)
	hash2 := hashToken(token2)

	if hash1 == hash2 {
		t.Error("different tokens should produce different hashes")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 17: Sealed Store Count
// ═══════════════════════════════════════════════════════════════════════════

func TestSealedSecretStore_Count(t *testing.T) {
	store := createTestStore(t)

	// Initially empty
	count, err := store.Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}

	// Add some tokens
	for i := 0; i < 3; i++ {
		token := []byte("device-token-" + string(rune('a'+i)))
		_ = store.StoreEncrypted(hashToken(token), token)
	}

	count, err = store.Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3, got %d", count)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 18: Context Cancellation
// ═══════════════════════════════════════════════════════════════════════════

func TestAPNsTransport_ContextCancellation(t *testing.T) {
	store := createTestStore(t)
	apns := createTestAPNsTransport(t, store)

	rawToken := []byte("device-token-context-test")
	tokenHash := hashToken(rawToken)
	_ = store.StoreEncrypted(tokenHash, rawToken)

	req := &pt.TransportRequest{
		ProviderKind: pt.ProviderAPNs,
		TokenHash:    tokenHash,
		Payload:      pt.DefaultTransportPayload("status-hash"),
		AttemptID:    "attempt-id",
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := apns.Send(ctx, req)

	if err == nil {
		t.Error("expected error for cancelled context")
	}

	if result.Success {
		t.Error("expected failure for cancelled context")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 19: Response Hash Computed
// ═══════════════════════════════════════════════════════════════════════════

func TestAPNsTransport_ResponseHashComputed(t *testing.T) {
	store := createTestStore(t)

	rawToken := []byte("device-token-hash-test")
	tokenHash := hashToken(rawToken)
	_ = store.StoreEncrypted(tokenHash, rawToken)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	apns := createTestAPNsTransport(t, store)
	apns.SetEndpoint(server.URL)
	apns.SetClient(server.Client())

	req := &pt.TransportRequest{
		ProviderKind: pt.ProviderAPNs,
		TokenHash:    tokenHash,
		Payload:      pt.DefaultTransportPayload("status-hash"),
		AttemptID:    "attempt-id",
	}

	ctx := context.Background()
	result, _ := apns.Send(ctx, req)

	if result.ResponseHash == "" {
		t.Error("expected response hash to be computed")
	}

	// Response hash should be hex
	if len(result.ResponseHash) != 32 { // 16 bytes = 32 hex chars
		t.Errorf("expected 32 char response hash, got %d", len(result.ResponseHash))
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 20: APNs Headers Set Correctly
// ═══════════════════════════════════════════════════════════════════════════

func TestAPNsTransport_HeadersSetCorrectly(t *testing.T) {
	store := createTestStore(t)

	rawToken := []byte("device-token-headers-test")
	tokenHash := hashToken(rawToken)
	_ = store.StoreEncrypted(tokenHash, rawToken)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check required headers
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		if r.Header.Get("apns-topic") != "com.quantumlife.app" {
			t.Errorf("expected apns-topic com.quantumlife.app, got %s", r.Header.Get("apns-topic"))
		}

		if r.Header.Get("apns-push-type") != "alert" {
			t.Errorf("expected apns-push-type alert, got %s", r.Header.Get("apns-push-type"))
		}

		if r.Header.Get("apns-priority") != "5" {
			t.Errorf("expected apns-priority 5 (low), got %s", r.Header.Get("apns-priority"))
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	apns := createTestAPNsTransport(t, store)
	apns.SetEndpoint(server.URL)
	apns.SetClient(server.Client())

	req := &pt.TransportRequest{
		ProviderKind: pt.ProviderAPNs,
		TokenHash:    tokenHash,
		Payload:      pt.DefaultTransportPayload("status-hash"),
		AttemptID:    "attempt-id",
	}

	ctx := context.Background()
	_, _ = apns.Send(ctx, req)
}

// ═══════════════════════════════════════════════════════════════════════════
// Helper Functions
// ═══════════════════════════════════════════════════════════════════════════

func createTestStore(t *testing.T) *persist.SealedSecretStore {
	return createTestStoreWithDir(t, t.TempDir())
}

func createTestStoreWithDir(t *testing.T, dataDir string) *persist.SealedSecretStore {
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	keyB64 := base64.StdEncoding.EncodeToString(key)

	cfg := persist.SealedSecretStoreConfig{
		EncryptionKeyBase64: keyB64,
		DataDir:             dataDir,
	}

	store, err := persist.NewSealedSecretStore(cfg)
	if err != nil {
		t.Fatalf("failed to create sealed store: %v", err)
	}

	return store
}

func createTestAPNsTransport(t *testing.T, store *persist.SealedSecretStore) *transport.APNsTransport {
	// Generate a test ECDSA key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate ECDSA key: %v", err)
	}

	cfg := transport.APNsTransportConfig{
		SealedStore: store,
		Endpoint:    "https://api.sandbox.push.apple.com",
		BundleID:    "com.quantumlife.app",
		TeamID:      "TESTTEAMID",
		KeyID:       "TESTKEYID",
	}

	apns, err := transport.NewAPNsTransport(cfg)
	if err != nil {
		t.Fatalf("failed to create APNs transport: %v", err)
	}

	// Set private key directly for testing
	_ = privateKey // We don't set it to avoid JWT complexity in tests

	return apns
}

func hashToken(token []byte) string {
	h := sha256.Sum256(token)
	return hex.EncodeToString(h[:])
}
