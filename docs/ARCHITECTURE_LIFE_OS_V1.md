# QuantumLife Architecture: Life Operating System v1

## Document Status

| Field | Value |
|-------|-------|
| Version | 1.0 |
| Status | Draft |
| Author | QuantumLife Core Team |
| Date | 2025-01-01 |
| Related | QUANTUMLIFE_END_STATE_V1.md, INTERRUPTION_CONTRACT_V1.md |

---

## 1. System Overview

QuantumLife operates as a closed-loop system with seven stages:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        QUANTUMLIFE CLOSED LOOP                               │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   ┌─────────┐    ┌─────────┐    ┌─────────┐    ┌─────────┐                 │
│   │  SENSE  │───▶│  MODEL  │───▶│ DECIDE  │───▶│ PROPOSE │                 │
│   └─────────┘    └─────────┘    └─────────┘    └─────────┘                 │
│        │                                             │                       │
│        │                                             ▼                       │
│   ┌─────────┐    ┌─────────┐    ┌─────────┐    ┌─────────┐                 │
│   │  LEARN  │◀───│  AUDIT  │◀───│ EXECUTE │◀───│ APPROVE │                 │
│   └─────────┘    └─────────┘    └─────────┘    └─────────┘                 │
│        │                                             ▲                       │
│        │                                             │                       │
│        └──────────────── feedback ──────────────────┘                       │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 2. Stage Definitions

### 2.1 SENSE (Data Ingestion)

**Purpose**: Ingest data from external systems into QuantumLife's internal model.

**CRITICAL: No Background Execution Compliance (v9.7)**

The v9.7 guardrail prohibits goroutines/timers in core packages. Therefore:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         INGESTION ARCHITECTURE                               │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   EXTERNAL SERVICES              │    CORE PACKAGES (v9.7 compliant)        │
│   ──────────────────             │    ──────────────────────────────        │
│                                  │                                           │
│   ┌──────────────────┐           │    ┌──────────────────┐                  │
│   │  Sync Scheduler  │           │    │  Ingestion       │                  │
│   │  (separate proc) │──────────▶│    │  Handler         │                  │
│   │  - cron jobs     │  HTTP     │    │  (synchronous)   │                  │
│   │  - user-trigger  │  request  │    │                  │                  │
│   └──────────────────┘           │    └──────────────────┘                  │
│                                  │             │                             │
│   ┌──────────────────┐           │             ▼                             │
│   │  External APIs   │           │    ┌──────────────────┐                  │
│   │  - Gmail         │◀──────────│    │  Adapter Layer   │                  │
│   │  - Plaid         │  API call │    │  (read-only)     │                  │
│   │  - Calendar      │           │    │                  │                  │
│   └──────────────────┘           │    └──────────────────┘                  │
│                                  │             │                             │
│                                  │             ▼                             │
│                                  │    ┌──────────────────┐                  │
│                                  │    │  Item Store      │                  │
│                                  │    │  (PostgreSQL)    │                  │
│                                  │    └──────────────────┘                  │
│                                  │                                           │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Ingestion Trigger Types**:

| Trigger | Description | v9.7 Compliance |
|---------|-------------|-----------------|
| User Pull | User taps "refresh" in app | Compliant: user-initiated |
| Scheduled Sync | External cron process calls HTTP endpoint | Compliant: separate process |
| Webhook | External service pushes to endpoint | Compliant: request-driven |

**NOT ALLOWED**: Background goroutines polling APIs in core packages.

---

### 2.2 MODEL (State Building)

