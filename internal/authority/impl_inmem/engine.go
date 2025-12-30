// Package impl_inmem provides an in-memory implementation of authority interfaces.
// This is for demo and testing purposes only.
//
// CRITICAL: All validation is deterministic — no LLM/SLM, no randomness.
//
// Reference: docs/TECHNICAL_SPLIT_V1.md §3.3 Authority & Policy Engine
package impl_inmem

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"quantumlife/internal/authority"
	"quantumlife/internal/intersection"
	"quantumlife/pkg/primitives"
)

// Engine implements deterministic authority validation.
type Engine struct {
	mu                sync.RWMutex
	intersectionStore intersection.Runtime
	proofs            map[string]*authority.AuthorizationProof
	clockFunc         func() time.Time
	idCounter         int
}

// NewEngine creates a new authority engine.
func NewEngine(intersectionStore intersection.Runtime) *Engine {
	return &Engine{
		intersectionStore: intersectionStore,
		proofs:            make(map[string]*authority.AuthorizationProof),
		clockFunc:         time.Now,
	}
}

// NewEngineWithClock creates an engine with an injected clock for determinism.
func NewEngineWithClock(intersectionStore intersection.Runtime, clockFunc func() time.Time) *Engine {
	return &Engine{
		intersectionStore: intersectionStore,
		proofs:            make(map[string]*authority.AuthorizationProof),
		clockFunc:         clockFunc,
	}
}

// AuthorizeAction performs a complete authorization check for an action.
// Returns an AuthorizationProof that can be attached to audit events.
// Note: For Execute mode with write scopes, use AuthorizeActionWithApproval instead.
func (e *Engine) AuthorizeAction(
	ctx context.Context,
	action *primitives.Action,
	requiredScopes []string,
	mode primitives.RunMode,
	traceID string,
) (*authority.AuthorizationProof, error) {
	return e.AuthorizeActionWithApproval(ctx, action, requiredScopes, mode, traceID, false, "")
}

// AuthorizeActionWithApproval performs authorization with explicit human approval.
// v6: This is required for Execute mode with write scopes.
//
// Parameters:
//   - approvedByHuman: true if explicit human approval was provided (e.g., --approve flag)
//   - approvalArtifact: how approval was obtained (e.g., "cli:--approve")
//
// CRITICAL: Execute mode with write scopes REQUIRES approvedByHuman=true.
func (e *Engine) AuthorizeActionWithApproval(
	ctx context.Context,
	action *primitives.Action,
	requiredScopes []string,
	mode primitives.RunMode,
	traceID string,
	approvedByHuman bool,
	approvalArtifact string,
) (*authority.AuthorizationProof, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.idCounter++
	proofID := fmt.Sprintf("authproof-%d", e.idCounter)
	now := e.clockFunc()

	proof := &authority.AuthorizationProof{
		ID:               proofID,
		ActionID:         action.ID,
		IntersectionID:   action.IntersectionID,
		ScopesUsed:       requiredScopes,
		Timestamp:        now,
		EvaluatedAt:      now,
		TraceID:          traceID,
		Authorized:       true, // Assume authorized until a check fails
		ApprovedByHuman:  approvedByHuman,
		ApprovalArtifact: approvalArtifact,
	}

	// 1. Validate run mode (with approval context)
	modeCheck := e.validateModeWithApproval(mode, approvedByHuman, requiredScopes)
	proof.ModeCheck = modeCheck
	if !modeCheck.Allowed {
		proof.Authorized = false
		proof.DenialReason = modeCheck.Reason
		e.proofs[proofID] = proof
		return proof, nil
	}

	// 2. Get intersection and contract
	inter, err := e.intersectionStore.Get(ctx, action.IntersectionID)
	if err != nil {
		proof.Authorized = false
		proof.DenialReason = fmt.Sprintf("intersection not found: %s", action.IntersectionID)
		e.proofs[proofID] = proof
		return proof, nil
	}

	contract, err := e.intersectionStore.GetContract(ctx, action.IntersectionID)
	if err != nil {
		proof.Authorized = false
		proof.DenialReason = fmt.Sprintf("contract not found: %s", action.IntersectionID)
		e.proofs[proofID] = proof
		return proof, nil
	}

	proof.ContractVersion = inter.Version

	// Extract scope names from contract
	var scopeNames []string
	for _, s := range contract.Scopes {
		scopeNames = append(scopeNames, s.Name)
	}
	proof.ScopesGranted = scopeNames

	// 3. Validate scopes
	scopeCheck := e.validateScopes(requiredScopes, scopeNames)
	if !scopeCheck.Passed {
		proof.Authorized = false
		proof.DenialReason = scopeCheck.Reason
		e.proofs[proofID] = proof
		return proof, nil
	}

	// 4. For Execute mode with write scopes, verify approval is present
	if mode == primitives.ModeExecute {
		for _, scope := range requiredScopes {
			if !primitives.IsReadOnlyScope(scope) && !approvedByHuman {
				proof.Authorized = false
				proof.DenialReason = fmt.Sprintf("write scope %s requires explicit human approval", scope)
				e.proofs[proofID] = proof
				return proof, nil
			}
		}
	}

	// 5. Validate ceilings
	ceilingChecks := e.validateCeilings(action, contract.Ceilings, now)
	proof.CeilingChecks = ceilingChecks
	for _, check := range ceilingChecks {
		if !check.Passed {
			proof.Authorized = false
			proof.DenialReason = check.Reason
			e.proofs[proofID] = proof
			return proof, nil
		}
	}

	e.proofs[proofID] = proof
	return proof, nil
}

