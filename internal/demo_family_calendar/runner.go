// Package demo_family_calendar demonstrates v5 calendar read operations.
// This demo reads from real calendar providers (Google/Microsoft) when configured,
// or falls back to mock data.
//
// CRITICAL: This demo is READ-ONLY. No external writes are performed.
//
// Reference: docs/TECHNICAL_SPLIT_V1.md §3.5 Action Execution Layer
package demo_family_calendar

import (
	"context"
	"fmt"
	"time"

	"quantumlife/internal/audit"
	auditImpl "quantumlife/internal/audit/impl_inmem"
	authorityImpl "quantumlife/internal/authority/impl_inmem"
	"quantumlife/internal/circle"
	circleImpl "quantumlife/internal/circle/impl_inmem"
	"quantumlife/internal/connectors/auth"
	authImpl "quantumlife/internal/connectors/auth/impl_inmem"
	"quantumlife/internal/connectors/calendar"
	calendarMock "quantumlife/internal/connectors/calendar/impl_mock"
	"quantumlife/internal/connectors/calendar/providers/google"
	"quantumlife/internal/connectors/calendar/providers/microsoft"
	"quantumlife/internal/intersection"
	intersectionImpl "quantumlife/internal/intersection/impl_inmem"
	"quantumlife/pkg/events"
	"quantumlife/pkg/primitives"
)

// Result contains the demo output.
type Result struct {
	// Success indicates if the demo completed successfully.
	Success bool

	// Error contains any error message.
	Error string

	// ProvidersUsed lists which calendar providers were used.
	ProvidersUsed []string

	// UsingMock indicates if the mock provider was used.
	UsingMock bool

	// EventsFetched is the total number of events fetched.
	EventsFetched int

	// EventsByProvider maps provider to event count.
	EventsByProvider map[string]int

	// FreeSlotsFound is the number of free slots found.
	FreeSlotsFound int

	// FreeSlots contains the top 3 free slots.
	FreeSlots []calendar.FreeSlot

	// AuditEntries contains audit log entries.
	AuditEntries []audit.Entry

	// IntersectionID is the intersection used.
	IntersectionID string

	// ContractVersion is the contract version.
	ContractVersion string

	// AuthorizationProofID is the authorization proof used.
	AuthorizationProofID string

	// TraceID is the distributed trace ID.
	TraceID string

	// Mode is the run mode used.
	Mode primitives.RunMode

	// TimeRange is the time range that was queried.
	TimeRange calendar.EventRange

	// Events contains the fetched events (for display).
	Events []calendar.Event
}

// Runner executes the calendar read demo.
type Runner struct {
	mode           primitives.RunMode
	clockFunc      func() time.Time
	config         auth.Config
	broker         *authImpl.Broker
	usePersistence bool
}

// NewRunner creates a new demo runner.
func NewRunner() *Runner {
	return &Runner{
		mode:      primitives.ModeSimulate,
		clockFunc: time.Now,
		config:    auth.LoadConfigFromEnv(),
	}
}

// NewRunnerWithMode creates a runner with a specific mode.
func NewRunnerWithMode(mode primitives.RunMode) *Runner {
	return &Runner{
		mode:      mode,
		clockFunc: time.Now,
		config:    auth.LoadConfigFromEnv(),
	}
}

// NewRunnerWithPersistence creates a runner that uses persistent token storage.
// This allows the runner to use tokens stored via CLI auth flow.
func NewRunnerWithPersistence(mode primitives.RunMode) (*Runner, error) {
	config := auth.LoadConfigFromEnv()

	// Try to create broker with persistence
	broker, err := authImpl.NewBrokerWithPersistence(config, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create persistent broker: %w", err)
	}

	return &Runner{
		mode:           mode,
		clockFunc:      time.Now,
		config:         config,
		broker:         broker,
		usePersistence: broker.IsPersistenceEnabled(),
	}, nil
}

