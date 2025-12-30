// Package impl_inmem provides conformance tests for v7 multi-party approval enforcement.
package impl_inmem

import (
	"context"
	"testing"
	"time"

	"quantumlife/internal/approval"
	approvalImpl "quantumlife/internal/approval/impl_inmem"
	auditImpl "quantumlife/internal/audit/impl_inmem"
	authorityImpl "quantumlife/internal/authority/impl_inmem"
	"quantumlife/internal/circle"
	circleImpl "quantumlife/internal/circle/impl_inmem"
	"quantumlife/internal/connectors/calendar"
	"quantumlife/internal/intersection"
	intersectionImpl "quantumlife/internal/intersection/impl_inmem"
	"quantumlife/pkg/primitives"
)

// mockWriteConnector is a mock write connector for testing.
type mockWriteConnector struct{}

func (m *mockWriteConnector) ID() string {
	return "mock"
}

func (m *mockWriteConnector) Capabilities() []string {
	return []string{"calendar:read", "calendar:write"}
}

func (m *mockWriteConnector) RequiredScopes() []string {
	return []string{"calendar:read", "calendar:write"}
}

func (m *mockWriteConnector) ListEvents(ctx context.Context, req calendar.ListEventsRequest) ([]calendar.Event, error) {
	return []calendar.Event{}, nil
}

func (m *mockWriteConnector) ProposeEvent(ctx context.Context, req calendar.ProposeEventRequest) (*calendar.ProposedEvent, error) {
	return &calendar.ProposedEvent{}, nil
}

func (m *mockWriteConnector) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *mockWriteConnector) ListEventsWithEnvelope(ctx context.Context, env primitives.ExecutionEnvelope, r calendar.EventRange) ([]calendar.Event, error) {
	return []calendar.Event{}, nil
}

func (m *mockWriteConnector) FindFreeSlots(ctx context.Context, env primitives.ExecutionEnvelope, r calendar.EventRange, minDuration time.Duration) ([]calendar.FreeSlot, error) {
	return []calendar.FreeSlot{}, nil
}

func (m *mockWriteConnector) ProposeEventWithEnvelope(ctx context.Context, env primitives.ExecutionEnvelope, req calendar.ProposeEventRequest) (*calendar.ProposedEvent, error) {
	return &calendar.ProposedEvent{}, nil
}

func (m *mockWriteConnector) ProviderInfo() calendar.ProviderInfo {
	return calendar.ProviderInfo{ID: "mock", Name: "Mock Provider"}
}

func (m *mockWriteConnector) SupportsWrite() bool {
	return true
}

func (m *mockWriteConnector) CreateEvent(ctx context.Context, env primitives.ExecutionEnvelope, req calendar.CreateEventRequest) (*calendar.CreateEventReceipt, error) {
	return &calendar.CreateEventReceipt{
		ExternalEventID: "mock-event-123",
		Provider:        "mock",
		CalendarID:      req.CalendarID,
		Status:          "confirmed",
		CreatedAt:       time.Now(),
	}, nil
}

func (m *mockWriteConnector) DeleteEvent(ctx context.Context, env primitives.ExecutionEnvelope, req calendar.DeleteEventRequest) (*calendar.DeleteEventReceipt, error) {
	return &calendar.DeleteEventReceipt{
		ExternalEventID: req.ExternalEventID,
		DeletedAt:       time.Now(),
	}, nil
}