// validateMode checks if the run mode is allowed (without approval context).
// For Execute mode, use validateModeWithApproval instead.
func (e *Engine) validateMode(mode primitives.RunMode) authority.ModeCheck {
	return e.validateModeWithApproval(mode, false, nil)
}

// validateModeWithApproval checks if the run mode is allowed with approval context.
// v6: Execute mode is allowed when:
// - No write scopes are requested (read-only Execute)
// - OR approvedByHuman is true (approved write Execute)
func (e *Engine) validateModeWithApproval(mode primitives.RunMode, approvedByHuman bool, requestedScopes []string) authority.ModeCheck {
	switch mode {
	case primitives.ModeSuggestOnly:
		return authority.ModeCheck{
			RequestedMode: string(mode),
			Allowed:       true,
			Reason:        "Suggest-only mode is always allowed",
		}
	case primitives.ModeSimulate:
		return authority.ModeCheck{
			RequestedMode: string(mode),
			Allowed:       true,
			Reason:        "Simulate mode is allowed (no external writes)",
		}
	case primitives.ModeExecute:
		// v6: Execute mode is allowed with proper authorization
		// Check if any write scopes are requested
		hasWriteScopes := false
		for _, scope := range requestedScopes {
			if !primitives.IsReadOnlyScope(scope) {
				hasWriteScopes = true
				break
			}
		}

		// If no write scopes, Execute mode is allowed for reads
		if !hasWriteScopes {
			return authority.ModeCheck{
				RequestedMode: string(mode),
				Allowed:       true,
				Reason:        "Execute mode allowed (read-only operation)",
			}
		}

		// Write scopes require explicit approval
		if approvedByHuman {
			return authority.ModeCheck{
				RequestedMode: string(mode),
				Allowed:       true,
				Reason:        "Execute mode allowed (explicit human approval provided)",
			}
		}

		// Reject: write scopes without approval
		return authority.ModeCheck{
			RequestedMode: string(mode),
			Allowed:       false,
			Reason:        "Execute mode with write scopes requires explicit human approval (--approve flag)",
		}
	default:
		return authority.ModeCheck{
			RequestedMode: string(mode),
			Allowed:       false,
			Reason:        fmt.Sprintf("Unknown run mode: %s", mode),
		}
	}
}

// scopeCheckResult is an internal type for scope validation.
type scopeCheckResult struct {
	Passed bool
	Reason string
}

// validateScopes checks if all required scopes are granted.
func (e *Engine) validateScopes(required, granted []string) scopeCheckResult {
	grantedSet := make(map[string]bool)
	for _, s := range granted {
		grantedSet[s] = true
	}

	var missing []string
	for _, s := range required {
		if !grantedSet[s] {
			missing = append(missing, s)
		}
	}

	if len(missing) > 0 {
		return scopeCheckResult{
			Passed: false,
			Reason: fmt.Sprintf("Missing required scopes: %s", strings.Join(missing, ", ")),
		}
	}

	return scopeCheckResult{
		Passed: true,
		Reason: "All required scopes are granted",
	}
}

// ceilingInfo is a helper type for ceiling validation.
type ceilingInfo struct {
	Type  string
	Value string
	Unit  string
}

// validateCeilings checks all ceiling constraints.
func (e *Engine) validateCeilings(action *primitives.Action, ceilings []intersection.Ceiling, now time.Time) []authority.CeilingCheck {
	var checks []authority.CeilingCheck

	for _, ceiling := range ceilings {
		c := ceilingInfo{Type: ceiling.Type, Value: ceiling.Value, Unit: ceiling.Unit}
		check := e.validateSingleCeiling(action, c, now)
		checks = append(checks, check)
	}

	return checks
}

