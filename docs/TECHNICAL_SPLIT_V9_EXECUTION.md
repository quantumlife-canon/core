# Technical Split v9: Financial Execution

**Status**: AUTHORITATIVE
**Version**: 1.0
**Subordinate to**: QUANTUMLIFE_CANON_V1.md, TECHNICAL_SPLIT_V1.md, CANON_ADDENDUM_V9_EXECUTION.md
**Effective**: Upon ratification

---

## 0. Document Hierarchy and Conflict Rule

This document defines technical boundaries for financial execution. It is strictly subordinate to:

1. **QUANTUMLIFE_CANON_V1.md** — The foundational constitution
2. **CANON_ADDENDUM_V9_EXECUTION.md** — The execution-specific constitutional extension

Where any provision in this document conflicts with Canon v1 or Canon Addendum v9, the Canon prevails without exception.

This document defines **boundaries only**. It does not prescribe implementation. It establishes what MUST and MUST NOT exist in any compliant v9 implementation. Implementation that violates these boundaries is defective regardless of functional correctness.

---

## 1. Definitions (Canon-Aligned)

All definitions map to existing Canon primitives. No new primitives are introduced.

### 1.1 Execution (Financial)

The act of initiating an external effect that moves money, creates binding financial commitments, or triggers irreversible downstream financial consequences. Execution is the transition from Proposal to Action to Settlement in the Canon pipeline.

### 1.2 Approval Artifact

A signed, timestamped, immutable record that binds a specific Circle's consent to a specific ActionHash. Approval Artifacts are Authority grants scoped to a single action instance.

### 1.3 ActionHash

A cryptographic hash that uniquely identifies an execution intent. The ActionHash binds together: the action specification, the referenced view, the caps, the expiry, and the intersection context. Any modification invalidates the hash.

### 1.4 ExecutionEnvelope

An immutable container holding all information required to execute a financial action. The ExecutionEnvelope is the sealed form of a Commitment ready for Action execution. Once sealed, it cannot be modified—only executed, revoked, or expired.

### 1.5 Revocation Window

A mandatory time period between approval and execution during which any approver may withdraw consent. Revocation Windows map to the Authority layer's temporal bounds.

### 1.6 Validity Check

An affirmative verification performed at the moment of execution confirming that all preconditions still hold. Absence of revocation is insufficient; positive confirmation is required.

### 1.7 Settlement

The confirmed completion of a financial action. Settlement occurs when the external financial system confirms the action is irrevocable. Settlement is recorded in the Memory layer and audited in the Audit layer.

---

## 2. Reinforced Control Plane vs Execution Plane

### 2.1 Separation Principle

The Control Plane and Execution Plane MUST be architecturally distinct. No component may span both. This separation is inherited from Technical Split v1 and reinforced for financial execution.

### 2.2 Control Plane Responsibilities

The Control Plane handles:

- Reading and aggregating financial data (v8)
- Computing views and symmetry proofs
- Validating Authority and Policy constraints
- Constructing Proposals
- Managing Negotiation and Commitment flows
- Constructing ExecutionEnvelopes
- Recording Approvals

The Control Plane MUST NOT:

- Hold credentials for external financial systems
- Initiate external financial effects
- Make runtime decisions during execution
- Modify execution parameters after envelope sealing

### 2.3 Execution Plane Responsibilities

The Execution Plane handles:

- Receiving sealed ExecutionEnvelopes
- Validating envelope integrity and expiry
- Performing affirmative Validity Checks
- Executing the specified action exactly as specified
- Recording execution outcomes
- Reporting to Settlement layer

The Execution Plane MUST NOT:

- Interpret intent
- Modify amounts, recipients, or timing
- Make judgment calls
- Retry without explicit instruction
- Access data beyond the envelope contents

### 2.4 Execution Plane Invariants

The Execution Plane MUST be:

- **Dumb**: No intelligence, inference, or optimization
- **Interruptible**: Revocation signals halt execution at any safe point
- **Deterministic**: Same envelope always produces same execution attempt
- **Auditable**: Every state transition is logged

---

## 3. Financial Execution Boundary (Hard Gate)

No external financial effect may occur unless ALL of the following preconditions are satisfied. This is the Hard Gate.

### 3.1 Mandatory Preconditions

1. **v8-Derived View Reference**: The execution MUST reference a valid SharedFinancialView with verified ContentHash from v8 infrastructure.