// TestPipeline_SingleApprovalMode tests that single approval mode works (v6 compatibility).
func TestPipeline_SingleApprovalMode(t *testing.T) {
	ctx := context.Background()

	// Create infrastructure
	circleStore := circleImpl.NewRuntime()
	intersectionStore := intersectionImpl.NewRuntime()
	auditStore := auditImpl.NewStore()

	// Create circle
	circ, _ := circleStore.Create(ctx, circle.CreateRequest{TenantID: "tenant-1"})

	// Create intersection with single approval mode
	inter, _ := intersectionStore.Create(ctx, intersection.CreateRequest{
		TenantID:    "tenant-1",
		InitiatorID: circ.ID,
		AcceptorID:  circ.ID,
		Contract: intersection.Contract{
			Parties: []intersection.Party{
				{CircleID: circ.ID, PartyType: "initiator", JoinedAt: time.Now()},
			},
			Scopes: []intersection.Scope{
				{Name: "calendar:write", Description: "Write calendar events", ReadWrite: "write"},
			},
			ApprovalPolicy: intersection.ApprovalPolicy{
				Mode: intersection.ApprovalModeSingle,
			},
		},
	})

	// Get the contract
	contract, _ := intersectionStore.GetContract(ctx, inter.ID)

	// Create authority engine
	authorityEngine := authorityImpl.NewEngine(intersectionStore)

	// Create approval store
	approvalStore := approvalImpl.NewStore(approvalImpl.StoreConfig{
		AuditStore: auditStore,
	})

	// Create pipeline with approval verifier
	pipeline := NewPipeline(PipelineConfig{
		AuthorityEngine:  authorityEngine,
		ApprovalVerifier: approvalStore,
		AuditStore:       auditStore,
	})

	// Create action
	action := &primitives.Action{
		ID:             "action-test-1",
		IntersectionID: inter.ID,
		Type:           "calendar.create_event",
		Parameters:     map[string]string{"title": "Test Meeting"},
	}

	// Execute - should succeed without multi-party approvals (single mode)
	result := pipeline.Execute(ctx, ExecuteRequest{
		TraceID:          "trace-1",
		ActorCircleID:    circ.ID,
		IntersectionID:   inter.ID,
		ContractVersion:  inter.Version,
		Contract:         contract,
		Action:           action,
		ApprovalArtifact: "cli:--approve",
		Connector:        &mockWriteConnector{},
		CreateRequest: calendar.CreateEventRequest{
			Title:      "Test Meeting",
			StartTime:  time.Now().Add(time.Hour),
			EndTime:    time.Now().Add(2 * time.Hour),
			CalendarID: "primary",
		},
	})

	if !result.Success {
		t.Errorf("Expected success in single approval mode, got error: %v", result.Error)
	}
	if result.SettlementStatus != SettlementSettled {
		t.Errorf("Expected SettlementSettled, got %s", result.SettlementStatus)
	}
}

