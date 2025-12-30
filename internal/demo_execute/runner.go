// Package demo_execute demonstrates v6 Execute mode operations.
// This demo runs the full two-phase execution pipeline with mock connectors.
//
// CRITICAL: This demo uses mock connectors - no real external writes occur.
// For real writes, use the CLI with --approve flag.
//
// Reference: docs/TECHNICAL_SPLIT_V1.md §3.5 Action Execution Layer
package demo_execute

import (
	"context"
	"errors"
	"fmt"
	"time"

	actionImpl "quantumlife/internal/action/impl_inmem"
	"quantumlife/internal/audit"
	auditImpl "quantumlife/internal/audit/impl_inmem"
	authorityImpl "quantumlife/internal/authority/impl_inmem"
	"quantumlife/internal/circle"
	circleImpl "quantumlife/internal/circle/impl_inmem"
	"quantumlife/internal/connectors/calendar"
	"quantumlife/internal/intersection"
	intersectionImpl "quantumlife/internal/intersection/impl_inmem"
	revocationImpl "quantumlife/internal/revocation/impl_inmem"
	"quantumlife/pkg/primitives"
)

// Result contains the demo output.
type Result struct {
	// Success indicates if the demo completed successfully.
	Success bool

	// Error contains any error message.
	Error string

	// ExecuteResult contains the pipeline execution result.
	ExecuteResult *actionImpl.ExecuteResult

	// IntersectionID is the intersection used.
	IntersectionID string

	// ContractVersion is the contract version.
	ContractVersion string

	// TraceID is the distributed trace ID.
	TraceID string

	// Mode is the run mode used.
	Mode primitives.RunMode

	// AuditEntries contains audit log entries.
	AuditEntries []audit.Entry

	// RevocationApplied indicates if revocation was tested.
	RevocationApplied bool
}

// Runner runs the v6 Execute mode demo.
type Runner struct {
	clockFunc func() time.Time
}

// NewRunner creates a new demo runner.
func NewRunner() *Runner {
	return &Runner{
		clockFunc: time.Now,
	}
}

// NewRunnerWithClock creates a runner with injected clock for determinism.
func NewRunnerWithClock(clockFunc func() time.Time) *Runner {
	return &Runner{
		clockFunc: clockFunc,
	}
}

// Run executes the v6 Execute mode demo.
// This demonstrates:
// - Two-phase execution pipeline
// - Authorization with approval
// - Settlement on success
// - Revocation handling
func (r *Runner) Run(ctx context.Context) (*Result, error) {
	result := &Result{
		Mode: primitives.ModeExecute,
	}

	// Create infrastructure
	circleStore := circleImpl.NewRuntime()
	intersectionStore := intersectionImpl.NewRuntime()
	auditStore := auditImpl.NewStore()
	revocationRegistry := revocationImpl.NewRegistryWithClock(r.clockFunc)
	authorityEngine := authorityImpl.NewEngine(intersectionStore)

	// Create circles
	parentCircle, err := circleStore.Create(ctx, circle.CreateRequest{TenantID: "demo-tenant"})
	if err != nil {
		result.Error = fmt.Sprintf("failed to create parent circle: %v", err)
		return result, nil
	}

	childCircle, err := circleStore.Create(ctx, circle.CreateRequest{TenantID: "demo-tenant"})
	if err != nil {
		result.Error = fmt.Sprintf("failed to create child circle: %v", err)
		return result, nil
	}

	// Create intersection with write scope
	inter, err := intersectionStore.Create(ctx, intersection.CreateRequest{
		TenantID:    "demo-tenant",
		InitiatorID: parentCircle.ID,
		AcceptorID:  childCircle.ID,
		Contract: intersection.Contract{
			Parties: []intersection.Party{
				{CircleID: parentCircle.ID, PartyType: "initiator", JoinedAt: r.clockFunc()},
				{CircleID: childCircle.ID, PartyType: "acceptor", JoinedAt: r.clockFunc()},
			},
			Scopes: []intersection.Scope{
				{Name: "calendar:read", Description: "Read calendar events", ReadWrite: "read"},
				{Name: "calendar:write", Description: "Write calendar events", ReadWrite: "write"},
			},
			Ceilings: []intersection.Ceiling{
				{Type: "time_window", Value: "09:00-17:00", Unit: "daily"},
				{Type: "max_events", Value: "10", Unit: "daily"},
			},
		},
	})
	if err != nil {
		result.Error = fmt.Sprintf("failed to create intersection: %v", err)
		return result, nil
	}

	result.IntersectionID = inter.ID
	result.ContractVersion = inter.Version
	result.TraceID = fmt.Sprintf("trace-demo-execute-%d", r.clockFunc().UnixNano())

	// Create pipeline
	pipeline := actionImpl.NewPipeline(actionImpl.PipelineConfig{
		AuthorityEngine:   authorityEngine,
		RevocationChecker: revocationRegistry,
		AuditStore:        auditStore,
		ClockFunc:         r.clockFunc,
	})

	// Create mock write connector
	connector := &mockWriteConnector{
		clockFunc: r.clockFunc,
	}

	// Demo 1: Successful event creation
	action := &primitives.Action{
		ID:             fmt.Sprintf("action-demo-%d", r.clockFunc().UnixNano()),
		IntersectionID: inter.ID,
		Type:           "calendar.create_event",
		Parameters:     map[string]string{"title": "Demo Meeting"},
	}

	startTime := r.clockFunc().Add(time.Hour)
	createReq := calendar.CreateEventRequest{
		Title:       "Demo Meeting",
		Description: "v6 Execute mode demonstration",
		StartTime:   startTime,
		EndTime:     startTime.Add(time.Hour),
		Location:    "Conference Room A",
		CalendarID:  "primary",
	}

	execResult := pipeline.Execute(ctx, actionImpl.ExecuteRequest{
		TraceID:          result.TraceID,
		ActorCircleID:    parentCircle.ID,
		IntersectionID:   inter.ID,
		ContractVersion:  inter.Version,
		Action:           action,
		ApprovalArtifact: "demo:automated-test",
		Connector:        connector,
		CreateRequest:    createReq,
	})

	result.ExecuteResult = execResult
	result.Success = execResult.Success
	if execResult.Error != nil {
		result.Error = execResult.Error.Error()
	}

	// Collect audit entries
	result.AuditEntries = auditStore.GetAllEntries()

	return result, nil
}

