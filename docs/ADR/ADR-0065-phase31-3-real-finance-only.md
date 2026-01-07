# ADR-0065: Phase 31.3 - Real Finance Sync → Commerce Observer (No Mock Path)

**Status:** Accepted
**Date:** 2025-01-07
**Version:** Phase 31.3

## Context

Phase 31.2 established TrueLayer transactions as a source for commerce observations. However, the initial implementation included mock transaction data for development purposes. This mock path creates risk:

1. **False confidence**: Mock data produces realistic-looking observations that don't reflect reality
2. **Testing gap**: Real API integration is never exercised
3. **Data provenance**: No clear audit trail distinguishing real from mock data

Phase 31.3 eliminates all mock transaction paths, ensuring:
- Only real TrueLayer API responses flow through the pipeline
- Mock/empty providers are explicitly rejected
- Clear audit trail via Provider field

## Decision

### 1. Provider Field Required

Add `Provider` field to `TransactionData` and `TransactionInput`:

```go
type ProviderKind string

const (
    ProviderTrueLayer ProviderKind = "truelayer"
    ProviderMock      ProviderKind = "mock"     // Exists only for rejection
    ProviderEmpty     ProviderKind = ""         // Exists only for rejection
)

type TransactionData struct {
    TransactionID      string
    Provider           ProviderKind  // REQUIRED, must be real
    ProviderCategory   string
    ProviderCategoryID string
    PaymentChannel     string
}
```

### 2. Provider Validation

```go
func ValidateProvider(p ProviderKind) error {
    if p == ProviderEmpty {
        return fmt.Errorf("phase31_3: provider is empty - real finance connection required")
    }
    if p == ProviderMock {
        return fmt.Errorf("phase31_3: mock provider rejected - real finance connection required")
    }
    if !IsValidProvider(p) {
        return fmt.Errorf("phase31_3: unknown provider %q - real finance connection required", p)
    }
    return nil
}

func IsValidProvider(p ProviderKind) bool {
    switch p {
    case ProviderTrueLayer:
        return true
    default:
        return false
    }
}
```

### 3. BuildFromTransactions Enforces Validation

```go
func (e *Engine) BuildFromTransactions(...) FinanceIngestResult {
    // Phase 31.3: Validate all transactions have a real provider
    for _, tx := range transactionData {
        if err := ValidateProvider(tx.Provider); err != nil {
            return FinanceIngestResult{
                Observations:     nil,
                OverallMagnitude: MagnitudeNothing,
                StatusHash:       "rejected_mock_provider",
            }
        }
    }
    // ... rest of processing
}
```

### 4. Mock Transactions Removed from Production

The `handleTrueLayerSync` handler no longer contains mock transaction data:

```go
// Before (Phase 31.2):
mockTransactions := []financetxscan.TransactionData{
    financetxscan.ExtractTransactionData("tx-001", "FOOD_AND_DRINK", ...),
    // ... more mock data
}

// After (Phase 31.3):
// Commerce observation building is skipped until real API integration
s.eventEmitter.Emit(events.Event{
    Type: events.Phase31_3RealFinanceReady,
    Metadata: map[string]string{
        "status": "awaiting_real_api",
    },
})
```

### 5. Events

```go
// Phase 31.3 events
Phase31_3RealFinanceReady          EventType = "phase31_3.real_finance.ready"
Phase31_3RealFinanceIngestStarted  EventType = "phase31_3.real_finance.ingest_started"
Phase31_3RealFinanceIngestComplete EventType = "phase31_3.real_finance.ingest_completed"
Phase31_3MockProviderRejected      EventType = "phase31_3.mock_provider.rejected"
```

## Consequences

### Positive

- **No false data**: Commerce observations are guaranteed to be from real sources
- **Clear audit trail**: Provider field enables tracing observation origin
- **Fail-fast**: Invalid providers are rejected immediately, not processed
- **Deterministic rejection**: `rejected_mock_provider` status is explicit

### Negative

- **No commerce observations until real API**: Until TrueLayer API is wired, no finance-sourced observations
- **Breaking change**: `ExtractTransactionData` signature changed to require Provider

### Mitigations

- Phase 31.3 event (`awaiting_real_api`) clearly indicates system is waiting for real integration
- Gmail receipt observers (Phase 31.1) still produce commerce observations
- Clear upgrade path when real TrueLayer API is added

## Implementation

### Files Modified

- `internal/financetxscan/model.go` - Added ProviderKind enum and validation
- `internal/financetxscan/engine.go` - Updated TransactionData, BuildFromTransactions, ExtractTransactionData
- `cmd/quantumlife-web/main.go` - Removed mock transactions from handleTrueLayerSync
- `pkg/events/events.go` - Added Phase 31.3 events

### Files Created

- `scripts/guardrails/commerce_real_finance_only_enforced.sh` - CI enforcement
- `internal/demo_phase31_3_real_finance_only/demo_test.go` - Tests
- `docs/ADR/ADR-0065-phase31-3-real-finance-only.md` - This document

## Hard Constraints (CI-Enforced)

- Provider field REQUIRED on TransactionData
- ValidateProvider function MUST exist and reject mock/empty
- NO mock transaction data in production paths
- ProviderTrueLayer is the ONLY valid provider
- Mock rejection returns "rejected_mock_provider" status

## References

- ADR-0064: Phase 31.2 - Commerce from Finance (TrueLayer → CommerceSignals)
- ADR-0062: Phase 31 - Commerce Observers (Silent by Default)
- ADR-0060: Phase 29 - TrueLayer Read-Only Connect