// TestPipeline_MultiApprovalMode_InsufficientApprovals tests that execution is blocked
// when multi-party approval threshold is not met.
func TestPipeline_MultiApprovalMode_InsufficientApprovals(t *testing.T) {
	ctx := context.Background()

	// Create infrastructure
	circleStore := circleImpl.NewRuntime()
	intersectionStore := intersectionImpl.NewRuntime()
	auditStore := auditImpl.NewStore()

	// Create circle
	circ, _ := circleStore.Create(ctx, circle.CreateRequest{TenantID: "tenant-1"})

	// Create intersection with MULTI approval mode requiring 2 approvals
	inter, _ := intersectionStore.Create(ctx, intersection.CreateRequest{
		TenantID:    "tenant-1",
		InitiatorID: circ.ID,
		AcceptorID:  circ.ID,
		Contract: intersection.Contract{
			IntersectionID: "ix-multi-1",
			Parties: []intersection.Party{
				{CircleID: circ.ID, PartyType: "initiator", JoinedAt: time.Now()},
			},
			Scopes: []intersection.Scope{
				{Name: "calendar:write", Description: "Write calendar events", ReadWrite: "write"},
			},
			ApprovalPolicy: intersection.ApprovalPolicy{
				Mode:      intersection.ApprovalModeMulti,
				Threshold: 2, // Requires 2 approvals
			},
		},
	})

	// Get the contract
	contract, _ := intersectionStore.GetContract(ctx, inter.ID)

	// Create authority engine
	authorityEngine := authorityImpl.NewEngine(intersectionStore)

	// Create approval store - NO approvals submitted
	approvalStore := approvalImpl.NewStore(approvalImpl.StoreConfig{
		AuditStore: auditStore,
	})

	// Create pipeline with approval verifier
	pipeline := NewPipeline(PipelineConfig{
		AuthorityEngine:  authorityEngine,
		ApprovalVerifier: approvalStore,
		AuditStore:       auditStore,
	})

	// Create action
	action := &primitives.Action{
		ID:             "action-test-1",
		IntersectionID: inter.ID,
		Type:           "calendar.create_event",
		Parameters:     map[string]string{"title": "Test Meeting"},
	}

	// Execute - should FAIL due to insufficient approvals
	result := pipeline.Execute(ctx, ExecuteRequest{
		TraceID:          "trace-1",
		ActorCircleID:    circ.ID,
		IntersectionID:   inter.ID,
		ContractVersion:  inter.Version,
		Contract:         contract,
		Action:           action,
		ApprovalArtifact: "cli:--approve",
		Connector:        &mockWriteConnector{},
		CreateRequest: calendar.CreateEventRequest{
			Title:      "Test Meeting",
			StartTime:  time.Now().Add(time.Hour),
			EndTime:    time.Now().Add(2 * time.Hour),
			CalendarID: "primary",
		},
	})

	if result.Success {
		t.Error("Expected failure due to insufficient approvals, but got success")
	}
	if result.SettlementStatus != SettlementBlockedApproval {
		t.Errorf("Expected SettlementBlockedApproval, got %s", result.SettlementStatus)
	}
}

// TestPipeline_MultiApprovalMode_SufficientApprovals tests that execution succeeds
// when multi-party approval threshold is met.
func TestPipeline_MultiApprovalMode_SufficientApprovals(t *testing.T) {
	ctx := context.Background()

	// Create infrastructure
	circleStore := circleImpl.NewRuntime()
	intersectionStore := intersectionImpl.NewRuntime()
	auditStore := auditImpl.NewStore()

	// Create circles
	circ1, _ := circleStore.Create(ctx, circle.CreateRequest{TenantID: "tenant-1"})
	circ2, _ := circleStore.Create(ctx, circle.CreateRequest{TenantID: "tenant-1"})

	// Create intersection with MULTI approval mode requiring 2 approvals
	inter, _ := intersectionStore.Create(ctx, intersection.CreateRequest{
		TenantID:    "tenant-1",
		InitiatorID: circ1.ID,
		AcceptorID:  circ2.ID,
		Contract: intersection.Contract{
			IntersectionID: "ix-multi-2",
			Parties: []intersection.Party{
				{CircleID: circ1.ID, PartyType: "initiator", JoinedAt: time.Now()},
				{CircleID: circ2.ID, PartyType: "acceptor", JoinedAt: time.Now()},
			},
			Scopes: []intersection.Scope{
				{Name: "calendar:write", Description: "Write calendar events", ReadWrite: "write"},
			},
			ApprovalPolicy: intersection.ApprovalPolicy{
				Mode:      intersection.ApprovalModeMulti,
				Threshold: 2, // Requires 2 approvals
			},
		},
	})

	// Get the contract
	contract, _ := intersectionStore.GetContract(ctx, inter.ID)

	// Create authority engine
	authorityEngine := authorityImpl.NewEngine(intersectionStore)

	// Create approval store
	approvalStore := approvalImpl.NewStore(approvalImpl.StoreConfig{
		AuditStore: auditStore,
	})

	// Create action
	action := &primitives.Action{
		ID:             "action-test-2",
		IntersectionID: inter.ID,
		Type:           "calendar.create_event",
		Parameters:     map[string]string{"title": "Test Meeting"},
	}

	// Request approval
	token, err := approvalStore.RequestApproval(ctx, approval.ApprovalRequest{
		IntersectionID:     inter.ID,
		ContractVersion:    inter.Version,
		Action:             action,
		ScopesRequired:     []string{"calendar:write"},
		RequestingCircleID: circ1.ID,
	})
	if err != nil {
		t.Fatalf("RequestApproval failed: %v", err)
	}

	encodedToken := primitives.EncodeApprovalToken(token)

	// Submit 2 approvals
	_, err = approvalStore.SubmitApproval(ctx, approval.SubmitApprovalRequest{
		Token:            encodedToken,
		ApproverCircleID: circ1.ID,
	})
	if err != nil {
		t.Fatalf("First SubmitApproval failed: %v", err)
	}

	_, err = approvalStore.SubmitApproval(ctx, approval.SubmitApprovalRequest{
		Token:            encodedToken,
		ApproverCircleID: circ2.ID,
	})
	if err != nil {
		t.Fatalf("Second SubmitApproval failed: %v", err)
	}

	// Create pipeline with approval verifier
	pipeline := NewPipeline(PipelineConfig{
		AuthorityEngine:  authorityEngine,
		ApprovalVerifier: approvalStore,
		AuditStore:       auditStore,
	})

	// Execute - should SUCCEED with sufficient approvals
	result := pipeline.Execute(ctx, ExecuteRequest{
		TraceID:          "trace-2",
		ActorCircleID:    circ1.ID,
		IntersectionID:   inter.ID,
		ContractVersion:  inter.Version,
		Contract:         contract,
		Action:           action,
		ApprovalArtifact: "cli:--approve",
		Connector:        &mockWriteConnector{},
		CreateRequest: calendar.CreateEventRequest{
			Title:      "Test Meeting",
			StartTime:  time.Now().Add(time.Hour),
			EndTime:    time.Now().Add(2 * time.Hour),
			CalendarID: "primary",
		},
	})

	if !result.Success {
		t.Errorf("Expected success with sufficient approvals, got error: %v", result.Error)
	}
	if result.SettlementStatus != SettlementSettled {
		t.Errorf("Expected SettlementSettled, got %s", result.SettlementStatus)
	}
}