// RunWithRevocation demonstrates revocation halting execution.
func (r *Runner) RunWithRevocation(ctx context.Context) (*Result, error) {
	result := &Result{
		Mode:              primitives.ModeExecute,
		RevocationApplied: true,
	}

	// Create infrastructure
	circleStore := circleImpl.NewRuntime()
	intersectionStore := intersectionImpl.NewRuntime()
	auditStore := auditImpl.NewStore()
	revocationRegistry := revocationImpl.NewRegistryWithClock(r.clockFunc)
	authorityEngine := authorityImpl.NewEngine(intersectionStore)

	// Create circle
	circ, err := circleStore.Create(ctx, circle.CreateRequest{TenantID: "demo-tenant"})
	if err != nil {
		result.Error = fmt.Sprintf("failed to create circle: %v", err)
		return result, nil
	}

	// Create intersection with write scope
	inter, err := intersectionStore.Create(ctx, intersection.CreateRequest{
		TenantID:    "demo-tenant",
		InitiatorID: circ.ID,
		AcceptorID:  circ.ID,
		Contract: intersection.Contract{
			Parties: []intersection.Party{
				{CircleID: circ.ID, PartyType: "initiator", JoinedAt: r.clockFunc()},
			},
			Scopes: []intersection.Scope{
				{Name: "calendar:write", Description: "Write calendar", ReadWrite: "write"},
			},
		},
	})
	if err != nil {
		result.Error = fmt.Sprintf("failed to create intersection: %v", err)
		return result, nil
	}

	result.IntersectionID = inter.ID
	result.ContractVersion = inter.Version
	result.TraceID = fmt.Sprintf("trace-demo-revoke-%d", r.clockFunc().UnixNano())

	// Create pipeline
	pipeline := actionImpl.NewPipeline(actionImpl.PipelineConfig{
		AuthorityEngine:   authorityEngine,
		RevocationChecker: revocationRegistry,
		AuditStore:        auditStore,
		ClockFunc:         r.clockFunc,
	})

	// Revoke the action BEFORE execution
	actionID := fmt.Sprintf("action-revoked-%d", r.clockFunc().UnixNano())
	err = revocationRegistry.RevokeAction(ctx, actionID, "cancelled by circle", circ.ID)
	if err != nil {
		result.Error = fmt.Sprintf("failed to revoke action: %v", err)
		return result, nil
	}

	// Create mock write connector
	connector := &mockWriteConnector{
		clockFunc: r.clockFunc,
	}

	// Attempt to execute the revoked action
	action := &primitives.Action{
		ID:             actionID,
		IntersectionID: inter.ID,
		Type:           "calendar.create_event",
	}

	startTime := r.clockFunc().Add(time.Hour)
	createReq := calendar.CreateEventRequest{
		Title:     "Should Not Be Created",
		StartTime: startTime,
		EndTime:   startTime.Add(time.Hour),
	}

	execResult := pipeline.Execute(ctx, actionImpl.ExecuteRequest{
		TraceID:          result.TraceID,
		ActorCircleID:    circ.ID,
		IntersectionID:   inter.ID,
		ContractVersion:  inter.Version,
		Action:           action,
		ApprovalArtifact: "demo:automated-test",
		Connector:        connector,
		CreateRequest:    createReq,
	})

	result.ExecuteResult = execResult
	result.Success = !connector.createCalled // Success if CreateEvent was NOT called
	if execResult.SettlementStatus != actionImpl.SettlementRevoked {
		result.Error = fmt.Sprintf("expected SettlementRevoked, got %s", execResult.SettlementStatus)
		result.Success = false
	}

	// Collect audit entries
	result.AuditEntries = auditStore.GetAllEntries()

	return result, nil
}