2. **Symmetry Proof**: For multi-party contexts, a SymmetryProof MUST confirm all parties received identical information.

3. **Explicit Bounded Authority**: Authority MUST satisfy all five bounds per Canon Addendum v9 §3.4:
   - Scope (what actions)
   - Amount (how much)
   - Time (until when)
   - Frequency (how often)
   - Explicit grant (not inferred)

4. **Per-Action Approval**: Each action instance requires its own approval. Standing approvals, blanket approvals, and category-wide approvals are forbidden per Canon Addendum v9 §5.4.

5. **Neutral Approval Language**: All approval prompts MUST be descriptive only, containing no urgency, fear, loss framing, authority language, or optimization framing per Canon Addendum v9 §3.6.

6. **Affirmative Validity Check**: At the moment of execution, an affirmative check MUST confirm all conditions still hold. Absence of revocation alone is insufficient per Canon Addendum v9 §8.3.

7. **Revocation Window Present**: A revocation window MUST exist unless the human explicitly waived it for this specific action per Canon Addendum v9 §3.5.

### 3.2 Silence Means No

If any precondition is not affirmatively satisfied, execution MUST NOT proceed. Ambiguity defaults to non-execution. Timeout defaults to non-execution. Missing approval defaults to non-execution.

---

## 4. ExecutionEnvelope v9 (Conceptual)

### 4.1 Required Fields

Every ExecutionEnvelope MUST contain:

| Field | Description |
|-------|-------------|
| EnvelopeID | Unique identifier for this envelope |
| ActorCircleID | The Circle initiating execution |
| IntersectionID | The Intersection context (if multi-party) |
| ViewHash | ContentHash of the referenced v8 SharedFinancialView |
| ActionHash | Cryptographic hash binding all action parameters |
| ActionSpec | Exact specification of what to execute |
| AmountCap | Maximum amount (hard ceiling) |
| FrequencyCap | Maximum frequency (if applicable) |
| DurationCap | Maximum duration of authority |
| Expiry | When this envelope becomes invalid |
| Approvals | List of Approval Artifacts with signatures |
| ApprovalThreshold | Required approval count and proof of satisfaction |
| RevocationWindowStart | When revocation window opened |
| RevocationWindowEnd | When revocation window closes |
| RevocationWaived | Boolean; true only if explicitly waived for this action |
| TraceID | Correlation ID for audit reconstruction |
| SealedAt | Timestamp when envelope was sealed |
| SealHash | Hash of all above fields proving immutability |

### 4.2 Forbidden Fields

ExecutionEnvelopes MUST NOT contain:

- Probabilistic scores or confidence values
- "Recommended" flags or suggestions
- "Urgency" indicators
- Optimization hints or preferences
- Batch identifiers linking to other executions
- Retry counters or retry policies
- Fallback specifications

### 4.3 Immutability Invariant

Once an ExecutionEnvelope is sealed (SealHash computed), no field may be modified. Any required change necessitates discarding the envelope and constructing a new one with fresh approvals.

---

## 5. Approval Graph Semantics

### 5.1 Single-Party Approval

For actions affecting only one Circle's resources:

- One Approval Artifact is required
- The artifact MUST be signed by an authorized member of that Circle
- The artifact MUST bind to the specific ActionHash
- The artifact MUST include a timestamp

### 5.2 Multi-Party Approval

For actions affecting multiple Circles' resources:

- Approval is required from all affected Circles
- Each Circle's Approval Artifact is independent
- The ExecutionEnvelope is valid only when all required approvals are present
- Approval order may be specified by the Intersection contract

### 5.3 Threshold Rules

Intersection contracts may specify approval thresholds:

- **Unanimous**: All parties must approve
- **Majority**: More than half must approve
- **Explicit Count**: N of M must approve

Threshold rules MUST be:

- Fixed at Intersection creation
- Visible to all parties
- Immutable without contract amendment (separate approval flow)

### 5.4 Symmetry Requirement

All approvers MUST receive identical information:

- Same view data (verified by ContentHash)
- Same action specification
- Same caps and limits
- Same approval prompt language

Asymmetric presentation to different approvers is forbidden.

### 5.5 Non-Repudiation Requirement

Every Approval Artifact MUST be:

- Cryptographically signed by the approver
- Bound to the specific ActionHash
- Timestamped with tamper-evident time
- Stored durably before execution proceeds

An approver cannot later deny approval if a valid artifact exists.