// TestPipeline_NoContractProvided_V6Compatibility tests that pipeline works
// without contract provided (v6 compatibility).
func TestPipeline_NoContractProvided_V6Compatibility(t *testing.T) {
	ctx := context.Background()

	// Create infrastructure
	circleStore := circleImpl.NewRuntime()
	intersectionStore := intersectionImpl.NewRuntime()
	auditStore := auditImpl.NewStore()

	// Create circle
	circ, _ := circleStore.Create(ctx, circle.CreateRequest{TenantID: "tenant-1"})

	// Create intersection (but we won't pass the contract)
	inter, _ := intersectionStore.Create(ctx, intersection.CreateRequest{
		TenantID:    "tenant-1",
		InitiatorID: circ.ID,
		AcceptorID:  circ.ID,
		Contract: intersection.Contract{
			Parties: []intersection.Party{
				{CircleID: circ.ID, PartyType: "initiator", JoinedAt: time.Now()},
			},
			Scopes: []intersection.Scope{
				{Name: "calendar:write", Description: "Write calendar events", ReadWrite: "write"},
			},
		},
	})

	// Create authority engine
	authorityEngine := authorityImpl.NewEngine(intersectionStore)

	// Create approval store
	approvalStore := approvalImpl.NewStore(approvalImpl.StoreConfig{
		AuditStore: auditStore,
	})

	// Create pipeline with approval verifier
	pipeline := NewPipeline(PipelineConfig{
		AuthorityEngine:  authorityEngine,
		ApprovalVerifier: approvalStore,
		AuditStore:       auditStore,
	})

	// Create action
	action := &primitives.Action{
		ID:             "action-test-v6",
		IntersectionID: inter.ID,
		Type:           "calendar.create_event",
		Parameters:     map[string]string{"title": "V6 Compat Test"},
	}

	// Execute without Contract - should succeed (v6 compatibility)
	result := pipeline.Execute(ctx, ExecuteRequest{
		TraceID:          "trace-v6",
		ActorCircleID:    circ.ID,
		IntersectionID:   inter.ID,
		ContractVersion:  inter.Version,
		Contract:         nil, // No contract provided
		Action:           action,
		ApprovalArtifact: "cli:--approve",
		Connector:        &mockWriteConnector{},
		CreateRequest: calendar.CreateEventRequest{
			Title:      "V6 Compat Test",
			StartTime:  time.Now().Add(time.Hour),
			EndTime:    time.Now().Add(2 * time.Hour),
			CalendarID: "primary",
		},
	})

	if !result.Success {
		t.Errorf("Expected success in v6 compatibility mode (no contract), got error: %v", result.Error)
	}
}

