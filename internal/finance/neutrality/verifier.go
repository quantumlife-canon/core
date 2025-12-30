// Package neutrality provides multi-party neutrality verification.
//
// This package proves that all parties in an intersection receive identical
// financial views when RequireSymmetry=true in the policy.
//
// CRITICAL: Symmetry verification is cryptographic proof, not trust-based.
// Any asymmetry is detectable and auditable.
//
// Reference: v8.6 Family Financial Intersections
package neutrality

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"

	"quantumlife/internal/finance/sharedview"
)

// NeutralityVerifier verifies that all parties receive identical views.
type NeutralityVerifier struct{}

// NewNeutralityVerifier creates a new verifier.
func NewNeutralityVerifier() *NeutralityVerifier {
	return &NeutralityVerifier{}
}

// SymmetryProof is cryptographic proof that all parties received identical views.
type SymmetryProof struct {
	// ProofID uniquely identifies this proof.
	ProofID string

	// IntersectionID is the intersection this proof applies to.
	IntersectionID string

	// ViewID is the view that was verified.
	ViewID string

	// ContentHash is the hash of the view content.
	ContentHash string

	// PartyHashes maps each party to the hash they computed.
	// All hashes must match for symmetry to hold.
	PartyHashes map[string]string

	// Symmetric is true if all parties have matching hashes.
	Symmetric bool

	// Discrepancies lists any hash mismatches (empty if symmetric).
	Discrepancies []HashDiscrepancy

	// VerifiedAt is when this verification was performed.
	VerifiedAt time.Time

	// ProofHash is a hash of this proof for integrity.
	ProofHash string
}

// HashDiscrepancy records a hash mismatch between parties.
type HashDiscrepancy struct {
	PartyA     string
	PartyAHash string
	PartyB     string
	PartyBHash string
}

// Verify checks that all parties received identical views.
func (v *NeutralityVerifier) Verify(req sharedview.VerifyRequest) (*SymmetryProof, error) {
	if req.View == nil {
		return nil, fmt.Errorf("view required")
	}
	if len(req.PartyViews) == 0 {
		return nil, fmt.Errorf("at least one party view required")
	}

	proof := &SymmetryProof{
		ProofID:        generateProofID(),
		IntersectionID: req.View.IntersectionID,
		ViewID:         req.View.ViewID,
		ContentHash:    req.View.ContentHash,
		PartyHashes:    make(map[string]string),
		Symmetric:      true,
		Discrepancies:  []HashDiscrepancy{},
		VerifiedAt:     time.Now().UTC(),
	}

	// Collect all party hashes
	for partyID, partyView := range req.PartyViews {
		proof.PartyHashes[partyID] = partyView.ContentHash
	}

	// Check for discrepancies
	parties := make([]string, 0, len(req.PartyViews))
	for p := range req.PartyViews {
		parties = append(parties, p)
	}
	sort.Strings(parties) // Deterministic ordering

	expectedHash := req.View.ContentHash
	for _, partyID := range parties {
		partyHash := proof.PartyHashes[partyID]
		if partyHash != expectedHash {
			proof.Symmetric = false
			proof.Discrepancies = append(proof.Discrepancies, HashDiscrepancy{
				PartyA:     "reference",
				PartyAHash: expectedHash,
				PartyB:     partyID,
				PartyBHash: partyHash,
			})
		}
	}

	// Also check party-to-party symmetry
	for i := 0; i < len(parties); i++ {
		for j := i + 1; j < len(parties); j++ {
			hashI := proof.PartyHashes[parties[i]]
			hashJ := proof.PartyHashes[parties[j]]
			if hashI != hashJ {
				proof.Symmetric = false
				// Only add if not already covered by reference comparison
				alreadyCovered := false
				for _, d := range proof.Discrepancies {
					if d.PartyB == parties[i] || d.PartyB == parties[j] {
						alreadyCovered = true
						break
					}
				}
				if !alreadyCovered {
					proof.Discrepancies = append(proof.Discrepancies, HashDiscrepancy{
						PartyA:     parties[i],
						PartyAHash: hashI,
						PartyB:     parties[j],
						PartyBHash: hashJ,
					})
				}
			}
		}
	}

	// Compute proof hash
	proof.ProofHash = v.computeProofHash(proof)

	return proof, nil
}

