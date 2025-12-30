// Package primitives provides deterministic action hashing for v7 approval binding.
//
// CRITICAL: The action hash binds approvals to specific actions.
// Any change to the action parameters results in a different hash,
// preventing approval reuse across different actions.
//
// Reference: v7 Multi-party approval governance
package primitives

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
)

// ActionHashInput contains all fields used to compute an action hash.
// This ensures approvals are bound to the exact action being executed.
type ActionHashInput struct {
	// ActionID uniquely identifies the action.
	ActionID string

	// ActionType identifies the kind of action.
	ActionType string

	// IntersectionID is the intersection governing the action.
	IntersectionID string

	// ContractVersion is the contract version at time of action.
	ContractVersion string

	// ScopesUsed lists the scopes the action requires.
	ScopesUsed []string

	// Mode is the execution mode (must be "execute" for writes).
	Mode RunMode

	// Parameters contains action-specific parameters.
	// These are sorted by key for deterministic hashing.
	Parameters map[string]string
}

// ComputeActionHash computes a SHA-256 hash of the action.
// The hash is deterministic for the same input.
func ComputeActionHash(input ActionHashInput) string {
	// Build canonical string representation
	canonical := buildCanonicalString(input)

	// Compute SHA-256 hash
	hash := sha256.Sum256([]byte(canonical))

	return hex.EncodeToString(hash[:])
}

// buildCanonicalString creates a deterministic string representation of the action.
func buildCanonicalString(input ActionHashInput) string {
	// Start with fixed fields in order
	result := fmt.Sprintf("action_id=%s\n", input.ActionID)
	result += fmt.Sprintf("action_type=%s\n", input.ActionType)
	result += fmt.Sprintf("intersection_id=%s\n", input.IntersectionID)
	result += fmt.Sprintf("contract_version=%s\n", input.ContractVersion)
	result += fmt.Sprintf("mode=%s\n", input.Mode)

	// Sort and add scopes
	sortedScopes := make([]string, len(input.ScopesUsed))
	copy(sortedScopes, input.ScopesUsed)
	sort.Strings(sortedScopes)
	for _, scope := range sortedScopes {
		result += fmt.Sprintf("scope=%s\n", scope)
	}

	// Sort and add parameters
	if len(input.Parameters) > 0 {
		keys := make([]string, 0, len(input.Parameters))
		for k := range input.Parameters {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			result += fmt.Sprintf("param.%s=%s\n", k, input.Parameters[k])
		}
	}

	return result
}

// ComputeActionHashFromAction computes a hash from an Action and envelope context.
func ComputeActionHashFromAction(action *Action, intersectionID, contractVersion string, scopesUsed []string, mode RunMode) string {
	return ComputeActionHash(ActionHashInput{
		ActionID:        action.ID,
		ActionType:      action.Type,
		IntersectionID:  intersectionID,
		ContractVersion: contractVersion,
		ScopesUsed:      scopesUsed,
		Mode:            mode,
		Parameters:      action.Parameters,
	})
}

// VerifyActionHash verifies that an action matches a given hash.
func VerifyActionHash(action *Action, intersectionID, contractVersion string, scopesUsed []string, mode RunMode, expectedHash string) bool {
	computedHash := ComputeActionHashFromAction(action, intersectionID, contractVersion, scopesUsed, mode)
	return computedHash == expectedHash
}
