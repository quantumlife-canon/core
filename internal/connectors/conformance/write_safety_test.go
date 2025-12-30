// Package conformance provides conformance tests for calendar connectors.
// These tests verify v6 Execute mode write safety requirements.
//
// CRITICAL: These tests enforce the safety invariants for Execute mode:
// - Execute mode requires explicit human approval (--approve flag)
// - Write operations require calendar:write scope
// - Revocation halts execution before and after writes
// - Settlement semantics are correctly implemented
//
// Reference: docs/TECHNICAL_SPLIT_V1.md §3.5 Action Execution Layer
package conformance

import (
	"context"
	"errors"
	"testing"
	"time"

	actionImpl "quantumlife/internal/action/impl_inmem"
	auditImpl "quantumlife/internal/audit/impl_inmem"
	authorityImpl "quantumlife/internal/authority/impl_inmem"
	"quantumlife/internal/connectors/calendar"
	"quantumlife/internal/intersection"
	intersectionImpl "quantumlife/internal/intersection/impl_inmem"
	"quantumlife/internal/revocation"
	revocationImpl "quantumlife/internal/revocation/impl_inmem"
	"quantumlife/pkg/primitives"
)

// Fixed time for deterministic testing.
var v6TestTime = time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

// ============================================================================
// Envelope Write Validation Tests
// ============================================================================