// TestPipeline_ScopeNotApplicable tests that multi-approval policy is skipped
// when scopes don't match AppliesToScopes.
func TestPipeline_ScopeNotApplicable(t *testing.T) {
	ctx := context.Background()

	// Create infrastructure
	circleStore := circleImpl.NewRuntime()
	intersectionStore := intersectionImpl.NewRuntime()
	auditStore := auditImpl.NewStore()

	// Create circle
	circ, _ := circleStore.Create(ctx, circle.CreateRequest{TenantID: "tenant-1"})

	// Create intersection with MULTI approval mode but only applies to "email:write"
	inter, _ := intersectionStore.Create(ctx, intersection.CreateRequest{
		TenantID:    "tenant-1",
		InitiatorID: circ.ID,
		AcceptorID:  circ.ID,
		Contract: intersection.Contract{
			Parties: []intersection.Party{
				{CircleID: circ.ID, PartyType: "initiator", JoinedAt: time.Now()},
			},
			Scopes: []intersection.Scope{
				{Name: "calendar:write", Description: "Write calendar events", ReadWrite: "write"},
			},
			ApprovalPolicy: intersection.ApprovalPolicy{
				Mode:            intersection.ApprovalModeMulti,
				Threshold:       2,
				AppliesToScopes: []string{"email:write"}, // Only applies to email:write
			},
		},
	})

	// Get the contract
	contract, _ := intersectionStore.GetContract(ctx, inter.ID)

	// Create authority engine
	authorityEngine := authorityImpl.NewEngine(intersectionStore)

	// Create approval store - NO approvals needed since policy doesn't apply
	approvalStore := approvalImpl.NewStore(approvalImpl.StoreConfig{
		AuditStore: auditStore,
	})

	// Create pipeline with approval verifier
	pipeline := NewPipeline(PipelineConfig{
		AuthorityEngine:  authorityEngine,
		ApprovalVerifier: approvalStore,
		AuditStore:       auditStore,
	})

	// Create action
	action := &primitives.Action{
		ID:             "action-test-scope",
		IntersectionID: inter.ID,
		Type:           "calendar.create_event",
		Parameters:     map[string]string{"title": "Test Meeting"},
	}

	// Execute - should SUCCEED because policy doesn't apply to calendar:write
	result := pipeline.Execute(ctx, ExecuteRequest{
		TraceID:          "trace-scope",
		ActorCircleID:    circ.ID,
		IntersectionID:   inter.ID,
		ContractVersion:  inter.Version,
		Contract:         contract,
		Action:           action,
		ApprovalArtifact: "cli:--approve",
		Connector:        &mockWriteConnector{},
		CreateRequest: calendar.CreateEventRequest{
			Title:      "Test Meeting",
			StartTime:  time.Now().Add(time.Hour),
			EndTime:    time.Now().Add(2 * time.Hour),
			CalendarID: "primary",
		},
	})

	if !result.Success {
		t.Errorf("Expected success when policy doesn't apply to scope, got error: %v", result.Error)
	}
	if result.SettlementStatus != SettlementSettled {
		t.Errorf("Expected SettlementSettled, got %s", result.SettlementStatus)
	}
}