// Run executes the demo.
func (r *Runner) Run(ctx context.Context) (*Result, error) {
	result := &Result{
		Mode:             r.mode,
		EventsByProvider: make(map[string]int),
	}

	// Validate mode
	if r.mode == primitives.ModeExecute {
		result.Error = "execute mode is not implemented"
		return result, primitives.ErrExecuteNotImplemented
	}

	// Generate trace ID
	traceID := fmt.Sprintf("trace-calendar-%d", r.clockFunc().UnixNano())
	result.TraceID = traceID

	// Create stores
	circleStore := circleImpl.NewRuntime()
	intersectionStore := intersectionImpl.NewRuntime()
	auditStore := auditImpl.NewStore()

	// Create circles for the demo
	parentCircle, err := createDemoCircle(ctx, circleStore, "parent-calendar-1")
	if err != nil {
		result.Error = fmt.Sprintf("failed to create parent circle: %v", err)
		return result, err
	}

	childCircle, err := createDemoCircle(ctx, circleStore, "child-calendar-1")
	if err != nil {
		result.Error = fmt.Sprintf("failed to create child circle: %v", err)
		return result, err
	}

	// Create family intersection with calendar:read scope only
	inter, err := createDemoIntersection(ctx, intersectionStore, parentCircle.ID, childCircle.ID)
	if err != nil {
		result.Error = fmt.Sprintf("failed to create intersection: %v", err)
		return result, err
	}
	result.IntersectionID = inter.ID
	result.ContractVersion = inter.Version

	// Create authority engine and get authorization proof
	authorityEngine := authorityImpl.NewEngineWithClock(intersectionStore, r.clockFunc)

	// Create a mock action for authorization
	action := &primitives.Action{
		ID:             fmt.Sprintf("action-calendar-%d", r.clockFunc().UnixNano()),
		IntersectionID: inter.ID,
		Type:           "calendar.read",
		Parameters:     map[string]string{},
	}

	proof, err := authorityEngine.AuthorizeAction(ctx, action, []string{"calendar:read"}, r.mode, traceID)
	if err != nil {
		result.Error = fmt.Sprintf("authorization failed: %v", err)
		return result, err
	}
	if !proof.Authorized {
		result.Error = fmt.Sprintf("authorization denied: %s", proof.DenialReason)
		return result, fmt.Errorf("authorization denied: %s", proof.DenialReason)
	}
	result.AuthorizationProofID = proof.ID

	// Log authorization event
	auditStore.Append(ctx, auditImpl.Entry{
		Type:                 string(events.EventAuthorizationChecked),
		CircleID:             parentCircle.ID,
		IntersectionID:       inter.ID,
		Action:               "calendar_read_authorized",
		Outcome:              "authorized",
		TraceID:              traceID,
		AuthorizationProofID: proof.ID,
	})

	// Build execution envelope
	env := primitives.ExecutionEnvelope{
		TraceID:              traceID,
		Mode:                 r.mode,
		ActorCircleID:        parentCircle.ID,
		IntersectionID:       inter.ID,
		ContractVersion:      inter.Version,
		ScopesUsed:           []string{"calendar:read"},
		AuthorizationProofID: proof.ID,
		IssuedAt:             r.clockFunc(),
	}

	// Set up time range for next 24 hours
	now := r.clockFunc()
	// Use a fixed base for demo if running with mock
	if !r.config.Google.IsConfigured() && !r.config.Microsoft.IsConfigured() {
		// Use the mock's fixed date for consistency
		now = time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	}
	timeRange := calendar.EventRange{
		Start: now,
		End:   now.Add(24 * time.Hour),
	}
	result.TimeRange = timeRange

	// Create token broker and connectors
	authorityChecker := &brokerAuthorityChecker{engine: authorityEngine}

	// Use injected broker if available (for persistence support), otherwise create new
	var broker *authImpl.Broker
	if r.broker != nil {
		broker = r.broker
	} else {
		broker = authImpl.NewBroker(r.config, authorityChecker)
	}

	var connectors []calendar.EnvelopeConnector

	// Check which providers are configured (via env vars)
	envConfiguredGoogle := r.config.Google.IsConfigured()
	envConfiguredMicrosoft := r.config.Microsoft.IsConfigured()

	// Also check if we have stored tokens (via CLI auth flow)
	hasStoredGoogle := false
	hasStoredMicrosoft := false
	if r.usePersistence && r.broker != nil {
		_, hasStoredGoogle = r.broker.GetTokenHandle(parentCircle.ID, auth.ProviderGoogle)
		_, hasStoredMicrosoft = r.broker.GetTokenHandle(parentCircle.ID, auth.ProviderMicrosoft)
	}

	// Use Google if env vars are set OR if we have stored tokens
	if envConfiguredGoogle || hasStoredGoogle {
		googleAdapter := google.NewAdapter(broker, envConfiguredGoogle || hasStoredGoogle)
		connectors = append(connectors, googleAdapter)
		result.ProvidersUsed = append(result.ProvidersUsed, "google")
	}

	// Use Microsoft if env vars are set OR if we have stored tokens
	if envConfiguredMicrosoft || hasStoredMicrosoft {
		msAdapter := microsoft.NewAdapter(broker, envConfiguredMicrosoft || hasStoredMicrosoft)
		connectors = append(connectors, msAdapter)
		result.ProvidersUsed = append(result.ProvidersUsed, "microsoft")
	}

	// Fall back to mock if no providers configured
	if len(connectors) == 0 {
		mockConn := calendarMock.NewMockConnectorWithClock(r.clockFunc)
		connectors = append(connectors, mockConn)
		result.ProvidersUsed = append(result.ProvidersUsed, "mock")
		result.UsingMock = true
	}

	// Fetch events from all connectors
	var allEvents []calendar.Event
	for _, conn := range connectors {
		providerInfo := conn.ProviderInfo()

		// Log token mint event
		auditStore.Append(ctx, auditImpl.Entry{
			Type:                 string(events.EventConnectorTokenMinted),
			CircleID:             parentCircle.ID,
			IntersectionID:       inter.ID,
			Action:               "token_minted",
			Outcome:              "success",
			TraceID:              traceID,
			AuthorizationProofID: proof.ID,
		})

		// List events
		evts, err := conn.ListEventsWithEnvelope(ctx, env, timeRange)
		if err != nil {
			// Log failure but continue
			auditStore.Append(ctx, auditImpl.Entry{
				Type:                 string(events.EventConnectorCallFailed),
				CircleID:             parentCircle.ID,
				IntersectionID:       inter.ID,
				Action:               "list_events",
				Outcome:              "failed",
				TraceID:              traceID,
				AuthorizationProofID: proof.ID,
			})
			continue
		}

		// Log success
		auditStore.Append(ctx, auditImpl.Entry{
			Type:                 string(events.EventConnectorReadCompleted),
			CircleID:             parentCircle.ID,
			IntersectionID:       inter.ID,
			Action:               "list_events",
			Outcome:              fmt.Sprintf("fetched %d events", len(evts)),
			TraceID:              traceID,
			AuthorizationProofID: proof.ID,
		})

		allEvents = append(allEvents, evts...)
		result.EventsByProvider[providerInfo.ID] = len(evts)
	}

	result.Events = allEvents
	result.EventsFetched = len(allEvents)

	// Find free slots using the first connector (or aggregated if multiple)
	if len(connectors) > 0 {
		slots, err := connectors[0].FindFreeSlots(ctx, env, timeRange, 30*time.Minute)
		if err == nil {
			result.FreeSlotsFound = len(slots)
			// Take top 3
			if len(slots) > 3 {
				slots = slots[:3]
			}
			result.FreeSlots = slots
		}
	}

	// Get audit entries
	result.AuditEntries = auditStore.GetAllEntries()

	result.Success = true
	return result, nil
}