// mockWriteConnector implements calendar.WriteConnector for demo purposes.
type mockWriteConnector struct {
	clockFunc    func() time.Time
	createCalled bool
	deleteCalled bool
	eventID      string
}

func (m *mockWriteConnector) ID() string {
	return "mock-write-connector"
}

func (m *mockWriteConnector) Capabilities() []string {
	return []string{"create_event", "delete_event"}
}

func (m *mockWriteConnector) RequiredScopes() []string {
	return []string{"calendar:write"}
}

func (m *mockWriteConnector) ListEvents(ctx context.Context, req calendar.ListEventsRequest) ([]calendar.Event, error) {
	return nil, errors.New("not implemented for demo")
}

func (m *mockWriteConnector) ProposeEvent(ctx context.Context, req calendar.ProposeEventRequest) (*calendar.ProposedEvent, error) {
	return nil, errors.New("not implemented for demo")
}

func (m *mockWriteConnector) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *mockWriteConnector) ListEventsWithEnvelope(ctx context.Context, env primitives.ExecutionEnvelope, r calendar.EventRange) ([]calendar.Event, error) {
	return nil, errors.New("not implemented for demo")
}

func (m *mockWriteConnector) FindFreeSlots(ctx context.Context, env primitives.ExecutionEnvelope, r calendar.EventRange, minDuration time.Duration) ([]calendar.FreeSlot, error) {
	return nil, errors.New("not implemented for demo")
}

func (m *mockWriteConnector) ProposeEventWithEnvelope(ctx context.Context, env primitives.ExecutionEnvelope, req calendar.ProposeEventRequest) (*calendar.ProposedEvent, error) {
	return nil, errors.New("not implemented for demo")
}

func (m *mockWriteConnector) ProviderInfo() calendar.ProviderInfo {
	return calendar.ProviderInfo{
		ID:           "mock",
		Name:         "Mock Write Connector",
		IsConfigured: true,
	}
}

func (m *mockWriteConnector) CreateEvent(ctx context.Context, env primitives.ExecutionEnvelope, req calendar.CreateEventRequest) (*calendar.CreateEventReceipt, error) {
	m.createCalled = true
	m.eventID = fmt.Sprintf("mock-event-%d", m.clockFunc().UnixNano())
	return &calendar.CreateEventReceipt{
		Provider:        calendar.SourceMock,
		CalendarID:      req.CalendarID,
		ExternalEventID: m.eventID,
		Status:          "created",
		CreatedAt:       m.clockFunc(),
		Link:            fmt.Sprintf("https://mock.calendar/events/%s", m.eventID),
	}, nil
}

func (m *mockWriteConnector) DeleteEvent(ctx context.Context, env primitives.ExecutionEnvelope, req calendar.DeleteEventRequest) (*calendar.DeleteEventReceipt, error) {
	m.deleteCalled = true
	return &calendar.DeleteEventReceipt{
		Provider:        calendar.SourceMock,
		ExternalEventID: req.ExternalEventID,
		Status:          "deleted",
		DeletedAt:       m.clockFunc(),
	}, nil
}

func (m *mockWriteConnector) SupportsWrite() bool {
	return true
}

// Verify interface compliance at compile time.
var _ calendar.WriteConnector = (*mockWriteConnector)(nil)
