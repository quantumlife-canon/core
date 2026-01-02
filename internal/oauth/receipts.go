// Package oauth provides OAuth receipts for audit logging.
//
// Phase 18.8: Real OAuth (Gmail Read-Only)
// Reference: docs/ADR/ADR-0041-phase18-8-real-oauth-gmail-readonly.md
//
// CRITICAL: Receipts contain NO sensitive data (no tokens, no secrets).
// CRITICAL: All receipts have canonical string representation for hashing.
package oauth

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// Provider identifies an OAuth provider.
type Provider string

const (
	ProviderGoogle Provider = "google"
)

// Product identifies what product/API we're accessing.
type Product string

const (
	ProductGmail Product = "gmail"
)

// ConnectionReceipt records what happened during a connect/sync operation.
type ConnectionReceipt struct {
	CircleID    string
	Provider    Provider
	Product     Product
	Action      ReceiptAction
	Success     bool
	FailReason  string // Only set if Success=false, no PII
	At          time.Time
	StateHash   string // Hash of OAuthState used (for correlation)
	TokenHandle string // Opaque handle ID (not the token itself)
}

// ReceiptAction describes what action was taken.
type ReceiptAction string

const (
	ActionOAuthStart    ReceiptAction = "oauth_start"
	ActionOAuthCallback ReceiptAction = "oauth_callback"
	ActionTokenMint     ReceiptAction = "token_mint"
	ActionSync          ReceiptAction = "sync"
	ActionRevoke        ReceiptAction = "revoke"
)

// CanonicalString returns the canonical string representation.
func (r *ConnectionReceipt) CanonicalString() string {
	return fmt.Sprintf("CONN_RECEIPT|v1|%s|%s|%s|%s|%t|%s|%s|%s|%s",
		r.CircleID,
		r.Provider,
		r.Product,
		r.Action,
		r.Success,
		r.FailReason,
		r.At.UTC().Format(time.RFC3339),
		r.StateHash,
		r.TokenHandle,
	)
}

// Hash returns the SHA256 hash of the receipt.
func (r *ConnectionReceipt) Hash() string {
	data := []byte(r.CanonicalString())
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// SyncReceipt records what happened during a Gmail sync.
type SyncReceipt struct {
	CircleID        string
	Provider        Provider
	Product         Product
	Success         bool
	FailReason      string
	At              time.Time
	MessagesFetched int    // Raw count for internal use
	MagnitudeBucket string // "handful", "many", "several" for display
	EventsGenerated int
	ConnectionHash  string // Hash of connection receipt for correlation
}

// CanonicalString returns the canonical string representation.
func (r *SyncReceipt) CanonicalString() string {
	return fmt.Sprintf("SYNC_RECEIPT|v1|%s|%s|%s|%t|%s|%s|%d|%s|%d|%s",
		r.CircleID,
		r.Provider,
		r.Product,
		r.Success,
		r.FailReason,
		r.At.UTC().Format(time.RFC3339),
		r.MessagesFetched,
		r.MagnitudeBucket,
		r.EventsGenerated,
		r.ConnectionHash,
	)
}

// Hash returns the SHA256 hash of the receipt.
func (r *SyncReceipt) Hash() string {
	data := []byte(r.CanonicalString())
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// RevokeReceipt records what happened during a revocation.
type RevokeReceipt struct {
	CircleID        string
	Provider        Provider
	Product         Product
	Success         bool
	FailReason      string
	At              time.Time
	ProviderRevoked bool   // Whether Google revoke succeeded
	LocalRemoved    bool   // Whether local token was removed
	ConnectionHash  string // Hash of original connection for correlation
}

// CanonicalString returns the canonical string representation.
func (r *RevokeReceipt) CanonicalString() string {
	return fmt.Sprintf("REVOKE_RECEIPT|v1|%s|%s|%s|%t|%s|%s|%t|%t|%s",
		r.CircleID,
		r.Provider,
		r.Product,
		r.Success,
		r.FailReason,
		r.At.UTC().Format(time.RFC3339),
		r.ProviderRevoked,
		r.LocalRemoved,
		r.ConnectionHash,
	)
}

// Hash returns the SHA256 hash of the receipt.
func (r *RevokeReceipt) Hash() string {
	data := []byte(r.CanonicalString())
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// MagnitudeBucket converts a count to a display bucket.
func MagnitudeBucket(count int) string {
	switch {
	case count == 0:
		return "none"
	case count <= 5:
		return "handful"
	case count <= 20:
		return "several"
	default:
		return "many"
	}
}
