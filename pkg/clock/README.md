# pkg/clock

Deterministic clock abstraction for QuantumLife.

## Guardrail

**NEVER call `time.Now()` directly in core logic packages.**

The v9.6.2 guardrail enforces this rule via CI. Violations in the following directories will fail the build:

- `internal/demo*`
- `internal/*/impl_inmem`
- `internal/*/impl_*`
- `internal/connectors/*` (providers/adapters)

## Why?

1. **Deterministic Testing**: Tests must produce the same result regardless of when they run.
2. **Timezone Safety**: Ceiling checks (e.g., time windows) must use consistent time.
3. **Auditability**: The time used in authorization decisions must be traceable.

## Usage

```go
// Inject Clock into your struct
type Service struct {
    clock clock.Clock
}

// Production: use clock.NewReal() at entry points
func main() {
    svc := NewService(clock.NewReal())
}

// Tests: use clock.NewFixed() for determinism
func TestService(t *testing.T) {
    fixed := clock.NewFixed(time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC))
    svc := NewService(fixed)
    // ...
}
```

## Allowed Exceptions

Only these locations may call `time.Now()` directly:

1. `pkg/clock/` - This package (the canonical source)
2. `cmd/*` - Application entry points
3. Specific connector clients for HTTP timeout/header use (documented in guardrail script)

## Reference

- v9.6.2 Clock Guardrail
- `scripts/guardrails/forbidden_time_now.sh`