// TestValidateForWrite_RequiresExecuteMode verifies Execute mode is required.
func TestValidateForWrite_RequiresExecuteMode(t *testing.T) {
	tests := []struct {
		name    string
		mode    primitives.RunMode
		wantErr bool
	}{
		{"SuggestOnly mode rejected", primitives.ModeSuggestOnly, true},
		{"Simulate mode rejected", primitives.ModeSimulate, true},
		{"Execute mode accepted", primitives.ModeExecute, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := primitives.NewExecutionEnvelopeWithApproval(
				"trace-123",
				"circle-123",
				"intersection-123",
				"v1",
				[]string{"calendar:write"},
				"proof-123",
				v6TestTime,
				"cli:--approve",
			)
			env.Mode = tt.mode

			err := env.ValidateForWrite()
			if tt.wantErr && err == nil {
				t.Error("expected error for non-Execute mode")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestValidateForWrite_RequiresApproval verifies explicit approval is required.
func TestValidateForWrite_RequiresApproval(t *testing.T) {
	// Without approval
	env := primitives.ExecutionEnvelope{
		TraceID:              "trace-123",
		Mode:                 primitives.ModeExecute,
		ActorCircleID:        "circle-123",
		IntersectionID:       "intersection-123",
		ContractVersion:      "v1",
		ScopesUsed:           []string{"calendar:write"},
		AuthorizationProofID: "proof-123",
		IssuedAt:             v6TestTime,
		ApprovedByHuman:      false, // NOT APPROVED
		ApprovalArtifact:     "",
	}

	err := env.ValidateForWrite()
	if err == nil {
		t.Error("expected error without approval")
	}
	if !errors.Is(err, primitives.ErrEnvelopeApprovalRequired) {
		t.Errorf("expected ErrEnvelopeApprovalRequired, got: %v", err)
	}

	// With approval
	env.ApprovedByHuman = true
	env.ApprovalArtifact = "cli:--approve"

	err = env.ValidateForWrite()
	if err != nil {
		t.Errorf("unexpected error with approval: %v", err)
	}
}

// TestValidateForWrite_RequiresWriteScope verifies calendar:write scope is required.
func TestValidateForWrite_RequiresWriteScope(t *testing.T) {
	tests := []struct {
		name    string
		scopes  []string
		wantErr bool
	}{
		{"read scope only", []string{"calendar:read"}, true},
		{"write scope", []string{"calendar:write"}, false},
		{"both scopes", []string{"calendar:read", "calendar:write"}, false},
		{"empty scopes", []string{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := primitives.NewExecutionEnvelopeWithApproval(
				"trace-123",
				"circle-123",
				"intersection-123",
				"v1",
				tt.scopes,
				"proof-123",
				v6TestTime,
				"cli:--approve",
			)

			err := env.ValidateForWrite()
			if tt.wantErr && err == nil {
				t.Error("expected error for missing write scope")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// ============================================================================
// Revocation Tests
// ============================================================================

// TestRevocation_BlocksBeforeWrite verifies revocation halts before external write.
func TestRevocation_BlocksBeforeWrite(t *testing.T) {
	registry := revocationImpl.NewRegistryWithClock(func() time.Time { return v6TestTime })
	ctx := context.Background()

	// Revoke an action
	err := registry.RevokeAction(ctx, "action-123", "cancelled by circle", "circle-456")
	if err != nil {
		t.Fatalf("RevokeAction failed: %v", err)
	}

	// Check should return error
	err = registry.CheckBeforeWrite(ctx, "action-123", "intersection-123", "proof-123")
	if err == nil {
		t.Error("expected error for revoked action")
	}
	if !errors.Is(err, revocation.ErrActionRevoked) {
		t.Errorf("expected ErrActionRevoked, got: %v", err)
	}
}

// TestRevocation_BlocksIntersectionRevocation verifies intersection revocation blocks write.
func TestRevocation_BlocksIntersectionRevocation(t *testing.T) {
	registry := revocationImpl.NewRegistryWithClock(func() time.Time { return v6TestTime })
	ctx := context.Background()

	// Revoke intersection
	err := registry.RevokeIntersection(ctx, "intersection-123", "dissolved", "circle-owner")
	if err != nil {
		t.Fatalf("RevokeIntersection failed: %v", err)
	}

	// Check should return error for any action in that intersection
	err = registry.CheckBeforeWrite(ctx, "action-456", "intersection-123", "proof-789")
	if err == nil {
		t.Error("expected error for dissolved intersection")
	}
	if !errors.Is(err, revocation.ErrIntersectionDissolved) {
		t.Errorf("expected ErrIntersectionDissolved, got: %v", err)
	}
}

// TestRevocation_NoBlockWithoutSignal verifies non-revoked actions proceed.
func TestRevocation_NoBlockWithoutSignal(t *testing.T) {
	registry := revocationImpl.NewRegistryWithClock(func() time.Time { return v6TestTime })
	ctx := context.Background()

	// Check should return nil for non-revoked action
	err := registry.CheckBeforeWrite(ctx, "action-999", "intersection-999", "proof-999")
	if err != nil {
		t.Errorf("expected nil for non-revoked action, got: %v", err)
	}
}

// ============================================================================
// Pipeline Settlement Tests
// ============================================================================

// fakeWriteConnector is a test connector that tracks write calls.
type fakeWriteConnector struct {
	createCalled bool
	deleteCalled bool
	failCreate   bool
	createdID    string
}

func (f *fakeWriteConnector) ID() string {
	return "fake-write-connector"
}

func (f *fakeWriteConnector) Capabilities() []string {
	return []string{"create_event", "delete_event"}
}

func (f *fakeWriteConnector) RequiredScopes() []string {
	return []string{"calendar:write"}
}

func (f *fakeWriteConnector) ListEvents(ctx context.Context, req calendar.ListEventsRequest) ([]calendar.Event, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeWriteConnector) ProposeEvent(ctx context.Context, req calendar.ProposeEventRequest) (*calendar.ProposedEvent, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeWriteConnector) HealthCheck(ctx context.Context) error {
	return nil
}

func (f *fakeWriteConnector) ListEventsWithEnvelope(ctx context.Context, env primitives.ExecutionEnvelope, r calendar.EventRange) ([]calendar.Event, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeWriteConnector) FindFreeSlots(ctx context.Context, env primitives.ExecutionEnvelope, r calendar.EventRange, minDuration time.Duration) ([]calendar.FreeSlot, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeWriteConnector) ProposeEventWithEnvelope(ctx context.Context, env primitives.ExecutionEnvelope, req calendar.ProposeEventRequest) (*calendar.ProposedEvent, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeWriteConnector) ProviderInfo() calendar.ProviderInfo {
	return calendar.ProviderInfo{
		ID:           "fake",
		Name:         "Fake Write Connector",
		IsConfigured: true,
	}
}

func (f *fakeWriteConnector) CreateEvent(ctx context.Context, env primitives.ExecutionEnvelope, req calendar.CreateEventRequest) (*calendar.CreateEventReceipt, error) {
	f.createCalled = true
	if f.failCreate {
		return nil, errors.New("external API error")
	}
	f.createdID = "external-event-123"
	return &calendar.CreateEventReceipt{
		Provider:        calendar.SourceMock,
		CalendarID:      "primary",
		ExternalEventID: f.createdID,
		Status:          "created",
		CreatedAt:       v6TestTime,
	}, nil
}

func (f *fakeWriteConnector) DeleteEvent(ctx context.Context, env primitives.ExecutionEnvelope, req calendar.DeleteEventRequest) (*calendar.DeleteEventReceipt, error) {
	f.deleteCalled = true
	return &calendar.DeleteEventReceipt{
		Provider:        calendar.SourceMock,
		ExternalEventID: req.ExternalEventID,
		Status:          "deleted",
		DeletedAt:       v6TestTime,
	}, nil
}

func (f *fakeWriteConnector) SupportsWrite() bool {
	return true
}

// setupTestPipeline creates a test pipeline with dependencies.
func setupTestPipeline(t *testing.T) (*actionImpl.Pipeline, *intersectionImpl.Runtime, *revocationImpl.Registry) {
	intersectionStore := intersectionImpl.NewRuntime()
	auditStore := auditImpl.NewStore()
	revocationRegistry := revocationImpl.NewRegistryWithClock(func() time.Time { return v6TestTime })
	authorityEngine := authorityImpl.NewEngine(intersectionStore)

	pipeline := actionImpl.NewPipeline(actionImpl.PipelineConfig{
		AuthorityEngine:   authorityEngine,
		RevocationChecker: revocationRegistry,
		AuditStore:        auditStore,
		ClockFunc:         func() time.Time { return v6TestTime },
	})

	return pipeline, intersectionStore, revocationRegistry
}

// createTestIntersection creates an intersection with write scope for testing.
func createTestIntersection(t *testing.T, store *intersectionImpl.Runtime, circleID string) *intersection.Intersection {
	ctx := context.Background()
	inter, err := store.Create(ctx, intersection.CreateRequest{
		TenantID:    "test-tenant",
		InitiatorID: circleID,
		AcceptorID:  circleID,
		Contract: intersection.Contract{
			Parties: []intersection.Party{
				{CircleID: circleID, PartyType: "initiator", JoinedAt: v6TestTime},
			},
			Scopes: []intersection.Scope{
				{Name: "calendar:read", Description: "Read calendar", ReadWrite: "read"},
				{Name: "calendar:write", Description: "Write calendar", ReadWrite: "write"},
			},
			Ceilings: []intersection.Ceiling{
				{Type: "time_window", Value: "00:00-23:59", Unit: "daily"},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to create intersection: %v", err)
	}
	return inter
}

// TestPipeline_SettlesOnSuccess verifies settlement after successful write.
func TestPipeline_SettlesOnSuccess(t *testing.T) {
	pipeline, store, _ := setupTestPipeline(t)
	inter := createTestIntersection(t, store, "circle-test")
	connector := &fakeWriteConnector{}

	ctx := context.Background()
	result := pipeline.Execute(ctx, actionImpl.ExecuteRequest{
		TraceID:         "trace-test",
		ActorCircleID:   "circle-test",
		IntersectionID:  inter.ID,
		ContractVersion: inter.Version,
		Action: &primitives.Action{
			ID:             "action-test",
			IntersectionID: inter.ID,
			Type:           "calendar.create_event",
		},
		ApprovalArtifact: "cli:--approve",
		Connector:        connector,
		CreateRequest: calendar.CreateEventRequest{
			Title:     "Test Event",
			StartTime: v6TestTime.Add(time.Hour),
			EndTime:   v6TestTime.Add(2 * time.Hour),
		},
	})

	if !result.Success {
		t.Errorf("expected success, got error: %v", result.Error)
	}
	if result.SettlementStatus != actionImpl.SettlementSettled {
		t.Errorf("expected settled, got: %s", result.SettlementStatus)
	}
	if !connector.createCalled {
		t.Error("expected CreateEvent to be called")
	}
	if result.Receipt == nil {
		t.Error("expected receipt")
	}
}

// TestPipeline_AbortsOnWriteFailure verifies settlement aborted on write failure.
func TestPipeline_AbortsOnWriteFailure(t *testing.T) {
	pipeline, store, _ := setupTestPipeline(t)
	inter := createTestIntersection(t, store, "circle-test")
	connector := &fakeWriteConnector{failCreate: true}

	ctx := context.Background()
	result := pipeline.Execute(ctx, actionImpl.ExecuteRequest{
		TraceID:         "trace-test",
		ActorCircleID:   "circle-test",
		IntersectionID:  inter.ID,
		ContractVersion: inter.Version,
		Action: &primitives.Action{
			ID:             "action-test",
			IntersectionID: inter.ID,
			Type:           "calendar.create_event",
		},
		ApprovalArtifact: "cli:--approve",
		Connector:        connector,
		CreateRequest: calendar.CreateEventRequest{
			Title:     "Test Event",
			StartTime: v6TestTime.Add(time.Hour),
			EndTime:   v6TestTime.Add(2 * time.Hour),
		},
	})

	if result.Success {
		t.Error("expected failure")
	}
	if result.SettlementStatus != actionImpl.SettlementAborted {
		t.Errorf("expected aborted, got: %s", result.SettlementStatus)
	}
	if result.Receipt != nil {
		t.Error("expected no receipt on failure")
	}
}

// TestPipeline_RevocationBlocksExecution verifies revocation blocks execution.
func TestPipeline_RevocationBlocksExecution(t *testing.T) {
	pipeline, store, registry := setupTestPipeline(t)
	inter := createTestIntersection(t, store, "circle-test")
	connector := &fakeWriteConnector{}

	ctx := context.Background()

	// Revoke the action before execution
	err := registry.RevokeAction(ctx, "action-revoked", "cancelled", "circle-owner")
	if err != nil {
		t.Fatalf("failed to revoke action: %v", err)
	}

	result := pipeline.Execute(ctx, actionImpl.ExecuteRequest{
		TraceID:         "trace-test",
		ActorCircleID:   "circle-test",
		IntersectionID:  inter.ID,
		ContractVersion: inter.Version,
		Action: &primitives.Action{
			ID:             "action-revoked",
			IntersectionID: inter.ID,
			Type:           "calendar.create_event",
		},
		ApprovalArtifact: "cli:--approve",
		Connector:        connector,
		CreateRequest: calendar.CreateEventRequest{
			Title:     "Test Event",
			StartTime: v6TestTime.Add(time.Hour),
			EndTime:   v6TestTime.Add(2 * time.Hour),
		},
	})

	if result.Success {
		t.Error("expected failure due to revocation")
	}
	if result.SettlementStatus != actionImpl.SettlementRevoked {
		t.Errorf("expected revoked, got: %s", result.SettlementStatus)
	}
	if connector.createCalled {
		t.Error("CreateEvent should NOT be called for revoked action")
	}
}

// TestPipeline_RequiresApprovalArtifact verifies approval artifact is required.
func TestPipeline_RequiresApprovalArtifact(t *testing.T) {
	pipeline, store, _ := setupTestPipeline(t)
	inter := createTestIntersection(t, store, "circle-test")
	connector := &fakeWriteConnector{}

	ctx := context.Background()
	result := pipeline.Execute(ctx, actionImpl.ExecuteRequest{
		TraceID:         "trace-test",
		ActorCircleID:   "circle-test",
		IntersectionID:  inter.ID,
		ContractVersion: inter.Version,
		Action: &primitives.Action{
			ID:             "action-test",
			IntersectionID: inter.ID,
			Type:           "calendar.create_event",
		},
		ApprovalArtifact: "", // MISSING APPROVAL
		Connector:        connector,
		CreateRequest: calendar.CreateEventRequest{
			Title:     "Test Event",
			StartTime: v6TestTime.Add(time.Hour),
			EndTime:   v6TestTime.Add(2 * time.Hour),
		},
	})

	if result.Success {
		t.Error("expected failure without approval artifact")
	}
	if connector.createCalled {
		t.Error("CreateEvent should NOT be called without approval")
	}
}

// TestPipeline_AuthorizationProofRecordsApproval verifies proof includes approval.
func TestPipeline_AuthorizationProofRecordsApproval(t *testing.T) {
	pipeline, store, _ := setupTestPipeline(t)
	inter := createTestIntersection(t, store, "circle-test")
	connector := &fakeWriteConnector{}

	ctx := context.Background()
	result := pipeline.Execute(ctx, actionImpl.ExecuteRequest{
		TraceID:         "trace-test",
		ActorCircleID:   "circle-test",
		IntersectionID:  inter.ID,
		ContractVersion: inter.Version,
		Action: &primitives.Action{
			ID:             "action-test",
			IntersectionID: inter.ID,
			Type:           "calendar.create_event",
		},
		ApprovalArtifact: "cli:--approve",
		Connector:        connector,
		CreateRequest: calendar.CreateEventRequest{
			Title:     "Test Event",
			StartTime: v6TestTime.Add(time.Hour),
			EndTime:   v6TestTime.Add(2 * time.Hour),
		},
	})

	if result.AuthorizationProof == nil {
		t.Fatal("expected authorization proof")
	}
	if !result.AuthorizationProof.ApprovedByHuman {
		t.Error("expected ApprovedByHuman to be true")
	}
	if result.AuthorizationProof.ApprovalArtifact != "cli:--approve" {
		t.Errorf("expected approval artifact 'cli:--approve', got: %s", result.AuthorizationProof.ApprovalArtifact)
	}
}

// ============================================================================
// Mode Enforcement Tests
// ============================================================================

// TestAuthorityEngine_ExecuteModeRequiresApproval verifies authority checks approval.
func TestAuthorityEngine_ExecuteModeRequiresApproval(t *testing.T) {
	intersectionStore := intersectionImpl.NewRuntime()
	engine := authorityImpl.NewEngine(intersectionStore)

	// Create intersection with write scope
	ctx := context.Background()
	inter, _ := intersectionStore.Create(ctx, intersection.CreateRequest{
		TenantID:    "test",
		InitiatorID: "circle-1",
		AcceptorID:  "circle-1",
		Contract: intersection.Contract{
			Parties: []intersection.Party{{CircleID: "circle-1", PartyType: "initiator", JoinedAt: v6TestTime}},
			Scopes:  []intersection.Scope{{Name: "calendar:write", ReadWrite: "write"}},
		},
	})

	action := &primitives.Action{
		ID:             "action-1",
		IntersectionID: inter.ID,
		Type:           "calendar.create_event",
	}

	// Without approval
	proof, err := engine.AuthorizeActionWithApproval(
		ctx,
		action,
		[]string{"calendar:write"},
		primitives.ModeExecute,
		"trace-1",
		false, // NOT APPROVED
		"",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proof.Authorized {
		t.Error("expected authorization denied without approval")
	}

	// With approval
	proof, err = engine.AuthorizeActionWithApproval(
		ctx,
		action,
		[]string{"calendar:write"},
		primitives.ModeExecute,
		"trace-1",
		true, // APPROVED
		"cli:--approve",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !proof.Authorized {
		t.Errorf("expected authorization granted with approval, denial reason: %s", proof.DenialReason)
	}
}

// TestAuthorityEngine_ExecuteModeRequiresWriteScope verifies scope requirements.
func TestAuthorityEngine_ExecuteModeRequiresWriteScope(t *testing.T) {
	intersectionStore := intersectionImpl.NewRuntime()
	engine := authorityImpl.NewEngine(intersectionStore)

	// Create intersection with ONLY read scope
	ctx := context.Background()
	inter, _ := intersectionStore.Create(ctx, intersection.CreateRequest{
		TenantID:    "test",
		InitiatorID: "circle-1",
		AcceptorID:  "circle-1",
		Contract: intersection.Contract{
			Parties: []intersection.Party{{CircleID: "circle-1", PartyType: "initiator", JoinedAt: v6TestTime}},
			Scopes:  []intersection.Scope{{Name: "calendar:read", ReadWrite: "read"}}, // NO WRITE
		},
	})

	action := &primitives.Action{
		ID:             "action-1",
		IntersectionID: inter.ID,
		Type:           "calendar.create_event",
	}

	// Request write scope
	proof, err := engine.AuthorizeActionWithApproval(
		ctx,
		action,
		[]string{"calendar:write"},
		primitives.ModeExecute,
		"trace-1",
		true,
		"cli:--approve",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proof.Authorized {
		t.Error("expected authorization denied when scope not in contract")
	}
}

// ============================================================================
// Verify interface compliance
// ============================================================================

var _ calendar.WriteConnector = (*fakeWriteConnector)(nil)
var _ revocation.Registry = (*revocationImpl.Registry)(nil)
