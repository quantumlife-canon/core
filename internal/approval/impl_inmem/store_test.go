// Package impl_inmem provides tests for the multi-party approval system (v7).
package impl_inmem

import (
	"context"
	"testing"
	"time"

	"quantumlife/internal/approval"
	auditImpl "quantumlife/internal/audit/impl_inmem"
	"quantumlife/pkg/primitives"
)

// mockIntersectionStore is a mock implementation for testing.
type mockIntersectionStore struct {
	contracts map[string]*approval.ContractForApproval
}

func (m *mockIntersectionStore) GetContractForApproval(ctx context.Context, intersectionID string) (*approval.ContractForApproval, error) {
	if contract, ok := m.contracts[intersectionID]; ok {
		return contract, nil
	}
	return nil, approval.ErrApprovalNotFound
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	return NewStore(StoreConfig{
		AuditStore: auditImpl.NewStore(),
	})
}

func newTestStoreWithClock(t *testing.T, clockFunc func() time.Time) *Store {
	t.Helper()
	return NewStore(StoreConfig{
		ClockFunc:  clockFunc,
		AuditStore: auditImpl.NewStore(),
	})
}

func TestRequestApproval_CreatesToken(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	action := &primitives.Action{
		ID:         "action-1",
		Type:       "calendar.create_event",
		Parameters: map[string]string{"title": "Test Meeting"},
	}

	token, err := store.RequestApproval(ctx, approval.ApprovalRequest{
		IntersectionID:     "ix-1",
		ContractVersion:    "v1",
		Action:             action,
		ScopesRequired:     []string{"calendar:write"},
		RequestingCircleID: "circle-1",
		ExpirySeconds:      3600,
	})

	if err != nil {
		t.Fatalf("RequestApproval failed: %v", err)
	}

	if token.TokenID == "" {
		t.Error("Token ID should not be empty")
	}
	if token.ActionHash == "" {
		t.Error("ActionHash should not be empty")
	}
	if token.IntersectionID != "ix-1" {
		t.Errorf("IntersectionID = %q, want %q", token.IntersectionID, "ix-1")
	}
	if token.ActionID != "action-1" {
		t.Errorf("ActionID = %q, want %q", token.ActionID, "action-1")
	}
	if len(token.Signature) == 0 {
		t.Error("Token should be signed")
	}
}

func TestSubmitApproval_ValidToken(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	action := &primitives.Action{
		ID:         "action-1",
		Type:       "calendar.create_event",
		Parameters: map[string]string{"title": "Test Meeting"},
	}

	// Request approval
	token, err := store.RequestApproval(ctx, approval.ApprovalRequest{
		IntersectionID:     "ix-1",
		ContractVersion:    "v1",
		Action:             action,
		ScopesRequired:     []string{"calendar:write"},
		RequestingCircleID: "circle-1",
	})
	if err != nil {
		t.Fatalf("RequestApproval failed: %v", err)
	}

	// Encode and submit
	encodedToken := primitives.EncodeApprovalToken(token)

	artifact, err := store.SubmitApproval(ctx, approval.SubmitApprovalRequest{
		Token:            encodedToken,
		ApproverCircleID: "circle-2",
	})

	if err != nil {
		t.Fatalf("SubmitApproval failed: %v", err)
	}

	if artifact.ApprovalID == "" {
		t.Error("ApprovalID should not be empty")
	}
	if artifact.ActionHash != token.ActionHash {
		t.Errorf("ActionHash mismatch: got %s, want %s", artifact.ActionHash, token.ActionHash)
	}
	if artifact.ApproverCircleID != "circle-2" {
		t.Errorf("ApproverCircleID = %q, want %q", artifact.ApproverCircleID, "circle-2")
	}
	if len(artifact.Signature) == 0 {
		t.Error("Artifact should be signed")
	}
}

