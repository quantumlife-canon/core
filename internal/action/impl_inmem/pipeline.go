// Package impl_inmem provides the execution pipeline for v6 Execute mode.
// This implements the two-phase pattern for safe external writes.
//
// CRITICAL: This is the primary entry point for all Execute mode operations.
// All external writes MUST go through this pipeline.
//
// Two-Phase Pattern:
// 1. Prepare: Validate, authorize, check revocation, emit action.pending
// 2. Execute: Check revocation again, perform write, settle
//
// Reference: docs/TECHNICAL_SPLIT_V1.md §3.5 Action Execution Layer
package impl_inmem

import (
	"context"
	"errors"
	"fmt"
	"time"

	auditImpl "quantumlife/internal/audit/impl_inmem"
	"quantumlife/internal/authority"
	authorityImpl "quantumlife/internal/authority/impl_inmem"
	"quantumlife/internal/connectors/calendar"
	"quantumlife/internal/revocation"
	"quantumlife/pkg/events"
	"quantumlife/pkg/primitives"
)

// Pipeline executes actions with the two-phase pattern.
// CRITICAL: All external writes MUST go through this pipeline.
type Pipeline struct {
	authority  *authorityImpl.Engine
	revocation revocation.Checker
	auditStore *auditImpl.Store
	clockFunc  func() time.Time
	idCounter  int
}

// PipelineConfig configures the execution pipeline.
type PipelineConfig struct {
	AuthorityEngine   *authorityImpl.Engine
	RevocationChecker revocation.Checker
	AuditStore        *auditImpl.Store
	ClockFunc         func() time.Time
}

// NewPipeline creates a new execution pipeline.
func NewPipeline(config PipelineConfig) *Pipeline {
	clockFunc := config.ClockFunc
	if clockFunc == nil {
		clockFunc = time.Now
	}

	return &Pipeline{
		authority:  config.AuthorityEngine,
		revocation: config.RevocationChecker,
		auditStore: config.AuditStore,
		clockFunc:  clockFunc,
	}
}

// ExecuteRequest contains the parameters for executing an action.
type ExecuteRequest struct {
	// TraceID links to a distributed trace.
	TraceID string

	// ActorCircleID is the circle initiating the action.
	ActorCircleID string

	// IntersectionID is the intersection authorizing the action.
	IntersectionID string

	// ContractVersion is the version of the contract.
	ContractVersion string

	// Action contains the action to execute.
	Action *primitives.Action

	// ApprovalArtifact records how approval was obtained (e.g., "cli:--approve").
	ApprovalArtifact string

	// Connector is the WriteConnector to use for execution.
	Connector calendar.WriteConnector

	// CreateRequest contains the event creation parameters.
	CreateRequest calendar.CreateEventRequest
}

// ExecuteResult contains the result of executing an action.
type ExecuteResult struct {
	// Success indicates if the action completed successfully.
	Success bool

	// AuthorizationProof is the authorization proof created.
	AuthorizationProof *authority.AuthorizationProof

	// Receipt is the create event receipt (if successful).
	Receipt *calendar.CreateEventReceipt

	// SettlementStatus is the final settlement status.
	SettlementStatus SettlementStatus

	// Error contains any error that occurred.
	Error error

	// RolledBack indicates if a rollback was performed.
	RolledBack bool

	// RollbackError contains any error during rollback.
	RollbackError error

	// AuditTrail contains the IDs of audit events generated.
	AuditTrail []string
}

// SettlementStatus represents the status of settlement.
type SettlementStatus string

// Settlement statuses.
const (
	SettlementPending SettlementStatus = "pending"
	SettlementSettled SettlementStatus = "settled"
	SettlementAborted SettlementStatus = "aborted"
	SettlementRevoked SettlementStatus = "revoked"
)

