# ADR-0012: Write Provider Registry Lock

**Status:** Accepted
**Date:** 2025-01-01
**Version:** v9.9

## Context

QuantumLife Canon v9 establishes strict constraints for financial execution. However, the ability to add new write providers (connectors that can move money) presented a drift risk: a developer could add a new provider without explicit approval, potentially bypassing the safety constraints established in the Canon.

Financial execution is the highest-risk area of the system. Adding new write providers should require:
1. Explicit code changes to the registry
2. CI guardrail approval
3. Review of Canon compliance

Without structural enforcement, "just code review" provides insufficient protection against:
- Accidental addition of unapproved providers
- Silent expansion of execution capabilities
- Live/production providers being used without explicit enablement

## Decision

Implement **Provider Registry Lock + Write Allowlist Enforcement** (v9.9) with two enforcement mechanisms:

### 1. Runtime Enforcement (Provider Registry)

Create an immutable provider registry that executors MUST consult before using any write connector.

**Registry Location:** `internal/connectors/finance/write/registry/registry.go`

**Default Allowlist:**
- `mock-write` - ALLOWED (simulated, never moves money)
- `truelayer-sandbox` - ALLOWED (sandbox testing environment)
- `truelayer-live` - EXISTS but BLOCKED by default

**Key Invariants:**
- Executors MUST call `registry.RequireAllowed(providerID)` before any write operation
- Blocked providers emit `v9.provider.blocked` audit events
- Live/production providers require explicit code changes to allow

### 2. CI Guardrail Enforcement

Create a guardrail script that scans the codebase for:
- Provider implementations that aren't registered
- Executors that don't consult the registry
- Provider IDs that don't match known constants

**Script:** `scripts/guardrails/forbidden_unregistered_write_provider.sh`

### WriteConnector Interface Extension

Extend the `WriteConnector` interface with:
```go
// ProviderID returns the canonical provider identifier for registry lookup.
ProviderID() string

// ProviderInfo returns the provider identifier and environment.
ProviderInfo() (id string, env string)
```

All write connectors MUST implement these methods.

## Alternatives Rejected

### "Just Code Review"
- Relies on human diligence
- No structural enforcement
- Silent additions possible
- Rejected: insufficient for financial execution safety

### "Config-Only Allowlist"
- Runtime configuration file for allowed providers
- Could be modified without code review
- Rejected: too easy to bypass

### "Compile-Time Type Constraints"
- Use Go generics or type system to restrict providers
- Over-engineered for current needs
- Rejected: registry pattern is simpler and sufficient

## Consequences

### Positive

- **Structural Safety**: New providers require code changes to registry
- **Audit Trail**: Blocked providers emit audit events
- **CI Enforcement**: Guardrail prevents silent additions
- **Live Protection**: Production providers blocked by default
- **Explicit Intent**: Adding providers requires deliberate action

### Negative

- **Slower Integration**: Adding new providers requires more steps
- **Breaking Change**: WriteConnector interface extended (minor)
- **Maintenance Burden**: Registry must be kept in sync with providers

### Mitigations

For slower integration:
1. Clear documentation on how to add providers
2. Registry changes are minimal (add entry, update allowlist)
3. CI provides immediate feedback

For interface changes:
1. ProviderID() can return Provider() value for backwards compatibility
2. Existing providers only need 2 new methods

## Implementation Notes

### Executor Integration

The registry check occurs BEFORE the ledger entry, ensuring blocked providers:
1. Don't create ledger entries
2. Don't trigger forced pause
3. Emit audit events immediately

```go
// Step 2.5 (v9.9): Check provider registry allowlist
providerID := registry.ProviderID(providerName)
if err := e.providerRegistry.RequireAllowed(providerID); err != nil {
    // Emit v9.provider.blocked event
    // Return blocked result
}
```

### Audit Events

New event types for v9.9:
- `v9.provider.registry.checked` - Registry lookup performed
- `v9.provider.allowed` - Provider passed allowlist check
- `v9.provider.blocked` - Provider blocked by registry
- `v9.provider.not_registered` - Unknown provider ID
- `v9.provider.live_blocked` - Live provider blocked by default

### Error Types

```go
var (
    ErrProviderNotRegistered = errors.New("provider not registered")
    ErrProviderNotAllowed    = errors.New("provider not on allowlist")
    ErrProviderLiveBlocked   = errors.New("live provider blocked by default")
)
```

## Canon Mapping

This ADR enforces Canon Addendum v9 red lines:

- **"No silent execution expansion"** - Registry prevents silent provider addition
- **"Bounded authority"** - Only allowlisted providers can execute
- **"Full audit trail"** - Blocked attempts are recorded
- **"Interruptible"** - Block occurs before any provider interaction

From TECHNICAL_SPLIT_V9_EXECUTION.md:
- **"Provider: TrueLayer ONLY"** - Registry enforces this constraint
- **"Sandbox mode default"** - Live providers blocked by default

## References

- QUANTUMLIFE_CANON_V1.md
- CANON_ADDENDUM_V9_FINANCIAL_EXECUTION.md
- TECHNICAL_SPLIT_V9_EXECUTION.md
- ADR-0010-no-background-execution-guardrail.md (Related: synchronous execution)
- ADR-0011-no-auto-retry-and-single-trace-finalization.md (Related: no retries)
