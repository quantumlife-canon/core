package execution

import (
	"fmt"
	"time"
)

// ExecutionRunner executes financial actions.
//
// CRITICAL: v9 Slice 1 is DRY-RUN ONLY.
// This runner MUST NOT:
// - Move real money
// - Call provider write methods
// - Create real financial commitments
//
// Per Technical Split v9 §10.4:
// - MUST perform affirmative validity check before acting
// - MUST execute exactly as specified in envelope
// - MUST halt at safe points upon interrupt
// - MUST record all state transitions
// - MUST NOT modify execution parameters
// - MUST NOT retry without instruction
// - MUST NOT proceed on ambiguous state
type ExecutionRunner struct {
	approvalVerifier  *ApprovalVerifier
	revocationChecker *RevocationChecker
	idGenerator       func() string

	// dryRunMode MUST be true in v9 Slice 1
	dryRunMode bool
}

// NewExecutionRunner creates a new execution runner.
// In v9 Slice 1, dryRunMode MUST be true.
func NewExecutionRunner(
	approvalVerifier *ApprovalVerifier,
	revocationChecker *RevocationChecker,
	idGen func() string,
) *ExecutionRunner {
	return &ExecutionRunner{
		approvalVerifier:  approvalVerifier,
		revocationChecker: revocationChecker,
		idGenerator:       idGen,
		dryRunMode:        true, // MUST be true in v9 Slice 1
	}
}

// Execute attempts to execute a sealed envelope.
// In v9 Slice 1, this always results in a non-success status.
//
// Execution flow per Technical Split v9:
// 1. Check envelope not expired
// 2. Check envelope not revoked
// 3. Verify approvals (hash-bound, not expired)
// 4. Perform affirmative validity check
// 5. Check revocation window state
// 6. Execute (DRY-RUN: always blocked/aborted)
func (r *ExecutionRunner) Execute(env *ExecutionEnvelope, now time.Time) (*ExecutionResult, error) {
	result := &ExecutionResult{
		EnvelopeID:   env.EnvelopeID,
		AttemptedAt:  now,
		AuditTraceID: env.TraceID,
	}

	// Step 1: Check envelope not expired
	if env.IsExpired(now) {
		result.Status = SettlementExpired
		result.BlockedReason = "envelope expired"
		result.CompletedAt = now
		return result, nil
	}

	// Step 2: Check envelope not already revoked
	if env.IsRevoked() {
		result.Status = SettlementRevoked
		result.RevokedBy = env.RevokedBy
		result.BlockedReason = "envelope was revoked"
		result.CompletedAt = now
		return result, nil
	}

	// Step 3: Check for revocation signal
	revCheck := r.revocationChecker.Check(env.EnvelopeID, now)
	if revCheck.Revoked {
		ApplyRevocationToEnvelope(env, revCheck.Signal)
		result.Status = SettlementRevoked
		result.RevokedBy = revCheck.Signal.RevokerCircleID
		result.BlockedReason = "revocation signal received"
		result.CompletedAt = now
		return result, nil
	}

	// Step 4: Verify approvals
	if err := r.verifyApprovals(env, now); err != nil {
		result.Status = SettlementBlocked
		result.BlockedReason = fmt.Sprintf("approval verification failed: %v", err)
		result.CompletedAt = now
		return result, nil
	}

	// Step 5: Perform affirmative validity check
	validityCheck := r.performValidityCheck(env, now)
	result.ValidityCheck = validityCheck

	if !validityCheck.Valid {
		result.Status = SettlementBlocked
		result.BlockedReason = validityCheck.FailureReason
		result.CompletedAt = now
		return result, nil
	}

	// Step 6: Final revocation check (mid-execution check point)
	revCheck = r.revocationChecker.Check(env.EnvelopeID, now)
	if revCheck.Revoked {
		ApplyRevocationToEnvelope(env, revCheck.Signal)
		result.Status = SettlementRevoked
		result.RevokedBy = revCheck.Signal.RevokerCircleID
		result.BlockedReason = "revocation during execution"
		result.CompletedAt = now
		return result, nil
	}

	// Step 7: DRY-RUN MODE - Abort before actual execution
	// In v9 Slice 1, we NEVER proceed to actual execution
	if r.dryRunMode {
		result.Status = SettlementAborted
		result.BlockedReason = "dry-run mode: execution halted before external effect"
		result.CompletedAt = now
		return result, nil
	}

	// This code path should NEVER be reached in v9 Slice 1
	return nil, fmt.Errorf("CRITICAL: execution reached forbidden code path")
}

