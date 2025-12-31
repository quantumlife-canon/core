# ADR-0010: No Background Execution Guardrail

**Status:** Accepted
**Date:** 2025-01-01
**Version:** v9.7

## Context

QuantumLife Canon Addendum v9 establishes the invariant: **NO BACKGROUND EXECUTION**.

All financial execution in QuantumLife must be:
- Synchronous and explicit
- Auditable at every step
- Deterministic and reproducible
- Free of hidden retry/timer logic

Background execution patterns (goroutines, hidden timers, fire-and-forget callbacks) violate these invariants by creating invisible execution paths that cannot be audited, may fail silently, and introduce non-determinism.

## Decision

Implement a CI guardrail that **forbids background execution patterns** in core runtime packages.

### Forbidden Patterns

1. **Goroutines in core paths**
   - `go func(...)`
   - `go someFunc(...)`

2. **Timers/Tickers/Delayed callbacks**
   - `time.NewTicker(`
   - `time.NewTimer(`
   - `time.AfterFunc(`
   - `time.After(`

3. **Async/Background function names**
   - `func ...Async(...)`
   - `func ...Background(...)`

### Scanned Directories

The guardrail scans these core runtime packages:
- `internal/authority`
- `internal/action`
- `internal/execution`
- `internal/finance`
- `internal/intersection`
- `internal/negotiation`
- `internal/revocation`
- `internal/memory`
- `internal/audit`
- `internal/approval`
- `internal/connectors`

### Excluded

- `cmd/` - Entry points may use goroutines for server setup
- `internal/demo*/` - Demo code is not production runtime
- `*_test.go` - Tests may use async patterns for test setup
- `pkg/clock/` - Clock abstraction is infrastructure
- `scripts/`, `docs/` - Not Go code

### Allowed Exceptions

1. **HTTP Client Timeouts**: Connector `client.go` files may use `time.After` or `time.NewTimer` when used with `http.Client` or `context.WithTimeout` for request timeouts. This is synchronous blocking, not background execution.

2. **Forced Pause Implementation**: The v9.3 forced pause requirement uses `time.After` in a `select` statement for cancellation support. This is synchronous blocking (the goroutine waits), not background execution.

## Consequences

### Positive

- All execution paths are visible and auditable
- No hidden failures or silent retries
- Deterministic behavior for testing
- Simpler reasoning about system state

### Negative

- Cannot use async patterns for performance optimization
- Must design synchronous alternatives for concurrent operations
- Some patterns require refactoring to explicit orchestration

## How to Bypass

**You generally cannot bypass this guardrail.** If you need async behavior:

1. Move the code to `cmd/` or `internal/demo*/` (non-core paths)
2. Refactor to explicit synchronous orchestration
3. If truly necessary, add to the allowlist with documented justification

Adding to the allowlist requires:
- Documented justification in the guardrail script
- Review demonstrating no hidden execution
- Proof that the pattern is synchronous blocking, not fire-and-forget

## References

- QUANTUMLIFE_CANON_V1.md
- TECHNICAL_SPLIT_V9_EXECUTION.md §Forced Pause
- ACCEPTANCE_TESTS_V9_EXECUTION.md
- Canon Addendum v9: Financial Execution Invariants
