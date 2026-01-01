// Package approvaltoken defines signed approval tokens for link-based approvals.
//
// Phase 15: Household Approvals + Intersections (Deterministic)
//
// Approval tokens allow household members to approve or reject actions via signed
// URLs without requiring login. Each token is bound to a specific action, person,
// and approval state.
//
// CRITICAL: All operations are deterministic. Same inputs + clock => same outputs.
// CRITICAL: No goroutines. No time.Now(). Clock must be injected.
// CRITICAL: Tokens must be Ed25519 signed for security.
//
// Reference: docs/ADR/ADR-0031-phase15-household-approvals.md
package approvaltoken

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"quantumlife/pkg/domain/identity"
)

// ActionType defines what the token authorizes.
type ActionType string

const (
	// ActionTypeApprove authorizes approval of the target.
	ActionTypeApprove ActionType = "approve"

	// ActionTypeReject authorizes rejection of the target.
	ActionTypeReject ActionType = "reject"
)

// Token represents a signed approval token.
type Token struct {
	// TokenID uniquely identifies this token.
	TokenID string

	// StateID is the approval state this token is for.
	StateID string

	// PersonID is the authorized user of this token.
	PersonID identity.EntityID

	// ActionType is what this token authorizes.
	ActionType ActionType

	// CreatedAt is when the token was created.
	CreatedAt time.Time

	// ExpiresAt is when the token expires.
	ExpiresAt time.Time

	// SignatureAlgorithm is the algorithm used to sign.
	SignatureAlgorithm string

	// KeyID identifies the signing key.
	KeyID string

	// Signature is the Ed25519 signature.
	Signature []byte

	// Hash is SHA256 of the canonical string (excluding signature).
	Hash string
}

// NewToken creates a new unsigned approval token.
func NewToken(
	stateID string,
	personID identity.EntityID,
	actionType ActionType,
	createdAt time.Time,
	expiresAt time.Time,
) *Token {
	t := &Token{
		StateID:    stateID,
		PersonID:   personID,
		ActionType: actionType,
		CreatedAt:  createdAt,
		ExpiresAt:  expiresAt,
	}

	// Generate deterministic token ID
	t.TokenID = t.computeTokenID()
	t.Hash = t.computeHash()

	return t
}

// computeTokenID generates a deterministic ID from token contents.
func (t *Token) computeTokenID() string {
	input := fmt.Sprintf("approval_token|%s|%s|%s|%s",
		t.StateID, t.PersonID, t.ActionType, t.CreatedAt.UTC().Format(time.RFC3339))
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:])[:16]
}

// computeHash computes SHA256 hash of the signable content.
func (t *Token) computeHash() string {
	canonical := t.SignableString()
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:])
}

// SignableString returns the canonical string that gets signed.
func (t *Token) SignableString() string {
	var sb strings.Builder

	sb.WriteString("approval_token|")
	sb.WriteString("token_id:")
	sb.WriteString(t.TokenID)
	sb.WriteString("|state_id:")
	sb.WriteString(t.StateID)
	sb.WriteString("|person_id:")
	sb.WriteString(string(t.PersonID))
	sb.WriteString("|action:")
	sb.WriteString(string(t.ActionType))
	sb.WriteString("|created:")
	sb.WriteString(t.CreatedAt.UTC().Format(time.RFC3339))
	sb.WriteString("|expires:")
	sb.WriteString(t.ExpiresAt.UTC().Format(time.RFC3339))

	return sb.String()
}

// SignableBytes returns the bytes that should be signed.
func (t *Token) SignableBytes() []byte {
	return []byte(t.SignableString())
}

// SetSignature sets the signature on the token.
func (t *Token) SetSignature(algorithm, keyID string, signature []byte) {
	t.SignatureAlgorithm = algorithm
	t.KeyID = keyID
	t.Signature = signature
}

// IsSigned returns whether the token has a signature.
func (t *Token) IsSigned() bool {
	return len(t.Signature) > 0
}

// IsExpired checks if the token has expired.
func (t *Token) IsExpired(now time.Time) bool {
	return now.After(t.ExpiresAt)
}

