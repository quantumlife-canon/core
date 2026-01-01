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