### 5.6 Forbidden Approval Patterns

The following are forbidden:

- **Blanket Approvals**: "Approve all payments under $X"
- **Standing Approvals**: "Approve payments to Merchant Y until revoked"
- **Approval Reuse**: Using one approval for multiple actions
- **Approval Inference**: Inferring approval from behavior or patterns
- **Predictive Approval**: Pre-approving anticipated future actions

---

## 6. Revocation Semantics (Mandatory)

### 6.1 Pre-Execution Revocation

If revocation occurs before execution begins:

- Execution MUST be blocked
- The ExecutionEnvelope MUST be marked invalid
- No partial effects may occur
- Revocation is immediate upon signal receipt

### 6.2 Mid-Execution Revocation

If revocation occurs during execution:

- Execution MUST halt at the next safe point
- "Safe point" means: no action in flight, state is consistent
- "Finish what you started" is forbidden
- Partial state MUST be recorded and surfaced

### 6.3 Post-Execution Irreversibility

After execution completes:

- Revocation cannot undo completed execution
- The system MUST honestly represent irreversibility
- No false "undo" claims
- Compensation (if available) is distinct from reversal

### 6.4 Revocation Signal Properties

Revocation signals MUST be:

- **Immediate**: Processed without queuing delay
- **Authoritative**: Any approver may revoke their approval
- **Durable**: Recorded with same durability as approvals
- **Propagated**: Reach execution plane within bounded time

---

## 7. Settlement Semantics (Financial)

### 7.1 Definition of Settled

An execution is "settled" when:

- The external financial system confirms irrevocable completion, OR
- The settlement window has closed without reversal, OR
- The destination confirms receipt

Until one of these conditions holds, status is "pending."

### 7.2 Atomicity Requirements

- No partial settlement without explicit representation
- If an action partially completes, the partial state MUST be recorded exactly
- Partial outcomes do not auto-complete
- Human decision required to proceed, retry, or abandon partial states

### 7.3 No Retries Without Fresh Approval

If execution fails:

- The system MUST NOT automatically retry
- Retry requires new Approval Artifacts
- The original ExecutionEnvelope is invalidated
- A new envelope with fresh approvals MUST be constructed

### 7.4 Failure Defaults to Non-Execution

If any failure occurs (network, timeout, ambiguous response):

- Default state is "not executed"
- Money stays where it was
- No "best effort" execution
- Human intervention required to resolve

---

## 8. Audit and Explainability (Financial Execution)

### 8.1 Mandatory Event Trail

Every execution MUST produce audit events for:

| Event | Description |
|-------|-------------|
| view.referenced | v8 view was referenced for this execution |
| approval.requested | Approval was requested from a party |
| approval.submitted | Approval artifact was submitted |
| approval.verified | Approval artifact was cryptographically verified |
| approval.expired | Approval artifact expired before use |
| envelope.sealed | ExecutionEnvelope was sealed and immutable |
| revocation.window.opened | Revocation window began |
| revocation.window.closed | Revocation window ended without revocation |
| revocation.received | Revocation signal was received |
| validity.checked | Affirmative validity check performed |
| execution.started | Execution began |
| execution.completed | Execution completed successfully |
| execution.blocked | Execution was blocked (precondition failed) |
| execution.aborted | Execution was aborted (revocation or error) |
| settlement.pending | Settlement is pending confirmation |
| settlement.settled | Settlement confirmed |
| settlement.failed | Settlement failed |
| settlement.disputed | Settlement is disputed |

### 8.2 Explainability Requirements

Every execution MUST be explainable in human-readable terms:

- **What happened**: Exact action taken
- **Why**: Reference to the Proposal and Commitment that led here
- **Whose authority**: Which Circles approved, with timestamps
- **Which caps**: What limits were in effect
- **Which approvals**: Each Approval Artifact, inspectable
- **Which validity check**: When and what was checked

### 8.3 Immutability of Audit Records

Audit records MUST be:

- Immutable once written
- Append-only (corrections are new entries referencing old)
- Reconstructable to human narrative
- Retained according to financial regulatory requirements

---

## 9. Forbidden Architectures (Explicit List)

The following architectural patterns are FORBIDDEN in any v9 implementation:

### 9.1 Background Executors

No component may execute financial actions in background processes, daemons, or scheduled jobs invisible to the user.

### 9.2 Schedulers and Cron Payments

No component may schedule future payments based on time alone. Scheduled actions require fresh validity checks and revocation window respect at execution time.