**Purpose**: Transform raw ingested data into structured domain models.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           MODEL LAYER                                        │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   Raw Items                    Domain Model                                  │
│   ─────────                    ────────────                                  │
│                                                                              │
│   ┌─────────────┐             ┌─────────────────────────────────┐           │
│   │ Email       │────────────▶│ Item                            │           │
│   │ - subject   │             │ - item_id (deterministic)       │           │
│   │ - from      │             │ - circle_id                     │           │
│   │ - body      │             │ - source_type                   │           │
│   │ - timestamp │             │ - classification                │           │
│   └─────────────┘             │ - obligation (if any)           │           │
│                               │ - requires_action: bool         │           │
│   ┌─────────────┐             └─────────────────────────────────┘           │
│   │ Transaction │                          │                                 │
│   │ - amount    │                          ▼                                 │
│   │ - merchant  │             ┌─────────────────────────────────┐           │
│   │ - date      │             │ CircleView                      │           │
│   └─────────────┘             │ - circle_id                     │           │
│                               │ - pending_count                 │           │
│   ┌─────────────┐             │ - obligation_count              │           │
│   │ Calendar    │             │ - next_deadline                 │           │
│   │ Event       │             │ - summary_text                  │           │
│   │ - title     │             │ - view_hash (v9.13)             │           │
│   │ - time      │             └─────────────────────────────────┘           │
│   └─────────────┘                          │                                 │
│                                            ▼                                 │
│                               ┌─────────────────────────────────┐           │
│                               │ Obligation                      │           │
│                               │ - obligation_id                 │           │
│                               │ - source_item_id                │           │
│                               │ - deadline                      │           │
│                               │ - regret_score                  │           │
│                               │ - status (pending/done/expired) │           │
│                               └─────────────────────────────────┘           │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

### 2.3 DECIDE (Interruption Logic)

**Purpose**: Determine whether and when to interrupt the user.

See INTERRUPTION_CONTRACT_V1.md for full specification.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        DECISION ENGINE                                       │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   Input: Item + CircleView + ObligationSet + UserContext                    │
│                                                                              │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                    INTERRUPTION EVALUATOR                            │   │
│   ├─────────────────────────────────────────────────────────────────────┤   │
│   │                                                                      │   │
│   │   1. Calculate regret_score(item, context)                          │   │
│   │      └─▶ P(user regrets not seeing this) ∈ [0.0, 1.0]               │   │
│   │                                                                      │   │
│   │   2. Check threshold: regret_score > circle.interrupt_threshold?    │   │
│   │      └─▶ Work: 0.3, Family: 0.5, Finance: 0.7                       │   │
│   │                                                                      │   │
│   │   3. Check time_relevance: deadline within attention_horizon?       │   │
│   │      └─▶ Urgent: 24h, Important: 7d, Routine: never interrupt       │   │
│   │                                                                      │   │
│   │   4. Check rate_limit: daily_interrupts < max_daily?                │   │
│   │      └─▶ Default: 5/day                                             │   │
│   │                                                                      │   │
│   │   5. Check dedup: not already_interrupted(item)?                    │   │
│   │      └─▶ Hash-based deduplication with time window                  │   │
│   │                                                                      │   │
│   │   6. Check circle_allows: circle.interrupt_schedule.allows(now)?    │   │
│   │      └─▶ Work: weekdays 9-18, Family: always, etc.                  │   │
│   │                                                                      │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│   Output: InterruptDecision { level, reason, suppress_until }               │
│                                                                              │
│   Levels: SILENT → AMBIENT → QUEUED → NOTIFY → URGENT                       │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

### 2.4 PROPOSE (Action Drafting)

**Purpose**: Generate proposed actions for user review.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        PROPOSAL ENGINE                                       │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   Triggered when: Item.requires_action == true                              │
│                                                                              │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                    DRAFT GENERATOR                                   │   │
│   ├─────────────────────────────────────────────────────────────────────┤   │
│   │                                                                      │   │
│   │   Email Reply:                                                       │   │
│   │   ┌───────────────────────────────────────────────────────────────┐ │   │
│   │   │ Input: original_email, context, user_style_profile            │ │   │
│   │   │ Output: DraftEnvelope { reply_text, tone, suggested_actions } │ │   │
│   │   └───────────────────────────────────────────────────────────────┘ │   │
│   │                                                                      │   │
│   │   Calendar Response:                                                 │   │
│   │   ┌───────────────────────────────────────────────────────────────┐ │   │
│   │   │ Input: invite, calendar_state, conflict_check                 │ │   │
│   │   │ Output: DraftEnvelope { response_type, message, alt_times }   │ │   │
│   │   └───────────────────────────────────────────────────────────────┘ │   │
│   │                                                                      │   │
│   │   Payment:                                                           │   │
│   │   ┌───────────────────────────────────────────────────────────────┐ │   │
│   │   │ Input: bill/invoice, payee_lookup, balance_check              │ │   │
│   │   │ Output: ExecutionEnvelope (v9+ canon)                         │ │   │
│   │   └───────────────────────────────────────────────────────────────┘ │   │
│   │                                                                      │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│   All drafts include:                                                        │
│   - PolicySnapshotHash (v9.12): state at draft creation                     │
│   - ViewSnapshotHash (v9.13): view state at draft creation                  │
│   - ExpiresAt: draft validity window (default 48h)                          │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

