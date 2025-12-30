// Package primitives provides approval artifacts for v7 multi-party approval.
//
// CRITICAL: Approvals are bound to specific actions via ActionHash.
// This prevents replay attacks where an approval for one action
// is used to authorize a different action.
//
// Reference: v7 Multi-party approval governance
package primitives

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// ApprovalArtifact represents a signed approval from a circle.
// Approvals are bound to a specific action and expire after a configured time.
type ApprovalArtifact struct {
	// ApprovalID uniquely identifies this approval.
	ApprovalID string

	// IntersectionID is the intersection this approval applies to.
	IntersectionID string

	// ContractVersion is the contract version at time of approval.
	ContractVersion string

	// ActionID is the action being approved.
	ActionID string

	// ActionHash is the SHA-256 hash of the canonical action representation.
	// This binds the approval to the exact action parameters.
	ActionHash string

	// ApproverCircleID is the circle that granted this approval.
	ApproverCircleID string

	// ScopesApproved lists the scopes this approval covers.
	ScopesApproved []string

	// ApprovedAt is when the approval was granted.
	ApprovedAt time.Time

	// ExpiresAt is when the approval expires and becomes invalid.
	ExpiresAt time.Time

	// Signature is the HMAC-SHA256 signature of the approval.
	// In production, this would be a proper cryptographic signature.
	Signature []byte
}

// IsExpired checks if the approval has expired.
func (a *ApprovalArtifact) IsExpired(now time.Time) bool {
	return now.After(a.ExpiresAt)
}

// IsValid checks if the approval is valid for the given action.
func (a *ApprovalArtifact) IsValid(actionHash string, now time.Time) bool {
	if a.IsExpired(now) {
		return false
	}
	return a.ActionHash == actionHash
}

// Validate checks that the approval has all required fields.
func (a *ApprovalArtifact) Validate() error {
	if a.ApprovalID == "" {
		return errors.New("approval ID is required")
	}
	if a.IntersectionID == "" {
		return errors.New("intersection ID is required")
	}
	if a.ActionID == "" {
		return errors.New("action ID is required")
	}
	if a.ActionHash == "" {
		return errors.New("action hash is required")
	}
	if a.ApproverCircleID == "" {
		return errors.New("approver circle ID is required")
	}
	if a.ApprovedAt.IsZero() {
		return errors.New("approved at timestamp is required")
	}
	if a.ExpiresAt.IsZero() {
		return errors.New("expires at timestamp is required")
	}
	if len(a.Signature) == 0 {
		return errors.New("signature is required")
	}
	return nil
}

// ApprovalRequestToken is a signed token requesting approval for an action.
// This is passed to approvers who can then submit their approval.
type ApprovalRequestToken struct {
	// TokenID uniquely identifies this request token.
	TokenID string

	// IntersectionID is the intersection this request applies to.
	IntersectionID string

	// ContractVersion is the contract version.
	ContractVersion string

	// ActionID is the action requiring approval.
	ActionID string

	// ActionHash is the SHA-256 hash of the canonical action.
	ActionHash string

	// ActionType describes the action (e.g., "calendar.create_event").
	ActionType string

	// ActionSummary is a human-readable summary of the action.
	ActionSummary string

	// RequestingCircleID is the circle that created this request.
	RequestingCircleID string

	// ScopesRequired lists the scopes the action requires.
	ScopesRequired []string

	// CreatedAt is when this request was created.
	CreatedAt time.Time

	// ExpiresAt is when this request token expires.
	ExpiresAt time.Time

	// Signature is the HMAC-SHA256 signature of the token.
	Signature []byte
}

// IsExpired checks if the request token has expired.
func (t *ApprovalRequestToken) IsExpired(now time.Time) bool {
	return now.After(t.ExpiresAt)
}

// Validate checks that the token has all required fields.
func (t *ApprovalRequestToken) Validate() error {
	if t.TokenID == "" {
		return errors.New("token ID is required")
	}
	if t.IntersectionID == "" {
		return errors.New("intersection ID is required")
	}
	if t.ActionID == "" {
		return errors.New("action ID is required")
	}
	if t.ActionHash == "" {
		return errors.New("action hash is required")
	}
	if t.RequestingCircleID == "" {
		return errors.New("requesting circle ID is required")
	}
	if t.CreatedAt.IsZero() {
		return errors.New("created at timestamp is required")
	}
	if t.ExpiresAt.IsZero() {
		return errors.New("expires at timestamp is required")
	}
	if len(t.Signature) == 0 {
		return errors.New("signature is required")
	}
	return nil
}

// ApprovalVerificationResult contains the result of verifying approvals.
type ApprovalVerificationResult struct {
	// Passed indicates if the verification passed.
	Passed bool

	// ThresholdMet is the number of valid approvals vs required.
	ThresholdMet      int
	ThresholdRequired int

	// ValidApprovals lists the approvals that were valid.
	ValidApprovals []string // Approval IDs

	// InvalidApprovals lists approvals that failed validation.
	InvalidApprovals []ApprovalFailure

	// MissingApprovers lists required approvers who haven't approved.
	MissingApprovers []string

	// Reason explains why verification failed (if Passed is false).
	Reason string
}

