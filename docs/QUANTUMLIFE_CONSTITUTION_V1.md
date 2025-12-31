# QuantumLife Constitution v1

## Document Status

| Field | Value |
|-------|-------|
| Version | 1.0 |
| Status | **RATIFIED** |
| Author | QuantumLife Founder |
| Date | 2025-01-01 |
| Authority | Non-negotiable product and system law |

---

## Preamble

This Constitution establishes the non-negotiable principles that govern QuantumLife. No feature, optimization, or business pressure may violate these provisions. Any code, design, or policy that conflicts with this Constitution is void.

---

## Article I: North Star

### Section 1.1: The Measure of Success

**"Nothing Needs You"** — The default home state must display this message when no obligations require attention. A quiet system means success. A noisy system means failure.

### Section 1.2: Primary Metrics

| Metric | Target | Rationale |
|--------|--------|-----------|
| Interruptions per day | < 3 | Silence is success |
| Regrets prevented per week | > 5 | Value delivered |
| Time reclaimed per day | > 30 minutes | Measurable benefit |
| Trust score (action taken on recommendation) | > 80% | System earns trust |

### Section 1.3: Anti-Metrics (Must NOT Optimize)

| Anti-Metric | Prohibition |
|-------------|-------------|
| Engagement time | We want LESS screen time |
| Notification open rate | More opens = more interruptions = failure |
| Daily active usage | Ideal user opens app rarely |
| Feature usage counts | Features exist to reduce friction, not increase interaction |

---

## Article II: Core Rights

### Section 2.1: User Sovereignty

1. **Circle Ownership**: Every Circle is sovereign. The user owns their data, policy, and memory.
2. **Data Portability**: Users may export all their data at any time in standard formats.
3. **Right to Deletion**: Users may delete any data, including memory and audit logs.
4. **No Lock-In**: Users may leave with full data export. No artificial barriers.

### Section 2.2: Transparency and Audit

1. **Explainability**: Any decision (interrupt, suppress, propose) must be explainable on demand.
2. **Full Audit Trail**: Every action, decision, and state change is logged immutably.
3. **Reconstruction**: Any past state must be reconstructible from audit logs.
4. **No Hidden Logic**: No opaque ML models that cannot explain their outputs.

### Section 2.3: Consent for Writes

1. **Explicit Approval**: ALL irreversible actions require explicit user approval.
2. **No Silent Execution**: The system NEVER acts on the real world without consent.
3. **No Standing Approvals**: Each action requires fresh approval. No blanket permissions.
4. **Revocation Window**: Users may revoke approval during forced pause before execution.

### Section 2.4: Safe Defaults

1. **Default Silent**: New integrations default to read-only, silent mode.
2. **Default Restrictive**: Caps, rate limits, and allowlists default to most restrictive.
3. **Default Private**: Data sharing defaults to none until explicitly enabled.
4. **Fail Safe**: On error, the system fails toward inaction, not action.

---

## Article III: Interruption Rights

### Section 3.1: Earned Interruptions Only

**Principle**: Every notification must be earned by preventing future regret. If an interruption does not prevent regret, it is forbidden.

### Section 3.2: Interruption Levels

| Level | Trigger | Example |
|-------|---------|---------|
| SILENT | Regret < 0.2 | Newsletter arrived |
| AMBIENT | 0.2 ≤ Regret < 0.4 | Package shipped |
| QUEUED | 0.4 ≤ Regret < 0.6 | Bill due next week |
| NOTIFY | 0.6 ≤ Regret < 0.8 | Form due tomorrow |
| URGENT | Regret ≥ 0.8 | Fraud alert, emergency |

### Section 3.3: Rate Limits

1. **Per-Circle Limit**: Maximum 3 NOTIFY+ interruptions per circle per day.
2. **Global Limit**: Maximum 5 NOTIFY+ interruptions total per day.
3. **Burst Protection**: Maximum 2 NOTIFY+ interruptions per hour.
4. **Night Silence**: No interruptions between 22:00-07:00 except URGENT (regret ≥ 0.95).

### Section 3.4: User Override

1. Users may dismiss any interruption. Dismissal is permanent for that item.
2. Users may adjust thresholds per circle.
3. Users may enable "Do Not Disturb" for any duration.
4. System respects these overrides absolutely.