### 2.5 APPROVE (User Consent)

**Purpose**: Obtain explicit user approval before any execution.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        APPROVAL GATE                                         │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   SINGLE-PARTY APPROVAL (Personal Account, Under Cap)                       │
│   ─────────────────────────────────────────────────────                     │
│                                                                              │
│   User ──▶ Review Draft ──▶ Tap "Approve" ──▶ Execute                       │
│                 │                                                            │
│                 ├──▶ Tap "Edit" ──▶ Modify ──▶ Approve ──▶ Execute          │
│                 │                                                            │
│                 └──▶ Tap "Reject" ──▶ Draft Discarded                       │
│                                                                              │
│                                                                              │
│   MULTI-PARTY APPROVAL (Joint Account, High Value, Intersection)            │
│   ──────────────────────────────────────────────────────────────            │
│                                                                              │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                                                                      │   │
│   │   Initiator ──▶ Create Envelope ──▶ Request Approval                │   │
│   │                                           │                          │   │
│   │                                           ▼                          │   │
│   │                    ┌─────────────────────────────────────┐          │   │
│   │                    │     APPROVAL BUNDLE (v9.4)          │          │   │
│   │                    │     - envelope_id                   │          │   │
│   │                    │     - action_hash                   │          │   │
│   │                    │     - policy_snapshot_hash (v9.12)  │          │   │
│   │                    │     - view_snapshot_hash (v9.13)    │          │   │
│   │                    │     - required_approvers            │          │   │
│   │                    │     - threshold                     │          │   │
│   │                    │     - content_hash (symmetry proof) │          │   │
│   │                    └─────────────────────────────────────┘          │   │
│   │                                           │                          │   │
│   │                              ┌────────────┼────────────┐             │   │
│   │                              ▼            ▼            ▼             │   │
│   │                         Approver 1   Approver 2   Approver N        │   │
│   │                              │            │            │             │   │
│   │                              └────────────┼────────────┘             │   │
│   │                                           ▼                          │   │
│   │                              Threshold Met? ──▶ Execute              │   │
│   │                                                                      │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

### 2.6 EXECUTE (Action Execution)

**Purpose**: Execute approved actions against external systems.

**This is where the v9+ canon applies universally.**

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                     UNIVERSAL EXECUTION ENVELOPE                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   ExecutionEnvelope (Universal)                                              │
│   ─────────────────────────────                                              │
│   │                                                                          │
│   │   Common Fields:                                                         │
│   │   - envelope_id: string (deterministic)                                  │
│   │   - circle_id: string                                                    │
│   │   - intersection_id: string (optional)                                   │
│   │   - action_class: ActionClass (email|calendar|finance|message|...)      │
│   │   - action_hash: string (content hash)                                   │
│   │   - seal_hash: string (envelope integrity)                               │
│   │   - policy_snapshot_hash: string (v9.12)                                 │
│   │   - view_snapshot_hash: string (v9.13)                                   │
│   │   - created_at: time.Time                                                │
│   │   - expires_at: time.Time                                                │
│   │   - approval_threshold: int                                              │
│   │   - approvals: []ApprovalArtifact                                        │
│   │                                                                          │
│   │   Action-Specific Payload:                                               │
│   │   - action_spec: interface{} (EmailSend|CalendarResponse|Payment|...)   │
│   │                                                                          │
│   └──────────────────────────────────────────────────────────────────────────│
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                     ACTION-SPECIFIC EXECUTORS                                │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │   EmailExecutor                                                      │   │
│   │   - Validates: policy_snapshot_hash, view_snapshot_hash              │   │
│   │   - Checks: rate limits (v9.11 pattern), provider allowlist          │   │
│   │   - Executes: Gmail API / Outlook API send                           │   │
│   │   - Audit: full trace, no retry on failure                           │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │   CalendarExecutor                                                   │   │
│   │   - Validates: policy_snapshot_hash, view_snapshot_hash              │   │
│   │   - Checks: conflict detection, calendar allowlist                   │   │
│   │   - Executes: Google Calendar API / Apple Calendar API               │   │
│   │   - Audit: full trace, no retry on failure                           │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │   FinanceExecutor (v9+ Canon - EXISTING)                            │   │
│   │   - Full v9.3-v9.13 implementation                                   │   │
│   │   - TrueLayer provider, payee registry, caps, multi-party            │   │
│   │   - Forced pause, revocation window, idempotency                     │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │   MessageExecutor                                                    │   │
│   │   - Validates: policy_snapshot_hash, view_snapshot_hash              │   │
│   │   - Checks: rate limits, contact allowlist                           │   │
│   │   - Executes: WhatsApp Business API / iMessage (future)              │   │
│   │   - Audit: full trace, no retry on failure                           │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