// QuickVerify checks if a view's content hash matches expected.
// This is a lightweight verification without full party comparison.
func (v *NeutralityVerifier) QuickVerify(view *sharedview.SharedFinancialView, expectedHash string) bool {
	return view.ContentHash == expectedHash
}

// computeProofHash creates a hash of the proof for integrity.
func (v *NeutralityVerifier) computeProofHash(proof *SymmetryProof) string {
	h := sha256.New()

	h.Write([]byte(proof.ProofID))
	h.Write([]byte(proof.IntersectionID))
	h.Write([]byte(proof.ViewID))
	h.Write([]byte(proof.ContentHash))
	h.Write([]byte(proof.VerifiedAt.Format(time.RFC3339)))

	if proof.Symmetric {
		h.Write([]byte("symmetric"))
	} else {
		h.Write([]byte("asymmetric"))
	}

	// Include party hashes in sorted order
	parties := make([]string, 0, len(proof.PartyHashes))
	for p := range proof.PartyHashes {
		parties = append(parties, p)
	}
	sort.Strings(parties)

	for _, p := range parties {
		h.Write([]byte(p))
		h.Write([]byte(proof.PartyHashes[p]))
	}

	return hex.EncodeToString(h.Sum(nil))
}

// generateProofID creates a unique proof identifier.
func generateProofID() string {
	h := sha256.New()
	h.Write([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	return "symp_" + hex.EncodeToString(h.Sum(nil))[:16]
}

// LanguageNeutralityCheck verifies that text uses neutral language.
// Returns any violations found.
type LanguageNeutralityCheck struct {
	violations []LanguageViolation
}

// LanguageViolation records a language guideline violation.
type LanguageViolation struct {
	// Word is the forbidden word that was found.
	Word string

	// Category is the type of violation (urgency, fear, shame, authority, optimization).
	Category string

	// Context is the surrounding text.
	Context string

	// Position is the character position in the text.
	Position int
}

// ForbiddenWordCategories maps words to their violation categories.
var ForbiddenWordCategories = map[string]string{
	// Urgency
	"urgent":      "urgency",
	"immediately": "urgency",
	"now":         "urgency",
	"asap":        "urgency",
	"critical":    "urgency",
	"deadline":    "urgency",

	// Fear
	"concerning": "fear",
	"worrying":   "fear",
	"alarming":   "fear",
	"dangerous":  "fear",
	"risk":       "fear",

	// Shame
	"excessive":    "shame",
	"too much":     "shame",
	"overspending": "shame",
	"wasteful":     "shame",
	"unnecessary":  "shame",

	// Authority
	"must":      "authority",
	"should":    "authority",
	"need to":   "authority",
	"have to":   "authority",
	"required":  "authority",
	"mandatory": "authority",

	// Optimization
	"optimize":  "optimization",
	"maximize":  "optimization",
	"efficient": "optimization",
	"better":    "optimization",
	"improve":   "optimization",
}

// NewLanguageChecker creates a new language neutrality checker.
func NewLanguageChecker() *LanguageNeutralityCheck {
	return &LanguageNeutralityCheck{
		violations: []LanguageViolation{},
	}
}

// Check analyzes text for language guideline violations.
func (c *LanguageNeutralityCheck) Check(text string) []LanguageViolation {
	c.violations = []LanguageViolation{}

	lowered := toLower(text)

	for word, category := range ForbiddenWordCategories {
		pos := indexOf(lowered, word)
		if pos >= 0 {
			// Get context (surrounding 20 chars)
			start := pos - 10
			if start < 0 {
				start = 0
			}
			end := pos + len(word) + 10
			if end > len(text) {
				end = len(text)
			}
			context := text[start:end]

			c.violations = append(c.violations, LanguageViolation{
				Word:     word,
				Category: category,
				Context:  context,
				Position: pos,
			})
		}
	}

	return c.violations
}

// IsNeutral returns true if no violations were found.
func (c *LanguageNeutralityCheck) IsNeutral() bool {
	return len(c.violations) == 0
}

// Violations returns the list of found violations.
func (c *LanguageNeutralityCheck) Violations() []LanguageViolation {
	return c.violations
}

// toLower converts string to lowercase.
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		result[i] = c
	}
	return string(result)
}

// indexOf finds the first occurrence of substr in s.
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
