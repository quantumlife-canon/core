# ADR-0011: No Auto-Retry and Single Trace Finalization

**Status:** Accepted
**Date:** 2025-01-01
**Version:** v9.8

## Context

QuantumLife Canon Addendum v9 establishes critical invariants for financial execution:

1. **NO RETRIES**: Financial execution failures MUST require new approvals. Automatic retry loops create hidden execution paths that bypass the approval gate.

2. **EXACTLY-ONCE TRACE FINALIZATION**: Each execution attempt must finalize its audit trace exactly once. Double finalization corrupts the audit trail; missing finalization leaves orphaned traces.

These invariants are non-negotiable for financial safety:
- Retries could cause duplicate payments
- Retries bypass the "no standing approvals" rule
- Double finalization creates misleading audit records
- Missing finalization leaves incomplete audit trails

## Decision

Implement two CI guardrails that enforce these invariants at build time.

### Guardrail A: No Auto-Retry

**Script:** `scripts/guardrails/forbidden_auto_retry.sh`

**Forbidden Patterns:**

1. **Retry-related identifiers**
   - `Retry*`, `Retrier`, `Backoff*`, `ExponentialBackoff`
   - `Jitter*`, `WithRetry`, `MaxRetries`, `RetryPolicy`

2. **Execute/Prepare in loops**
   - `for` loops containing `.Execute(` or `.Prepare(` calls
   - Exception: Loops iterating approvals (validation, not execution)

3. **time.Sleep as backoff**
   - `time.Sleep` in files containing Execute/Prepare calls
   - Exception: `ForcedPauseDuration` (mandatory pre-execution pause)

4. **Error-handling retry**
   - Execute/Prepare calls inside `if err != nil` blocks

**Scanned Directories:**
- `internal/finance/execution/`
- `internal/connectors/finance/write/`
- `internal/action/`

**Excluded:**
- `*_test.go` - Tests may use patterns for test setup
- `cmd/` - Entry points not subject to execution constraints
- `internal/demo*/` - Demo code is not production runtime
- `pkg/clock/` - Clock abstraction is infrastructure

### Guardrail B: Single Trace Finalization

**Script:** `scripts/guardrails/single_trace_finalization.sh`

**Forbidden Patterns:**

1. **Multiple finalization calls in one function**
   - More than one call to finalization functions
   - Detected patterns: `FinalizeAttempt`, `emitAttemptFinalized`, `EmitTraceFinalized`
   - Event types: `EventV9AuditTraceFinalized`, `EventV95AttemptFinalized`, `EventV96AttemptFinalized`

2. **Defer AND explicit finalization**
   - `defer finalize(...)` combined with explicit `finalize(...)` in same function
   - Either pattern alone is allowed, but not both

**Allowed Patterns:**
- Exactly one `defer` finalization (guaranteed single execution)
- Exactly one explicit finalization (single code path)
- Helper functions that centralize finalization (called once from main function)

**Scanned Directories:**
- `internal/finance/execution/`
- `internal/audit/`
- `internal/action/`

## Consequences

### Positive

- **Payment safety**: No duplicate payments from retry loops
- **Approval integrity**: Every execution requires explicit, fresh approval
- **Audit accuracy**: Exactly-once trace records prevent corruption
- **Determinism**: Execution paths are explicit and auditable
- **Simpler debugging**: No hidden retry state to investigate

### Negative

- **Slower recovery**: Failures require human intervention for new approval
- **More explicit code**: Cannot use common retry patterns for resilience
- **Manual handling**: Transient failures (network blips) require re-initiation

### Mitigation

For transient failures:
1. The attempt ledger tracks failure state
2. Users can initiate a new attempt with new approval
3. Idempotency keys prevent duplicate execution if provider succeeded
4. Clear error messages guide users on next steps

## Implementation Notes

### No Auto-Retry Guardrail

The guardrail uses pattern matching to detect retry patterns:
- Regex patterns for retry identifiers
- AST-like line-by-line parsing for loop detection
- Context-aware checking for error blocks

False positives are minimized by:
- Allowing approval iteration loops (contain "approval" in context)
- Allowing forced pause patterns (contain "ForcedPauseDuration")
- Excluding test files and demo code

### Single Trace Finalization Guardrail

The guardrail tracks function boundaries and counts finalization calls:
- Parses function declarations
- Tracks brace depth for function scope
- Counts finalization calls within each function
- Distinguishes defer from explicit calls

False positives are minimized by:
- Allowing exactly one finalization (defer OR explicit)
- Ignoring functions without any finalization
- Excluding helper/utility functions

## How to Fix Violations

### Retry Violations

```go
// VIOLATION: Retry loop
for i := 0; i < 3; i++ {
    result, err := provider.Execute(ctx, req)
    if err == nil {
        return result, nil
    }
    time.Sleep(backoff)
}

// FIX: Single attempt, fail explicitly
result, err := provider.Execute(ctx, req)
if err != nil {
    return nil, fmt.Errorf("execution failed (new approval required): %w", err)
}
return result, nil
```

### Double Finalization Violations

```go
// VIOLATION: Defer + explicit
func execute() {
    defer emitAttemptFinalized()

    if err != nil {
        emitAttemptFinalized() // VIOLATION: double finalize
        return
    }
}

// FIX: Use defer only (covers all paths)
func execute() {
    defer emitAttemptFinalized()

    if err != nil {
        return
    }
}

// OR FIX: Use explicit only (single exit point)
func execute() {
    result := doWork()
    emitAttemptFinalized()
    return result
}
```

## References

- QUANTUMLIFE_CANON_V1.md
- CANON_ADDENDUM_V9_FINANCIAL_EXECUTION.md
- TECHNICAL_SPLIT_V9_EXECUTION.md (No Retries, Audit Requirements)
- ADR-0010-no-background-execution-guardrail.md (Related: synchronous execution)
