# ADR-0031: Phase 15 - Household Approvals + Intersections (Deterministic)

## Status
ACCEPTED

## Context
QuantumLife needs multi-person approval workflows for household intersections. Wife should be able to approve/decline actions requiring multi-party consent (family calendar, shared expenses). The system must enforce intersection policies deterministically without accounts or authentication infrastructure.

### Real-World Scenario
Satish receives a calendar invite for Saturday family dinner. The response affects the whole family, so both Satish and Wife should approve before responding. The system:
1. Creates an approval state requiring both approvers
2. Generates signed approval tokens for each approver
3. Sends approval links via existing notification channels
4. Waits for threshold approvals before allowing execution
5. Blocks execution if rejected or expired

## Decision

### Intersection Policy Model (`pkg/domain/intersection/`)
```go
type IntersectionPolicy struct {
    IntersectionID string
    Name           string
    Members        []MemberRef       // {PersonID, Role, DisplayName}
    Requirements   []ApprovalRequirement
    CreatedAt      time.Time
    Version        int
    Hash           string           // SHA256 of canonical string
}

type ApprovalRequirement struct {
    ActionClass   ActionClass        // email_send, calendar_respond, finance_payment
    RequiredRoles []MemberRole       // owner, spouse, parent, child, guardian
    Threshold     int                // minimum approvals needed
    MaxAgeMinutes int                // approval freshness window
}
```

### Approval Flow Model (`pkg/domain/approvalflow/`)
```go
type ApprovalState struct {
    StateID           string
    TargetType        TargetType      // draft, execution_intent, envelope
    TargetID          string
    IntersectionID    string
    ActionClass       ActionClass
    RequiredApprovers []ApproverRef
    Threshold         int
    MaxAgeMinutes     int
    Approvals         []ApprovalRecord
    CreatedAt         time.Time
    ExpiresAt         time.Time
    Version           int
    Hash              string
}

type ApprovalRecord struct {
    PersonID  identity.EntityID
    Decision  Decision           // approved, rejected
    Timestamp time.Time
    Reason    string
    TokenID   string
}
```

### Status Computation
```go
func (s *ApprovalState) ComputeStatus(now time.Time) Status {
    if now.After(s.ExpiresAt) {
        return StatusExpired
    }
    for _, approval := range s.Approvals {
        if approval.Decision == DecisionRejected {
            return StatusRejected
        }
    }
    freshApprovals := countFreshApprovals(s.Approvals, s.MaxAgeMinutes, now)
    if freshApprovals >= s.Threshold {
        return StatusApproved
    }
    return StatusPending
}
```

### Signed Approval Tokens (`pkg/domain/approvaltoken/`)
```go
type Token struct {
    TokenID            string
    StateID            string
    PersonID           identity.EntityID
    ActionType         ActionType      // approve, reject
    CreatedAt          time.Time
    ExpiresAt          time.Time
    SignatureAlgorithm string          // Ed25519
    KeyID              string
    Signature          []byte
    Hash               string
}
```

Tokens are:
- Deterministically generated (same inputs → same ID)
- Ed25519 signed for security
- URL-safe encoded for link-based approvals
- Time-bounded (expire with the approval state)

### Persistence (`internal/persist/approval_ledger.go`)
Uses Phase 12 storelog with new record types:
- `INTERSECTION_POLICY` - Policy definitions
- `APPROVAL_STATE_CREATE` - New approval states
- `APPROVAL_STATE_RECORD` - Approval decisions
- `APPROVAL_TOKEN_CREATE` - Token creation
- `APPROVAL_TOKEN_REVOKE` - Token revocation

### Events (`pkg/events/events.go`)
Phase 15 events cover:
- Intersection policy lifecycle
- Approval state lifecycle
- Approval record events
- Token lifecycle
- Execution gating
- Web approval flow
- Ledger operations

## Absolute Constraints
1. **stdlib only** - No external dependencies
2. **No goroutines** - Synchronous operations only
3. **No time.Now()** - Clock injection required
4. **Deterministic** - Same inputs + clock → same outputs
5. **Ed25519 signed** - All tokens cryptographically verified
6. **Append-only ledger** - No record modification or deletion

## Consequences

### Positive
- Multi-person approval without account infrastructure
- Cryptographically secure approval links
- Deterministic status computation for testing
- Full audit trail via storelog
- Threshold-based approval (not all-or-nothing)

### Negative
- Token management complexity
- Approval freshness requires re-approval after expiry
- No real-time approval status updates (poll-based)

### Neutral
- Web-first approach (mobile browser friendly)
- Links work without authentication
- Approval tokens are single-use

## Member Roles
- `owner` - Primary account holder
- `spouse` - Partner with equal authority
- `parent` - Parent in child-related intersections
- `child` - Dependent child
- `guardian` - Legal guardian

## Action Classes
- `email_send` - Sending emails
- `calendar_respond` - Responding to calendar invites
- `calendar_create` - Creating calendar events
- `finance_payment` - Financial payments
- `finance_transfer` - Fund transfers

## Files Created
```
pkg/domain/intersection/
├── types.go           # Intersection policy model
└── types_test.go      # Determinism tests

pkg/domain/approvalflow/
├── types.go           # Approval state model
└── types_test.go      # Status computation tests

pkg/domain/approvaltoken/
├── types.go           # Signed token model
└── types_test.go      # Token encode/decode tests

internal/persist/
├── approval_ledger.go     # Ledger persistence
└── approval_ledger_test.go

internal/demo_phase15_household_approvals/
└── demo_test.go       # Demo scenarios

scripts/guardrails/
└── household_approvals_enforced.sh

docs/ADR/
└── ADR-0031-phase15-household-approvals.md
```

## Guardrail Script
`scripts/guardrails/household_approvals_enforced.sh` validates:
1. Deterministic hashing in all domain packages
2. No goroutines in approval domain
3. No time.Now() in approval domain
4. Storelog usage in ledger
5. Phase 15 events defined
6. Demo tests exist

## References
- Phase 12: Persistence & Replay (storelog foundation)
- Phase 14: Circle Policies + Preference Learning (policy patterns)
- `pkg/crypto/agility.go` (Ed25519 signing)
