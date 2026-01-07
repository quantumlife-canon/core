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

	// v9.5 Real Multi-Party Sandbox Execution events
	// CRITICAL: v9.5 enables real TrueLayer sandbox execution with strengthened presentation semantics.
	// Provider: TrueLayer ONLY (sandbox enforced), Cap: £1.00, Multi-party threshold required.
	//
	// NON-NEGOTIABLE:
	// - Bundle MUST be presented before approval can be submitted
	// - Each approver MUST receive identical bundle (proven via ContentHash)
	// - Revocation during forced pause MUST abort BEFORE provider call
	// - No retries without new approval artifacts

	// Presentation lifecycle events
	EventV95ApprovalPresentationRecorded EventType = "v9.approval.presentation.recorded"
	EventV95ApprovalPresentationMissing  EventType = "v9.approval.presentation.missing"
	EventV95ApprovalPresentationExpired  EventType = "v9.approval.presentation.expired"
	EventV95ApprovalPresentationVerified EventType = "v9.approval.presentation.verified"

	// Provider selection events
	EventV95ExecutionProviderSelected  EventType = "v9.execution.provider.selected"
	EventV95ExecutionProviderMock      EventType = "v9.execution.provider.mock"
	EventV95ExecutionProviderTrueLayer EventType = "v9.execution.provider.truelayer"

	// TrueLayer sandbox execution events
	EventV95PaymentTrueLayerCreated   EventType = "v9.payment.truelayer.created"
	EventV95PaymentTrueLayerSucceeded EventType = "v9.payment.truelayer.succeeded"
	EventV95PaymentTrueLayerFailed    EventType = "v9.payment.truelayer.failed"
	EventV95PaymentTrueLayerPending   EventType = "v9.payment.truelayer.pending"

	// Revocation during forced pause events
	EventV95RevocationDuringPause          EventType = "v9.revocation.during_pause"
	EventV95ExecutionAbortedRevocation     EventType = "v9.execution.aborted.revocation_during_pause"
	EventV95ExecutionAbortedBeforeProvider EventType = "v9.execution.aborted.before_provider"

	// Sandbox enforcement events
	EventV95SandboxEnforced  EventType = "v9.sandbox.enforced"
	EventV95SandboxViolation EventType = "v9.sandbox.violation"

	// Attempt tracking events
	EventV95AttemptStarted   EventType = "v9.attempt.started"
	EventV95AttemptFinalized EventType = "v9.attempt.finalized"

	// v9.6 Idempotency + Replay Defense events
	// CRITICAL: v9.6 prevents duplicate payments and replays via:
	// - Deterministic idempotency keys derived from envelope + action hash + attempt ID
	// - Attempt ledger enforcing exactly-once semantics
	// - Provider idempotency key propagation (when supported)
	//
	// NON-NEGOTIABLE:
	// - Each (envelope_id, attempt_id) pair is unique
	// - Terminal attempts (settled/aborted/blocked/revoked/expired/simulated) cannot be retried
	// - One in-flight attempt per envelope at any time
	// - Idempotency keys are NOT logged in full (prefix only for privacy)

	// Idempotency key lifecycle events
	EventV96IdempotencyKeyDerived EventType = "v9.idempotency.key.derived"

	// Attempt lifecycle events
	EventV96AttemptStarted         EventType = "v9.execution.attempt.started"
	EventV96AttemptReplayBlocked   EventType = "v9.execution.attempt.replay_blocked"
	EventV96AttemptInflightBlocked EventType = "v9.execution.attempt.inflight_blocked"
	EventV96AttemptRecorded        EventType = "v9.execution.attempt.recorded"
	EventV96AttemptFinalized       EventType = "v9.execution.attempt.finalized"

	// Provider idempotency events
	EventV96ProviderIdempotencyAttached EventType = "v9.provider.idempotency.attached"

	// Ledger events
	EventV96LedgerEntryCreated   EventType = "v9.ledger.entry.created"
	EventV96LedgerEntryUpdated   EventType = "v9.ledger.entry.updated"
	EventV96LedgerDuplicateFound EventType = "v9.ledger.duplicate.found"

	// v9.9 Provider Registry Lock + Write Allowlist Enforcement events
	// CRITICAL: v9.9 prevents unapproved/unregistered write providers from being used.
	// Executors MUST consult the registry before invoking any WriteConnector.
	//
	// NON-NEGOTIABLE:
	// - Only allowlisted providers may execute financial writes
	// - Live/production providers are blocked by default
	// - Registry violations produce blocking events with audit trail

	// Provider registry check events
	EventV99ProviderRegistryChecked  EventType = "v9.provider.registry.checked"
	EventV99ProviderAllowed          EventType = "v9.provider.allowed"
	EventV99ProviderBlocked          EventType = "v9.provider.blocked"
	EventV99ProviderNotRegistered    EventType = "v9.provider.not_registered"
	EventV99ProviderLiveBlocked      EventType = "v9.provider.live_blocked"
	EventV99ProviderAllowlistChecked EventType = "v9.provider.allowlist.checked"

	// v9.10 Payee Registry Lock + Free-Text Recipient Elimination events
	// CRITICAL: v9.10 prevents free-text recipients from being used in execution.
	// ALL executions MUST reference a registered PayeeID.
	//
	// NON-NEGOTIABLE:
	// - No free-text recipients in any write execution path
	// - No runtime-supplied payment destinations
	// - Payee must be registered and allowed for the provider being used
	// - Live payees are blocked by default
	// - Registry violations produce blocking events with audit trail

	// Payee registry check events
	EventV910PayeeRegistryChecked  EventType = "v9.payee.registry.checked"
	EventV910PayeeAllowed          EventType = "v9.payee.allowed"
	EventV910PayeeNotRegistered    EventType = "v9.payee.not_registered"
	EventV910PayeeNotAllowed       EventType = "v9.payee.not_allowed"
	EventV910PayeeLiveBlocked      EventType = "v9.payee.live_blocked"
	EventV910PayeeProviderMismatch EventType = "v9.payee.provider_mismatch"

	// Execution blocking events for payee validation
	EventV910ExecutionBlockedInvalidPayee EventType = "v9.execution.blocked.invalid_payee"

	// v9.11 Daily Caps + Rate-Limited Execution Ledger events
	// CRITICAL: v9.11 enforces daily caps and rate limits on financial execution.
	// Prevents "slow drain" attacks and burst execution patterns.
	//
	// NON-NEGOTIABLE:
	// - Per-circle, per-intersection, per-payee daily caps (by currency)
	// - Maximum attempts per day limits
	// - Caps are hard blocks with no partial execution
	// - All enforcement before provider Prepare/Execute
	// - Neutral language in all blocking reasons

	// Caps policy and check events
	EventV911CapsPolicyApplied EventType = "v9.caps.policy.applied"
	EventV911CapsChecked       EventType = "v9.caps.checked"
	EventV911CapsBlocked       EventType = "v9.caps.blocked"

	// Caps tracking events
	EventV911CapsAttemptCounted EventType = "v9.caps.attempt.counted"
	EventV911CapsSpendCounted   EventType = "v9.caps.spend.counted"

	// Caps enforcement blocking events
	EventV911ExecutionBlockedDailyCap     EventType = "v9.execution.blocked.daily_cap"
	EventV911ExecutionBlockedAttemptLimit EventType = "v9.execution.blocked.attempt_limit"

	// v9.11.1 Rate-limit specific events
	// CRITICAL: These events provide granular audit trail for rate-limit checks.
	// Every rate-limit check emits either "checked" (passed) or "blocked" event.
	EventV911RateLimitChecked EventType = "v9.ratelimit.checked"
	EventV911RateLimitBlocked EventType = "v9.ratelimit.blocked"

	// v9.12 Policy Snapshot Hash Binding events
	// CRITICAL: These events audit policy snapshot computation, binding, and verification.
	// Policy drift between approval and execution is prevented by hash verification.

	// EventV912PolicySnapshotComputed is emitted when a policy snapshot hash is computed.
	EventV912PolicySnapshotComputed EventType = "v9.policy.snapshot.computed"

	// EventV912PolicySnapshotBound is emitted when policy snapshot is bound to envelope/bundle.
	EventV912PolicySnapshotBound EventType = "v9.policy.snapshot.bound"

	// EventV912PolicySnapshotVerified is emitted when execution-time verification passes.
	EventV912PolicySnapshotVerified EventType = "v9.policy.snapshot.verified"

	// EventV912PolicySnapshotMismatch is emitted when execution-time verification fails.
	// CRITICAL: This indicates policy drift - execution MUST be blocked.
	EventV912PolicySnapshotMismatch EventType = "v9.policy.snapshot.mismatch"

	// EventV912ExecutionBlockedPolicyDrift is emitted when execution is blocked due to policy drift.
	EventV912ExecutionBlockedPolicyDrift EventType = "v9.execution.blocked.policy_drift"

	// v9.12.1 Policy Snapshot Hash Hardening events
	// CRITICAL: These events enforce that PolicySnapshotHash is REQUIRED, not optional.
	// Empty hash is a hard block to prevent legacy envelopes from executing.

	// EventV912PolicySnapshotMissing is emitted when envelope lacks PolicySnapshotHash.
	// CRITICAL: Execution MUST be blocked when this event is emitted.
	EventV912PolicySnapshotMissing EventType = "v9.policy.snapshot.missing"

	// EventV912ExecutionBlockedMissingHash is emitted when execution is blocked due to missing hash.
	EventV912ExecutionBlockedMissingHash EventType = "v9.execution.blocked.missing_hash"

	// v9.13 View Freshness Binding events
	// CRITICAL: These events track view snapshot verification for read-before-write enforcement.
	// View must be fresh and hash must match for execution to proceed.

	// EventV913ViewSnapshotRequested is emitted when a view snapshot is requested.
	EventV913ViewSnapshotRequested EventType = "v9.view.snapshot.requested"

	// EventV913ViewSnapshotReceived is emitted when a view snapshot is received.
	EventV913ViewSnapshotReceived EventType = "v9.view.snapshot.received"

	// EventV913ViewFreshnessChecked is emitted when view freshness is checked.
	EventV913ViewFreshnessChecked EventType = "v9.view.freshness.checked"

	// EventV913ViewHashVerified is emitted when view hash is verified against envelope.
	EventV913ViewHashVerified EventType = "v9.view.hash.verified"

	// EventV913ViewHashMismatch is emitted when view hash doesn't match envelope.
	// CRITICAL: Execution MUST be blocked when this event is emitted.
	EventV913ViewHashMismatch EventType = "v9.view.hash.mismatch"

	// EventV913ExecutionBlockedViewStale is emitted when execution is blocked due to stale view.
	EventV913ExecutionBlockedViewStale EventType = "v9.execution.blocked.view_stale"

	// EventV913ExecutionBlockedViewHashMismatch is emitted when execution is blocked due to view hash mismatch.
	EventV913ExecutionBlockedViewHashMismatch EventType = "v9.execution.blocked.view_hash_mismatch"

	// EventV913ExecutionBlockedViewHashMissing is emitted when execution is blocked due to missing view hash.
	EventV913ExecutionBlockedViewHashMissing EventType = "v9.execution.blocked.view_hash_missing"

	// EventV913ViewSnapshotBound is emitted when view snapshot is bound to envelope at creation.
	EventV913ViewSnapshotBound EventType = "v9.view.snapshot.bound"

	// Phase 4: Drafts-Only Assistance events
	// CRITICAL: Drafts are INTERNAL ONLY. NO external writes from draft events.
	// CRITICAL: Drafts require explicit user approval before any execution.
	// CRITICAL: These events provide audit trail for the propose → review → (approve|reject) cycle.
	//
	// Reference: docs/ADR/ADR-0021-phase4-drafts-only-assistance.md

	// Draft lifecycle events
	EventDraftGenerated EventType = "draft.generated"
	EventDraftStored    EventType = "draft.stored"
	EventDraftDedupe    EventType = "draft.dedupe"
	EventDraftExpired   EventType = "draft.expired"

	// Draft review events
	EventDraftReviewStarted   EventType = "draft.review.started"
	EventDraftReviewCompleted EventType = "draft.review.completed"

	// Draft approval events
	EventDraftApproved       EventType = "draft.approved"
	EventDraftRejected       EventType = "draft.rejected"
	EventDraftApprovalFailed EventType = "draft.approval.failed"

	// Draft safety events
	EventDraftSafetyCheckPassed EventType = "draft.safety.check.passed"
	EventDraftSafetyCheckFailed EventType = "draft.safety.check.failed"
	EventDraftSafetyWarning     EventType = "draft.safety.warning"

	// Draft rate limit events
	EventDraftRateLimitChecked EventType = "draft.ratelimit.checked"
	EventDraftRateLimitBlocked EventType = "draft.ratelimit.blocked"

	// Draft generation rule events
	EventDraftRuleMatched   EventType = "draft.rule.matched"
	EventDraftRuleSkipped   EventType = "draft.rule.skipped"
	EventDraftNoRuleMatched EventType = "draft.no_rule.matched"

	// Phase 5: Calendar Execution Boundary events
	// CRITICAL: This is the FIRST real external write in QuantumLife.
	// CRITICAL: Execution ONLY from approved drafts.
	// CRITICAL: No auto-retries. No background execution.
	// CRITICAL: Must be idempotent - same envelope executed twice returns same result.
	//
	// Reference: docs/ADR/ADR-0022-phase5-calendar-execution-boundary.md

	// Envelope lifecycle events
	Phase5CalendarEnvelopeCreated    EventType = "phase5.calendar.envelope.created"
	Phase5CalendarEnvelopeValidated  EventType = "phase5.calendar.envelope.validated"
	Phase5CalendarEnvelopeStoreError EventType = "phase5.calendar.envelope.store_error"

	// Policy snapshot events
	Phase5CalendarPolicySnapshotTaken    EventType = "phase5.calendar.policy.snapshot.taken"
	Phase5CalendarPolicySnapshotVerified EventType = "phase5.calendar.policy.snapshot.verified"
	Phase5CalendarPolicySnapshotMismatch EventType = "phase5.calendar.policy.snapshot.mismatch"

	// View snapshot events
	Phase5CalendarViewSnapshotTaken   EventType = "phase5.calendar.view.snapshot.taken"
	Phase5CalendarViewSnapshotFresh   EventType = "phase5.calendar.view.snapshot.fresh"
	Phase5CalendarViewSnapshotStale   EventType = "phase5.calendar.view.snapshot.stale"
	Phase5CalendarViewSnapshotChanged EventType = "phase5.calendar.view.snapshot.changed"

	// Execution lifecycle events
	Phase5CalendarExecutionStarted    EventType = "phase5.calendar.execution.started"
	Phase5CalendarExecutionSuccess    EventType = "phase5.calendar.execution.success"
	Phase5CalendarExecutionFailed     EventType = "phase5.calendar.execution.failed"
	Phase5CalendarExecutionBlocked    EventType = "phase5.calendar.execution.blocked"
	Phase5CalendarExecutionIdempotent EventType = "phase5.calendar.execution.idempotent"

	// Provider events
	Phase5CalendarProviderCalled    EventType = "phase5.calendar.provider.called"
	Phase5CalendarProviderSucceeded EventType = "phase5.calendar.provider.succeeded"
	Phase5CalendarProviderFailed    EventType = "phase5.calendar.provider.failed"

	// Safety events
	Phase5CalendarSandboxEnforced EventType = "phase5.calendar.sandbox.enforced"
	Phase5CalendarDryRunEnforced  EventType = "phase5.calendar.dryrun.enforced"

	// Phase 6: The Quiet Loop events
	// CRITICAL: Web-first daily loop with explicit trigger.
	// CRITICAL: All loop execution is synchronous per request.
	// CRITICAL: Feedback signals captured for future improvement.
	//
	// Reference: docs/ADR/ADR-0023-phase6-quiet-loop-web.md

	// Daily loop lifecycle events
	Phase6DailyRunStarted   EventType = "phase6.daily.run.started"
	Phase6DailyRunCompleted EventType = "phase6.daily.run.completed"

	// View computation events
	Phase6ViewComputed     EventType = "phase6.view.computed"
	Phase6NeedsYouComputed EventType = "phase6.needs_you.computed"

	// Feedback events
	Phase6FeedbackRecorded EventType = "phase6.feedback.recorded"

	// Web request events (minimal)
	Phase6WebRequestServed EventType = "phase6.web.request.served"

	// Phase 7: Email Execution Boundary events
	// CRITICAL: This is the ONLY path to external email writes.
	// CRITICAL: Execution ONLY from approved drafts.
	// CRITICAL: No auto-retries. No background execution.
	// CRITICAL: Must be idempotent - same envelope executed twice returns same result.
	// CRITICAL: Reply-only - no new thread creation.
	//
	// Reference: Phase 7 Email Execution Boundary

	// Envelope lifecycle events
	EmailEnvelopeCreated   EventType = "email.envelope.created"
	EmailEnvelopeValidated EventType = "email.envelope.validated"

	// Execution lifecycle events
	EmailExecutionAttempted  EventType = "email.execution.attempted"
	EmailExecutionSucceeded  EventType = "email.execution.succeeded"
	EmailExecutionFailed     EventType = "email.execution.failed"
	EmailExecutionBlocked    EventType = "email.execution.blocked"
	EmailExecutionIdempotent EventType = "email.execution.idempotent"
	EmailExecutionStoreError EventType = "email.execution.store_error"

	// Policy snapshot events
	EmailPolicySnapshotTaken    EventType = "email.policy.snapshot.taken"
	EmailPolicySnapshotVerified EventType = "email.policy.snapshot.verified"
	EmailPolicySnapshotMismatch EventType = "email.policy.snapshot.mismatch"

	// View snapshot events
	EmailViewSnapshotTaken   EventType = "email.view.snapshot.taken"
	EmailViewSnapshotFresh   EventType = "email.view.snapshot.fresh"
	EmailViewSnapshotStale   EventType = "email.view.snapshot.stale"
	EmailViewSnapshotChanged EventType = "email.view.snapshot.changed"

	// Provider events
	EmailProviderCalled    EventType = "email.provider.called"
	EmailProviderSucceeded EventType = "email.provider.succeeded"
	EmailProviderFailed    EventType = "email.provider.failed"

	// Safety events
	EmailSandboxEnforced EventType = "email.sandbox.enforced"
	EmailDryRunEnforced  EventType = "email.dryrun.enforced"

	// Phase 10: Approved Draft → Execution Routing events
	// CRITICAL: Execution ONLY via boundary executors (Phase 5 calendar, Phase 7 email).
	// CRITICAL: No background execution. Execution occurs during user HTTP request.
	// CRITICAL: Execution is idempotent and blocked if hashes missing/mismatch.
	//
	// Reference: Phase 10 - Approved Draft → Execution Routing

	// Intent lifecycle events
	Phase10IntentBuilt           EventType = "phase10.intent.built"
	Phase10IntentValidated       EventType = "phase10.intent.validated"
	Phase10IntentValidationError EventType = "phase10.intent.validation_error"

	// Execution routing events
	Phase10ExecutionRequested EventType = "phase10.intent.execution.requested"
	Phase10ExecutionRouted    EventType = "phase10.intent.execution.routed"
	Phase10ExecutionSucceeded EventType = "phase10.intent.execution.succeeded"
	Phase10ExecutionBlocked   EventType = "phase10.intent.execution.blocked"
	Phase10ExecutionFailed    EventType = "phase10.intent.execution.failed"

	// Draft execution events (web layer)
	Phase10DraftExecuteRequested EventType = "phase10.draft.execute.requested"
	Phase10DraftExecuteCompleted EventType = "phase10.draft.execute.completed"
	Phase10DraftExecuteBlocked   EventType = "phase10.draft.execute.blocked"
	Phase10DraftExecuteFailed    EventType = "phase10.draft.execute.failed"

	// Hash binding events
	Phase10HashBindingMissing          EventType = "phase10.hash.binding.missing"
	Phase10HashBindingVerified         EventType = "phase10.hash.binding.verified"
	Phase10PolicyHashMissing           EventType = "phase10.policy_hash.missing"
	Phase10ViewHashMissing             EventType = "phase10.view_hash.missing"
	Phase10ExecutionBlockedNoHash      EventType = "phase10.execution.blocked.no_hash"
	Phase10ExecutionBlockedNotApproved EventType = "phase10.execution.blocked.not_approved"

	// Phase 11: Multi-Circle Real Loop events
	Phase11MultiCircleRunStarted   EventType = "phase11.multicircle.run.started"
	Phase11MultiCircleRunCompleted EventType = "phase11.multicircle.run.completed"
	Phase11IngestionStarted        EventType = "phase11.ingestion.started"
	Phase11IngestionCompleted      EventType = "phase11.ingestion.completed"
	Phase11CircleSynced            EventType = "phase11.circle.synced"
	Phase11ConfigLoaded            EventType = "phase11.config.loaded"
	Phase11ConfigError             EventType = "phase11.config.error"
	Phase11AdapterRegistered       EventType = "phase11.adapter.registered"
	Phase11AdapterMissing          EventType = "phase11.adapter.missing"

	// Phase 14: Circle Policies + Preference Learning events
	// CRITICAL: All policy changes are persisted to storelog.
	// CRITICAL: All learning is deterministic and auditable.
	// CRITICAL: No background learning. All feedback processing is synchronous.
	//
	// Reference: docs/ADR/ADR-0030-phase14-policy-learning.md

	// Policy lifecycle events
	Phase14PolicyUpdated       EventType = "phase14.policy.updated"
	Phase14PolicyCircleUpdated EventType = "phase14.policy.circle.updated"
	Phase14PolicyTriggerAdded  EventType = "phase14.policy.trigger.added"
	Phase14PolicyLoaded        EventType = "phase14.policy.loaded"
	Phase14PolicyValidated     EventType = "phase14.policy.validated"

	// Suppression lifecycle events
	Phase14SuppressRuleAdded   EventType = "phase14.suppress.rule.added"
	Phase14SuppressRuleRemoved EventType = "phase14.suppress.rule.removed"
	Phase14SuppressPruned      EventType = "phase14.suppress.pruned"
	Phase14SuppressRuleMatched EventType = "phase14.suppress.rule.matched"
	Phase14SuppressLoaded      EventType = "phase14.suppress.loaded"

	// Explainability events
	Phase14ExplainComputed  EventType = "phase14.explain.computed"
	Phase14ExplainRequested EventType = "phase14.explain.requested"
	Phase14ExplainDelivered EventType = "phase14.explain.delivered"

	// Interruption suppression events
	Phase14InterruptionSuppressed EventType = "phase14.interruption.suppressed"
	Phase14InterruptionAllowed    EventType = "phase14.interruption.allowed"

	// Preference learning events
	Phase14LearningApplied          EventType = "phase14.learning.applied"
	Phase14LearningDecisionRecorded EventType = "phase14.learning.decision.recorded"
	Phase14LearningNoChange         EventType = "phase14.learning.no_change"
	Phase14FeedbackProcessed        EventType = "phase14.feedback.processed"

	// Threshold adjustment events
	Phase14ThresholdIncreased  EventType = "phase14.threshold.increased"
	Phase14ThresholdDecreased  EventType = "phase14.threshold.decreased"
	Phase14TriggerBiasAdjusted EventType = "phase14.trigger.bias.adjusted"

	// Quota events
	Phase14QuotaChecked    EventType = "phase14.quota.checked"
	Phase14QuotaExceeded   EventType = "phase14.quota.exceeded"
	Phase14QuotaDowngraded EventType = "phase14.quota.downgraded"

	// Hours policy events
	Phase14HoursChecked EventType = "phase14.hours.checked"
	Phase14HoursBlocked EventType = "phase14.hours.blocked"
	Phase14HoursAllowed EventType = "phase14.hours.allowed"

	// Phase 15: Household Approvals + Intersections (Deterministic, Web-first)
	// CRITICAL: These events track multi-party approval lifecycle for household intersections.
	// Approvals happen via web UI (mobile browser friendly) using signed tokens.
	//
	// Reference: docs/ADR/ADR-0031-phase15-household-approvals.md

	// Intersection policy lifecycle events
	Phase15IntersectionPolicyCreated EventType = "phase15.intersection.policy.created"
	Phase15IntersectionPolicyUpdated EventType = "phase15.intersection.policy.updated"
	Phase15IntersectionPolicyLoaded  EventType = "phase15.intersection.policy.loaded"
	Phase15IntersectionMemberAdded   EventType = "phase15.intersection.member.added"
	Phase15IntersectionMemberRemoved EventType = "phase15.intersection.member.removed"

	// Approval state lifecycle events
	Phase15ApprovalStateCreated   EventType = "phase15.approval.state.created"
	Phase15ApprovalStateUpdated   EventType = "phase15.approval.state.updated"
	Phase15ApprovalStateExpired   EventType = "phase15.approval.state.expired"
	Phase15ApprovalStateCompleted EventType = "phase15.approval.state.completed"
	Phase15ApprovalStateRejected  EventType = "phase15.approval.state.rejected"

	// Approval record events
	Phase15ApprovalRecorded     EventType = "phase15.approval.recorded"
	Phase15ApprovalApproved     EventType = "phase15.approval.approved"
	Phase15ApprovalRejected     EventType = "phase15.approval.rejected"
	Phase15ApprovalFreshnessOK  EventType = "phase15.approval.freshness.ok"
	Phase15ApprovalStale        EventType = "phase15.approval.stale"
	Phase15ApprovalThresholdMet EventType = "phase15.approval.threshold.met"

	// Approval token lifecycle events
	Phase15TokenCreated   EventType = "phase15.token.created"
	Phase15TokenSigned    EventType = "phase15.token.signed"
	Phase15TokenVerified  EventType = "phase15.token.verified"
	Phase15TokenExpired   EventType = "phase15.token.expired"
	Phase15TokenRevoked   EventType = "phase15.token.revoked"
	Phase15TokenInvalid   EventType = "phase15.token.invalid"
	Phase15TokenUsed      EventType = "phase15.token.used"
	Phase15TokenDuplicate EventType = "phase15.token.duplicate"

	// Execution gating events
	Phase15ExecutionBlockedApprovalsRequired EventType = "phase15.execution.blocked.approvals_required"
	Phase15ExecutionBlockedApprovalsPending  EventType = "phase15.execution.blocked.approvals_pending"
	Phase15ExecutionBlockedApprovalsExpired  EventType = "phase15.execution.blocked.approvals_expired"
	Phase15ExecutionBlockedApprovalsRejected EventType = "phase15.execution.blocked.approvals_rejected"
	Phase15ExecutionGatePassed               EventType = "phase15.execution.gate.passed"
	Phase15ExecutionGateChecked              EventType = "phase15.execution.gate.checked"

	// Web approval flow events
	Phase15ApprovalPageRequested EventType = "phase15.approval.page.requested"
	Phase15ApprovalPageRendered  EventType = "phase15.approval.page.rendered"
	Phase15ApprovalFormSubmitted EventType = "phase15.approval.form.submitted"
	Phase15ApprovalLinkGenerated EventType = "phase15.approval.link.generated"
	Phase15ApprovalLinkClicked   EventType = "phase15.approval.link.clicked"

	// Ledger events
	Phase15LedgerAppend    EventType = "phase15.ledger.append"
	Phase15LedgerReplay    EventType = "phase15.ledger.replay"
	Phase15LedgerVerified  EventType = "phase15.ledger.verified"
	Phase15LedgerCorrupted EventType = "phase15.ledger.corrupted"

	// ==========================================================================
	// Phase 16: Notification Projection
	// ==========================================================================

	// Notification plan events
	Phase16NotifyPlanCreated      EventType = "phase16.notify.plan.created"
	Phase16NotifyPlanComputed     EventType = "phase16.notify.plan.computed"
	Phase16NotifyPlanSuppressed   EventType = "phase16.notify.plan.suppressed"
	Phase16NotifyPlanEmpty        EventType = "phase16.notify.plan.empty"
	Phase16NotifyPlanHashComputed EventType = "phase16.notify.plan.hash_computed"

	// Notification lifecycle events
	Phase16NotifyCreated    EventType = "phase16.notify.created"
	Phase16NotifyPlanned    EventType = "phase16.notify.planned"
	Phase16NotifyDowngraded EventType = "phase16.notify.downgraded"
	Phase16NotifySuppressed EventType = "phase16.notify.suppressed"
	Phase16NotifyExpired    EventType = "phase16.notify.expired"

	// Envelope events
	Phase16NotifyEnvelopeCreated  EventType = "phase16.notify.envelope.created"
	Phase16NotifyEnvelopeVerified EventType = "phase16.notify.envelope.verified"
	Phase16NotifyEnvelopeBlocked  EventType = "phase16.notify.envelope.blocked"
	Phase16NotifyEnvelopeExecuted EventType = "phase16.notify.envelope.executed"

	// Delivery events
	Phase16NotifyDelivered EventType = "phase16.notify.delivered"
	Phase16NotifyFailed    EventType = "phase16.notify.failed"
	Phase16NotifyBlocked   EventType = "phase16.notify.blocked"
	Phase16NotifyRetryable EventType = "phase16.notify.retryable"

	// Channel events
	Phase16NotifyWebBadgeAdded    EventType = "phase16.notify.web_badge.added"
	Phase16NotifyWebBadgeCleared  EventType = "phase16.notify.web_badge.cleared"
	Phase16NotifyWebBadgeExpired  EventType = "phase16.notify.web_badge.expired"
	Phase16NotifyEmailDraftCreate EventType = "phase16.notify.email_draft.created"
	Phase16NotifyEmailDigestSent  EventType = "phase16.notify.email_digest.sent"
	Phase16NotifyEmailAlertSent   EventType = "phase16.notify.email_alert.sent"
	Phase16NotifyPushSent         EventType = "phase16.notify.push.sent"
	Phase16NotifyPushBlocked      EventType = "phase16.notify.push.blocked"
	Phase16NotifySMSSent          EventType = "phase16.notify.sms.sent"
	Phase16NotifySMSBlocked       EventType = "phase16.notify.sms.blocked"

	// Policy events
	Phase16NotifyQuietHoursActive   EventType = "phase16.notify.quiet_hours.active"
	Phase16NotifyQuietHoursInactive EventType = "phase16.notify.quiet_hours.inactive"
	Phase16NotifyQuotaExceeded      EventType = "phase16.notify.quota.exceeded"
	Phase16NotifyQuotaReset         EventType = "phase16.notify.quota.reset"
	Phase16NotifyPrivacyBlocked     EventType = "phase16.notify.privacy.blocked"

	// Audience events
	Phase16NotifyAudienceResolved   EventType = "phase16.notify.audience.resolved"
	Phase16NotifyAudienceOwnerOnly  EventType = "phase16.notify.audience.owner_only"
	Phase16NotifyAudienceSpouseOnly EventType = "phase16.notify.audience.spouse_only"
	Phase16NotifyAudienceBoth       EventType = "phase16.notify.audience.both"

	// Store events
	Phase16NotifyStoreAppend EventType = "phase16.notify.store.append"
	Phase16NotifyStoreReplay EventType = "phase16.notify.store.replay"
	Phase16NotifyStoreError  EventType = "phase16.notify.store.error"

	// Digest events
	Phase16NotifyDigestPlanned    EventType = "phase16.notify.digest.planned"
	Phase16NotifyDigestSuppressed EventType = "phase16.notify.digest.suppressed"
	Phase16NotifyDigestSent       EventType = "phase16.notify.digest.sent"
	Phase16NotifyDigestEmpty      EventType = "phase16.notify.digest.empty"

	// ==========================================================================
	// Phase 17: Finance Execution Boundary
	// ==========================================================================
	//
	// CRITICAL: This is the ONLY path to external financial writes.
	// CRITICAL: Execution ONLY from approved drafts with household approvals.
	// CRITICAL: No auto-retries. No background execution.
	// CRITICAL: Must be idempotent - same envelope cannot execute twice.
	// CRITICAL: Hard-block on missing PolicySnapshotHash or ViewSnapshotHash.
	//
	// Reference: docs/ADR/ADR-0033-phase17-finance-execution-boundary.md

	// Draft lifecycle events
	Phase17FinanceDraftGenerated      EventType = "phase17.finance.draft.generated"
	Phase17FinanceDraftRejectedPolicy EventType = "phase17.finance.draft.rejected_by_policy"
	Phase17FinanceDraftStored         EventType = "phase17.finance.draft.stored"
	Phase17FinanceDraftExpired        EventType = "phase17.finance.draft.expired"

	// Envelope lifecycle events
	Phase17FinanceEnvelopeCreated   EventType = "phase17.finance.envelope.created"
	Phase17FinanceEnvelopeValidated EventType = "phase17.finance.envelope.validated"
	Phase17FinanceEnvelopeSealed    EventType = "phase17.finance.envelope.sealed"
	Phase17FinanceEnvelopeExpired   EventType = "phase17.finance.envelope.expired"
	Phase17FinanceEnvelopeStored    EventType = "phase17.finance.envelope.stored"
	Phase17FinanceEnvelopeRetrieved EventType = "phase17.finance.envelope.retrieved"

	// Policy snapshot events
	Phase17FinancePolicyVerified      EventType = "phase17.finance.policy.verified"
	Phase17FinancePolicyBlockedDrift  EventType = "phase17.finance.policy.blocked_drift"
	Phase17FinancePolicyBlockedNoHash EventType = "phase17.finance.policy.blocked_missing_hash"

	// View snapshot events
	Phase17FinanceViewVerified      EventType = "phase17.finance.view.verified"
	Phase17FinanceViewBlockedStale  EventType = "phase17.finance.view.blocked_stale"
	Phase17FinanceViewBlockedHash   EventType = "phase17.finance.view.blocked_mismatch"
	Phase17FinanceViewBlockedNoHash EventType = "phase17.finance.view.blocked_missing_hash"
	Phase17FinanceViewSnapshotTaken EventType = "phase17.finance.view.snapshot.taken"

	// Approval events
	Phase17FinanceApprovalRequired EventType = "phase17.finance.approval.required"
	Phase17FinanceApprovalReceived EventType = "phase17.finance.approval.received"
	Phase17FinanceApprovalComplete EventType = "phase17.finance.approval.complete"
	Phase17FinanceApprovalExpired  EventType = "phase17.finance.approval.expired"
	Phase17FinanceApprovalRejected EventType = "phase17.finance.approval.rejected"
	Phase17FinanceApprovalPending  EventType = "phase17.finance.approval.pending"

	// Execution lifecycle events
	Phase17FinanceExecutionStarted   EventType = "phase17.finance.execution.started"
	Phase17FinanceExecutionSucceeded EventType = "phase17.finance.execution.succeeded"
	Phase17FinanceExecutionFailed    EventType = "phase17.finance.execution.failed"
	Phase17FinanceExecutionBlocked   EventType = "phase17.finance.execution.blocked"
	Phase17FinanceExecutionAborted   EventType = "phase17.finance.execution.aborted"

	// Idempotency events
	Phase17FinanceIdempotencyReplayBlocked EventType = "phase17.finance.idempotency.replay_blocked"
	Phase17FinanceIdempotencyKeyDerived    EventType = "phase17.finance.idempotency.key_derived"

	// Caps enforcement events
	Phase17FinanceCapsChecked      EventType = "phase17.finance.caps.checked"
	Phase17FinanceCapsBlocked      EventType = "phase17.finance.caps.blocked"
	Phase17FinanceCapsAmountFailed EventType = "phase17.finance.caps.amount_failed"
	Phase17FinanceCapsRateFailed   EventType = "phase17.finance.caps.rate_failed"

	// Provider/payee registry events
	Phase17FinanceProviderAllowed      EventType = "phase17.finance.provider.allowed"
	Phase17FinanceProviderBlocked      EventType = "phase17.finance.provider.blocked"
	Phase17FinancePayeeAllowed         EventType = "phase17.finance.payee.allowed"
	Phase17FinancePayeeBlocked         EventType = "phase17.finance.payee.blocked"
	Phase17FinancePayeeNotRegistered   EventType = "phase17.finance.payee.not_registered"
	Phase17FinancePayeeProviderNoMatch EventType = "phase17.finance.payee.provider_mismatch"

	// Adapter events
	Phase17FinanceAdapterPrepared EventType = "phase17.finance.adapter.prepared"
	Phase17FinanceAdapterInvoked  EventType = "phase17.finance.adapter.invoked"
	Phase17FinanceAdapterBlocked  EventType = "phase17.finance.adapter.blocked"
	Phase17FinanceAdapterFailed   EventType = "phase17.finance.adapter.failed"

	// Settlement events
	Phase17FinanceSettlementRecorded  EventType = "phase17.finance.settlement.recorded"
	Phase17FinanceSettlementSimulated EventType = "phase17.finance.settlement.simulated"
	Phase17FinanceSettlementBlocked   EventType = "phase17.finance.settlement.blocked"

	// Store events
	Phase17FinanceStoreAppend EventType = "phase17.finance.store.append"
	Phase17FinanceStoreReplay EventType = "phase17.finance.store.replay"
	Phase17FinanceStoreError  EventType = "phase17.finance.store.error"

	// Config events
	Phase17FinanceConfigMode     EventType = "phase17.finance.config.mode"
	Phase17FinanceConfigProvider EventType = "phase17.finance.config.provider"
	Phase17FinanceSandboxOnly    EventType = "phase17.finance.sandbox.only"
	Phase17FinanceLiveBlocked    EventType = "phase17.finance.live.blocked"

	// ═══════════════════════════════════════════════════════════════════════════
	// Phase 18.1: The Moment - Interest Registration
	// Reference: Phase 18.1 specification
	// ═══════════════════════════════════════════════════════════════════════════

	// Interest events - email registration for early access
	Phase18_1InterestRegistered EventType = "phase18_1.interest.registered"
	Phase18_1InterestDuplicate  EventType = "phase18_1.interest.duplicate"
	Phase18_1InterestInvalid    EventType = "phase18_1.interest.invalid"

	// ═══════════════════════════════════════════════════════════════════════════
	// Phase 18.2: Today, quietly - Recognition + Suppression + Preference
	// Reference: Phase 18.2 specification
	// ═══════════════════════════════════════════════════════════════════════════

	// Page rendered event - emitted when /today is rendered
	Phase18_2TodayRendered EventType = "phase18_2.today.rendered"

	// Preference recorded event - emitted when user submits preference
	Phase18_2PreferenceRecorded EventType = "phase18_2.preference.recorded"

	// Suppression demonstrated event - emitted when suppressed insight is shown
	Phase18_2SuppressionDemonstrated EventType = "phase18_2.suppression.demonstrated"

	// ═══════════════════════════════════════════════════════════════════════════
	// Phase 18.3: The Proof of Care - Held, not shown
	// Reference: docs/ADR/ADR-0035-phase18-3-proof-of-care.md
	// ═══════════════════════════════════════════════════════════════════════════

	// Held computed event - emitted when held summary is computed
	// CRITICAL: Contains hash only, never raw data
	Phase18_3HeldComputed EventType = "phase18_3.held.computed"

	// Held presented event - emitted when /held page is rendered
	Phase18_3HeldPresented EventType = "phase18_3.held.presented"

	// ═══════════════════════════════════════════════════════════════════════════
	// Phase 18.4: Quiet Shift - Subtle Availability
	// Reference: docs/ADR/ADR-0036-phase18-4-quiet-shift.md
	// ═══════════════════════════════════════════════════════════════════════════

	// Surface cue computed event - emitted when availability cue is determined
	// CRITICAL: Contains hash only, never raw data or identifiers
	Phase18_4SurfaceCueComputed EventType = "phase18_4.surface.cue.computed"

	// Surface page rendered event - emitted when /surface page is shown
	Phase18_4SurfacePageRendered EventType = "phase18_4.surface.page.rendered"

	// Surface action events - emitted when user interacts with surfaced item
	Phase18_4SurfaceActionViewed        EventType = "phase18_4.surface.action.viewed"
	Phase18_4SurfaceActionHeld          EventType = "phase18_4.surface.action.held"
	Phase18_4SurfaceActionWhy           EventType = "phase18_4.surface.action.why"
	Phase18_4SurfaceActionPreferShowAll EventType = "phase18_4.surface.action.prefer_show_all"

	// ═══════════════════════════════════════════════════════════════════════════
	// Phase 18.5: Quiet Proof - Restraint Ledger
	// Reference: docs/ADR/ADR-0037-phase18-5-quiet-proof.md
	// ═══════════════════════════════════════════════════════════════════════════

	// Proof computed event - emitted when restraint proof is computed
	// CRITICAL: Contains hash only, never raw data or counts
	Phase18_5ProofComputed EventType = "phase18_5.proof.computed"

	// Proof viewed event - emitted when /proof page is rendered
	Phase18_5ProofViewed EventType = "phase18_5.proof.viewed"

	// Proof dismissed event - emitted when user dismisses the proof
	Phase18_5ProofDismissed EventType = "phase18_5.proof.dismissed"

	// ═══════════════════════════════════════════════════════════════════════════
	// Phase 18.6: First Connect - Consent-first Onboarding
	// Reference: docs/ADR/ADR-0038-phase18-6-first-connect.md
	// ═══════════════════════════════════════════════════════════════════════════

	// Connection intent events - emitted when connection intents are recorded
	Phase18_6ConnectionIntentRecorded EventType = "phase18_6.connection.intent.recorded"

	// Connection state events - emitted when state is computed
	Phase18_6ConnectionStateComputed EventType = "phase18_6.connection.state.computed"

	// Connection request events - emitted when user initiates connect/disconnect
	Phase18_6ConnectionConnectRequested    EventType = "phase18_6.connection.connect.requested"
	Phase18_6ConnectionDisconnectRequested EventType = "phase18_6.connection.disconnect.requested"

	// ═══════════════════════════════════════════════════════════════════════════
	// Phase 18.7: Mirror Proof - Trust Through Evidence of Reading
	// Reference: docs/ADR/ADR-0039-phase18-7-mirror-proof.md
	// ═══════════════════════════════════════════════════════════════════════════

	// Mirror computed event - emitted when mirror page is computed
	// CRITICAL: Contains hash only, never raw data or identifiers
	Phase18_7MirrorComputed EventType = "phase18_7.mirror.computed"

	// Mirror viewed event - emitted when /mirror page is rendered
	Phase18_7MirrorViewed EventType = "phase18_7.mirror.viewed"

	// Mirror acknowledged event - emitted when user acknowledges the mirror
	Phase18_7MirrorAcknowledged EventType = "phase18_7.mirror.acknowledged"

	// ═══════════════════════════════════════════════════════════════════════════
	// PHASE 18.8: Real OAuth (Gmail Read-Only)
	// Reference: docs/ADR/ADR-0041-phase18-8-real-oauth-gmail-readonly.md
	//
	// CRITICAL: These events audit the OAuth flow and Gmail sync.
	// CRITICAL: No tokens or secrets are ever included in events.
	// ═══════════════════════════════════════════════════════════════════════════

	// OAuth start event - emitted when OAuth flow is initiated
	Phase18_8OAuthStarted EventType = "phase18_8.oauth.started"

	// OAuth callback event - emitted when OAuth callback is received
	Phase18_8OAuthCallback EventType = "phase18_8.oauth.callback"

	// OAuth token minted event - emitted when token is stored
	Phase18_8OAuthTokenMinted EventType = "phase18_8.oauth.token_minted"

	// OAuth revoke requested event - emitted when revocation is requested
	Phase18_8OAuthRevokeRequested EventType = "phase18_8.oauth.revoke_requested"

	// OAuth revoke completed event - emitted when revocation completes
	Phase18_8OAuthRevokeCompleted EventType = "phase18_8.oauth.revoke_completed"

	// Gmail sync started event - emitted when sync begins
	Phase18_8GmailSyncStarted EventType = "phase18_8.gmail.sync_started"

	// Gmail sync completed event - emitted when sync completes
	Phase18_8GmailSyncCompleted EventType = "phase18_8.gmail.sync_completed"

	// Gmail sync failed event - emitted when sync fails
	Phase18_8GmailSyncFailed EventType = "phase18_8.gmail.sync_failed"

	// ═══════════════════════════════════════════════════════════════════════════
	// PHASE 18 WEB CONTROL CENTER
	// Reference: Phase 18 "Nothing Needs You" Control Center specification
	//
	// CRITICAL: All views read-only (no automatic execution).
	// CRITICAL: Approval tokens use Ed25519 signatures.
	// CRITICAL: Run logs are deterministic and replayable.
	// ═══════════════════════════════════════════════════════════════════════════

	// Web view events
	Phase18WebApproveViewed      EventType = "phase18.web.approve.viewed"
	Phase18WebRunsViewed         EventType = "phase18.web.runs.viewed"
	Phase18WebRunDetailViewed    EventType = "phase18.web.run_detail.viewed"
	Phase18WebSuppressionsViewed EventType = "phase18.web.suppressions.viewed"

	// Approval token events
	Phase18ApprovalTokenVerified EventType = "phase18.approval.token.verified"
	Phase18ApprovalTokenExpired  EventType = "phase18.approval.token.expired"
	Phase18ApprovalTokenInvalid  EventType = "phase18.approval.token.invalid"

	// Run log events
	Phase18RunSnapshotCreated EventType = "phase18.run.snapshot.created"
	Phase18RunReplayRequested EventType = "phase18.run.replay.requested"
	Phase18RunReplaySucceeded EventType = "phase18.run.replay.succeeded"
	Phase18RunReplayFailed    EventType = "phase18.run.replay.failed"

	// Suppression events
	Phase18SuppressionCreated EventType = "phase18.suppression.created"
	Phase18SuppressionRemoved EventType = "phase18.suppression.removed"
	Phase18SuppressionExpired EventType = "phase18.suppression.expired"

	// ═══════════════════════════════════════════════════════════════════════════
	// PHASE 19: LLM Shadow-Mode Contract
	// Reference: docs/ADR/ADR-0043-phase19-shadow-mode-contract.md
	//
	// CRITICAL: Shadow mode emits METADATA ONLY - never content.
	// CRITICAL: Shadow mode is OFF by default.
	// CRITICAL: Shadow mode NEVER affects UI, obligations, drafts, or execution.
	// ═══════════════════════════════════════════════════════════════════════════

	// Shadow run lifecycle events
	Phase19ShadowRunStarted   EventType = "phase19.shadow.run.started"
	Phase19ShadowRunCompleted EventType = "phase19.shadow.run.completed"
	Phase19ShadowRunPersisted EventType = "phase19.shadow.run.persisted"
	Phase19ShadowRunBlocked   EventType = "phase19.shadow.run.blocked"
	Phase19ShadowRunFailed    EventType = "phase19.shadow.run.failed"

	// Shadow mode configuration events
	Phase19ShadowModeOff     EventType = "phase19.shadow.mode.off"
	Phase19ShadowModeObserve EventType = "phase19.shadow.mode.observe"

	// Shadow signal events
	Phase19ShadowSignalEmitted EventType = "phase19.shadow.signal.emitted"

	// Shadow violation events - emitted when invariants would be violated
	Phase19ShadowViolationDetected EventType = "phase19.shadow.violation.detected"

	// Shadow replay events
	Phase19ShadowReplayStarted   EventType = "phase19.shadow.replay.started"
	Phase19ShadowReplayCompleted EventType = "phase19.shadow.replay.completed"
	Phase19ShadowReplayMismatch  EventType = "phase19.shadow.replay.mismatch"

	// ═══════════════════════════════════════════════════════════════════════════
	// PHASE 19.1: Real Gmail Connection (You-only)
	// Reference: Phase 19.1 specification
	//
	// CRITICAL: Explicit sync only - NO background polling.
	// CRITICAL: Max 25 messages, last 7 days.
	// CRITICAL: DefaultToHold = true for all Gmail obligations.
	// CRITICAL: Magnitude buckets only - no raw counts in UI.
	// CRITICAL: No storage of raw message content.
	// ═══════════════════════════════════════════════════════════════════════════

	// Sync lifecycle events
	Phase19_1GmailSyncRequested EventType = "phase19_1.gmail.sync.requested"
	Phase19_1GmailSyncStarted   EventType = "phase19_1.gmail.sync.started"
	Phase19_1GmailSyncCompleted EventType = "phase19_1.gmail.sync.completed"
	Phase19_1GmailSyncFailed    EventType = "phase19_1.gmail.sync.failed"

	// Sync receipt events
	Phase19_1SyncReceiptCreated  EventType = "phase19_1.sync.receipt.created"
	Phase19_1SyncReceiptStored   EventType = "phase19_1.sync.receipt.stored"
	Phase19_1SyncReceiptVerified EventType = "phase19_1.sync.receipt.verified"

	// Event store events
	Phase19_1EventStored      EventType = "phase19_1.event.stored"
	Phase19_1EventDeduplicate EventType = "phase19_1.event.deduplicate"

	// Obligation events
	Phase19_1ObligationCreated EventType = "phase19_1.obligation.created"
	Phase19_1ObligationHeld    EventType = "phase19_1.obligation.held"

	// Quiet check events
	Phase19_1QuietCheckRequested EventType = "phase19_1.quiet_check.requested"
	Phase19_1QuietCheckComputed  EventType = "phase19_1.quiet_check.computed"
	Phase19_1QuietCheckVerified  EventType = "phase19_1.quiet_check.verified"

	// ═══════════════════════════════════════════════════════════════════════════
	// PHASE 19.2: LLM Shadow Mode Contract
	// Reference: docs/ADR/ADR-0043-phase19-2-shadow-mode-contract.md
	//
	// CRITICAL: Shadow mode produces METADATA ONLY - never content.
	// CRITICAL: Shadow mode does NOT affect behavior - observation only.
	// CRITICAL: Shadow mode is OFF by default, explicit user action required.
	// CRITICAL: No goroutines. No background polling. Explicit trigger only.
	// ═══════════════════════════════════════════════════════════════════════════

	// Shadow mode request/run lifecycle events
	Phase19_2ShadowRequested EventType = "phase19_2.shadow.requested"
	Phase19_2ShadowComputed  EventType = "phase19_2.shadow.computed"
	Phase19_2ShadowPersisted EventType = "phase19_2.shadow.persisted"
	Phase19_2ShadowBlocked   EventType = "phase19_2.shadow.blocked"
	Phase19_2ShadowFailed    EventType = "phase19_2.shadow.failed"

	// Shadow receipt events
	Phase19_2ShadowReceiptCreated  EventType = "phase19_2.shadow.receipt.created"
	Phase19_2ShadowReceiptVerified EventType = "phase19_2.shadow.receipt.verified"

	// Shadow suggestion events (aggregated, no per-suggestion events for privacy)
	Phase19_2ShadowSuggestionsComputed EventType = "phase19_2.shadow.suggestions.computed"

	// =============================================================================
	// Phase 19.3: Azure OpenAI Shadow Provider Events
	// =============================================================================

	// Phase19_3 Azure shadow provider events
	Phase19_3AzureShadowRequested    EventType = "phase19_3.azure.shadow.requested"
	Phase19_3AzureShadowCompleted    EventType = "phase19_3.azure.shadow.completed"
	Phase19_3AzureShadowFailed       EventType = "phase19_3.azure.shadow.failed"
	Phase19_3AzureShadowTimeout      EventType = "phase19_3.azure.shadow.timeout"
	Phase19_3AzureShadowNotPermitted EventType = "phase19_3.azure.shadow.not_permitted"
	Phase19_3AzureShadowDisabled     EventType = "phase19_3.azure.shadow.disabled"

	// Privacy guard events
	Phase19_3PrivacyGuardPassed  EventType = "phase19_3.privacy.guard.passed"
	Phase19_3PrivacyGuardBlocked EventType = "phase19_3.privacy.guard.blocked"

	// Output validation events
	Phase19_3OutputValidationPassed EventType = "phase19_3.output.validation.passed"
	Phase19_3OutputValidationFailed EventType = "phase19_3.output.validation.failed"

	// Consent events
	Phase19_3ShadowConsentGranted EventType = "phase19_3.shadow.consent.granted"
	Phase19_3ShadowConsentRevoked EventType = "phase19_3.shadow.consent.revoked"

	// Provider selection events
	Phase19_3ProviderSelected EventType = "phase19_3.provider.selected"
	Phase19_3ProviderFallback EventType = "phase19_3.provider.fallback"

	// ==========================================================================
	// Phase 19.4: Shadow Diff + Calibration Events
	// ==========================================================================
	// CRITICAL: Contains ONLY hashes and buckets - never content.
	// Reference: docs/ADR/ADR-0045-phase19-4-shadow-diff-calibration.md

	// Diff computation events
	Phase19_4DiffComputed  EventType = "phase19_4.diff.computed"
	Phase19_4DiffPersisted EventType = "phase19_4.diff.persisted"

	// Vote recording events
	Phase19_4VoteRecorded  EventType = "phase19_4.vote.recorded"
	Phase19_4VotePersisted EventType = "phase19_4.vote.persisted"

	// Stats computation events
	Phase19_4StatsComputed EventType = "phase19_4.stats.computed"
	Phase19_4StatsViewed   EventType = "phase19_4.stats.viewed"

	// Report events
	Phase19_4ReportRequested EventType = "phase19_4.report.requested"
	Phase19_4ReportRendered  EventType = "phase19_4.report.rendered"

	// =============================================================================
	// Phase 19.5: Shadow Gating + Promotion Candidates
	// CRITICAL: Shadow does NOT affect behavior. Candidates are observational only.
	// Reference: docs/ADR/ADR-0046-phase19-5-shadow-gating-and-promotion-candidates.md
	// =============================================================================

	// Candidate computation events
	Phase19_5CandidatesRefreshRequested EventType = "phase19_5.candidates.refresh_requested"
	Phase19_5CandidatesComputed         EventType = "phase19_5.candidates.computed"
	Phase19_5CandidatesPersisted        EventType = "phase19_5.candidates.persisted"
	Phase19_5CandidatesViewed           EventType = "phase19_5.candidates.viewed"

	// Promotion intent events
	// CRITICAL: Intent only. Does NOT change behavior.
	Phase19_5PromotionProposed  EventType = "phase19_5.promotion.proposed"
	Phase19_5PromotionPersisted EventType = "phase19_5.promotion.persisted"

	// =========================================================================
	// Phase 19.6: Rule Pack Export (Promotion Pipeline)
	// =========================================================================
	// Reference: docs/ADR/ADR-0047-phase19-6-rulepack-export.md
	//
	// CRITICAL: RulePack export does NOT apply itself. No behavior change.

	// Pack lifecycle events
	Phase19_6PackBuildRequested EventType = "phase19_6.pack.build_requested"
	Phase19_6PackBuilt          EventType = "phase19_6.pack.built"
	Phase19_6PackPersisted      EventType = "phase19_6.pack.persisted"
	Phase19_6PackViewed         EventType = "phase19_6.pack.viewed"
	Phase19_6PackExported       EventType = "phase19_6.pack.exported"
	Phase19_6PackDismissed      EventType = "phase19_6.pack.dismissed"

	// =========================================================================
	// Phase 19.3b: Go Real Azure + Embeddings Health Events
	// =========================================================================
	//
	// Reference: docs/ADR/ADR-0049-phase19-3b-go-real-azure-and-embeddings.md
	//
	// CRITICAL INVARIANTS:
	//   - No secrets logged
	//   - No identifiers sent to provider
	//   - Safe constant input only for embeddings healthcheck

	// Health page events
	Phase19_3bHealthViewed       EventType = "phase19_3b.health.viewed"
	Phase19_3bHealthRunBlocked   EventType = "phase19_3b.health.run.blocked"
	Phase19_3bHealthRunFailed    EventType = "phase19_3b.health.run.failed"
	Phase19_3bHealthRunCompleted EventType = "phase19_3b.health.run.completed"

	// Embeddings healthcheck events
	Phase19_3bEmbedHealthRequested EventType = "phase19_3b.embed.health.requested"
	Phase19_3bEmbedHealthCompleted EventType = "phase19_3b.embed.health.completed"
	Phase19_3bEmbedHealthFailed    EventType = "phase19_3b.embed.health.failed"
	Phase19_3bEmbedHealthSkipped   EventType = "phase19_3b.embed.health.skipped"

	// =========================================================================
	// Phase 20: Trust Accrual Layer (Proof Over Time)
	// =========================================================================
	//
	// Reference: docs/ADR/ADR-0048-phase20-trust-accrual-layer.md
	//
	// CRITICAL INVARIANTS:
	//   - Trust signals are NEVER pushed
	//   - Trust signals are NEVER frequent
	//   - Trust signals are NEVER actionable
	//   - Events include canonical hashes only

	// Trust summary lifecycle events
	Phase20TrustComputed  EventType = "phase20.trust.computed"
	Phase20TrustPersisted EventType = "phase20.trust.persisted"
	Phase20TrustViewed    EventType = "phase20.trust.viewed"
	Phase20TrustDismissed EventType = "phase20.trust.dismissed"

	// ======================================================================
	// Phase 21: Unified Onboarding + Shadow Receipt Viewer
	// ======================================================================
	//
	// CRITICAL INVARIANTS:
	//   - Mode is DERIVED not stored
	//   - Receipt displays ONLY abstract buckets and hashes
	//   - Dismissal stores ONLY hashes
	//   - Events include canonical hashes only

	// Onboarding lifecycle events
	Phase21OnboardingViewed EventType = "phase21.onboarding.viewed"
	Phase21ModeComputed     EventType = "phase21.mode.computed"

	// Shadow receipt viewer lifecycle events
	Phase21ShadowReceiptViewed    EventType = "phase21.shadow.receipt.viewed"
	Phase21ShadowReceiptDismissed EventType = "phase21.shadow.receipt.dismissed"
	Phase21ShadowReceiptCueShown  EventType = "phase21.shadow.receipt.cue.shown"

	// ======================================================================
	// Phase 22: Quiet Inbox Mirror (First Real Value Moment)
	// ======================================================================
	//
	// CRITICAL INVARIANTS:
	//   - Events contain ONLY hashes, never content/counts/identifiers
	//   - Abstraction over explanation
	//   - Silence is success
	//   - No engagement loops

	// Quiet mirror lifecycle events
	Phase22QuietMirrorComputed EventType = "phase22.quiet_mirror.computed"
	Phase22QuietMirrorViewed   EventType = "phase22.quiet_mirror.viewed"
	Phase22QuietMirrorAbsent   EventType = "phase22.quiet_mirror.absent"

	// Whisper cue events
	Phase22WhisperCueShown     EventType = "phase22.whisper_cue.shown"
	Phase22WhisperCueDismissed EventType = "phase22.whisper_cue.dismissed"

	// ==========================================================================
	// Phase 23: Gentle Action Invitation
	// Reference: docs/ADR/ADR-0053-phase23-gentle-invitation.md
	// ==========================================================================

	// Phase23InvitationEligible - invitation eligibility computed.
	// Payload: hashes only, enums only, no text.
	Phase23InvitationEligible EventType = "phase23.invitation.eligible"

	// Phase23InvitationRendered - invitation page rendered.
	Phase23InvitationRendered EventType = "phase23.invitation.rendered"

	// Phase23InvitationAccepted - user accepted an invitation.
	Phase23InvitationAccepted EventType = "phase23.invitation.accepted"

	// Phase23InvitationDismissed - user dismissed an invitation.
	Phase23InvitationDismissed EventType = "phase23.invitation.dismissed"

	// Phase23InvitationPersisted - invitation decision persisted.
	Phase23InvitationPersisted EventType = "phase23.invitation.persisted"

	// Phase23InvitationSkipped - invitation not shown (not eligible).
	Phase23InvitationSkipped EventType = "phase23.invitation.skipped"

	// ==========================================================================
	// Phase 24: First Reversible Real Action
	// Reference: docs/ADR/ADR-0054-phase24-first-reversible-action.md
	// ==========================================================================

	// Phase24InvitationOffered - action invitation was offered.
	// Payload: hashes only, period ID, no content.
	Phase24InvitationOffered EventType = "phase24.invitation.offered"

	// Phase24ActionViewed - circle viewed the action preview.
	Phase24ActionViewed EventType = "phase24.action.viewed"

	// Phase24ActionDismissed - circle dismissed the action invitation.
	Phase24ActionDismissed EventType = "phase24.action.dismissed"

	// Phase24PreviewRendered - preview was rendered.
	Phase24PreviewRendered EventType = "phase24.preview.rendered"

	// Phase24PeriodClosed - period was closed (action taken or dismissed).
	Phase24PeriodClosed EventType = "phase24.period.closed"

	// ==========================================================================
	// Phase 25: First Undoable Execution Events
	// Reference: docs/ADR/ADR-0055-phase25-first-undoable-execution.md
	// ==========================================================================
	// CRITICAL: All payloads contain hashes only - never identifiers.

	// Phase25UndoableViewed - undoable action page was viewed.
	Phase25UndoableViewed EventType = "phase25.undoable.viewed"

	// Phase25EligibleComputed - eligibility was computed.
	Phase25EligibleComputed EventType = "phase25.undoable.eligible.computed"

	// Phase25RunRequested - execution was requested.
	Phase25RunRequested EventType = "phase25.undoable.run.requested"

	// Phase25RunExecuted - execution completed via calendar boundary.
	Phase25RunExecuted EventType = "phase25.undoable.run.executed"

	// Phase25RecordPersisted - undo record was persisted.
	Phase25RecordPersisted EventType = "phase25.undoable.record.persisted"

	// Phase25UndoViewed - undo page was viewed.
	Phase25UndoViewed EventType = "phase25.undoable.undo.viewed"

	// Phase25UndoRequested - undo was requested.
	Phase25UndoRequested EventType = "phase25.undoable.undo.requested"

	// Phase25UndoExecuted - undo completed via calendar boundary.
	Phase25UndoExecuted EventType = "phase25.undoable.undo.executed"

	// Phase25AckPersisted - undo acknowledgement was persisted.
	Phase25AckPersisted EventType = "phase25.undoable.ack.persisted"

	// Phase25Dismissed - undoable action was dismissed.
	Phase25Dismissed EventType = "phase25.undoable.dismissed"

	// ==========================================================================
	// Phase 26A: Guided Journey Events
	// Reference: docs/ADR/ADR-0056-phase26A-guided-journey.md
	// ==========================================================================
	// CRITICAL: All payloads contain hashes only - never identifiers.

	// Phase26AJourneyRequested - journey page was requested.
	Phase26AJourneyRequested EventType = "phase26a.journey.requested"

	// Phase26AJourneyComputed - journey step was computed.
	Phase26AJourneyComputed EventType = "phase26a.journey.computed"

	// Phase26AJourneyDismissed - journey was dismissed for period.
	Phase26AJourneyDismissed EventType = "phase26a.journey.dismissed"

	// Phase26AJourneyNextRedirected - user clicked next and was redirected.
	Phase26AJourneyNextRedirected EventType = "phase26a.journey.next.redirected"

	// ==========================================================================
	// Phase 26B: First Five Minutes Proof Events
	// Reference: docs/ADR/ADR-0056-phase26B-first-five-minutes-proof.md
	// ==========================================================================
	// This is NOT analytics. This is NOT telemetry. This is narrative proof.
	// CRITICAL: All payloads contain hashes only - never identifiers.

	// Phase26BFirstMinutesComputed - first minutes summary was computed.
	Phase26BFirstMinutesComputed EventType = "phase26b.first_minutes.computed"

	// Phase26BFirstMinutesPersisted - first minutes summary was persisted.
	Phase26BFirstMinutesPersisted EventType = "phase26b.first_minutes.persisted"

	// Phase26BFirstMinutesViewed - first minutes receipt was viewed.
	Phase26BFirstMinutesViewed EventType = "phase26b.first_minutes.viewed"

	// Phase26BFirstMinutesDismissed - first minutes receipt was dismissed.
	Phase26BFirstMinutesDismissed EventType = "phase26b.first_minutes.dismissed"

	// ==========================================================================
	// Phase 26C: Connected Reality Check
	// ==========================================================================
	// Reference: docs/ADR/ADR-0057-phase26C-connected-reality-check.md
	// ==========================================================================
	// This is NOT analytics. This is a trust proof page.
	// CRITICAL: All payloads contain hashes only - never identifiers.

	// Phase26CRealityRequested - reality page was requested.
	Phase26CRealityRequested EventType = "phase26c.reality.requested"

	// Phase26CRealityComputed - reality page was computed.
	Phase26CRealityComputed EventType = "phase26c.reality.computed"

	// Phase26CRealityViewed - reality page was viewed.
	Phase26CRealityViewed EventType = "phase26c.reality.viewed"

	// Phase26CRealityAckRecorded - reality page acknowledgement was recorded.
	Phase26CRealityAckRecorded EventType = "phase26c.reality.ack.recorded"

	// ==========================================================================
	// Phase 27: Real Shadow Receipt (Primary Proof of Intelligence)
	// ==========================================================================
	//
	// CRITICAL INVARIANTS:
	//   - Shadow remains observation-only
	//   - Shadow never alters runtime behavior
	//   - Vote does NOT change behavior (feeds Phase 19 calibration only)
	//   - Payloads must be hash-only
	//
	// Reference: docs/ADR/ADR-0058-phase27-real-shadow-receipt-primary-proof.md

	// Phase27ShadowReceiptRendered - shadow receipt primary page was rendered.
	Phase27ShadowReceiptRendered EventType = "phase27.shadow_receipt.rendered"

	// Phase27ShadowReceiptVoted - user voted on shadow receipt restraint.
	Phase27ShadowReceiptVoted EventType = "phase27.shadow_receipt.voted"

	// Phase27ShadowReceiptDismissed - shadow receipt cue was dismissed.
	Phase27ShadowReceiptDismissed EventType = "phase27.shadow_receipt.dismissed"

	// ==========================================================================
	// Phase 28: Trust Kept — First Real Act, Then Silence
	// ==========================================================================
	//
	// CRITICAL INVARIANTS:
	//   - Only calendar_respond action allowed
	//   - Single execution per period (day)
	//   - 15-minute undo window (bucketed)
	//   - After execution: silence forever
	//   - No growth mechanics, engagement loops, or escalation paths
	//   - Payloads contain hashes only - never identifiers
	//
	// Reference: docs/ADR/ADR-0059-phase28-trust-kept.md

	// Phase28TrustActionEligible - trust action eligibility computed.
	Phase28TrustActionEligible EventType = "phase28.trust_action.eligible"

	// Phase28TrustActionPreviewViewed - trust action preview was viewed.
	Phase28TrustActionPreviewViewed EventType = "phase28.trust_action.preview.viewed"

	// Phase28TrustActionExecuted - trust action was executed.
	Phase28TrustActionExecuted EventType = "phase28.trust_action.executed"

	// Phase28TrustActionUndone - trust action was undone.
	Phase28TrustActionUndone EventType = "phase28.trust_action.undone"

	// Phase28TrustActionExpired - undo window expired.
	Phase28TrustActionExpired EventType = "phase28.trust_action.expired"

	// Phase28TrustActionReceiptViewed - trust action receipt was viewed.
	Phase28TrustActionReceiptViewed EventType = "phase28.trust_action.receipt.viewed"

	// Phase28TrustActionReceiptDismissed - trust action receipt was dismissed.
	Phase28TrustActionReceiptDismissed EventType = "phase28.trust_action.receipt.dismissed"

	// Phase28TrustActionDismissed - trust action invitation was dismissed (kept holding).
	Phase28TrustActionDismissed EventType = "phase28.trust_action.dismissed"

	// Phase 29: TrueLayer Read-Only Connect + Finance Mirror Proof
	// Reference: docs/ADR/ADR-0060-phase29-truelayer-readonly-finance-mirror.md
	//
	// CRITICAL INVARIANTS:
	//   - Read-only scopes only (accounts, balance, transactions)
	//   - No payment scopes allowed
	//   - Abstract buckets only in events (no raw amounts, merchants, or identifiers)
	//   - Bounded sync (25 items max, 7 days)

	// TrueLayer OAuth lifecycle events
	Phase29TrueLayerOAuthStart    EventType = "phase29.truelayer.oauth.start"
	Phase29TrueLayerOAuthCallback EventType = "phase29.truelayer.oauth.callback"
	Phase29TrueLayerOAuthRevoke   EventType = "phase29.truelayer.oauth.revoke"

	// TrueLayer sync events
	Phase29TrueLayerSyncRequested EventType = "phase29.truelayer.sync.requested"
	Phase29TrueLayerSyncCompleted EventType = "phase29.truelayer.sync.completed"
	Phase29TrueLayerSyncFailed    EventType = "phase29.truelayer.sync.failed"
	Phase29TrueLayerSyncPersisted EventType = "phase29.truelayer.sync.persisted"

	// Finance mirror page events
	Phase29FinanceMirrorRendered EventType = "phase29.finance_mirror.rendered"
	Phase29FinanceMirrorViewed   EventType = "phase29.finance_mirror.viewed"
	Phase29FinanceMirrorAcked    EventType = "phase29.finance_mirror.acked"

	// Phase 30A: Identity + Replay
	// CRITICAL: No raw identifiers in event metadata - hashes only.
	// CRITICAL: No goroutines, no time.Now() - clock injection only.

	// Device identity events
	Phase30AIdentityCreated EventType = "phase30A.identity.created"
	Phase30AIdentityViewed  EventType = "phase30A.identity.viewed"
	Phase30AIdentityBound   EventType = "phase30A.identity.bound"

	// Replay bundle events
	Phase30AReplayExported EventType = "phase30A.replay.exported"
	Phase30AReplayImported EventType = "phase30A.replay.imported"
	Phase30AReplayRejected EventType = "phase30A.replay.rejected"
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

// Emitter provides the interface for emitting events.
type Emitter interface {
	// Emit emits an event.
	Emit(event Event)
}

// NoopEmitter is an emitter that does nothing.
type NoopEmitter struct{}

// Emit does nothing.
func (n NoopEmitter) Emit(event Event) {}

// Verify interface compliance.
var _ Emitter = NoopEmitter{}
