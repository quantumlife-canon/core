# ADR-0013: Payee Registry Lock + Free-Text Recipient Elimination

**Status:** Accepted
**Date:** 2025-01-01
**Version:** v9.10

## Context

QuantumLife Canon v9.9 established provider registry enforcement to control which write connectors can execute financial transfers. However, a similar risk existed for payment recipients: code could pass arbitrary free-text strings as recipients, bypassing any structural control over where money can be sent.

Free-text recipients present multiple risks:
1. **Injection attacks**: Malformed recipient data could exploit provider vulnerabilities
2. **Unauthorized destinations**: Any string could be passed, making money flow unpredictable
3. **Audit opacity**: Free-text recipients are harder to trace and verify
4. **No structural enforcement**: "Just code review" is insufficient for financial execution safety

This mirrors the provider registry problem (ADR-0012) but for the recipient side of transactions.

## Decision

Implement **Payee Registry Lock + Free-Text Recipient Elimination** (v9.10) with structural enforcement:

### 1. PayeeID Type (No Free-Text)

Replace all `Recipient string` fields with `PayeeID string` where:
- PayeeID MUST reference a registered payee
- No free-text recipients allowed anywhere in execution path
- Registry lookup required before any provider call

**Affected Types:**
- `ExecutionIntent.PayeeID` (was `Recipient`)
- `ActionSpec.PayeeID` (was `Recipient`)
- All demo/test code

### 2. Payee Registry

**Location:** `internal/connectors/finance/write/payees/registry.go`

**Registry Structure:**
```go
type Entry struct {
    ID                PayeeID
    DisplayName       string
    ProviderID        string    // Must match write provider
    Environment       Environment
    Allowed           bool
    AccountIdentifier string
    Currency          string
    BlockReason       string
}
```

**Default Allowlist:**
- `sandbox-utility` - ALLOWED (mock-write)
- `sandbox-rent` - ALLOWED (mock-write)
- `sandbox-merchant` - ALLOWED (mock-write)
- `sandbox-utility-tl` - ALLOWED (truelayer-sandbox)
- `sandbox-rent-tl` - ALLOWED (truelayer-sandbox)

**Key Invariants:**
- Unknown PayeeIDs → BLOCKED
- Live payees → BLOCKED by default
- Payee must match provider (or use mock-write for testing)

### 3. Executor Enforcement

Registry check occurs BEFORE ledger entry, immediately after provider check:

```go
// Step 2.6 (v9.10): Check payee registry allowlist
payeeID := payees.PayeeID(req.PayeeID)
if err := e.payeeRegistry.RequireAllowed(payeeID, providerName); err != nil {
    // Emit v9.payee.not_registered / v9.payee.not_allowed event
    // Return blocked result
}
```

### 4. CI Guardrail

**Script:** `scripts/guardrails/forbidden_free_text_recipient.sh`

Fails CI if:
- `Recipient string` found in write execution paths
- Payee registry is missing
- Executor doesn't use `payeeRegistry.RequireAllowed`

## Alternatives Rejected

### "Validate Recipient Format"
- Regex/format validation on free-text recipients
- Still allows arbitrary destinations
- Rejected: doesn't provide structural control

### "Payee Lookup by Name"
- Allow free-text that maps to registered payees
- Opens injection/typosquatting attacks
- Rejected: PayeeID-only is safer

### "Runtime Payee Registration"
- Allow registering payees during execution
- Defeats the purpose of pre-registration
- Rejected: payees must be registered before execution

## Consequences

### Positive

- **No Free-Text Recipients**: Architecturally impossible to pass arbitrary destinations
- **Audit Trail**: All payments go to known, registered payees
- **Provider Binding**: Payee must match provider (no cross-provider confusion)
- **CI Enforcement**: Guardrail prevents regression
- **Mirrors Provider Registry**: Consistent pattern (ADR-0012)

### Negative

- **Breaking Change**: All code using `Recipient` must change to `PayeeID`
- **Demo Updates**: All demos need registered PayeeIDs
- **Less Flexibility**: Cannot use arbitrary recipients (by design)

### Mitigations

For breaking changes:
1. Sed/replace tooling to update field names
2. Tests catch compile errors immediately
3. Guardrail provides clear failure messages

For flexibility:
1. Easy to add new sandbox payees for testing
2. Custom registry available for special test cases
3. Mock-write provider accepts any sandbox payee

## Audit Events

New event types for v9.10:
- `v9.payee.registry.checked` - Registry lookup performed
- `v9.payee.allowed` - Payee passed allowlist check
- `v9.payee.not_registered` - Unknown payee ID
- `v9.payee.not_allowed` - Payee explicitly blocked
- `v9.payee.live_blocked` - Live payee blocked by default
- `v9.payee.provider_mismatch` - Payee registered for different provider
- `v9.execution.blocked.invalid_payee` - Execution blocked due to payee validation

## Canon Mapping

This ADR enforces Canon principles:

- **§3.4 Bounded Authority**: Payments only to pre-registered destinations
- **§5.4 Full Audit Trail**: All payees are known and traceable
- **§7.5 No Silent Expansion**: Adding payees requires explicit code changes

From CANON_ADDENDUM_V9_FINANCIAL_EXECUTION.md:
- **"Pre-defined payees only"**: Registry enforces this constraint
- **"No free-text recipients"**: Structurally eliminated

## References

- QUANTUMLIFE_CANON_V1.md
- CANON_ADDENDUM_V9_FINANCIAL_EXECUTION.md
- TECHNICAL_SPLIT_V9_EXECUTION.md
- ADR-0012-write-provider-registry-lock.md (Related: provider registry)