// Execute runs the two-phase execution pipeline.
//
// Phase 1 (Prepare):
// - Validate request parameters
// - Authorize action with approval
// - Check revocation status
// - Emit action.pending event
//
// Phase 2 (Execute):
// - Check revocation again (final safety check)
// - Perform external write
// - Record receipt and settle
// - Rollback on failure
func (p *Pipeline) Execute(ctx context.Context, req ExecuteRequest) *ExecuteResult {
	result := &ExecuteResult{
		SettlementStatus: SettlementPending,
		AuditTrail:       make([]string, 0),
	}

	// =========================================================================
	// PHASE 1: PREPARE
	// =========================================================================

	// 1.1 Validate request
	if err := p.validateRequest(req); err != nil {
		result.Error = fmt.Errorf("validation failed: %w", err)
		return result
	}

	// 1.2 Authorize action with approval
	proof, err := p.authority.AuthorizeActionWithApproval(
		ctx,
		req.Action,
		[]string{"calendar:write"},
		primitives.ModeExecute,
		req.TraceID,
		true, // ApprovedByHuman
		req.ApprovalArtifact,
	)
	if err != nil {
		result.Error = fmt.Errorf("authorization failed: %w", err)
		return result
	}
	result.AuthorizationProof = proof

	if !proof.Authorized {
		result.Error = fmt.Errorf("authorization denied: %s", proof.DenialReason)
		p.auditAuthorizationDenied(ctx, req, proof)
		return result
	}

	// 1.3 Check revocation status
	if p.revocation != nil {
		err := p.revocation.CheckBeforeWrite(ctx, req.Action.ID, req.IntersectionID, proof.ID)
		if err != nil {
			result.Error = fmt.Errorf("revocation check failed: %w", err)
			result.SettlementStatus = SettlementRevoked
			p.auditRevocationReceived(ctx, req, err)
			return result
		}
	}

	// 1.4 Emit action.pending event
	p.auditActionPending(ctx, req, proof)

	// =========================================================================
	// PHASE 2: EXECUTE
	// =========================================================================

	// 2.1 Final revocation check (immediately before write)
	if p.revocation != nil {
		err := p.revocation.CheckBeforeWrite(ctx, req.Action.ID, req.IntersectionID, proof.ID)
		if err != nil {
			result.Error = fmt.Errorf("final revocation check failed: %w", err)
			result.SettlementStatus = SettlementRevoked
			p.auditRevocationApplied(ctx, req, err)
			return result
		}
	}

	// 2.2 Build execution envelope with approval
	env := primitives.NewExecutionEnvelopeWithApproval(
		req.TraceID,
		req.ActorCircleID,
		req.IntersectionID,
		req.ContractVersion,
		[]string{"calendar:write"},
		proof.ID,
		p.clockFunc(),
		req.ApprovalArtifact,
	)

	// 2.3 Audit write attempt
	p.auditWriteAttempt(ctx, req, proof)

	// 2.4 Perform external write - THIS IS THE CRITICAL MOMENT
	receipt, err := req.Connector.CreateEvent(ctx, *env, req.CreateRequest)
	if err != nil {
		// Write failed
		result.Error = fmt.Errorf("create event failed: %w", err)
		result.SettlementStatus = SettlementAborted
		p.auditWriteFailed(ctx, req, proof, err)
		p.auditSettlementAborted(ctx, req, proof, err)
		return result
	}

	// 2.5 Write succeeded - record receipt
	result.Receipt = receipt
	p.auditWriteSucceeded(ctx, req, proof, receipt)

	// 2.6 Final revocation check before settlement
	// (Could have been revoked during the write)
	if p.revocation != nil {
		err := p.revocation.CheckBeforeWrite(ctx, req.Action.ID, req.IntersectionID, proof.ID)
		if err != nil {
			// Revoked after write - need to rollback
			result.Error = fmt.Errorf("revoked after write: %w", err)
			result.SettlementStatus = SettlementRevoked
			p.auditRevocationApplied(ctx, req, err)

			// Attempt rollback
			p.rollback(ctx, req, env, receipt, result)
			return result
		}
	}

	// 2.7 Settlement complete
	result.Success = true
	result.SettlementStatus = SettlementSettled
	p.auditSettlementSettled(ctx, req, proof, receipt)

	return result
}

