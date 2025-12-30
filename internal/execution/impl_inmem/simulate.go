// Package impl_inmem provides an in-memory implementation of execution interfaces.
// This is for demo and testing purposes only.
//
// CRITICAL: This is DATA PLANE. It MUST NOT import negotiation or authority packages.
// All behavior is deterministic — no LLM/SLM, no randomness, no real clocks.
//
// Reference: docs/TECHNICAL_SPLIT_V1.md §3.5 Action Execution Layer
package impl_inmem

import (
	"context"
	"fmt"
	"sync"
	"time"

	"quantumlife/internal/connectors/calendar"
	"quantumlife/internal/execution"
	"quantumlife/pkg/primitives"
)

// Simulator implements deterministic action simulation.
// It uses connectors to propose actions without performing real writes.
type Simulator struct {
	mu              sync.RWMutex
	calendarConn    calendar.Connector
	actions         map[string]*primitives.Action
	outcomes        map[string]*execution.ExecutionOutcome
	clockFunc       func() time.Time
	idCounter       int
	authProofLookup func(actionID string) string // Lookup function for auth proof ID
}

// NewSimulator creates a new execution simulator.
func NewSimulator(calendarConn calendar.Connector) *Simulator {
	return &Simulator{
		calendarConn: calendarConn,
		actions:      make(map[string]*primitives.Action),
		outcomes:     make(map[string]*execution.ExecutionOutcome),
		clockFunc:    time.Now,
	}
}

// NewSimulatorWithClock creates a simulator with an injected clock for determinism.
func NewSimulatorWithClock(calendarConn calendar.Connector, clockFunc func() time.Time) *Simulator {
	return &Simulator{
		calendarConn: calendarConn,
		actions:      make(map[string]*primitives.Action),
		outcomes:     make(map[string]*execution.ExecutionOutcome),
		clockFunc:    clockFunc,
	}
}

// SetAuthProofLookup sets the function to lookup authorization proof IDs.
func (s *Simulator) SetAuthProofLookup(lookup func(actionID string) string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.authProofLookup = lookup
}

// CreateActionFromCommitment creates an action from a commitment.
// Returns the action without executing it.
func (s *Simulator) CreateActionFromCommitment(ctx context.Context, commitment *primitives.Commitment) (*primitives.Action, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.idCounter++
	actionID := fmt.Sprintf("action-%d", s.idCounter)

	now := s.clockFunc()
	action := &primitives.Action{
		ID:             actionID,
		Version:        1,
		CreatedAt:      now,
		Issuer:         commitment.Issuer,
		CommitmentID:   commitment.ID,
		IntersectionID: commitment.IntersectionID,
		Type:           commitment.ActionSpec.Type,
		Parameters:     commitment.ActionSpec.Parameters,
		State:          "pending",
	}

	s.actions[actionID] = action
	return action, nil
}

// SimulateExecution simulates the execution of an action.
// CRITICAL: This does NOT perform any real external writes.
func (s *Simulator) SimulateExecution(ctx context.Context, action *primitives.Action) (*execution.ExecutionOutcome, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.clockFunc()

	// Mark action as executing
	if stored, ok := s.actions[action.ID]; ok {
		stored.State = "executing"
		stored.StartedAt = &now
	}

	// Route to appropriate connector based on action type
	var outcome *execution.ExecutionOutcome
	var err error

	switch action.Type {
	case "calendar_suggestion", "calendar_proposal", "propose_family_activity":
		outcome, err = s.simulateCalendarAction(ctx, action, now)
	default:
		outcome = &execution.ExecutionOutcome{
			ActionID:     action.ID,
			Success:      true,
			Simulated:    true,
			ResultCode:   "generic_simulated",
			ResultData:   map[string]string{"message": "Action simulated successfully"},
			ConnectorID:  "generic",
			ExecutedAt:   now,
			ErrorMessage: "",
		}
	}

	if err != nil {
		return nil, err
	}

	// Set auth proof ID if lookup is available
	if s.authProofLookup != nil {
		outcome.AuthorizationProofID = s.authProofLookup(action.ID)
	}

	// Mark action as completed
	if stored, ok := s.actions[action.ID]; ok {
		stored.State = "completed"
		stored.CompletedAt = &now
	}

	s.outcomes[action.ID] = outcome
	return outcome, nil
}