// IsValid performs basic validation.
func (t *Token) IsValid() error {
	if t.TokenID == "" {
		return errors.New("missing token ID")
	}
	if t.StateID == "" {
		return errors.New("missing state ID")
	}
	if t.PersonID == "" {
		return errors.New("missing person ID")
	}
	if t.ActionType != ActionTypeApprove && t.ActionType != ActionTypeReject {
		return fmt.Errorf("invalid action type: %s", t.ActionType)
	}
	if t.CreatedAt.IsZero() {
		return errors.New("missing created timestamp")
	}
	if t.ExpiresAt.IsZero() {
		return errors.New("missing expiry timestamp")
	}
	return nil
}

// CanonicalString returns the full canonical string including signature info.
func (t *Token) CanonicalString() string {
	var sb strings.Builder

	sb.WriteString(t.SignableString())
	sb.WriteString("|alg:")
	sb.WriteString(t.SignatureAlgorithm)
	sb.WriteString("|key:")
	sb.WriteString(t.KeyID)
	sb.WriteString("|sig_len:")
	sb.WriteString(fmt.Sprintf("%d", len(t.Signature)))

	return sb.String()
}

// Encode encodes the token to a URL-safe string.
// Format: base64url(token_id|state_id|person_id|action|created_unix|expires_unix|alg|key|sig_b64)
func (t *Token) Encode() string {
	var sb strings.Builder

	sb.WriteString(t.TokenID)
	sb.WriteString("|")
	sb.WriteString(t.StateID)
	sb.WriteString("|")
	sb.WriteString(string(t.PersonID))
	sb.WriteString("|")
	sb.WriteString(string(t.ActionType))
	sb.WriteString("|")
	sb.WriteString(fmt.Sprintf("%d", t.CreatedAt.Unix()))
	sb.WriteString("|")
	sb.WriteString(fmt.Sprintf("%d", t.ExpiresAt.Unix()))
	sb.WriteString("|")
	sb.WriteString(t.SignatureAlgorithm)
	sb.WriteString("|")
	sb.WriteString(t.KeyID)
	sb.WriteString("|")
	sb.WriteString(base64.RawURLEncoding.EncodeToString(t.Signature))

	return base64.RawURLEncoding.EncodeToString([]byte(sb.String()))
}

// Decode decodes a token from a URL-safe string.
func Decode(encoded string) (*Token, error) {
	// Decode base64
	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}

	// Split components
	parts := strings.Split(string(data), "|")
	if len(parts) != 9 {
		return nil, fmt.Errorf("invalid token format: expected 9 parts, got %d", len(parts))
	}

	// Parse timestamps (Unix format)
	var createdUnix, expiresUnix int64
	_, err = fmt.Sscanf(parts[4], "%d", &createdUnix)
	if err != nil {
		return nil, fmt.Errorf("parse created timestamp: %w", err)
	}
	_, err = fmt.Sscanf(parts[5], "%d", &expiresUnix)
	if err != nil {
		return nil, fmt.Errorf("parse expires timestamp: %w", err)
	}

	// Decode signature
	sig, err := base64.RawURLEncoding.DecodeString(parts[8])
	if err != nil {
		return nil, fmt.Errorf("decode signature: %w", err)
	}

	t := &Token{
		TokenID:            parts[0],
		StateID:            parts[1],
		PersonID:           identity.EntityID(parts[2]),
		ActionType:         ActionType(parts[3]),
		CreatedAt:          time.Unix(createdUnix, 0).UTC(),
		ExpiresAt:          time.Unix(expiresUnix, 0).UTC(),
		SignatureAlgorithm: parts[6],
		KeyID:              parts[7],
		Signature:          sig,
	}

	// Recompute hash
	t.Hash = t.computeHash()

	return t, nil
}

// TokenSet holds multiple tokens for batch operations.
type TokenSet struct {
	// Tokens maps token ID to token.
	Tokens map[string]*Token

	// Version is incremented on each update.
	Version int

	// Hash is SHA256 of the set.
	Hash string
}