// validateRequest validates the execute request.
func (p *Pipeline) validateRequest(req ExecuteRequest) error {
	if req.TraceID == "" {
		return errors.New("trace ID is required")
	}
	if req.ActorCircleID == "" {
		return errors.New("actor circle ID is required")
	}
	if req.IntersectionID == "" {
		return errors.New("intersection ID is required")
	}
	if req.Action == nil {
		return errors.New("action is required")
	}
	if req.ApprovalArtifact == "" {
		return errors.New("approval artifact is required (e.g., 'cli:--approve')")
	}
	if req.Connector == nil {
		return errors.New("write connector is required")
	}
	if !req.Connector.SupportsWrite() {
		return errors.New("connector does not support write operations")
	}
	return nil
}

// rollback attempts to rollback a failed or revoked action.
func (p *Pipeline) rollback(ctx context.Context, req ExecuteRequest, env *primitives.ExecutionEnvelope, receipt *calendar.CreateEventReceipt, result *ExecuteResult) {
	p.auditRollbackAttempted(ctx, req, receipt)

	deleteReq := calendar.DeleteEventRequest{
		CalendarID:      receipt.CalendarID,
		ExternalEventID: receipt.ExternalEventID,
	}

	_, err := req.Connector.DeleteEvent(ctx, *env, deleteReq)
	if err != nil {
		result.RollbackError = err
		p.auditRollbackFailed(ctx, req, receipt, err)
	} else {
		result.RolledBack = true
		p.auditRollbackSucceeded(ctx, req, receipt)
	}
}

// Audit helper methods

func (p *Pipeline) auditActionPending(ctx context.Context, req ExecuteRequest, proof *authority.AuthorizationProof) {
	if p.auditStore == nil {
		return
	}
	p.auditStore.Append(ctx, auditImpl.Entry{
		Type:                 string(events.EventActionPending),
		CircleID:             req.ActorCircleID,
		IntersectionID:       req.IntersectionID,
		Action:               "create_event",
		Outcome:              "pending",
		TraceID:              req.TraceID,
		AuthorizationProofID: proof.ID,
	})
}

func (p *Pipeline) auditAuthorizationDenied(ctx context.Context, req ExecuteRequest, proof *authority.AuthorizationProof) {
	if p.auditStore == nil {
		return
	}
	p.auditStore.Append(ctx, auditImpl.Entry{
		Type:                 string(events.EventAuthorizationChecked),
		CircleID:             req.ActorCircleID,
		IntersectionID:       req.IntersectionID,
		Action:               "authorization_denied",
		Outcome:              proof.DenialReason,
		TraceID:              req.TraceID,
		AuthorizationProofID: proof.ID,
	})
}

func (p *Pipeline) auditRevocationReceived(ctx context.Context, req ExecuteRequest, err error) {
	if p.auditStore == nil {
		return
	}
	p.auditStore.Append(ctx, auditImpl.Entry{
		Type:           string(events.EventRevocationReceived),
		CircleID:       req.ActorCircleID,
		IntersectionID: req.IntersectionID,
		Action:         "revocation_check",
		Outcome:        err.Error(),
		TraceID:        req.TraceID,
	})
}

func (p *Pipeline) auditRevocationApplied(ctx context.Context, req ExecuteRequest, err error) {
	if p.auditStore == nil {
		return
	}
	p.auditStore.Append(ctx, auditImpl.Entry{
		Type:           string(events.EventRevocationApplied),
		CircleID:       req.ActorCircleID,
		IntersectionID: req.IntersectionID,
		Action:         "revocation_applied",
		Outcome:        err.Error(),
		TraceID:        req.TraceID,
	})
}

func (p *Pipeline) auditWriteAttempt(ctx context.Context, req ExecuteRequest, proof *authority.AuthorizationProof) {
	if p.auditStore == nil {
		return
	}
	p.auditStore.Append(ctx, auditImpl.Entry{
		Type:                 string(events.EventConnectorWriteAttempted),
		CircleID:             req.ActorCircleID,
		IntersectionID:       req.IntersectionID,
		Action:               "create_event",
		Outcome:              "attempting",
		TraceID:              req.TraceID,
		AuthorizationProofID: proof.ID,
	})
}

