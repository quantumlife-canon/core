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

	// v7 Multi-party approval events
	EventApprovalRequested           EventType = "approval.requested"
	EventApprovalSubmitted           EventType = "approval.submitted"
	EventApprovalExpired             EventType = "approval.expired"
	EventApprovalVerified            EventType = "approval.verified"
	EventApprovalVerificationFailed  EventType = "approval.verification.failed"
	EventExecutionBlockedNoApprovals EventType = "execution.blocked.missing_approvals"
	EventApprovalPolicyChecked       EventType = "approval.policy.checked"

	// v8 Financial Read events
	// CRITICAL: These are READ-ONLY events. No execution events exist.
	EventFinanceReadStarted         EventType = "finance.read.started"
	EventFinanceReadCompleted       EventType = "finance.read.completed"
	EventFinanceNormalized          EventType = "finance.normalized"
	EventFinanceVisibilityFiltered  EventType = "finance.visibility.filtered"
	EventFinanceObservationCreated  EventType = "finance.observation.created"
	EventFinanceProposalGenerated   EventType = "finance.proposal.generated"
	EventFinanceProposalSuppressed  EventType = "finance.proposal.suppressed"
	EventFinanceDismissalRecorded   EventType = "finance.dismissal.recorded"
	EventFinanceSyncCompleted       EventType = "finance.sync.completed"
	EventFinanceDataStale           EventType = "finance.data.stale"
	EventFinanceDataPartial         EventType = "finance.data.partial"
	EventFinanceProviderUnavailable EventType = "finance.provider.unavailable"
	EventFinanceScopeRejected       EventType = "finance.scope.rejected"
	EventFinanceModeRejected        EventType = "finance.mode.rejected"

	// v8.4 Canonical Normalization + Reconciliation events
	// CRITICAL: Contains COUNTS ONLY. No raw amounts logged.
	EventFinanceReconciled           EventType = "finance.reconciled"
	EventFinanceDeduplicationApplied EventType = "finance.deduplication.applied"
	EventFinancePendingMerged        EventType = "finance.pending.merged"
	EventFinanceCanonicalIDComputed  EventType = "finance.canonical_id.computed"
	EventFinanceMatchKeyComputed     EventType = "finance.match_key.computed"
	EventFinanceReconcileFailed      EventType = "finance.reconcile.failed"
	EventFinanceMultiProviderMerged  EventType = "finance.multi_provider.merged"

	// v8.5 Edge Case Handling events
	// CRITICAL: Contains COUNTS ONLY. No raw amounts logged.

	// Adjustment classification events
	EventFinanceAdjustmentClassified EventType = "finance.adjustment.classified"
	EventFinanceRefundDetected       EventType = "finance.refund.detected"
	EventFinanceReversalDetected     EventType = "finance.reversal.detected"
	EventFinanceChargebackDetected   EventType = "finance.chargeback.detected"
	EventFinanceRelatedMatched       EventType = "finance.related.matched"
	EventFinanceRelatedAmbiguous     EventType = "finance.related.ambiguous"

	// Partial capture events
	EventFinancePartialCaptureDetected EventType = "finance.partial_capture.detected"

	// Multi-currency events
	EventFinanceMultiCurrencyWarning EventType = "finance.multi_currency.warning"
	EventFinanceCurrencyAggregated   EventType = "finance.currency.aggregated"

	// Merchant normalization events
	EventFinanceMerchantNormalized EventType = "finance.merchant.normalized"
	EventFinanceMerchantAliasUsed  EventType = "finance.merchant.alias_used"

	// Pagination events
	EventFinancePaginationStarted     EventType = "finance.pagination.started"
	EventFinancePaginationCompleted   EventType = "finance.pagination.completed"
	EventFinancePaginationCursorStale EventType = "finance.pagination.cursor_stale"

	// v8.6 Family Financial Intersections events
	// CRITICAL: READ + PROPOSE ONLY. No execution events.

	// Shared view lifecycle events
	EventSharedViewRequested     EventType = "sharedview.requested"
	EventSharedViewBuilt         EventType = "sharedview.built"
	EventSharedViewDelivered     EventType = "sharedview.delivered"
	EventSharedViewExpired       EventType = "sharedview.expired"
	EventSharedViewRefreshed     EventType = "sharedview.refreshed"
	EventSharedViewPolicyApplied EventType = "sharedview.policy.applied"

	// Visibility filtering events
	EventSharedViewFiltered           EventType = "sharedview.filtered"
	EventSharedViewCategoryExcluded   EventType = "sharedview.category.excluded"
	EventSharedViewAccountExcluded    EventType = "sharedview.account.excluded"
	EventSharedViewAmountAnonymized   EventType = "sharedview.amount.anonymized"
	EventSharedViewMerchantAnonymized EventType = "sharedview.merchant.anonymized"

	// Symmetry verification events
	EventSymmetryVerified     EventType = "symmetry.verified"
	EventSymmetryViolation    EventType = "symmetry.violation"
	EventSymmetryProofCreated EventType = "symmetry.proof.created"

	// Multi-party aggregation events
	EventAggregationStarted   EventType = "aggregation.started"
	EventAggregationCompleted EventType = "aggregation.completed"
	EventContributionReceived EventType = "contribution.received"
	EventContributionMissing  EventType = "contribution.missing"

	// Proposal lifecycle events (v8.6)
	EventSharedProposalGenerated  EventType = "sharedproposal.generated"
	EventSharedProposalDelivered  EventType = "sharedproposal.delivered"
	EventSharedProposalDismissed  EventType = "sharedproposal.dismissed"
	EventSharedProposalExpired    EventType = "sharedproposal.expired"
	EventSharedProposalSuppressed EventType = "sharedproposal.suppressed"

	// Language neutrality events
	EventLanguageChecked   EventType = "language.checked"
	EventLanguageViolation EventType = "language.violation"
	EventLanguageApproved  EventType = "language.approved"

	// Intersection financial policy events
	EventFinancialPolicyEnabled  EventType = "financial.policy.enabled"
	EventFinancialPolicyDisabled EventType = "financial.policy.disabled"
	EventFinancialPolicyUpdated  EventType = "financial.policy.updated"
	EventSymmetryRequirementSet  EventType = "symmetry.requirement.set"

	// v9 Financial Execution events
	// CRITICAL: v9 Slice 1 is DRY-RUN ONLY. No real money moves.
	//
	// Per TECHNICAL_SPLIT_V9_EXECUTION.md §8.1, these events are MANDATORY.
	// Per ACCEPTANCE_TESTS_V9_EXECUTION.md, all events must be auditable.

	// Intent lifecycle events
	EventExecutionIntentCreated EventType = "execution.intent.created"
	EventExecutionIntentExpired EventType = "execution.intent.expired"

	// Envelope lifecycle events
	EventExecutionEnvelopeBuilt   EventType = "execution.envelope.built"
	EventExecutionEnvelopeSealed  EventType = "execution.envelope.sealed"
	EventExecutionEnvelopeExpired EventType = "execution.envelope.expired"

	// Approval lifecycle events (v9 specific - per-action only)
	EventV9ApprovalRequested         EventType = "v9.approval.requested"
	EventV9ApprovalSubmitted         EventType = "v9.approval.submitted"
	EventV9ApprovalVerified          EventType = "v9.approval.verified"
	EventV9ApprovalExpired           EventType = "v9.approval.expired"
	EventV9ApprovalRejected          EventType = "v9.approval.rejected"
	EventV9ApprovalLanguageChecked   EventType = "v9.approval.language.checked"
	EventV9ApprovalLanguageViolation EventType = "v9.approval.language.violation"

	// Revocation lifecycle events
	EventV9RevocationWindowOpened EventType = "v9.revocation.window.opened"
	EventV9RevocationWindowClosed EventType = "v9.revocation.window.closed"
	EventV9RevocationTriggered    EventType = "v9.revocation.triggered"
	EventV9RevocationApplied      EventType = "v9.revocation.applied"

	// Validity check events
	EventV9ValidityChecked     EventType = "v9.validity.checked"
	EventV9ValidityCheckPassed EventType = "v9.validity.check.passed"
	EventV9ValidityCheckFailed EventType = "v9.validity.check.failed"

	// Execution lifecycle events
	EventV9ExecutionStarted   EventType = "v9.execution.started"
	EventV9ExecutionBlocked   EventType = "v9.execution.blocked"
	EventV9ExecutionAborted   EventType = "v9.execution.aborted"
	EventV9ExecutionCompleted EventType = "v9.execution.completed"
	EventV9ExecutionRevoked   EventType = "v9.execution.revoked"

	// Settlement events (v9 - always non-success in Slice 1)
	EventV9SettlementRecorded EventType = "v9.settlement.recorded"
	EventV9SettlementPending  EventType = "v9.settlement.pending"
	EventV9SettlementBlocked  EventType = "v9.settlement.blocked"
	EventV9SettlementRevoked  EventType = "v9.settlement.revoked"
	EventV9SettlementExpired  EventType = "v9.settlement.expired"
	EventV9SettlementAborted  EventType = "v9.settlement.aborted"

	// v9 Slice 2: Adapter events
	// CRITICAL: These events prove execution was attempted but BLOCKED.
	// No money moves in v9 Slice 2.
	EventV9AdapterPrepared EventType = "v9.adapter.prepared"
	EventV9AdapterInvoked  EventType = "v9.adapter.invoked"
	EventV9AdapterBlocked  EventType = "v9.adapter.blocked"

	// Audit finalization events
	EventV9AuditTraceFinalized EventType = "v9.audit.trace.finalized"

	// v9 Slice 3: Real Payment events
	// CRITICAL: These events record REAL money movement.
	// v9 Slice 3 is the FIRST slice where money may actually move.
	//
	// HARD CONSTRAINTS:
	// - TrueLayer ONLY
	// - Cap: £1.00 (100 pence) default
	// - Pre-defined payees only
	// - Explicit per-action approval
	// - Full audit trail

	// Payment preparation events
	EventV9PaymentPrepared EventType = "v9.payment.prepared"

	// Payment execution events
	EventV9PaymentCreated   EventType = "v9.payment.created"
	EventV9PaymentPending   EventType = "v9.payment.pending"
	EventV9PaymentSucceeded EventType = "v9.payment.succeeded"
	EventV9PaymentFailed    EventType = "v9.payment.failed"

	// Settlement events (v9.3 - real settlement)
	EventV9SettlementSucceeded EventType = "v9.settlement.succeeded"

	// Forced pause event
	EventV9ForcedPauseStarted   EventType = "v9.forced_pause.started"
	EventV9ForcedPauseCompleted EventType = "v9.forced_pause.completed"

	// Cap enforcement events
	EventV9CapChecked  EventType = "v9.cap.checked"
	EventV9CapExceeded EventType = "v9.cap.exceeded"

	// Payee validation events
	EventV9PayeeValidated EventType = "v9.payee.validated"
	EventV9PayeeRejected  EventType = "v9.payee.rejected"

	// Provider configuration events
	EventV9ProviderConfigured    EventType = "v9.provider.configured"
	EventV9ProviderNotConfigured EventType = "v9.provider.not_configured"

	// Simulated execution events
	// CRITICAL: These events indicate NO real money moved.
	// Used when TrueLayer is not configured and mock connector is active.
	EventV9PaymentSimulated    EventType = "v9.payment.simulated"
	EventV9SettlementSimulated EventType = "v9.settlement.simulated"

	// v9.4 Multi-party Financial Execution events
	// CRITICAL: Multi-party approvals for shared money via intersections.
	// Uses threshold approvals and symmetry guarantees while keeping v9.3 constraints.
	//
	// NON-NEGOTIABLE:
	// - No blanket/standing approvals
	// - Neutral approval language (no urgency/fear/shame/authority)
	// - Symmetry: every approver receives IDENTICAL approval payload
	// - Approvals do NOT bypass revocation windows

	// Approval bundle lifecycle events
	EventV94ApprovalBundleCreated   EventType = "v9.approval.bundle.created"
	EventV94ApprovalBundlePresented EventType = "v9.approval.bundle.presented"

	// Symmetry verification events
	EventV94ApprovalSymmetryVerified EventType = "v9.approval.symmetry.verified"
	EventV94ApprovalSymmetryFailed   EventType = "v9.approval.symmetry.failed"

	// Threshold approval events
	EventV94ApprovalThresholdChecked EventType = "v9.approval.threshold.checked"
	EventV94ApprovalThresholdMet     EventType = "v9.approval.threshold.met"
	EventV94ApprovalThresholdNotMet  EventType = "v9.approval.threshold.not_met"

	// Multi-party execution gating events
	EventV94MultiPartyRequired       EventType = "v9.multiparty.required"
	EventV94MultiPartyGatePassed     EventType = "v9.multiparty.gate.passed"
	EventV94MultiPartyGateBlocked    EventType = "v9.multiparty.gate.blocked"
	EventV94MultiPartySingleFallback EventType = "v9.multiparty.single.fallback"

	// Execution blocking events for multi-party
	EventV94ExecutionBlockedInsufficientApprovals EventType = "v9.execution.blocked.insufficient_approvals"
	EventV94ExecutionBlockedAsymmetricPayload     EventType = "v9.execution.blocked.asymmetric_payload"
	EventV94ExecutionBlockedNeutralityViolation   EventType = "v9.execution.blocked.neutrality_violation"

	// Approval reuse prevention events
	EventV94ApprovalReuseAttempted EventType = "v9.approval.reuse.attempted"
	EventV94ApprovalReuseBlocked   EventType = "v9.approval.reuse.blocked"
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