// simulateCalendarAction simulates a calendar-related action.
func (s *Simulator) simulateCalendarAction(ctx context.Context, action *primitives.Action, now time.Time) (*execution.ExecutionOutcome, error) {
	if s.calendarConn == nil {
		return &execution.ExecutionOutcome{
			ActionID:     action.ID,
			Success:      false,
			Simulated:    true,
			ResultCode:   "no_connector",
			ErrorMessage: "No calendar connector available",
			ExecutedAt:   now,
		}, nil
	}

	// Build proposal request from action parameters
	title := action.Parameters["title"]
	if title == "" {
		title = "Family Activity"
	}

	description := action.Parameters["description"]
	if description == "" {
		description = "Proposed family activity"
	}

	// Parse time window from parameters
	startTime := now
	endTime := now.Add(1 * time.Hour)

	if tw := action.Parameters["time_window"]; tw != "" {
		// For demo, we'll use a fixed time based on the window
		startTime = time.Date(now.Year(), now.Month(), now.Day(), 18, 0, 0, 0, time.UTC)
		endTime = time.Date(now.Year(), now.Month(), now.Day(), 19, 0, 0, 0, time.UTC)
	}

	req := calendar.ProposeEventRequest{
		Title:       title,
		Description: description,
		StartTime:   startTime,
		EndTime:     endTime,
		Location:    action.Parameters["location"],
		CalendarID:  "family",
	}

	proposed, err := s.calendarConn.ProposeEvent(ctx, req)
	if err != nil {
		return &execution.ExecutionOutcome{
			ActionID:     action.ID,
			Success:      false,
			Simulated:    true,
			ResultCode:   "proposal_error",
			ErrorMessage: err.Error(),
			ConnectorID:  s.calendarConn.ID(),
			ExecutedAt:   now,
		}, nil
	}

	return &execution.ExecutionOutcome{
		ActionID:   action.ID,
		Success:    true,
		Simulated:  true,
		ResultCode: "simulated_proposal",
		ResultData: map[string]string{
			"proposal_id": proposed.ProposalID,
			"event_title": proposed.Event.Title,
			"event_start": proposed.Event.StartTime.Format(time.RFC3339),
			"event_end":   proposed.Event.EndTime.Format(time.RFC3339),
			"conflicts":   fmt.Sprintf("%d", len(proposed.ConflictingEvents)),
		},
		ConnectorID: s.calendarConn.ID(),
		ProposedPayload: map[string]string{
			"title":       proposed.Event.Title,
			"description": proposed.Event.Description,
			"start_time":  proposed.Event.StartTime.Format(time.RFC3339),
			"end_time":    proposed.Event.EndTime.Format(time.RFC3339),
			"location":    proposed.Event.Location,
			"calendar_id": proposed.Event.CalendarID,
			"simulated":   "true",
			"message":     proposed.Message,
		},
		ExecutedAt: now,
	}, nil
}

// GetAction retrieves an action by ID.
func (s *Simulator) GetAction(ctx context.Context, actionID string) (*primitives.Action, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if action, ok := s.actions[actionID]; ok {
		return action, nil
	}
	return nil, fmt.Errorf("action not found: %s", actionID)
}

// GetOutcome retrieves an execution outcome by action ID.
func (s *Simulator) GetOutcome(ctx context.Context, actionID string) (*execution.ExecutionOutcome, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if outcome, ok := s.outcomes[actionID]; ok {
		return outcome, nil
	}
	return nil, fmt.Errorf("outcome not found: %s", actionID)
}

// GetStatus returns the current execution status.
func (s *Simulator) GetStatus(ctx context.Context, actionID string) (*execution.ExecutionStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	action, ok := s.actions[actionID]
	if !ok {
		return nil, fmt.Errorf("action not found: %s", actionID)
	}

	return &execution.ExecutionStatus{
		ActionID:  action.ID,
		State:     execution.State(action.State),
		Progress:  1.0, // Simulated actions complete instantly
		StartedAt: action.StartedAt,
		Message:   "Simulated execution",
	}, nil
}