// verifyApprovals verifies all approvals on an envelope.
func (r *ExecutionRunner) verifyApprovals(env *ExecutionEnvelope, now time.Time) error {
	if len(env.Approvals) < env.ApprovalThreshold {
		return fmt.Errorf("insufficient approvals: have %d, need %d",
			len(env.Approvals), env.ApprovalThreshold)
	}

	validCount := 0
	for _, approval := range env.Approvals {
		if err := r.approvalVerifier.VerifyApproval(&approval, env.ActionHash, now); err != nil {
			// Log but continue checking other approvals
			continue
		}
		validCount++
	}

	if validCount < env.ApprovalThreshold {
		return fmt.Errorf("insufficient valid approvals: have %d valid, need %d",
			validCount, env.ApprovalThreshold)
	}

	return nil
}

// performValidityCheck performs an affirmative validity check.
// Per Canon Addendum v9 §8.3: absence of revocation alone is insufficient.
func (r *ExecutionRunner) performValidityCheck(env *ExecutionEnvelope, now time.Time) ValidityCheckResult {
	result := ValidityCheckResult{
		Valid:      true,
		CheckedAt:  now,
		Conditions: []ConditionResult{},
	}

	// Condition 1: Envelope not expired
	notExpired := !env.IsExpired(now)
	result.Conditions = append(result.Conditions, ConditionResult{
		Condition: "envelope_not_expired",
		Satisfied: notExpired,
		Details:   fmt.Sprintf("expiry: %s, now: %s", env.Expiry.Format(time.RFC3339), now.Format(time.RFC3339)),
	})
	if !notExpired {
		result.Valid = false
		result.FailureReason = "envelope has expired"
	}

	// Condition 2: Envelope not revoked
	notRevoked := !env.IsRevoked()
	result.Conditions = append(result.Conditions, ConditionResult{
		Condition: "envelope_not_revoked",
		Satisfied: notRevoked,
		Details:   fmt.Sprintf("revoked: %t", env.Revoked),
	})
	if !notRevoked && result.Valid {
		result.Valid = false
		result.FailureReason = "envelope has been revoked"
	}

	// Condition 3: No pending revocation signal
	noRevocationSignal := !r.revocationChecker.IsRevoked(env.EnvelopeID)
	result.Conditions = append(result.Conditions, ConditionResult{
		Condition: "no_revocation_signal",
		Satisfied: noRevocationSignal,
		Details:   "checked revocation registry",
	})
	if !noRevocationSignal && result.Valid {
		result.Valid = false
		result.FailureReason = "revocation signal detected"
	}

	// Condition 4: Sufficient valid approvals
	hasApprovals := env.HasSufficientApprovals()
	result.Conditions = append(result.Conditions, ConditionResult{
		Condition: "sufficient_approvals",
		Satisfied: hasApprovals,
		Details:   fmt.Sprintf("have %d, need %d", len(env.Approvals), env.ApprovalThreshold),
	})
	if !hasApprovals && result.Valid {
		result.Valid = false
		result.FailureReason = "insufficient approvals"
	}

	// Condition 5: Amount within cap
	withinCap := env.ActionSpec.AmountCents <= env.AmountCap
	result.Conditions = append(result.Conditions, ConditionResult{
		Condition: "amount_within_cap",
		Satisfied: withinCap,
		Details:   fmt.Sprintf("amount: %d, cap: %d", env.ActionSpec.AmountCents, env.AmountCap),
	})
	if !withinCap && result.Valid {
		result.Valid = false
		result.FailureReason = "amount exceeds cap"
	}

	// Condition 6: Revocation window closed (or waived)
	windowState := "closed"
	if env.RevocationWaived {
		windowState = "waived"
	} else if now.Before(env.RevocationWindowEnd) {
		windowState = "open"
	}
	windowOK := env.RevocationWaived || now.After(env.RevocationWindowEnd)
	result.Conditions = append(result.Conditions, ConditionResult{
		Condition: "revocation_window_closed",
		Satisfied: windowOK,
		Details:   fmt.Sprintf("window state: %s", windowState),
	})
	if !windowOK && result.Valid {
		result.Valid = false
		result.FailureReason = "revocation window still open"
	}

	return result
}

// ExecuteWithRevocationDuringWindow demonstrates revocation during window.
// This is a helper for the demo that exercises the revocation path.
func (r *ExecutionRunner) ExecuteWithRevocationDuringWindow(
	env *ExecutionEnvelope,
	revokerCircleID string,
	revokerID string,
	now time.Time,
) (*ExecutionResult, *RevocationSignal) {
	// First, trigger revocation
	signal := r.revocationChecker.Revoke(
		env.EnvelopeID,
		revokerCircleID,
		revokerID,
		"circle-initiated revocation during window",
		now,
	)

	// Then attempt execution (which will be blocked)
	result, _ := r.Execute(env, now)

	return result, signal
}

