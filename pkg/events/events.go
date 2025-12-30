// Package events defines event types for system observability.
// Events are used for audit logging and inter-service communication.
//
// Reference: docs/TECHNICAL_SPLIT_V1.md §3.7 Audit & Governance Layer
package events

import (
	"time"
)

// EventType identifies the kind of event.
type EventType string

// Event types for the irreducible loop.
const (
	// Intent events
	EventIntentCreated   EventType = "intent.created"
	EventIntentProcessed EventType = "intent.processed"

	// Proposal events
	EventProposalCreated     EventType = "proposal.created"
	EventProposalSubmitted   EventType = "proposal.submitted"
	EventProposalAccepted    EventType = "proposal.accepted"
	EventProposalRejected    EventType = "proposal.rejected"
	EventCounterproposalMade EventType = "proposal.counterproposal"

	// Commitment events
	EventCommitmentFormed EventType = "commitment.formed"

	// Action events
	EventActionPending   EventType = "action.pending"
	EventActionExecuting EventType = "action.executing"
	EventActionPaused    EventType = "action.paused"
	EventActionResumed   EventType = "action.resumed"
	EventActionAborted   EventType = "action.aborted"
	EventActionCompleted EventType = "action.completed"

	// Settlement events
	EventSettlementPending  EventType = "settlement.pending"
	EventSettlementComplete EventType = "settlement.complete"
	EventSettlementDisputed EventType = "settlement.disputed"
	EventSettlementResolved EventType = "settlement.resolved"

	// Authority events
	EventAuthorityGranted EventType = "authority.granted"
	EventAuthorityRevoked EventType = "authority.revoked"
	EventAuthorityExpired EventType = "authority.expired"

	// Circle events
	EventCircleCreated    EventType = "circle.created"
	EventCircleSuspended  EventType = "circle.suspended"
	EventCircleResumed    EventType = "circle.resumed"
	EventCircleTerminated EventType = "circle.terminated"

	// Intersection events
	EventIntersectionCreated   EventType = "intersection.created"
	EventIntersectionAmended   EventType = "intersection.amended"
	EventIntersectionDissolved EventType = "intersection.dissolved"

	// Invite token events
	EventInviteTokenIssued   EventType = "invite.token.issued"
	EventInviteTokenAccepted EventType = "invite.token.accepted"
	EventInviteTokenRejected EventType = "invite.token.rejected"
	EventInviteTokenExpired  EventType = "invite.token.expired"
	EventInviteTokenInvalid  EventType = "invite.token.invalid"

	// Intersection scope events
	EventIntersectionScopeUsed    EventType = "intersection.scope.used"
	EventIntersectionScopeChecked EventType = "intersection.scope.checked"
	EventIntersectionScopeDenied  EventType = "intersection.scope.denied"

	// Negotiation events
	EventNegotiationStarted   EventType = "negotiation.started"
	EventNegotiationFinalized EventType = "negotiation.finalized"
	EventNegotiationAborted   EventType = "negotiation.aborted"

	// Contract amendment events
	EventContractAmended EventType = "contract.amended"

	// Trust events
	EventTrustUpdated  EventType = "trust.updated"
	EventTrustDegraded EventType = "trust.degraded"
	EventTrustImproved EventType = "trust.improved"

	// v4 Simulation events
	EventActionCreated               EventType = "action.created"
	EventAuthorizationChecked        EventType = "authorization.checked"
	EventSimulatedExecutionCompleted EventType = "simulated.execution.completed"
	EventSettlementRecorded          EventType = "settlement.recorded"
	EventMemoryWritten               EventType = "memory.written"

	// v5 Connector events
	EventConnectorTokenMinted   EventType = "connector.token.minted"
	EventConnectorCallPerformed EventType = "connector.call.performed"
	EventConnectorReadCompleted EventType = "connector.read.completed"
	EventConnectorCallFailed    EventType = "connector.call.failed"

	// v6 Execute mode events
	EventExecutionApprovalRequired EventType = "execution.approval.required"
	EventExecutionApproved         EventType = "execution.approved"
	EventConnectorWriteAttempted   EventType = "connector.write.attempted"
	EventConnectorWriteSucceeded   EventType = "connector.write.succeeded"
	EventConnectorWriteFailed      EventType = "connector.write.failed"
	EventRollbackAttempted         EventType = "rollback.attempted"
	EventRollbackSucceeded         EventType = "rollback.succeeded"
	EventRollbackFailed            EventType = "rollback.failed"
	EventSettlementSettled         EventType = "settlement.settled"
	EventSettlementAborted         EventType = "settlement.aborted"
	EventRevocationReceived        EventType = "revocation.received"
	EventRevocationApplied         EventType = "revocation.applied"
)

// Event represents a system event for audit and observability.
type Event struct {
	// ID uniquely identifies this event.
	ID string

	// Type identifies the kind of event.
	Type EventType

	// Timestamp is when the event occurred.
	Timestamp time.Time

	// TenantID identifies the tenant (for multi-tenancy isolation).
	TenantID string

	// CircleID identifies the circle that triggered or is affected by this event.
	CircleID string

	// IntersectionID identifies the related intersection (if applicable).
	IntersectionID string

	// SubjectID identifies the primary subject (action, proposal, etc.).
	SubjectID string

	// SubjectType identifies the type of subject.
	SubjectType string

	// Metadata contains additional event-specific data.
	Metadata map[string]string

	// TraceID links this event to a distributed trace.
	TraceID string

	// AuthorizationProofID links to the authorization proof (for v4 events).
	AuthorizationProofID string

	// Provider identifies the external provider (for v5 connector events).
	// Examples: "google", "microsoft", "mock"
	Provider string

	// Operation identifies the operation performed (for v5 connector events).
	// Examples: "list_events", "find_free_slots"
	Operation string
}

// Validate checks that the event has all required fields.
func (e *Event) Validate() error {
	if e.ID == "" {
		return ErrMissingEventID
	}
	if e.Type == "" {
		return ErrMissingEventType
	}
	if e.Timestamp.IsZero() {
		return ErrMissingTimestamp
	}
	return nil
}

// Event validation errors.
var (
	ErrMissingEventID   = eventError("missing event id")
	ErrMissingEventType = eventError("missing event type")
	ErrMissingTimestamp = eventError("missing timestamp")
)

type eventError string

func (e eventError) Error() string { return string(e) }