// createDemoCircle creates a circle for the demo.
func createDemoCircle(ctx context.Context, store circle.Runtime, tenantID string) (*circle.Circle, error) {
	return store.Create(ctx, circle.CreateRequest{
		TenantID: tenantID,
	})
}

// createDemoIntersection creates an intersection with calendar:read scope.
func createDemoIntersection(ctx context.Context, store intersection.Runtime, parentID, childID string) (*intersection.Intersection, error) {
	contract := intersection.Contract{
		Parties: []intersection.Party{
			{CircleID: parentID, PartyType: "initiator", JoinedAt: time.Now()},
			{CircleID: childID, PartyType: "acceptor", JoinedAt: time.Now()},
		},
		Scopes: []intersection.Scope{
			{Name: "calendar:read", Description: "Read calendar events", ReadWrite: "read"},
		},
		Ceilings: []intersection.Ceiling{
			{Type: "time_window", Value: "00:00-23:59", Unit: "daily"},
		},
	}

	return store.Create(ctx, intersection.CreateRequest{
		TenantID:    "demo-tenant",
		InitiatorID: parentID,
		AcceptorID:  childID,
		Contract:    contract,
	})
}

// brokerAuthorityChecker adapts the authority engine for the token broker.
type brokerAuthorityChecker struct {
	engine *authorityImpl.Engine
}

func (b *brokerAuthorityChecker) GetProof(ctx context.Context, proofID string) (auth.AuthProofSummary, error) {
	proof, err := b.engine.GetProof(ctx, proofID)
	if err != nil {
		return auth.AuthProofSummary{}, err
	}

	return auth.AuthProofSummary{
		ID:              proof.ID,
		Authorized:      proof.Authorized,
		ScopesGranted:   proof.ScopesGranted,
		IntersectionID:  proof.IntersectionID,
		ContractVersion: proof.ContractVersion,
	}, nil
}