func TestSubmitApproval_ExpiredToken(t *testing.T) {
	// Use a fixed time that is past the token expiry
	baseTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	currentTime := baseTime

	store := newTestStoreWithClock(t, func() time.Time {
		return currentTime
	})
	ctx := context.Background()

	action := &primitives.Action{
		ID:   "action-1",
		Type: "calendar.create_event",
	}

	// Request approval with short expiry
	token, err := store.RequestApproval(ctx, approval.ApprovalRequest{
		IntersectionID:     "ix-1",
		ContractVersion:    "v1",
		Action:             action,
		ScopesRequired:     []string{"calendar:write"},
		RequestingCircleID: "circle-1",
		ExpirySeconds:      60, // 1 minute
	})
	if err != nil {
		t.Fatalf("RequestApproval failed: %v", err)
	}

	// Advance time past expiry
	currentTime = baseTime.Add(2 * time.Minute)

	// Encode and submit
	encodedToken := primitives.EncodeApprovalToken(token)

	_, err = store.SubmitApproval(ctx, approval.SubmitApprovalRequest{
		Token:            encodedToken,
		ApproverCircleID: "circle-2",
	})

	if err != approval.ErrRequestTokenExpired {
		t.Errorf("Expected ErrRequestTokenExpired, got %v", err)
	}
}

func TestSubmitApproval_DuplicateApproval(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	action := &primitives.Action{
		ID:   "action-1",
		Type: "calendar.create_event",
	}

	token, _ := store.RequestApproval(ctx, approval.ApprovalRequest{
		IntersectionID:     "ix-1",
		ContractVersion:    "v1",
		Action:             action,
		ScopesRequired:     []string{"calendar:write"},
		RequestingCircleID: "circle-1",
	})

	encodedToken := primitives.EncodeApprovalToken(token)

	// First approval succeeds
	_, err := store.SubmitApproval(ctx, approval.SubmitApprovalRequest{
		Token:            encodedToken,
		ApproverCircleID: "circle-2",
	})
	if err != nil {
		t.Fatalf("First approval failed: %v", err)
	}

	// Second approval from same circle should fail
	_, err = store.SubmitApproval(ctx, approval.SubmitApprovalRequest{
		Token:            encodedToken,
		ApproverCircleID: "circle-2",
	})
	if err != approval.ErrDuplicateApproval {
		t.Errorf("Expected ErrDuplicateApproval, got %v", err)
	}
}

func TestVerifyApprovals_SingleMode(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	contract := &approval.ContractForApproval{
		IntersectionID: "ix-1",
		ApprovalPolicy: approval.ApprovalPolicy{
			Mode: approval.ApprovalModeSingle,
		},
	}

	action := &primitives.Action{
		ID:   "action-1",
		Type: "calendar.create_event",
	}

	result, err := store.VerifyApprovals(ctx, approval.VerifyApprovalsRequest{
		Contract:   contract,
		Action:     action,
		ActionHash: "hash-1",
		ScopesUsed: []string{"calendar:write"},
	})

	if err != nil {
		t.Fatalf("VerifyApprovals failed: %v", err)
	}

	if !result.Passed {
		t.Errorf("Single mode should pass without approvals, got passed=%v", result.Passed)
	}
}

func TestVerifyApprovals_MultiMode_ThresholdMet(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	action := &primitives.Action{
		ID:         "action-1",
		Type:       "calendar.create_event",
		Parameters: map[string]string{"title": "Test"},
	}

	// Request approval
	token, _ := store.RequestApproval(ctx, approval.ApprovalRequest{
		IntersectionID:     "ix-1",
		ContractVersion:    "v1",
		Action:             action,
		ScopesRequired:     []string{"calendar:write"},
		RequestingCircleID: "circle-1",
	})

	encodedToken := primitives.EncodeApprovalToken(token)

	// Submit 2 approvals
	store.SubmitApproval(ctx, approval.SubmitApprovalRequest{
		Token:            encodedToken,
		ApproverCircleID: "circle-2",
	})
	store.SubmitApproval(ctx, approval.SubmitApprovalRequest{
		Token:            encodedToken,
		ApproverCircleID: "circle-3",
	})

	contract := &approval.ContractForApproval{
		IntersectionID: "ix-1",
		ApprovalPolicy: approval.ApprovalPolicy{
			Mode:      approval.ApprovalModeMulti,
			Threshold: 2,
		},
	}

	result, err := store.VerifyApprovals(ctx, approval.VerifyApprovalsRequest{
		Contract:   contract,
		Action:     action,
		ActionHash: token.ActionHash,
		ScopesUsed: []string{"calendar:write"},
	})

	if err != nil {
		t.Fatalf("VerifyApprovals failed: %v", err)
	}

	if !result.Passed {
		t.Errorf("Should pass with 2 approvals and threshold 2, got passed=%v, reason=%s",
			result.Passed, result.Reason)
	}
	if result.ThresholdMet != 2 {
		t.Errorf("ThresholdMet = %d, want 2", result.ThresholdMet)
	}
}

