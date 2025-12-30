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