**v9+ Constraints Applied to ALL Executors:**

| Constraint | Source | Application |
|------------|--------|-------------|
| No background execution | v9.7 | Execution is synchronous, request-driven |
| No auto-retry | v9.8 | Failure is terminal; user must re-initiate |
| Single trace finalization | v9.8 | Each attempt has exactly one outcome |
| Policy snapshot binding | v9.12 | Envelope bound to policy hash; drift blocks |
| View freshness binding | v9.13 | Envelope bound to view hash; stale blocks |
| Idempotency | v9.6 | Replay defense via idempotency key |
| Explicit approval | v9.4 | Multi-party gate for intersections |
| Full audit | All | Every step logged with context |

---

### 2.7 AUDIT (Logging & Reconstruction)

**Purpose**: Record every decision for accountability and debugging.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        AUDIT SYSTEM                                          │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   Event Types:                                                               │
│   ─────────────                                                              │
│                                                                              │
│   SENSE Events:                                                              │
│   - item.ingested: Raw item received from external system                   │
│   - item.classified: Item assigned to circle with classification            │
│   - item.deduplicated: Item identified as duplicate, skipped                │
│                                                                              │
│   DECIDE Events:                                                             │
│   - interrupt.evaluated: Interruption decision made                         │
│   - interrupt.suppressed: Item suppressed (with reason)                     │
│   - interrupt.queued: Item queued for user attention                        │
│   - interrupt.notified: Push notification sent                              │
│                                                                              │
│   PROPOSE Events:                                                            │
│   - draft.created: Draft envelope generated                                 │
│   - draft.expired: Draft expired without action                             │
│                                                                              │
│   APPROVE Events:                                                            │
│   - approval.requested: Approval request sent                               │
│   - approval.granted: User approved action                                  │
│   - approval.rejected: User rejected action                                 │
│   - approval.expired: Approval window expired                               │
│                                                                              │
│   EXECUTE Events (v9+ canon):                                                │
│   - v9.envelope.created                                                      │
│   - v9.policy.snapshot.bound (v9.12)                                        │
│   - v9.view.snapshot.bound (v9.13)                                          │
│   - v9.execution.started                                                     │
│   - v9.execution.blocked.*                                                   │
│   - v9.execution.completed                                                   │
│   - v9.attempt.finalized                                                     │
│                                                                              │
│   LEARN Events:                                                              │
│   - feedback.implicit: User behavior indicates preference                   │
│   - feedback.explicit: User provides direct feedback                        │
│   - model.updated: System model adjusted                                    │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                        AUDIT EVENT SCHEMA                                    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   {                                                                          │
│     "event_id": "evt_abc123",           // Deterministic ID                 │
│     "event_type": "interrupt.evaluated", // Type from catalog               │
│     "timestamp": "2025-01-15T09:30:00Z", // Injected clock (v9.6.2)         │
│     "trace_id": "trace_xyz789",          // Correlation ID                  │
│     "circle_id": "circle_work",          // Context                         │
│     "subject_id": "item_email_456",      // What this is about              │
│     "subject_type": "email",             // Type of subject                 │
│     "actor_id": "user_satish",           // Who/what caused this            │
│     "decision": "suppress",              // Outcome                         │
│     "reason": "below_threshold",         // Why                             │
│     "metadata": {                        // Additional context              │
│       "regret_score": 0.25,                                                 │
│       "threshold": 0.30,                                                    │
│       "circle_policy": "work_default"                                       │
│     }                                                                        │
│   }                                                                          │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