func TestVerifyApprovals_MultiMode_ThresholdNotMet(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	action := &primitives.Action{
		ID:         "action-1",
		Type:       "calendar.create_event",
		Parameters: map[string]string{"title": "Test"},
	}

	// Request approval
	token, _ := store.RequestApproval(ctx, approval.ApprovalRequest{
		IntersectionID:     "ix-1",
		ContractVersion:    "v1",
		Action:             action,
		ScopesRequired:     []string{"calendar:write"},
		RequestingCircleID: "circle-1",
	})

	encodedToken := primitives.EncodeApprovalToken(token)

	// Submit only 1 approval
	store.SubmitApproval(ctx, approval.SubmitApprovalRequest{
		Token:            encodedToken,
		ApproverCircleID: "circle-2",
	})

	contract := &approval.ContractForApproval{
		IntersectionID: "ix-1",
		ApprovalPolicy: approval.ApprovalPolicy{
			Mode:      approval.ApprovalModeMulti,
			Threshold: 2,
		},
	}

	result, err := store.VerifyApprovals(ctx, approval.VerifyApprovalsRequest{
		Contract:   contract,
		Action:     action,
		ActionHash: token.ActionHash,
		ScopesUsed: []string{"calendar:write"},
	})

	if err != nil {
		t.Fatalf("VerifyApprovals failed: %v", err)
	}

	if result.Passed {
		t.Errorf("Should fail with 1 approval and threshold 2, got passed=%v", result.Passed)
	}
	if result.ThresholdMet != 1 {
		t.Errorf("ThresholdMet = %d, want 1", result.ThresholdMet)
	}
}

func TestVerifyApprovals_RequiredApprovers(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	action := &primitives.Action{
		ID:         "action-1",
		Type:       "calendar.create_event",
		Parameters: map[string]string{"title": "Test"},
	}

	// Request approval
	token, _ := store.RequestApproval(ctx, approval.ApprovalRequest{
		IntersectionID:     "ix-1",
		ContractVersion:    "v1",
		Action:             action,
		ScopesRequired:     []string{"calendar:write"},
		RequestingCircleID: "circle-1",
	})

	encodedToken := primitives.EncodeApprovalToken(token)

	// Submit approval from circle-2 only
	store.SubmitApproval(ctx, approval.SubmitApprovalRequest{
		Token:            encodedToken,
		ApproverCircleID: "circle-2",
	})

	contract := &approval.ContractForApproval{
		IntersectionID: "ix-1",
		ApprovalPolicy: approval.ApprovalPolicy{
			Mode:              approval.ApprovalModeMulti,
			Threshold:         1,
			RequiredApprovers: []string{"circle-2", "circle-3"}, // Both required
		},
	}

	result, err := store.VerifyApprovals(ctx, approval.VerifyApprovalsRequest{
		Contract:   contract,
		Action:     action,
		ActionHash: token.ActionHash,
		ScopesUsed: []string{"calendar:write"},
	})

	if err != nil {
		t.Fatalf("VerifyApprovals failed: %v", err)
	}

	if result.Passed {
		t.Errorf("Should fail without all required approvers")
	}
	if len(result.MissingApprovers) != 1 || result.MissingApprovers[0] != "circle-3" {
		t.Errorf("MissingApprovers = %v, want [circle-3]", result.MissingApprovers)
	}
}

func TestVerifyApprovals_ActionHashMismatch(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	action := &primitives.Action{
		ID:         "action-1",
		Type:       "calendar.create_event",
		Parameters: map[string]string{"title": "Test"},
	}

	// Request approval
	token, _ := store.RequestApproval(ctx, approval.ApprovalRequest{
		IntersectionID:     "ix-1",
		ContractVersion:    "v1",
		Action:             action,
		ScopesRequired:     []string{"calendar:write"},
		RequestingCircleID: "circle-1",
	})

	encodedToken := primitives.EncodeApprovalToken(token)

	// Submit approval
	store.SubmitApproval(ctx, approval.SubmitApprovalRequest{
		Token:            encodedToken,
		ApproverCircleID: "circle-2",
	})

	contract := &approval.ContractForApproval{
		IntersectionID: "ix-1",
		ApprovalPolicy: approval.ApprovalPolicy{
			Mode:      approval.ApprovalModeMulti,
			Threshold: 1,
		},
	}

	// Verify with DIFFERENT action hash (replay protection)
	result, err := store.VerifyApprovals(ctx, approval.VerifyApprovalsRequest{
		Contract:   contract,
		Action:     action,
		ActionHash: "different-hash", // Mismatch!
		ScopesUsed: []string{"calendar:write"},
	})

	if err != nil {
		t.Fatalf("VerifyApprovals failed: %v", err)
	}

	if result.Passed {
		t.Errorf("Should fail with action hash mismatch (replay protection)")
	}
	if len(result.InvalidApprovals) == 0 {
		t.Errorf("Should have invalid approvals due to hash mismatch")
	}
}