// ExecuteWithAdapter attempts execution using a provider adapter.
//
// CRITICAL: In v9 Slice 2, the adapter ALWAYS blocks execution.
// NO REAL MONEY MOVES. This method exists to prove the execution
// pipeline structure while guaranteeing safety.
//
// Flow:
// 1. Perform all pre-execution checks (expiry, revocation, approval, validity)
// 2. Wait for revocation window to close (or verify closed)
// 3. Invoke adapter.Prepare()
// 4. Invoke adapter.Execute() - ALWAYS blocked by guarded adapter
// 5. Record settlement (blocked, not succeeded)
func (r *ExecutionRunner) ExecuteWithAdapter(
	env *ExecutionEnvelope,
	adapter ExecutionAdapter,
	now time.Time,
) (*ExecutionResult, *ExecutionAttempt, error) {
	result := &ExecutionResult{
		EnvelopeID:   env.EnvelopeID,
		AttemptedAt:  now,
		AuditTraceID: env.TraceID,
	}

	// Step 1: Check envelope not expired
	if env.IsExpired(now) {
		result.Status = SettlementExpired
		result.BlockedReason = "envelope expired"
		result.CompletedAt = now
		return result, nil, nil
	}

	// Step 2: Check envelope not already revoked
	if env.IsRevoked() {
		result.Status = SettlementRevoked
		result.RevokedBy = env.RevokedBy
		result.BlockedReason = "envelope was revoked"
		result.CompletedAt = now
		return result, nil, nil
	}

	// Step 3: Check for revocation signal
	revCheck := r.revocationChecker.Check(env.EnvelopeID, now)
	if revCheck.Revoked {
		ApplyRevocationToEnvelope(env, revCheck.Signal)
		result.Status = SettlementRevoked
		result.RevokedBy = revCheck.Signal.RevokerCircleID
		result.BlockedReason = "revocation signal received"
		result.CompletedAt = now
		return result, nil, nil
	}

	// Step 4: Verify approvals
	if err := r.verifyApprovals(env, now); err != nil {
		result.Status = SettlementBlocked
		result.BlockedReason = fmt.Sprintf("approval verification failed: %v", err)
		result.CompletedAt = now
		return result, nil, nil
	}

	// Step 5: Perform affirmative validity check
	validityCheck := r.performValidityCheck(env, now)
	result.ValidityCheck = validityCheck

	if !validityCheck.Valid {
		result.Status = SettlementBlocked
		result.BlockedReason = validityCheck.FailureReason
		result.CompletedAt = now
		return result, nil, nil
	}

	// Step 6: Final revocation check (mid-execution check point)
	revCheck = r.revocationChecker.Check(env.EnvelopeID, now)
	if revCheck.Revoked {
		ApplyRevocationToEnvelope(env, revCheck.Signal)
		result.Status = SettlementRevoked
		result.RevokedBy = revCheck.Signal.RevokerCircleID
		result.BlockedReason = "revocation during execution"
		result.CompletedAt = now
		return result, nil, nil
	}

	// Step 7: Get approval for adapter execution
	var approval *ApprovalArtifact
	if len(env.Approvals) > 0 {
		approval = &env.Approvals[0]
	} else {
		result.Status = SettlementBlocked
		result.BlockedReason = "no approval artifact available"
		result.CompletedAt = now
		return result, nil, nil
	}

	// Step 8: Prepare via adapter
	prepareResult, err := adapter.Prepare(env)
	if err != nil {
		result.Status = SettlementBlocked
		result.BlockedReason = fmt.Sprintf("adapter prepare failed: %v", err)
		result.CompletedAt = now
		return result, nil, nil
	}

	if !prepareResult.Valid {
		result.Status = SettlementBlocked
		result.BlockedReason = fmt.Sprintf("adapter prepare invalid: %s", prepareResult.InvalidReason)
		result.CompletedAt = now
		return result, nil, nil
	}

	// Step 9: Execute via adapter
	// CRITICAL: In v9 Slice 2, this ALWAYS returns GuardedExecutionError
	attempt, execErr := adapter.Execute(env, approval)

	// Step 10: Record result
	if attempt != nil {
		result.CompletedAt = attempt.AttemptedAt
	} else {
		result.CompletedAt = now
	}

	// Check if this is the expected GuardedExecutionError
	if IsGuardedExecutionError(execErr) {
		// This is the expected outcome in v9 Slice 2
		result.Status = SettlementBlocked
		result.BlockedReason = "guarded adapter blocked execution"
		return result, attempt, nil
	}

	// Any other error is unexpected
	if execErr != nil {
		result.Status = SettlementAborted
		result.BlockedReason = fmt.Sprintf("adapter execution error: %v", execErr)
		return result, attempt, execErr
	}

	// CRITICAL: If we somehow reach here with a "succeeded" attempt, that's wrong
	if attempt != nil && attempt.Status == AttemptSucceeded {
		// This should NEVER happen in v9 Slice 2
		return nil, attempt, fmt.Errorf("CRITICAL: adapter reported success - this is forbidden in v9 Slice 2")
	}

	// Default to blocked if we get here
	result.Status = SettlementBlocked
	result.BlockedReason = "execution did not complete"
	return result, attempt, nil
}
