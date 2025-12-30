# ADR-0009: v9 Execution Technical Split

**Status**: Accepted
**Date**: 2024-12-30
**Deciders**: Constitutional Architects
**Subordinate to**: QUANTUMLIFE_CANON_V1.md, CANON_ADDENDUM_V9_EXECUTION.md

---

## Context

QuantumLife v8 established financial READ and PROPOSE capabilities. These capabilities proved the system can be trusted with financial information through accuracy, neutrality, and symmetry. v9 introduces financial EXECUTION—the power to move money and create binding commitments.

Execution is categorically different from reading:

| Property | Reading (v8) | Execution (v9) |
|----------|--------------|----------------|
| Reversibility | Always reversible | Often irreversible |
| Failure cost | Lost information | Lost money |
| Trust recovery | Immediate | May be permanent |
| Consent scope | Implicit (viewing) | Explicit (acting) |
| Error tolerance | High | Near-zero |

The existing v6 calendar execution model provides a foundation, but financial execution requires stricter gates. Calendar events can be rescheduled; money transfers cannot be "unexecuted."

v9 requires its own Technical Split because:

1. **Power demands constraint**: Greater capability requires greater safeguards
2. **Irreversibility demands precision**: No ambiguity can exist in execution preconditions
3. **Trust demands slowness**: Execution must be deliberately slower than reading
4. **Sovereignty demands consent**: Every action requires explicit, bounded, per-instance approval

---

## Decision

Adopt TECHNICAL_SPLIT_V9_EXECUTION.md as the mandatory boundary document for all v9 financial execution implementation.

### Core Boundaries Adopted

1. **Hard Gate**: No external financial effect without ALL preconditions satisfied
2. **Per-Action Approval**: No standing, blanket, or reusable approvals
3. **Affirmative Validity Check**: Positive confirmation required at execution moment
4. **Revocation Windows**: Mandatory unless explicitly waived per action
5. **Neutral Approval Language**: No persuasion in approval flows
6. **Multi-Party Symmetry**: Identical information for all approvers
7. **Failure Defaults to Non-Execution**: Ambiguity means stop
8. **Forbidden Architectures**: Explicit list of banned patterns

---

## Alternatives Rejected

### Alternative 1: Reuse v6 Execute Model Without Finance-Specific Gates

**Proposal**: Apply the existing v6 calendar execution model directly to financial execution.

**Rejection Rationale**:
- v6 was designed for calendar actions where errors are correctable
- Financial execution has irreversibility that calendar execution does not
- v6 does not require per-action approval (Canon Addendum v9 §5.4 requires this)
- v6 does not mandate affirmative validity checks (Canon Addendum v9 §8.3 requires this)
- v6 approval flows may contain urgency framing (Canon Addendum v9 §3.6 forbids this)

**Canon Conflict**: Would violate §3.6, §5.4, §8.3 of Canon Addendum v9.

---

### Alternative 2: Standing Approvals for Convenience

**Proposal**: Allow users to create standing approvals such as "approve all payments under $100 to Merchant X."

**Rejection Rationale**:
- Standing approvals enable execution without per-action human engagement
- Standing approvals cannot be bound to a specific ActionHash (no immutability guarantee)
- Standing approvals create attack surface for unauthorized execution
- Standing approvals drift toward "set and forget" behavior
- Standing approvals are explicitly forbidden by Canon Addendum v9 §5.4

**Canon Conflict**: Directly violates §5.4: "Approval MUST be bound to a specific action instance. Standing approvals... are forbidden."

---

### Alternative 3: Auto-Retry for Reliability

**Proposal**: Automatically retry failed executions to improve reliability and user experience.

**Rejection Rationale**:
- Auto-retry executes without fresh human consent
- Network failures may be transient but state may have changed
- Retry on ambiguous state may cause double-execution
- User may have changed their mind between attempts
- Canon Addendum v9 §3.2 requires no default execution; auto-retry creates default execution path

**Canon Conflict**: Violates §3.2 (no default execution) and §3.4 (explicit, bounded authority for each action).

---

### Alternative 4: Asymmetric Approvals to Reduce Friction