func TestActionHash_Deterministic(t *testing.T) {
	action := &primitives.Action{
		ID:         "action-1",
		Type:       "calendar.create_event",
		Parameters: map[string]string{"title": "Test", "location": "Room A"},
	}

	hash1 := primitives.ComputeActionHashFromAction(action, "ix-1", "v1", []string{"calendar:write"}, primitives.ModeExecute)
	hash2 := primitives.ComputeActionHashFromAction(action, "ix-1", "v1", []string{"calendar:write"}, primitives.ModeExecute)

	if hash1 != hash2 {
		t.Errorf("Action hash not deterministic: %s != %s", hash1, hash2)
	}
}

func TestActionHash_DifferentActions(t *testing.T) {
	action1 := &primitives.Action{
		ID:         "action-1",
		Type:       "calendar.create_event",
		Parameters: map[string]string{"title": "Meeting 1"},
	}
	action2 := &primitives.Action{
		ID:         "action-2", // Different ID
		Type:       "calendar.create_event",
		Parameters: map[string]string{"title": "Meeting 1"},
	}

	hash1 := primitives.ComputeActionHashFromAction(action1, "ix-1", "v1", []string{"calendar:write"}, primitives.ModeExecute)
	hash2 := primitives.ComputeActionHashFromAction(action2, "ix-1", "v1", []string{"calendar:write"}, primitives.ModeExecute)

	if hash1 == hash2 {
		t.Errorf("Different actions should have different hashes")
	}
}

func TestActionHash_DifferentParameters(t *testing.T) {
	action1 := &primitives.Action{
		ID:         "action-1",
		Type:       "calendar.create_event",
		Parameters: map[string]string{"title": "Meeting 1"},
	}
	action2 := &primitives.Action{
		ID:         "action-1",
		Type:       "calendar.create_event",
		Parameters: map[string]string{"title": "Meeting 2"}, // Different title
	}

	hash1 := primitives.ComputeActionHashFromAction(action1, "ix-1", "v1", []string{"calendar:write"}, primitives.ModeExecute)
	hash2 := primitives.ComputeActionHashFromAction(action2, "ix-1", "v1", []string{"calendar:write"}, primitives.ModeExecute)

	if hash1 == hash2 {
		t.Errorf("Different parameters should have different hashes")
	}
}

func TestDeleteExpiredApprovals(t *testing.T) {
	baseTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	currentTime := baseTime

	store := newTestStoreWithClock(t, func() time.Time {
		return currentTime
	})
	ctx := context.Background()

	action := &primitives.Action{
		ID:   "action-1",
		Type: "calendar.create_event",
	}

	// Request approval with short expiry
	token, _ := store.RequestApproval(ctx, approval.ApprovalRequest{
		IntersectionID:     "ix-1",
		ContractVersion:    "v1",
		Action:             action,
		ScopesRequired:     []string{"calendar:write"},
		RequestingCircleID: "circle-1",
		ExpirySeconds:      60,
	})

	encodedToken := primitives.EncodeApprovalToken(token)

	// Submit approval
	store.SubmitApproval(ctx, approval.SubmitApprovalRequest{
		Token:            encodedToken,
		ApproverCircleID: "circle-2",
	})

	// Verify approval exists
	approvals, _ := store.GetApprovals(ctx, "ix-1", "action-1")
	if len(approvals) != 1 {
		t.Fatalf("Expected 1 approval, got %d", len(approvals))
	}

	// Advance time past expiry
	currentTime = baseTime.Add(2 * time.Hour)

	// Delete expired
	count, err := store.DeleteExpiredApprovals(ctx, currentTime)
	if err != nil {
		t.Fatalf("DeleteExpiredApprovals failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 deleted, got %d", count)
	}

	// Verify approval is gone
	approvals, _ = store.GetApprovals(ctx, "ix-1", "action-1")
	if len(approvals) != 0 {
		t.Errorf("Expected 0 approvals after delete, got %d", len(approvals))
	}
}
