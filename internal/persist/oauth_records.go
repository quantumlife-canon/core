// Package persist provides OAuth record types for persistence/replay.
//
// Phase 18.8: Real OAuth (Gmail Read-Only)
// Reference: docs/ADR/ADR-0041-phase18-8-real-oauth-gmail-readonly.md
//
// CRITICAL: No tokens or secrets stored in records - only hashes and metadata.
// CRITICAL: All records have canonical string representation for deterministic replay.
package persist

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// OAuthStateRecord represents a persisted OAuth state for replay.
// CRITICAL: Does not contain the actual state token - only metadata and hash.
type OAuthStateRecord struct {
	CircleID       string
	Provider       string
	Product        string
	StateHash      string // SHA256 of the state token
	IssuedAt       time.Time
	ExpiresAt      time.Time
	Consumed       bool
	ConsumedAt     time.Time
	ReceiptHash    string // Hash of the connection receipt
	ReplaySequence int    // For deterministic replay ordering
}

// CanonicalString returns the canonical string representation.
func (r *OAuthStateRecord) CanonicalString() string {
	consumed := "false"
	consumedAt := ""
	if r.Consumed {
		consumed = "true"
		consumedAt = r.ConsumedAt.UTC().Format(time.RFC3339)
	}
	return fmt.Sprintf("OAUTH_STATE|v1|%s|%s|%s|%s|%s|%s|%s|%s|%s|%d",
		r.CircleID,
		r.Provider,
		r.Product,
		r.StateHash,
		r.IssuedAt.UTC().Format(time.RFC3339),
		r.ExpiresAt.UTC().Format(time.RFC3339),
		consumed,
		consumedAt,
		r.ReceiptHash,
		r.ReplaySequence,
	)
}

