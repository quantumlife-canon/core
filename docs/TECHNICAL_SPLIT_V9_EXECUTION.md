# Technical Split v9: Financial Execution

**Status**: AUTHORITATIVE
**Subordinate to**: QUANTUMLIFE_CANON_V1.md, TECHNICAL_SPLIT_V1.md, CANON_ADDENDUM_V9_EXECUTION.md
**Effective**: Upon ratification
**Audience**: System architects and implementers

---

## Purpose

This document defines the technical boundaries that MUST govern financial execution. It specifies what structures exist, how they relate, and what behaviors are forbidden—without prescribing implementation technologies.

Implementation that violates these boundaries is defective, regardless of whether it achieves the desired outcome.

---

## 1. Control Plane vs Execution Plane (Reinforced)

### 1.1 Separation Is Absolute

The control plane and execution plane MUST be architecturally distinct. No component may span both planes. No data structure may serve both judgment and execution.

### 1.2 Control Plane Responsibilities

The control plane handles:

- Reading and aggregating financial data
- Computing views and proposals
- Validating authority and caps
- Constructing execution envelopes
- Recording approvals
- Verifying pre-conditions

The control plane MUST NOT:

- Initiate external financial operations
- Hold credentials for financial execution
- Make decisions during execution
- Modify execution parameters after envelope construction

### 1.3 Execution Plane Responsibilities

The execution plane handles:

- Receiving sealed execution envelopes
- Validating envelope integrity
- Executing the specified action exactly as specified
- Returning success, failure, or uncertainty
- Recording execution outcomes

The execution plane MUST NOT:

- Interpret intent
- Modify amounts, recipients, or timing
- Make judgment calls about whether to proceed
- Retry without explicit instruction
- Access financial data beyond what is in the envelope

### 1.4 The Execution Plane Is Dumb

"Dumb" is a design virtue in the execution plane. The execution plane does not think. It does not optimize. It does not infer. It receives an envelope, validates it, executes it, and reports the result. Intelligence belongs in the control plane. The execution plane is a faithful executor, not a smart one.

### 1.5 The Execution Plane Is Interruptible

At any point before execution completes, the execution plane MUST be interruptible. Revocation signals MUST halt execution. Timeout signals MUST halt execution. System shutdown MUST halt execution. The execution plane never "pushes through."

---

## 2. Execution Envelope (Conceptual)

### 2.1 Definition

An execution envelope is an immutable, signed data structure that contains everything needed to execute a financial action and nothing more. Once constructed, an envelope cannot be modified—only executed or discarded.

### 2.2 Required Contents

Every execution envelope MUST contain:

**Intent**
- The specific action to perform
- Unambiguous specification (no interpretation required)
- Action type and parameters

**Scope**
- What resources are affected
- Which accounts, instruments, or commitments
- Exactly one action (no batching without explicit multi-action envelopes)

**Caps**
- Maximum amount (absolute ceiling)
- Maximum frequency (if recurring)
- Maximum duration (if time-bound)

**Approval Artifacts**
- Signatures from all required approvers
- Timestamps of approval
- Hash binding approvals to this specific envelope

**Expiry**
- When this envelope becomes invalid
- Execution after expiry is forbidden

**Verification Hash**
- A cryptographic hash of all contents
- Any modification invalidates the envelope

### 2.3 Envelope Immutability

Once an envelope is constructed and signed, it MUST NOT be modified. There is no "update envelope." There is only "discard envelope and construct new one."

If conditions change and the envelope no longer reflects intent, the envelope is discarded. There is no in-flight editing.

### 2.4 Single Action Per Envelope

Each envelope represents exactly one atomic action. Batch operations require multiple envelopes, each independently approved. "Do these five things" is five envelopes, not one envelope with five items.

---

## 3. Approval Graph

### 3.1 Single-Party Approval

For actions affecting only one circle's resources:

- One approval is required
- Approval comes from an authorized member of that circle
- Approval artifact includes approver identity and timestamp

### 3.2 Multi-Party Approval

For actions affecting multiple circles' resources:

- Approval is required from all affected circles
- Each circle's approval is independent
- The envelope is valid only when all required approvals are present

### 3.3 Threshold-Based Approval

For actions governed by intersection contracts:

- The contract specifies the approval threshold
- Threshold may be unanimous, majority, or explicit count
- Threshold rules are fixed at contract creation
- Changing thresholds requires contract amendment (separate approval flow)

### 3.4 Approval Dependencies

Approvals MAY have dependencies:

- "A must approve before B can approve"
- "Approval is valid only if conditions X, Y, Z hold"

Dependencies are encoded in the approval graph, not inferred at execution time.

### 3.5 Expiry Semantics

Approvals expire independently of envelopes:

- If any required approval expires before execution, the envelope is invalid
- Expired approvals cannot be retroactively extended
- Renewal requires new approval, not extension of old approval

### 3.6 Non-Repudiation

Every approval MUST be:

- Cryptographically signed by the approver
- Bound to the specific envelope content hash
- Timestamped with tamper-evident time
- Stored durably before execution proceeds

An approver cannot later deny having approved if the artifact exists.

---

## 4. Revocation Semantics

### 4.1 Pre-Execution Revocation

Before execution begins:

- Any approver MAY revoke their approval
- Revocation invalidates the envelope
- Revocation is immediate upon receipt
- No delay, no confirmation, no waiting period

### 4.2 Mid-Execution Revocation

During execution:

- Revocation signals MUST be checked at all interruptible points
- Upon revocation signal, execution halts at the next safe point
- "Safe point" means: no action in progress, state is consistent
- Partial results are recorded and surfaced

### 4.3 Post-Execution Irreversibility Boundaries

After execution completes:

- Revocation cannot undo completed execution
- The system MUST clearly communicate when irreversibility boundary is crossed
- Users MUST understand: "after this point, revocation cannot undo"

### 4.4 Revocation Recording

Revocations are recorded with the same durability as approvals:

- Who revoked
- When they revoked
- What they revoked
- The state at revocation time

---

## 5. Settlement Semantics

### 5.1 What "Done" Means

An execution is "done" when:

- The external financial system has accepted the action
- Confirmation has been received and validated
- The outcome is recorded in the audit trail

"Done" does not mean "initiated." "Done" means "confirmed complete."

### 5.2 When Money Is Considered Moved

Money is considered moved when:

- The destination system confirms receipt, OR
- The originating system confirms irrevocable commitment, OR
- The settlement window has closed without reversal

Until one of these conditions holds, money is "in transit" and status is uncertain.

### 5.3 Partial Outcomes

If an execution partially completes:

- The partial state MUST be recorded exactly
- What succeeded and what did not MUST be distinguishable
- Partial outcomes do not trigger automatic completion
- Human decision is required to proceed, retry, or abandon

### 5.4 Uncertain Outcomes

If the system cannot determine whether execution succeeded:

- The state is recorded as "uncertain"
- No dependent actions proceed
- Human intervention is required to resolve
- The system does not guess

---

## 6. Audit and Explainability

### 6.1 Mandatory Event Trail

Every execution MUST produce an audit trail containing:

- Envelope construction (what, when, by whom)
- Approval events (each approval, when, by whom)
- Execution initiation (when, envelope hash)
- Execution outcome (success, failure, partial, uncertain)
- Settlement confirmation (when, from what source)
- Revocation events (if any)

### 6.2 Human-Readable Reconstruction

The audit trail MUST support reconstruction into human-readable narrative:

- "On [date], [approver] approved [action] for [amount] to [recipient]"
- "Execution began at [time] and completed at [time]"
- "Settlement confirmed by [source] at [time]"

Technical identifiers MUST map to human-comprehensible names.

### 6.3 No Redacted Authority Paths

The authority path—from approval to execution—MUST be fully visible. There are no hidden approvers, no system-level overrides, no redacted steps. If you can trace the execution, you can trace the authority.

### 6.4 Audit Immutability

Audit records MUST be immutable once written. There is no editing of history. Corrections are new entries that reference old entries, never modifications of old entries.

---

## 7. Forbidden Architectures

The following architectural patterns are FORBIDDEN in v9 execution:

### 7.1 Background Executors

No system component MAY execute financial actions without an active user session or explicit scheduling. "Background job that processes payments" is forbidden.

### 7.2 Auto-Retries

No system component MAY automatically retry failed executions. Retry requires new human approval. "Retry until success" is forbidden.

### 7.3 Optimizing Schedulers

No system component MAY reschedule executions to optimize timing, batching, or cost. Executions happen when approved to happen. "Wait for a better rate" is forbidden.

### 7.4 Machine Learning in Execution Decisions

No machine learning model MAY influence whether execution proceeds, when it proceeds, or how it proceeds. ML may inform the control plane; it MUST NOT touch the execution plane.

### 7.5 Asymmetric Data Planes

In multi-party contexts, all parties MUST operate on identical data. No system architecture MAY present different data to different parties in an attempt to obtain approval.

### 7.6 Implicit Authority Escalation

No system architecture MAY allow authority to escalate implicitly. If an action requires Level 2 approval, having Level 1 approval does not partially satisfy it. Authority is explicit and complete or it is absent.

### 7.7 Execution Without Read

No execution MAY proceed without first reading current state. Architectures that execute based on cached or stale data are forbidden. "Read-then-execute" is mandatory; "execute-based-on-memory" is forbidden.

### 7.8 Shared Execution Credentials

Execution credentials MUST NOT be shared across contexts. Each execution context has isolated credentials. Compromise of one context does not compromise others.

---

## 8. Relationship to Prior Technical Splits

### 8.1 Subordinate to v1

This document is subordinate to TECHNICAL_SPLIT_V1.md. Where v1 specifies a constraint, that constraint applies to v9. v9 adds constraints; it does not relax them.

### 8.2 Builds on v8

v8 established read-only financial data planes. v9 execution consumes v8 views. The integrity guarantees of v8 (symmetry, neutrality, determinism) are prerequisites for v9 execution safety.

### 8.3 Extends v6-v7

v6-v7 established execution patterns for calendar actions. v9 applies stricter versions of those patterns to financial execution, recognizing that money requires more caution than time.

---

## 9. Compliance Verification

Implementation compliance with this technical split MUST be verifiable through:

### 9.1 Static Analysis

- Control plane and execution plane code MUST be in separate modules
- Execution plane MUST NOT import control plane judgment components
- Envelope structures MUST be immutable by construction

### 9.2 Runtime Verification

- Execution MUST fail if envelope validation fails
- Revocation MUST halt execution within bounded time
- Expired approvals MUST be rejected

### 9.3 Audit Verification

- Every execution MUST have complete audit trail
- Audit trails MUST be reconstructible to human narrative
- Authority paths MUST be fully traceable

---

**End of Technical Split v9**