// NewTokenSet creates an empty token set.
func NewTokenSet() *TokenSet {
	s := &TokenSet{
		Tokens:  make(map[string]*Token),
		Version: 1,
	}
	s.ComputeHash()
	return s
}

// Add adds a token to the set.
func (s *TokenSet) Add(token *Token) {
	s.Tokens[token.TokenID] = token
	s.Version++
	s.ComputeHash()
}

// Get returns a token by ID.
func (s *TokenSet) Get(tokenID string) *Token {
	return s.Tokens[tokenID]
}

// GetByStateAndPerson returns tokens for a specific state and person.
func (s *TokenSet) GetByStateAndPerson(stateID string, personID identity.EntityID) []*Token {
	var result []*Token
	for _, t := range s.Tokens {
		if t.StateID == stateID && t.PersonID == personID {
			result = append(result, t)
		}
	}
	return result
}

// List returns all tokens in deterministic order.
func (s *TokenSet) List() []*Token {
	ids := make([]string, 0, len(s.Tokens))
	for id := range s.Tokens {
		ids = append(ids, id)
	}
	bubbleSort(ids)

	result := make([]*Token, len(ids))
	for i, id := range ids {
		result[i] = s.Tokens[id]
	}
	return result
}

// ListActive returns all non-expired tokens.
func (s *TokenSet) ListActive(now time.Time) []*Token {
	var result []*Token
	for _, t := range s.List() {
		if !t.IsExpired(now) {
			result = append(result, t)
		}
	}
	return result
}

// Remove removes a token from the set.
func (s *TokenSet) Remove(tokenID string) bool {
	if _, exists := s.Tokens[tokenID]; exists {
		delete(s.Tokens, tokenID)
		s.Version++
		s.ComputeHash()
		return true
	}
	return false
}

// PruneExpired removes all expired tokens.
func (s *TokenSet) PruneExpired(now time.Time) int {
	pruned := 0
	for id, t := range s.Tokens {
		if t.IsExpired(now) {
			delete(s.Tokens, id)
			pruned++
		}
	}
	if pruned > 0 {
		s.Version++
		s.ComputeHash()
	}
	return pruned
}

// CanonicalString returns a deterministic representation.
func (s *TokenSet) CanonicalString() string {
	var sb strings.Builder
	sb.WriteString("token_set|version:")
	sb.WriteString(fmt.Sprintf("%d", s.Version))
	sb.WriteString("|tokens:[")

	tokens := s.List()
	for i, t := range tokens {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(t.CanonicalString())
	}
	sb.WriteString("]")

	return sb.String()
}

// ComputeHash computes and sets the Hash field.
func (s *TokenSet) ComputeHash() string {
	canonical := s.CanonicalString()
	hash := sha256.Sum256([]byte(canonical))
	s.Hash = hex.EncodeToString(hash[:])
	return s.Hash
}

// bubbleSort sorts strings in place.
func bubbleSort(s []string) {
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[i] > s[j] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

// TokenStats holds token statistics.
type TokenStats struct {
	TotalTokens  int
	ActiveCount  int
	ExpiredCount int
	ApproveCount int
	RejectCount  int
}

// GetStats returns statistics for the set.
func (s *TokenSet) GetStats(now time.Time) TokenStats {
	stats := TokenStats{
		TotalTokens: len(s.Tokens),
	}

	for _, t := range s.Tokens {
		if t.IsExpired(now) {
			stats.ExpiredCount++
		} else {
			stats.ActiveCount++
		}

		if t.ActionType == ActionTypeApprove {
			stats.ApproveCount++
		} else if t.ActionType == ActionTypeReject {
			stats.RejectCount++
		}
	}

	return stats
}

// Errors for token operations.
var (
	ErrTokenNotFound    = errors.New("token not found")
	ErrTokenExpired     = errors.New("token expired")
	ErrTokenInvalid     = errors.New("token invalid")
	ErrSignatureInvalid = errors.New("signature invalid")
	ErrPersonMismatch   = errors.New("person ID mismatch")
	ErrStateMismatch    = errors.New("state ID mismatch")
)