// validateSingleCeiling validates a single ceiling constraint.
func (e *Engine) validateSingleCeiling(action *primitives.Action, ceiling ceilingInfo, now time.Time) authority.CeilingCheck {
	check := authority.CeilingCheck{
		CeilingType:  ceiling.Type,
		CeilingValue: ceiling.Value,
		CeilingUnit:  ceiling.Unit,
	}

	switch ceiling.Type {
	case "time_window":
		return e.validateTimeWindow(action, ceiling, now)
	case "duration":
		return e.validateDuration(action, ceiling)
	default:
		// Unknown ceiling type - pass by default
		check.Passed = true
		check.Reason = fmt.Sprintf("Unknown ceiling type %s - passed by default", ceiling.Type)
	}

	return check
}

// validateTimeWindow checks if the action falls within the allowed time window.
func (e *Engine) validateTimeWindow(action *primitives.Action, ceiling ceilingInfo, now time.Time) authority.CeilingCheck {
	check := authority.CeilingCheck{
		CeilingType:  ceiling.Type,
		CeilingValue: ceiling.Value,
		CeilingUnit:  ceiling.Unit,
	}

	// Parse time window format: "HH:MM-HH:MM"
	parts := strings.Split(ceiling.Value, "-")
	if len(parts) != 2 {
		check.Passed = false
		check.Reason = fmt.Sprintf("Invalid time window format: %s", ceiling.Value)
		return check
	}

	startParts := strings.Split(parts[0], ":")
	endParts := strings.Split(parts[1], ":")
	if len(startParts) != 2 || len(endParts) != 2 {
		check.Passed = false
		check.Reason = fmt.Sprintf("Invalid time window format: %s", ceiling.Value)
		return check
	}

	startHour, _ := strconv.Atoi(startParts[0])
	endHour, _ := strconv.Atoi(endParts[0])

	// Check if action's requested time falls within the window
	// For demo, we use the action's time_window parameter or current hour
	requestedHour := now.Hour()
	if tw := action.Parameters["time_window"]; tw != "" {
		// Parse from action parameters
		twParts := strings.Split(tw, "-")
		if len(twParts) >= 1 {
			hourParts := strings.Split(twParts[0], ":")
			if len(hourParts) >= 1 {
				requestedHour, _ = strconv.Atoi(hourParts[0])
			}
		}
	}

	check.RequestedValue = fmt.Sprintf("%02d:00", requestedHour)

	if requestedHour >= startHour && requestedHour < endHour {
		check.Passed = true
		check.Reason = fmt.Sprintf("Time %02d:00 is within window %s", requestedHour, ceiling.Value)
	} else {
		check.Passed = false
		check.Reason = fmt.Sprintf("Time %02d:00 is outside allowed window %s", requestedHour, ceiling.Value)
	}

	return check
}

// validateDuration checks if the action duration is within the ceiling.
func (e *Engine) validateDuration(action *primitives.Action, ceiling ceilingInfo) authority.CeilingCheck {
	check := authority.CeilingCheck{
		CeilingType:  ceiling.Type,
		CeilingValue: ceiling.Value,
		CeilingUnit:  ceiling.Unit,
	}

	// Parse ceiling duration
	ceilingDuration, err := strconv.Atoi(ceiling.Value)
	if err != nil {
		check.Passed = false
		check.Reason = fmt.Sprintf("Invalid duration value: %s", ceiling.Value)
		return check
	}

	// Get requested duration from action parameters (default 1 hour)
	requestedDuration := 1
	if d := action.Parameters["duration"]; d != "" {
		requestedDuration, _ = strconv.Atoi(d)
	}

	check.RequestedValue = fmt.Sprintf("%d %s", requestedDuration, ceiling.Unit)

	if requestedDuration <= ceilingDuration {
		check.Passed = true
		check.Reason = fmt.Sprintf("Duration %d is within ceiling %d %s", requestedDuration, ceilingDuration, ceiling.Unit)
	} else {
		check.Passed = false
		check.Reason = fmt.Sprintf("Duration %d exceeds ceiling %d %s", requestedDuration, ceilingDuration, ceiling.Unit)
	}

	return check
}

// GetProof retrieves an authorization proof by ID.
func (e *Engine) GetProof(ctx context.Context, proofID string) (*authority.AuthorizationProof, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if proof, ok := e.proofs[proofID]; ok {
		return proof, nil
	}
	return nil, fmt.Errorf("authorization proof not found: %s", proofID)
}

// GetProofByAction retrieves the authorization proof for an action.
func (e *Engine) GetProofByAction(ctx context.Context, actionID string) (*authority.AuthorizationProof, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, proof := range e.proofs {
		if proof.ActionID == actionID {
			return proof, nil
		}
	}
	return nil, fmt.Errorf("no authorization proof found for action: %s", actionID)
}