// Hash returns the SHA256 hash of the record.
func (r *OAuthStateRecord) Hash() string {
	data := []byte(r.CanonicalString())
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// OAuthTokenHandleRecord represents a persisted token handle for replay.
// CRITICAL: Does not contain the actual token - only the handle ID and metadata.
type OAuthTokenHandleRecord struct {
	HandleID       string
	CircleID       string
	Provider       string
	Product        string
	Scopes         []string
	CreatedAt      time.Time
	ExpiresAt      time.Time
	Revoked        bool
	RevokedAt      time.Time
	ReceiptHash    string // Hash of the connection receipt
	ReplaySequence int
}

// CanonicalString returns the canonical string representation.
func (r *OAuthTokenHandleRecord) CanonicalString() string {
	revoked := "false"
	revokedAt := ""
	if r.Revoked {
		revoked = "true"
		revokedAt = r.RevokedAt.UTC().Format(time.RFC3339)
	}
	scopes := ""
	for i, s := range r.Scopes {
		if i > 0 {
			scopes += ","
		}
		scopes += s
	}
	return fmt.Sprintf("OAUTH_TOKEN_HANDLE|v1|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%d",
		r.HandleID,
		r.CircleID,
		r.Provider,
		r.Product,
		scopes,
		r.CreatedAt.UTC().Format(time.RFC3339),
		r.ExpiresAt.UTC().Format(time.RFC3339),
		revoked,
		revokedAt,
		r.ReceiptHash,
		r.ReplaySequence,
	)
}

// Hash returns the SHA256 hash of the record.
func (r *OAuthTokenHandleRecord) Hash() string {
	data := []byte(r.CanonicalString())
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// GmailSyncReceiptRecord represents a persisted Gmail sync receipt for replay.
type GmailSyncReceiptRecord struct {
	CircleID        string
	Provider        string
	Product         string
	Success         bool
	FailReason      string
	At              time.Time
	MessagesFetched int
	MagnitudeBucket string // "handful", "several", "many"
	EventsGenerated int
	ConnectionHash  string // Hash of the connection receipt
	ReceiptHash     string // Hash of the sync receipt
	ReplaySequence  int
}

// CanonicalString returns the canonical string representation.
func (r *GmailSyncReceiptRecord) CanonicalString() string {
	return fmt.Sprintf("GMAIL_SYNC_RECEIPT|v1|%s|%s|%s|%t|%s|%s|%d|%s|%d|%s|%s|%d",
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
		r.ReceiptHash,
		r.ReplaySequence,
	)
}

// Hash returns the SHA256 hash of the record.
func (r *GmailSyncReceiptRecord) Hash() string {
	data := []byte(r.CanonicalString())
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// OAuthRevokeReceiptRecord represents a persisted revoke receipt for replay.
type OAuthRevokeReceiptRecord struct {
	CircleID        string
	Provider        string
	Product         string
	Success         bool
	FailReason      string
	At              time.Time
	ProviderRevoked bool
	LocalRemoved    bool
	ConnectionHash  string // Hash of original connection
	ReceiptHash     string // Hash of the revoke receipt
	ReplaySequence  int
}

// CanonicalString returns the canonical string representation.
func (r *OAuthRevokeReceiptRecord) CanonicalString() string {
	return fmt.Sprintf("OAUTH_REVOKE_RECEIPT|v1|%s|%s|%s|%t|%s|%s|%t|%t|%s|%s|%d",
		r.CircleID,
		r.Provider,
		r.Product,
		r.Success,
		r.FailReason,
		r.At.UTC().Format(time.RFC3339),
		r.ProviderRevoked,
		r.LocalRemoved,
		r.ConnectionHash,
		r.ReceiptHash,
		r.ReplaySequence,
	)
}

// Hash returns the SHA256 hash of the record.
func (r *OAuthRevokeReceiptRecord) Hash() string {
	data := []byte(r.CanonicalString())
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// OAuthRecordStore provides in-memory storage for OAuth records.
// For demo/testing - production would use persistent storage.
type OAuthRecordStore struct {
	states         []*OAuthStateRecord
	tokenHandles   []*OAuthTokenHandleRecord
	syncReceipts   []*GmailSyncReceiptRecord
	revokeReceipts []*OAuthRevokeReceiptRecord
	sequence       int
}

// NewOAuthRecordStore creates a new OAuth record store.
func NewOAuthRecordStore() *OAuthRecordStore {
	return &OAuthRecordStore{
		states:         make([]*OAuthStateRecord, 0),
		tokenHandles:   make([]*OAuthTokenHandleRecord, 0),
		syncReceipts:   make([]*GmailSyncReceiptRecord, 0),
		revokeReceipts: make([]*OAuthRevokeReceiptRecord, 0),
	}
}

// AppendState appends an OAuth state record.
func (s *OAuthRecordStore) AppendState(record *OAuthStateRecord) {
	s.sequence++
	record.ReplaySequence = s.sequence
	s.states = append(s.states, record)
}

// AppendTokenHandle appends a token handle record.
func (s *OAuthRecordStore) AppendTokenHandle(record *OAuthTokenHandleRecord) {
	s.sequence++
	record.ReplaySequence = s.sequence
	s.tokenHandles = append(s.tokenHandles, record)
}

// AppendSyncReceipt appends a sync receipt record.
func (s *OAuthRecordStore) AppendSyncReceipt(record *GmailSyncReceiptRecord) {
	s.sequence++
	record.ReplaySequence = s.sequence
	s.syncReceipts = append(s.syncReceipts, record)
}

// AppendRevokeReceipt appends a revoke receipt record.
func (s *OAuthRecordStore) AppendRevokeReceipt(record *OAuthRevokeReceiptRecord) {
	s.sequence++
	record.ReplaySequence = s.sequence
	s.revokeReceipts = append(s.revokeReceipts, record)
}

// States returns all state records.
func (s *OAuthRecordStore) States() []*OAuthStateRecord {
	return s.states
}

// TokenHandles returns all token handle records.
func (s *OAuthRecordStore) TokenHandles() []*OAuthTokenHandleRecord {
	return s.tokenHandles
}

// SyncReceipts returns all sync receipt records.
func (s *OAuthRecordStore) SyncReceipts() []*GmailSyncReceiptRecord {
	return s.syncReceipts
}

// RevokeReceipts returns all revoke receipt records.
func (s *OAuthRecordStore) RevokeReceipts() []*OAuthRevokeReceiptRecord {
	return s.revokeReceipts
}

// Sequence returns the current sequence number.
func (s *OAuthRecordStore) Sequence() int {
	return s.sequence
}