### 2.8 LEARN (Feedback Loop)

**Purpose**: Improve system behavior based on user actions.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        LEARNING SYSTEM                                       │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   Implicit Signals:                                                          │
│   ─────────────────                                                          │
│   - User dismissed item without action → lower regret score for similar     │
│   - User acted on item quickly → higher regret score for similar            │
│   - User edited draft significantly → adjust generation style               │
│   - User rejected draft → negative signal for that pattern                  │
│                                                                              │
│   Explicit Signals:                                                          │
│   ─────────────────                                                          │
│   - "Don't show me this again" → permanent suppression rule                 │
│   - "This was important" → boost regret score for similar                   │
│   - Threshold adjustment → user changes interrupt threshold                 │
│                                                                              │
│   Learning Constraints:                                                      │
│   ────────────────────                                                       │
│   - NO unsupervised model updates in production                             │
│   - Learning adjusts parameters, not core logic                             │
│   - All adjustments logged to audit trail                                   │
│   - User can always override learned behavior                               │
│   - Learning is per-user, never cross-user                                  │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 3. Data Flow Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        COMPLETE DATA FLOW                                    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   EXTERNAL WORLD                  QUANTUMLIFE                  USER          │
│   ──────────────                  ───────────                  ────          │
│                                                                              │
│   Gmail ─────────┐                                                           │
│   Outlook ───────┤                                                           │
│   Calendar ──────┤    ┌───────────────────────────────────┐                 │
│   Plaid ─────────┼───▶│         INGESTION SERVICE         │                 │
│   WhatsApp ──────┤    │      (separate process, v9.7)     │                 │
│   GitHub ────────┤    └───────────────┬───────────────────┘                 │
│   School Portal ─┘                    │                                      │
│                                       ▼                                      │
│                       ┌───────────────────────────────────┐                 │
│                       │           ITEM STORE              │                 │
│                       │         (PostgreSQL)              │                 │
│                       └───────────────┬───────────────────┘                 │
│                                       │                                      │
│                                       ▼                                      │
│                       ┌───────────────────────────────────┐                 │
│                       │        MODEL SERVICE              │                 │
│                       │  - Classification                 │                 │
│                       │  - Obligation extraction          │                 │
│                       │  - Summarization                  │                 │
│                       └───────────────┬───────────────────┘                 │
│                                       │                                      │
│                                       ▼                                      │
│                       ┌───────────────────────────────────┐                 │
│                       │       DECISION ENGINE             │                 │
│                       │  - Interruption evaluation        │◀─── User        │
│                       │  - Draft generation               │     Context     │
│                       └───────────────┬───────────────────┘                 │
│                                       │                                      │
│                                       ▼                                      │
│                       ┌───────────────────────────────────┐                 │
│                       │         DRAFT STORE               │                 │
│                       │  - Pending proposals              │                 │
│                       │  - Approval state                 │                 │
│                       └───────────────┬───────────────────┘                 │
│                                       │                                      │
│                                       ▼                                      │
│                       ┌───────────────────────────────────┐    ┌─────────┐  │
│                       │          iOS APP                  │───▶│  USER   │  │
│                       │  - Nothing Needs You              │    │         │  │
│                       │  - Circle views                   │◀───│ Approve │  │
│                       │  - Draft review                   │    │ Reject  │  │
│                       └───────────────┬───────────────────┘    │ Edit    │  │
│                                       │                        └─────────┘  │
│                                       ▼                                      │
│                       ┌───────────────────────────────────┐                 │
│                       │      EXECUTION ENGINE             │                 │
│                       │  (v9+ Canon, core package)        │                 │
│                       │  - Policy validation              │                 │
│                       │  - View freshness                 │                 │
│                       │  - Provider call                  │                 │
│                       │  - Audit logging                  │                 │
│                       └───────────────┬───────────────────┘                 │
│                                       │                                      │
│   Gmail ◀────────────────────────────┼────────────────────────────────────  │
│   Calendar ◀─────────────────────────┤   (Write actions)                    │
│   TrueLayer ◀────────────────────────┘                                      │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 4. Package Structure