// ApprovalFailure describes why an approval was rejected.
type ApprovalFailure struct {
	ApprovalID string
	CircleID   string
	Reason     string
}

// SignApprovalArtifact creates a signature for an approval artifact.
// In production, this would use proper asymmetric cryptography.
// For now, we use HMAC-SHA256 with a shared secret.
func SignApprovalArtifact(artifact *ApprovalArtifact, secret []byte) []byte {
	data := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%d|%d",
		artifact.ApprovalID,
		artifact.IntersectionID,
		artifact.ActionID,
		artifact.ActionHash,
		artifact.ApproverCircleID,
		artifact.ContractVersion,
		artifact.ApprovedAt.Unix(),
		artifact.ExpiresAt.Unix(),
	)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}

// VerifyApprovalSignature verifies the signature on an approval artifact.
func VerifyApprovalSignature(artifact *ApprovalArtifact, secret []byte) bool {
	expected := SignApprovalArtifact(artifact, secret)
	return hmac.Equal(artifact.Signature, expected)
}

// SignRequestToken creates a signature for a request token.
func SignRequestToken(token *ApprovalRequestToken, secret []byte) []byte {
	data := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%d|%d",
		token.TokenID,
		token.IntersectionID,
		token.ActionID,
		token.ActionHash,
		token.RequestingCircleID,
		token.ContractVersion,
		token.CreatedAt.Unix(),
		token.ExpiresAt.Unix(),
	)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}

// VerifyRequestTokenSignature verifies the signature on a request token.
func VerifyRequestTokenSignature(token *ApprovalRequestToken, secret []byte) bool {
	expected := SignRequestToken(token, secret)
	return hmac.Equal(token.Signature, expected)
}

// EncodeApprovalToken encodes an approval request token to a string.
// This is a simple hex encoding for the demo.
func EncodeApprovalToken(token *ApprovalRequestToken) string {
	// Format: tokenID:intersectionID:actionID:actionHash:requestingCircle:contractVersion:created:expires:signature
	data := fmt.Sprintf("%s:%s:%s:%s:%s:%s:%d:%d:%s",
		token.TokenID,
		token.IntersectionID,
		token.ActionID,
		token.ActionHash,
		token.RequestingCircleID,
		token.ContractVersion,
		token.CreatedAt.Unix(),
		token.ExpiresAt.Unix(),
		hex.EncodeToString(token.Signature),
	)
	return hex.EncodeToString([]byte(data))
}

// DecodeApprovalToken decodes an approval request token from a string.
func DecodeApprovalToken(encoded string) (*ApprovalRequestToken, error) {
	data, err := hex.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("invalid token encoding: %w", err)
	}

	var tokenID, intersectionID, actionID, actionHash, requestingCircle, contractVersion, sigHex string
	var createdUnix, expiresUnix int64

	_, err = fmt.Sscanf(string(data), "%s:%s:%s:%s:%s:%s:%d:%d:%s",
		&tokenID, &intersectionID, &actionID, &actionHash,
		&requestingCircle, &contractVersion, &createdUnix, &expiresUnix, &sigHex)
	if err != nil {
		// Try parsing with split
		parts := splitToken(string(data))
		if len(parts) != 9 {
			return nil, fmt.Errorf("invalid token format: expected 9 parts, got %d", len(parts))
		}
		tokenID = parts[0]
		intersectionID = parts[1]
		actionID = parts[2]
		actionHash = parts[3]
		requestingCircle = parts[4]
		contractVersion = parts[5]
		_, _ = fmt.Sscanf(parts[6], "%d", &createdUnix)
		_, _ = fmt.Sscanf(parts[7], "%d", &expiresUnix)
		sigHex = parts[8]
	}

	sig, err := hex.DecodeString(sigHex)
	if err != nil {
		return nil, fmt.Errorf("invalid signature encoding: %w", err)
	}

	return &ApprovalRequestToken{
		TokenID:            tokenID,
		IntersectionID:     intersectionID,
		ActionID:           actionID,
		ActionHash:         actionHash,
		RequestingCircleID: requestingCircle,
		ContractVersion:    contractVersion,
		CreatedAt:          time.Unix(createdUnix, 0),
		ExpiresAt:          time.Unix(expiresUnix, 0),
		Signature:          sig,
	}, nil
}

// splitToken splits a token string by colons.
func splitToken(s string) []string {
	var parts []string
	var current []byte
	for i := 0; i < len(s); i++ {
		if s[i] == ':' {
			parts = append(parts, string(current))
			current = nil
		} else {
			current = append(current, s[i])
		}
	}
	parts = append(parts, string(current))
	return parts
}