**Proposal**: Present simplified approval prompts to secondary approvers while showing full details to primary approvers.

**Rejection Rationale**:
- Asymmetric information enables manipulation of approvers
- Secondary approvers cannot give informed consent without full information
- Violates multi-party symmetry requirement
- Creates legal liability if approvers later claim they were misled
- Canon Addendum v9 §6.3 requires symmetric information

**Canon Conflict**: Directly violates §6.3: "Multi-party approval MUST be... symmetric in information (all approvers see identical data)."

---

## Consequences

### Positive Consequences

1. **Higher Trust**: Users can trust that execution only happens with explicit, informed consent
2. **Audit Clarity**: Every execution is fully traceable with human-readable explanation
3. **Legal Defensibility**: Clear consent trail for every financial action
4. **Drift Resistance**: Forbidden architectures prevent gradual erosion of safeguards
5. **Multi-Party Fairness**: All parties in shared contexts have equal information and power

### Negative Consequences (Accepted by Design)

1. **Slower Execution**: Deliberate friction adds time to execution flows
2. **Higher User Effort**: Each action requires explicit approval (no "set and forget")
3. **Reduced Convenience Features**: Cannot implement common "helpful" patterns (auto-pay, smart retry)
4. **Audit Overhead**: Comprehensive logging increases storage and processing requirements
5. **Development Constraints**: Forbidden architectures limit implementation options

### Trade-off Acceptance

These negative consequences are accepted because:
- Speed is not a virtue in financial execution; safety is
- User effort is consent; reduced effort is reduced consent
- "Helpful" patterns that bypass consent are harmful patterns
- Audit overhead is the cost of accountability
- Constrained implementation prevents constrained trust

---

## Mapping to Canon Addendum v9

| Technical Split Section | Canon Addendum v9 Section |
|------------------------|---------------------------|
| §3.1 Mandatory Precondition 4 (Per-Action Approval) | §5.4 No Standing or Blanket Approval |
| §3.1 Mandatory Precondition 5 (Neutral Language) | §3.6 Neutral Approval Language |
| §3.1 Mandatory Precondition 6 (Affirmative Validity) | §8.3 Read Before Write (affirmative check) |
| §3.1 Mandatory Precondition 7 (Revocation Window) | §3.5 No Execution Without Revocation Windows |
| §3.1 Mandatory Precondition 3 (Five Bounds) | §3.4 No Execution Without Explicit, Bounded Authority |
| §3.2 Silence Means No | §3.2 No Default Execution |
| §5.4 Symmetry Requirement | §6.3 Thresholds and Symmetry Are Mandatory |
| §5.6 Forbidden Approval Patterns | §5.4 No Standing or Blanket Approval |
| §6.2 Mid-Execution Revocation | §3.5 Revocation Windows |
| §7.3 No Retries Without Fresh Approval | §3.4 Authority for each action |
| §7.4 Failure Defaults to Non-Execution | §3.2 No Default Execution |
| §9.1 Forbidden: Background Executors | §3.3 No Background Execution |
| §9.3 Forbidden: Auto-Retry | §3.4 Explicit authority per action |
| §9.7 Forbidden: ML-Driven Decisions | §4.1 Agents Recommend, Humans Decide |

---

## Compliance Verification

Implementation compliance MUST be verified through:

1. **Architectural Review**: Confirm no forbidden patterns exist
2. **Approval Flow Audit**: Confirm per-action approval with neutral language
3. **Revocation Testing**: Confirm revocation halts execution
4. **Failure Testing**: Confirm failures default to non-execution
5. **Multi-Party Testing**: Confirm symmetric information presentation
6. **Audit Reconstruction**: Confirm human-readable explanation is always possible

---

## References

- QUANTUMLIFE_CANON_V1.md
- CANON_ADDENDUM_V9_EXECUTION.md
- HUMAN_GUARANTEES_V9_EXECUTION.md
- TECHNICAL_SPLIT_V9_EXECUTION.md
- TECHNICAL_SPLIT_V1.md (for v6 execute model context)

---

**End of ADR-0009**