```
quantumlife/
├── cmd/
│   ├── api/                    # Main API server (core)
│   ├── sync/                   # Ingestion service (separate process, v9.7 compliant)
│   └── worker/                 # Background jobs (separate process, v9.7 compliant)
│
├── internal/
│   ├── circles/                # Circle domain model
│   ├── items/                  # Item storage and retrieval
│   ├── obligations/            # Obligation extraction and tracking
│   ├── interrupts/             # Interruption decision engine
│   ├── drafts/                 # Draft generation and storage
│   ├── finance/
│   │   └── execution/          # v9+ canon (EXISTING)
│   ├── email/
│   │   ├── adapters/           # Gmail, Outlook adapters
│   │   └── execution/          # Email executor (mirrors finance patterns)
│   ├── calendar/
│   │   ├── adapters/           # Google, Apple adapters
│   │   └── execution/          # Calendar executor (mirrors finance patterns)
│   ├── integrations/           # External service adapters (read-only)
│   └── audit/                  # Audit event logging
│
├── pkg/
│   ├── domain/                 # Shared domain types
│   ├── events/                 # Event definitions (EXISTING)
│   ├── clock/                  # Injected clock (EXISTING, v9.6.2)
│   └── policy/                 # Policy snapshot types
│
├── docs/                       # This documentation
│
└── scripts/
    └── guardrails/             # CI guardrail scripts (EXISTING)
```

---

## 5. Security Model

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        SECURITY BOUNDARIES                                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   Authentication:                                                            │
│   ───────────────                                                            │
│   - User auth via Clerk (existing)                                          │
│   - OAuth tokens encrypted at rest (AES-256)                                │
│   - Tokens scoped to minimum required permissions                           │
│                                                                              │
│   Authorization:                                                             │
│   ──────────────                                                             │
│   - All actions require explicit user approval                              │
│   - Multi-party actions require all party approvals                         │
│   - No system-initiated writes without approval                             │
│                                                                              │
│   Data Protection:                                                           │
│   ────────────────                                                           │
│   - All data encrypted at rest                                              │
│   - All data encrypted in transit (TLS 1.3)                                 │
│   - No cross-user data access (single-tenant model initially)               │
│   - Audit logs immutable (append-only)                                      │
│                                                                              │
│   External API Security:                                                     │
│   ──────────────────────                                                     │
│   - Provider allowlist (v9.9 pattern) for all integrations                  │
│   - Rate limiting on all external calls                                     │
│   - Credential rotation supported                                           │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 6. Deployment Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        DEPLOYMENT TOPOLOGY                                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                         AZURE (UK South)                             │   │
│   │                                                                      │   │
│   │   ┌─────────────┐   ┌─────────────┐   ┌─────────────┐              │   │
│   │   │   API Pod   │   │  Sync Pod   │   │ Worker Pod  │              │   │
│   │   │   (core)    │   │ (separate)  │   │ (separate)  │              │   │
│   │   │             │   │             │   │             │              │   │
│   │   │ - HTTP API  │   │ - Cron      │   │ - LLM calls │              │   │
│   │   │ - Execution │   │ - Webhooks  │   │ - Summaries │              │   │
│   │   │ - Auth      │   │ - Polling   │   │ - Classify  │              │   │
│   │   └──────┬──────┘   └──────┬──────┘   └──────┬──────┘              │   │
│   │          │                 │                 │                      │   │
│   │          └─────────────────┼─────────────────┘                      │   │
│   │                            │                                        │   │
│   │                            ▼                                        │   │
│   │                    ┌───────────────┐                                │   │
│   │                    │  PostgreSQL   │                                │   │
│   │                    │  (Azure DB)   │                                │   │
│   │                    └───────────────┘                                │   │
│   │                            │                                        │   │
│   │                            ▼                                        │   │
│   │                    ┌───────────────┐                                │   │
│   │                    │    Redis      │                                │   │
│   │                    │   (Cache)     │                                │   │
│   │                    └───────────────┘                                │   │
│   │                                                                      │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│   v9.7 Compliance:                                                           │
│   - API Pod: Core packages, no background goroutines                        │
│   - Sync Pod: Separate process, handles polling/webhooks                    │
│   - Worker Pod: Separate process, handles async LLM/summarization           │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 7. Document History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2025-01-01 | Core Team | Initial version |
