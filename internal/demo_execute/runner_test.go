// Package demo_execute tests for v6 Execute mode demo.
package demo_execute

import (
	"context"
	"testing"
	"time"

	actionImpl "quantumlife/internal/action/impl_inmem"
)

// Fixed time for deterministic testing.
var testTime = time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

// TestRunner_Success verifies successful execution.
func TestRunner_Success(t *testing.T) {
	runner := NewRunnerWithClock(func() time.Time { return testTime })

	ctx := context.Background()
	result, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}

	if result.ExecuteResult == nil {
		t.Fatal("expected ExecuteResult")
	}

	if result.ExecuteResult.SettlementStatus != actionImpl.SettlementSettled {
		t.Errorf("expected SettlementSettled, got %s", result.ExecuteResult.SettlementStatus)
	}

	if result.ExecuteResult.Receipt == nil {
		t.Error("expected receipt")
	}

	if result.IntersectionID == "" {
		t.Error("expected IntersectionID")
	}

	if result.TraceID == "" {
		t.Error("expected TraceID")
	}

	if len(result.AuditEntries) == 0 {
		t.Error("expected audit entries")
	}
}

// TestRunner_WithRevocation verifies revocation halts execution.
func TestRunner_WithRevocation(t *testing.T) {
	runner := NewRunnerWithClock(func() time.Time { return testTime })

	ctx := context.Background()
	result, err := runner.RunWithRevocation(ctx)
	if err != nil {
		t.Fatalf("RunWithRevocation returned error: %v", err)
	}

	if !result.Success {
		t.Errorf("expected success (revocation demonstration worked), got error: %s", result.Error)
	}

	if result.ExecuteResult == nil {
		t.Fatal("expected ExecuteResult")
	}

	if result.ExecuteResult.SettlementStatus != actionImpl.SettlementRevoked {
		t.Errorf("expected SettlementRevoked, got %s", result.ExecuteResult.SettlementStatus)
	}

	if result.ExecuteResult.Receipt != nil {
		t.Error("expected no receipt when revoked")
	}

	if !result.RevocationApplied {
		t.Error("expected RevocationApplied flag")
	}
}

// TestRunner_AuthorizationProof verifies proof is generated correctly.
func TestRunner_AuthorizationProof(t *testing.T) {
	runner := NewRunnerWithClock(func() time.Time { return testTime })

	ctx := context.Background()
	result, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if result.ExecuteResult == nil || result.ExecuteResult.AuthorizationProof == nil {
		t.Fatal("expected AuthorizationProof")
	}

	proof := result.ExecuteResult.AuthorizationProof
	if !proof.Authorized {
		t.Errorf("expected Authorized=true, got denial: %s", proof.DenialReason)
	}
	if !proof.ApprovedByHuman {
		t.Error("expected ApprovedByHuman=true")
	}
	if proof.ApprovalArtifact != "demo:automated-test" {
		t.Errorf("expected approval artifact 'demo:automated-test', got: %s", proof.ApprovalArtifact)
	}
}

// TestRunner_Verbose runs the demo with verbose output for manual inspection.
// Run with: go test -v -run TestRunner_Verbose
func TestRunner_Verbose(t *testing.T) {
	runner := NewRunnerWithClock(func() time.Time { return testTime })
	ctx := context.Background()

	t.Log("╔═══════════════════════════════════════════════════════════════╗")
	t.Log("║           QuantumLife v6 Execute Mode Demo                    ║")
	t.Log("╚═══════════════════════════════════════════════════════════════╝")

	// Demo 1: Successful execution
	t.Log("")
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Log("Demo 1: Successful Event Creation")
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	result, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	t.Logf("Mode:           %s", result.Mode)
	t.Logf("Trace ID:       %s", result.TraceID)
	t.Logf("Intersection:   %s", result.IntersectionID)

	if result.ExecuteResult.Success {
		t.Log("Status:         ✓ SUCCESS")
	} else {
		t.Log("Status:         ✗ FAILED")
	}
	t.Logf("Settlement:     %s", result.ExecuteResult.SettlementStatus)

	if result.ExecuteResult.AuthorizationProof != nil {
		proof := result.ExecuteResult.AuthorizationProof
		t.Log("")
		t.Log("Authorization Proof:")
		t.Logf("  ID:           %s", proof.ID)
		t.Logf("  Authorized:   %v", proof.Authorized)
		t.Logf("  Approved:     %v (artifact: %s)", proof.ApprovedByHuman, proof.ApprovalArtifact)
	}

	if result.ExecuteResult.Receipt != nil {
		receipt := result.ExecuteResult.Receipt
		t.Log("")
		t.Log("Event Receipt:")
		t.Logf("  Provider:     %s", receipt.Provider)
		t.Logf("  External ID:  %s", receipt.ExternalEventID)
		t.Logf("  Status:       %s", receipt.Status)
		t.Logf("  Link:         %s", receipt.Link)
	}

	t.Logf("Audit Entries:  %d", len(result.AuditEntries))

	// Demo 2: Revocation
	t.Log("")
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Log("Demo 2: Revocation Blocks Execution")
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	result2, err := runner.RunWithRevocation(ctx)
	if err != nil {
		t.Fatalf("RunWithRevocation error: %v", err)
	}

	t.Logf("Mode:           %s", result2.Mode)
	t.Logf("Settlement:     %s", result2.ExecuteResult.SettlementStatus)
	t.Log("")
	t.Log("⚠ Action was REVOKED before external write")
	t.Log("  No calendar event was created (safety guarantee)")
	t.Logf("Audit Entries:  %d", len(result2.AuditEntries))

	t.Log("")
	t.Log("╔═══════════════════════════════════════════════════════════════╗")
	t.Log("║              v6 Execute Mode Demo Complete                    ║")
	t.Log("╚═══════════════════════════════════════════════════════════════╝")
}

// TestRunner_Deterministic verifies deterministic output for same input.
func TestRunner_Deterministic(t *testing.T) {
	runner1 := NewRunnerWithClock(func() time.Time { return testTime })
	runner2 := NewRunnerWithClock(func() time.Time { return testTime })

	ctx := context.Background()
	result1, err := runner1.Run(ctx)
	if err != nil {
		t.Fatalf("Run 1 returned error: %v", err)
	}
	result2, err := runner2.Run(ctx)
	if err != nil {
		t.Fatalf("Run 2 returned error: %v", err)
	}

	// Key invariants should match
	if result1.Success != result2.Success {
		t.Error("non-deterministic: Success differs")
	}
	if result1.ExecuteResult.SettlementStatus != result2.ExecuteResult.SettlementStatus {
		t.Error("non-deterministic: SettlementStatus differs")
	}
	if len(result1.AuditEntries) != len(result2.AuditEntries) {
		t.Errorf("non-deterministic: audit entries differ (%d vs %d)",
			len(result1.AuditEntries), len(result2.AuditEntries))
	}
}
