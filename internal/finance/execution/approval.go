package execution

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// ApprovalManager handles approval requests and verification.
// Per Technical Split v9 §5, approvals must be:
// - Per-action (no standing approvals)
// - Hash-bound to specific ActionHash
// - Time-bounded
// - Signed and timestamped
type ApprovalManager struct {
	idGenerator     func() string
	languageChecker *ApprovalLanguageChecker
	signingKey      []byte // For demo purposes only
}

// NewApprovalManager creates a new approval manager.
func NewApprovalManager(idGen func() string, signingKey []byte) *ApprovalManager {
	return &ApprovalManager{
		idGenerator:     idGen,
		languageChecker: NewApprovalLanguageChecker(),
		signingKey:      signingKey,
	}
}

// CreateApprovalRequest creates an approval request for an envelope.
// The prompt text MUST be neutral per Canon Addendum v9 §3.6.
func (m *ApprovalManager) CreateApprovalRequest(
	env *ExecutionEnvelope,
	targetCircleID string,
	expiresAt time.Time,
	now time.Time,
) (*ApprovalRequest, error) {
	// Generate neutral prompt text
	promptText := m.generateNeutralPrompt(env)

	// Verify prompt is neutral
	if violations := m.languageChecker.Check(promptText); len(violations) > 0 {
		return nil, fmt.Errorf("approval prompt contains forbidden language: %v", violations)
	}

	return &ApprovalRequest{
		RequestID:      m.idGenerator(),
		EnvelopeID:     env.EnvelopeID,
		ActionHash:     env.ActionHash,
		PromptText:     promptText,
		RequestedAt:    now,
		ExpiresAt:      expiresAt,
		TargetCircleID: targetCircleID,
	}, nil
}

// generateNeutralPrompt generates a neutral, factual approval prompt.
// Per Canon Addendum v9 §3.6: descriptive only, no urgency/fear/authority/optimization.
func (m *ApprovalManager) generateNeutralPrompt(env *ExecutionEnvelope) string {
	return fmt.Sprintf(
		"Approval requested for %s of %s %d.%02d to %s. Action hash: %s",
		env.ActionSpec.Type,
		env.ActionSpec.Currency,
		env.ActionSpec.AmountCents/100,
		env.ActionSpec.AmountCents%100,
		env.ActionSpec.Recipient,
		env.ActionHash[:16]+"...",
	)
}

// SubmitApproval creates an approval artifact for a request.
// The approval is bound to the specific ActionHash.
func (m *ApprovalManager) SubmitApproval(
	request *ApprovalRequest,
	approverCircleID string,
	approverID string,
	expiresAt time.Time,
	now time.Time,
) (*ApprovalArtifact, error) {
	// Validate request is not expired
	if now.After(request.ExpiresAt) {
		return nil, fmt.Errorf("approval request has expired")
	}

	// Create signature
	signature := m.sign(request.ActionHash, approverCircleID, approverID, now)

	return &ApprovalArtifact{
		ArtifactID:         m.idGenerator(),
		ApproverCircleID:   approverCircleID,
		ApproverID:         approverID,
		ActionHash:         request.ActionHash,
		ApprovedAt:         now,
		ExpiresAt:          expiresAt,
		Signature:          signature,
		SignatureAlgorithm: "HMAC-SHA256",
	}, nil
}

// sign creates a signature for an approval.
func (m *ApprovalManager) sign(actionHash, circleID, approverID string, timestamp time.Time) string {
	mac := hmac.New(sha256.New, m.signingKey)
	mac.Write([]byte(actionHash))
	mac.Write([]byte(circleID))
	mac.Write([]byte(approverID))
	mac.Write([]byte(timestamp.Format(time.RFC3339Nano)))
	return hex.EncodeToString(mac.Sum(nil))
}

// ApprovalVerifier verifies approval artifacts.
// Per Technical Split v9 §10.2.
type ApprovalVerifier struct {
	signingKey []byte
}

// NewApprovalVerifier creates a new approval verifier.
func NewApprovalVerifier(signingKey []byte) *ApprovalVerifier {
	return &ApprovalVerifier{
		signingKey: signingKey,
	}
}

// VerifyApproval verifies an approval artifact.
// Returns error if verification fails.
func (v *ApprovalVerifier) VerifyApproval(
	artifact *ApprovalArtifact,
	expectedActionHash string,
	now time.Time,
) error {
	// Check ActionHash binding
	if artifact.ActionHash != expectedActionHash {
		return fmt.Errorf("approval bound to different ActionHash: expected %s, got %s",
			expectedActionHash[:16], artifact.ActionHash[:16])
	}

	// Check expiry
	if artifact.IsExpired(now) {
		return fmt.Errorf("approval expired at %s", artifact.ExpiresAt.Format(time.RFC3339))
	}

	// Verify signature
	expectedSig := v.computeExpectedSignature(artifact)
	if artifact.Signature != expectedSig {
		return fmt.Errorf("invalid signature")
	}

	return nil
}

// computeExpectedSignature computes what the signature should be.
func (v *ApprovalVerifier) computeExpectedSignature(artifact *ApprovalArtifact) string {
	mac := hmac.New(sha256.New, v.signingKey)
	mac.Write([]byte(artifact.ActionHash))
	mac.Write([]byte(artifact.ApproverCircleID))
	mac.Write([]byte(artifact.ApproverID))
	mac.Write([]byte(artifact.ApprovedAt.Format(time.RFC3339Nano)))
	return hex.EncodeToString(mac.Sum(nil))
}

// ApprovalLanguageChecker verifies approval language is neutral.
// Per Canon Addendum v9 §3.6.
type ApprovalLanguageChecker struct {
	forbiddenWords map[string]string // word -> category
}

// LanguageViolation represents a forbidden language violation.
type LanguageViolation struct {
	Word     string
	Category string
	Position int
}

// NewApprovalLanguageChecker creates a new language checker.
func NewApprovalLanguageChecker() *ApprovalLanguageChecker {
	return &ApprovalLanguageChecker{
		forbiddenWords: map[string]string{
			// Urgency
			"urgent":         "urgency",
			"immediately":    "urgency",
			"now":            "urgency",
			"hurry":          "urgency",
			"deadline":       "urgency",
			"last chance":    "urgency",
			"time-sensitive": "urgency",
			"expiring":       "urgency",

			// Fear
			"risk":    "fear",
			"danger":  "fear",
			"warning": "fear",
			"alert":   "fear",
			"problem": "fear",
			"issue":   "fear",
			"concern": "fear",
			"threat":  "fear",
			"loss":    "fear",

			// Authority
			"recommend":   "authority",
			"recommended": "authority",
			"suggest":     "authority",
			"suggested":   "authority",
			"should":      "authority",
			"must":        "authority",
			"best":        "authority",
			"optimal":     "authority",
			"advised":     "authority",

			// Optimization
			"save":      "optimization",
			"optimize":  "optimization",
			"better":    "optimization",
			"improve":   "optimization",
			"efficient": "optimization",
			"maximize":  "optimization",
			"minimize":  "optimization",
		},
	}
}

// Check checks text for forbidden language.
func (c *ApprovalLanguageChecker) Check(text string) []LanguageViolation {
	var violations []LanguageViolation
	lowerText := strings.ToLower(text)

	for word, category := range c.forbiddenWords {
		if pos := strings.Index(lowerText, word); pos >= 0 {
			violations = append(violations, LanguageViolation{
				Word:     word,
				Category: category,
				Position: pos,
			})
		}
	}

	return violations
}
