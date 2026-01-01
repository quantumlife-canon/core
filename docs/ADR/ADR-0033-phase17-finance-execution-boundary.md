# ADR-0033: Phase 17 - Finance Execution Boundary (Sandbox→Live) + Household Approvals

## Status
ACCEPTED

## Context
QuantumLife requires a financial execution boundary that integrates with Phase 15 household approvals. The system must:
- Execute payments only through mock connectors in sandbox mode
- Never move real money without explicit multi-party approval
- Enforce pre-defined payees (no free-text recipients)
- Use policy and view snapshot binding for drift protection
- Support idempotent execution with replay protection
- Integrate with household intersection approvals

### Real-World Scenario
Satish wants to pay £50 for a utility bill from the family joint account:
1. Creates payment draft with PaymentDraftContent
2. System checks intersection policy: requires both Satish and Wife approval
3. Both approve via signed approval tokens
4. System creates sealed ExecutionEnvelope with:
   - PolicySnapshotHash (v9.12 binding)
   - ViewSnapshotHash (v9.13 binding)
   - ActionHash (for approval binding)
5. Mock connector executes with Simulated=true
6. Receipt returned with NO REAL MONEY MOVED

## Decision

### Finance Write Connector Interface (`internal/connectors/finance/write/`)
```go
type WriteConnector interface {
    Provider() string
    ProviderID() string
    ProviderInfo() (id string, env string)
    Prepare(ctx context.Context, req PrepareRequest) (*PrepareResult, error)
    Execute(ctx context.Context, req ExecuteRequest) (*PaymentReceipt, error)
    Abort(ctx context.Context, envelopeID string) (bool, error)
}
```

### Mock Connector (`internal/connectors/finance/write/providers/mock/`)
```go
type Connector struct {
    config           write.WriteConfig
    payeeRegistry    payees.Registry
    executedPayments map[string]*write.PaymentReceipt  // Idempotency store
    abortedEnvelopes map[string]bool
    clock            func() time.Time  // Clock injection
    idGenerator      func(input string) string
}
```

CRITICAL GUARANTEES:
- `Simulated: true` always set on receipts
- `Status: PaymentSimulated` (never PaymentSucceeded)
- NO REAL MONEY ever moves
- Deterministic receipt generation
- Clock injection (no time.Now())
- No goroutines

### Payment Draft Content (`pkg/domain/draft/payment_content.go`)
```go
type PaymentDraftContent struct {
    PayeeID                    string   // Pre-defined payee (v9.10)
    AmountCents                int64    // Amount in minor units
    Currency                   string   // ISO currency code
    Description                string
    IntersectionID             string   // For household context
    RequiresMultiPartyApproval bool
    ApprovalThreshold          int
    RequiredApproverCircleIDs  []string
    EnvelopeID                 string   // Set after envelope creation
    ActionHash                 string   // For approval binding
}
```

### Payee Registry (v9.10) (`internal/connectors/finance/write/payees/`)
```go
type Registry interface {
    RequireAllowed(payeeID PayeeID, providerID string) error
    AllowedPayeeIDs() []string
    BlockedPayeeIDs() []string
}

const (
    PayeeSandboxUtility  PayeeID = "sandbox-utility"
    PayeeSandboxRent     PayeeID = "sandbox-rent"
    PayeeSandboxMerchant PayeeID = "sandbox-merchant"
)
```

Sandbox payees are:
- Allowed for mock-write provider
- Blocked for truelayer-live provider

### Provider Registry (v9.9) (`internal/connectors/finance/write/registry/`)
```go
type Registry interface {
    RequireAllowed(providerID ProviderID) error
    AllowedProviderIDs() []ProviderID
    BlockedProviderIDs() []ProviderID
}
```

### Household Approval Integration
Uses Phase 15 intersection policies:
```go
policy.AddRequirement(intersection.ApprovalRequirement{
    ActionClass:   intersection.ActionFinancePayment,
    RequiredRoles: []intersection.MemberRole{RoleOwner, RoleSpouse},
    Threshold:     2,  // Both must approve
    MaxAgeMinutes: 60,
})
```

Approval flow:
1. Create ApprovalState for payment draft
2. Generate signed approval tokens for each member
3. Record approvals as they arrive
4. Check threshold before execution
5. Any rejection blocks payment

### Execution Envelope
```go
type ExecutionEnvelope struct {
    EnvelopeID          string
    SealHash            string       // Immutable seal
    ActionHash          string       // For approval binding
    PolicySnapshotHash  string       // v9.12 binding
    ViewSnapshotHash    string       // v9.13 binding
    ActionSpec          ActionSpec
    Expiry              time.Time
    RevocationWindowEnd time.Time
    RevocationWaived    bool
    Revoked             bool
}
```

### Idempotency and Replay Protection (v9.6)
```go
// Same idempotency key returns same receipt
if existing, ok := c.executedPayments[req.IdempotencyKey]; ok {
    return existing, nil  // Replay blocked
}

// New execution stored for future idempotency
c.executedPayments[req.IdempotencyKey] = receipt
```

### Phase 17 Events
55 new events covering:
- Draft lifecycle: `phase17.finance.draft.generated`, etc.
- Envelope lifecycle: `phase17.finance.envelope.created`, etc.
- Policy/View verification: `phase17.finance.policy.verified`, etc.
- Approval events: `phase17.finance.approval.required`, etc.
- Execution events: `phase17.finance.execution.started/succeeded/failed`, etc.
- Idempotency: `phase17.finance.idempotency.replay_blocked`
- Caps: `phase17.finance.caps.checked/blocked`
- Provider/Payee: `phase17.finance.provider.allowed`, etc.

## Consequences

### Positive
- Financial execution has single deterministic boundary
- Mock connector guarantees no real money moves
- Household approvals integrate seamlessly
- Pre-defined payees eliminate free-text recipient risks
- Full idempotency prevents duplicate payments
- Policy/view binding prevents drift attacks
- Comprehensive audit trail via events

### Negative
- TrueLayer connector excluded from time.Now() guardrail (OAuth tokens)
- Sandbox-only operation limits real-world testing
- £1.00 cap extremely restrictive

### Constraints
- Mock connector MUST set Simulated=true
- Payees MUST be pre-defined (v9.10)
- Providers MUST be registered (v9.9)
- PolicySnapshotHash REQUIRED (v9.12)
- ViewSnapshotHash REQUIRED (v9.13)
- Clock injection REQUIRED (no time.Now() in mock)
- No goroutines
- No retries - failures require new approval

## References
- Canon v1: docs/QUANTUMLIFE_CANON_V1.md
- Technical Split v9: docs/TECHNICAL_SPLIT_V9_EXECUTION.md
- Phase 15 Approvals: docs/ADR/ADR-0031-phase15-household-approvals.md
- Phase 16 Notifications: docs/ADR/ADR-0032-phase16-notification-projection.md

## Files Changed
```
internal/connectors/finance/write/providers/mock/mock.go  (NEW)
pkg/domain/draft/payment_content.go                       (NEW)
pkg/events/events.go                                      (MODIFIED - 55 events)
internal/demo_phase17_finance_execution/demo_test.go      (NEW)
scripts/guardrails/finance_execution_enforced.sh          (NEW)
docs/ADR/ADR-0033-phase17-finance-execution-boundary.md   (NEW)
```