func (p *Pipeline) auditWriteSucceeded(ctx context.Context, req ExecuteRequest, proof *authority.AuthorizationProof, receipt *calendar.CreateEventReceipt) {
	if p.auditStore == nil {
		return
	}
	p.auditStore.Append(ctx, auditImpl.Entry{
		Type:                 string(events.EventConnectorWriteSucceeded),
		CircleID:             req.ActorCircleID,
		IntersectionID:       req.IntersectionID,
		Action:               "create_event",
		Outcome:              fmt.Sprintf("created: %s", calendar.RedactedExternalID(receipt.ExternalEventID)),
		TraceID:              req.TraceID,
		AuthorizationProofID: proof.ID,
	})
}

func (p *Pipeline) auditWriteFailed(ctx context.Context, req ExecuteRequest, proof *authority.AuthorizationProof, err error) {
	if p.auditStore == nil {
		return
	}
	p.auditStore.Append(ctx, auditImpl.Entry{
		Type:                 string(events.EventConnectorWriteFailed),
		CircleID:             req.ActorCircleID,
		IntersectionID:       req.IntersectionID,
		Action:               "create_event",
		Outcome:              err.Error(),
		TraceID:              req.TraceID,
		AuthorizationProofID: proof.ID,
	})
}

func (p *Pipeline) auditSettlementSettled(ctx context.Context, req ExecuteRequest, proof *authority.AuthorizationProof, receipt *calendar.CreateEventReceipt) {
	if p.auditStore == nil {
		return
	}
	p.auditStore.Append(ctx, auditImpl.Entry{
		Type:                 string(events.EventSettlementSettled),
		CircleID:             req.ActorCircleID,
		IntersectionID:       req.IntersectionID,
		Action:               "settlement",
		Outcome:              fmt.Sprintf("settled with receipt: %s", calendar.RedactedExternalID(receipt.ExternalEventID)),
		TraceID:              req.TraceID,
		AuthorizationProofID: proof.ID,
	})
}

func (p *Pipeline) auditSettlementAborted(ctx context.Context, req ExecuteRequest, proof *authority.AuthorizationProof, err error) {
	if p.auditStore == nil {
		return
	}
	p.auditStore.Append(ctx, auditImpl.Entry{
		Type:                 string(events.EventSettlementAborted),
		CircleID:             req.ActorCircleID,
		IntersectionID:       req.IntersectionID,
		Action:               "settlement",
		Outcome:              fmt.Sprintf("aborted: %s", err.Error()),
		TraceID:              req.TraceID,
		AuthorizationProofID: proof.ID,
	})
}

func (p *Pipeline) auditRollbackAttempted(ctx context.Context, req ExecuteRequest, receipt *calendar.CreateEventReceipt) {
	if p.auditStore == nil {
		return
	}
	p.auditStore.Append(ctx, auditImpl.Entry{
		Type:           string(events.EventRollbackAttempted),
		CircleID:       req.ActorCircleID,
		IntersectionID: req.IntersectionID,
		Action:         "rollback",
		Outcome:        fmt.Sprintf("attempting to delete: %s", calendar.RedactedExternalID(receipt.ExternalEventID)),
		TraceID:        req.TraceID,
	})
}

func (p *Pipeline) auditRollbackSucceeded(ctx context.Context, req ExecuteRequest, receipt *calendar.CreateEventReceipt) {
	if p.auditStore == nil {
		return
	}
	p.auditStore.Append(ctx, auditImpl.Entry{
		Type:           string(events.EventRollbackSucceeded),
		CircleID:       req.ActorCircleID,
		IntersectionID: req.IntersectionID,
		Action:         "rollback",
		Outcome:        fmt.Sprintf("deleted: %s", calendar.RedactedExternalID(receipt.ExternalEventID)),
		TraceID:        req.TraceID,
	})
}

func (p *Pipeline) auditRollbackFailed(ctx context.Context, req ExecuteRequest, receipt *calendar.CreateEventReceipt, err error) {
	if p.auditStore == nil {
		return
	}
	p.auditStore.Append(ctx, auditImpl.Entry{
		Type:           string(events.EventRollbackFailed),
		CircleID:       req.ActorCircleID,
		IntersectionID: req.IntersectionID,
		Action:         "rollback",
		Outcome:        fmt.Sprintf("failed to delete %s: %s", calendar.RedactedExternalID(receipt.ExternalEventID), err.Error()),
		TraceID:        req.TraceID,
	})
}