---

## Article IV: Execution Constitution

### Section 4.1: v9+ Action Core Invariants

ALL write actions MUST comply with these invariants:

| Version | Invariant | Enforcement |
|---------|-----------|-------------|
| v9.6 | Idempotency + Replay Defense | Deterministic keys, attempt ledger |
| v9.7 | No Background Execution | Core packages never spawn goroutines |
| v9.8 | No Auto-Retry | Failed actions fail permanently |
| v9.8 | Single Trace Finalization | Exactly one terminal state per attempt |
| v9.9 | Write Provider Registry Lock | Only allowlisted providers |
| v9.10 | Payee Registry Lock | Only registered payees |
| v9.11 | Daily Caps + Rate Limits | Per-circle, per-payee, per-currency |
| v9.12 | Policy Snapshot Binding | Hash verification prevents drift |
| v9.13 | View Freshness Binding | Stale views block execution |

### Section 4.2: Approval Requirements

1. **Single-Party**: Personal accounts under caps require one approval.
2. **Multi-Party**: Joint accounts require threshold approvals (e.g., 2-of-2).
3. **Symmetry**: All approvers receive identical approval payload.
4. **Neutral Language**: No urgency, fear, shame, or authority in approval requests.

### Section 4.3: Forced Pause

1. All executions have a mandatory pause before provider call.
2. Minimum pause: 3 seconds for payments, 2 seconds for other writes.
3. Revocation during pause aborts execution BEFORE provider contact.
4. Pause cannot be bypassed by any means.

---

## Article V: Non-Goals

### Section 5.1: QuantumLife Must NOT Become

| Forbidden Pattern | Rationale |
|-------------------|-----------|
| Engagement-optimized app | We optimize for silence, not screen time |
| Notification spam system | Interruptions are earned, not sprayed |
| AI overlord | System proposes, user decides. Always. |
| Social network | No feeds, shares, likes, or social graphs |
| General chatbot | Purpose-built for life management only |
| Gamification platform | No streaks, badges, points, or rewards |
| Advertising vehicle | No ads, no data monetization |
| Workflow automation | Propose actions, never automate unseen |

### Section 5.2: Absolute Prohibitions

1. **No Silent Writes**: System NEVER changes external state without approval.
2. **No Data Sales**: User data is NEVER sold or shared for advertising.
3. **No Dark Patterns**: No tricks to increase engagement or prevent leaving.
4. **No Surveillance**: No tracking beyond what user explicitly enables.
5. **No AI Boss**: System suggests; user commands. Never the reverse.

---

## Article VI: Amendment Process

### Section 6.1: Amendment Requirements

1. **Founder Approval**: All amendments require explicit founder approval.
2. **Written Rationale**: Changes must have documented justification.
3. **Version Bump**: Each amendment increments the version number.
4. **Downstream Review**: All impacts on code/design must be identified.

### Section 6.2: Immutable Provisions

The following provisions MAY NOT be amended:

1. Article II, Section 2.1 (User Sovereignty)
2. Article II, Section 2.3 (Consent for Writes)
3. Article IV, Section 4.1 (v9+ Invariants)
4. Article V, Section 5.2 (Absolute Prohibitions)

---

## Article VII: Enforcement

### Section 7.1: Build-Time Enforcement

Guardrail scripts enforce constitutional provisions at build time:
- Forbidden terms detection
- Forbidden imports detection
- No time.Now() in core (use injected Clock)
- No background execution in core
- No auto-retry patterns
- Provider and payee registry enforcement
- Policy and view snapshot enforcement

### Section 7.2: Runtime Enforcement

- All execution paths emit audit events
- Caps and rate limits enforced before provider calls
- Approval verification before any write
- View freshness checked before execution

### Section 7.3: Violation Response

Constitutional violations:
1. Block the violating code/action
2. Emit audit event recording the violation
3. Fail safe (no action taken)
4. Alert for human review

---

## Signatures

```
Ratified: 2025-01-01
Authority: QuantumLife Founder

This Constitution is binding on all code, design, and policy.
Violations are void. Silence is success.
```

---

## Document History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2025-01-01 | Initial ratification |