### 9.3 Auto-Retry

No component may automatically retry failed executions. Every retry requires fresh human approval.

### 9.4 Conditional Execution

No component may implement "if X then pay" logic that executes without human approval at the moment of execution.

### 9.5 Approval Batching or Optimization

No component may batch multiple actions under a single approval or optimize approval flows for reduced friction.

### 9.6 Asymmetric Data Presentation

No component may present different information to different approvers in the same multi-party approval flow.

### 9.7 ML-Driven Execution Decisions

No machine learning model may influence whether, when, or how execution proceeds.

### 9.8 Small Amount Exceptions

No component may bypass any precondition for "small amounts." All amounts receive identical treatment.

### 9.9 Default Execute Paths

No component may create flows where execution is the default outcome. Non-execution MUST always be the default.

---

## 10. Minimal Interfaces (Conceptual)

The following conceptual interfaces represent the minimum required boundaries between layers. Names indicate responsibilities only.

### 10.1 FinanceExecutionAuthorizer

**Responsibilities**:
- MUST verify all five Authority bounds are satisfied
- MUST verify per-action approval exists (no standing approvals)
- MUST verify approval language neutrality
- MUST reject if any bound is missing or exceeded

**MUST NOT**:
- Infer or assume authority
- Accept blanket or standing approvals
- Weaken bounds for convenience

### 10.2 ApprovalVerifier

**Responsibilities**:
- MUST cryptographically verify Approval Artifact signatures
- MUST verify ActionHash binding
- MUST verify timestamp validity
- MUST verify approval has not expired
- MUST verify approver is authorized for the Circle

**MUST NOT**:
- Accept unsigned or tampered artifacts
- Accept approvals bound to different ActionHashes
- Accept expired approvals

### 10.3 RevocationChecker

**Responsibilities**:
- MUST check for revocation signals before execution
- MUST check for revocation signals during execution
- MUST halt execution upon revocation detection
- MUST record revocation events

**MUST NOT**:
- Ignore revocation signals
- Delay revocation processing
- Allow "finish what you started"

### 10.4 ExecutionRunner

**Responsibilities**:
- MUST perform affirmative validity check before acting
- MUST execute exactly as specified in envelope
- MUST halt at safe points upon interrupt
- MUST record all state transitions

**MUST NOT**:
- Modify execution parameters
- Retry without instruction
- Proceed on ambiguous state
- Execute beyond envelope specification

### 10.5 SettlementRecorder

**Responsibilities**:
- MUST record pending, settled, failed, disputed states
- MUST record partial outcomes exactly
- MUST integrate with Memory layer

**MUST NOT**:
- Mark settled without confirmation
- Hide partial outcomes
- Auto-complete partial states

### 10.6 AuditLogger

**Responsibilities**:
- MUST log all mandatory events
- MUST ensure immutability of logs
- MUST support human-readable reconstruction
- MUST retain logs per regulatory requirements

**MUST NOT**:
- Omit any mandatory event
- Allow log modification
- Redact authority paths

---

## 11. Compliance Checklist (For Reviewers)

Before any v9 implementation is approved, reviewers MUST verify:

### Authority and Approval

- [ ] All five Authority bounds are enforced (scope, amount, time, frequency, explicit)
- [ ] Per-action approval is required (no standing/blanket approvals)
- [ ] Approval language is neutral (no urgency/fear/authority/optimization)
- [ ] Approval artifacts are signed, timestamped, and bound to ActionHash
- [ ] Multi-party approvals present identical information to all approvers

### Execution Control

- [ ] Affirmative validity check occurs at moment of execution
- [ ] Revocation windows exist and are enforced
- [ ] Revocation halts execution immediately
- [ ] No "finish what you started" behavior exists
- [ ] Silence/timeout defaults to non-execution

### Forbidden Patterns

- [ ] No background executors exist
- [ ] No auto-retry mechanisms exist
- [ ] No conditional execution without approval
- [ ] No approval batching or optimization
- [ ] No ML in execution decisions
- [ ] No small amount exceptions
- [ ] No default execute paths

### Audit and Settlement

- [ ] All mandatory events are logged
- [ ] Logs are immutable
- [ ] Human-readable reconstruction is possible
- [ ] Settlement states are explicit
- [ ] Partial outcomes are represented exactly
- [ ] Failures default to non-execution

---

**End of Technical Split v9**
