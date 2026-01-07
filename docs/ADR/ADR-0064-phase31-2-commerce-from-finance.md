# ADR-0064: Phase 31.2 - Commerce from Finance (TrueLayer -> CommerceSignals)

**Status:** Accepted
**Date:** 2025-01-07
**Version:** Phase 31.2

## Context

Phase 31.1 established Gmail receipts as a source for commerce observations. However, email receipts only capture a subset of transactions. Bank transactions provide more complete coverage.

Phase 31.2 extends the Commerce Observer pipeline to accept TrueLayer transaction sync data as an additional source. This creates a multi-rail ingestion system:

```
                    ┌─────────────────────────────┐
                    │  Commerce Observer Store    │
                    │  (Hash-only, abstract)      │
                    └─────────────┬───────────────┘
                                  ▲
                    ┌─────────────┴───────────────┐
                    │                             │
        ┌───────────┴───────────┐   ┌─────────────┴───────────┐
        │  Phase 31.1           │   │  Phase 31.2             │
        │  Gmail Receipts       │   │  TrueLayer Transactions │
        │  SourceGmailReceipt   │   │  SourceFinanceTrueLayer │
        └───────────────────────┘   └─────────────────────────┘
```

### Why Bank Transactions Complement Email Receipts

1. **Coverage**: Bank transactions capture all card purchases, not just those with email receipts.

2. **Consistency**: Bank-assigned categories (MCC codes) provide standardized classification.

3. **Reliability**: Transactions are authoritative - no missed or spam-filtered emails.

4. **Complementary**: Email receipts add context (items, delivery status) that bank data lacks.

### What Is NOT Used

To preserve privacy, transaction classification uses ONLY:
- `ProviderCategory` - Bank-assigned category (e.g., "FOOD_AND_DRINK")
- `ProviderCategoryID` - MCC code (e.g., "5812")
- `PaymentChannel` - Payment type (e.g., "online", "in_store")

NEVER used:
- `MerchantName` - Reveals spending patterns
- `Amount` - Reveals financial state
- `TransactionDate` - Reveals temporal patterns beyond period buckets
- Raw `TransactionID` - Hashed immediately

## Decision

Implement Finance Transaction Observers with these core properties:

### 1. Privacy Model (Same as Phase 31.1)

**What IS stored:**
- Source kind: `finance_truelayer`
- Category bucket: `food_delivery` | `transport` | `retail` | `subscriptions` | `utilities` | `other`
- Frequency bucket: `rare` | `occasional` | `frequent`
- Stability bucket: `stable` | `drifting` | `volatile`
- Evidence hash: SHA256 of abstract classification tokens
- Period: ISO week format (e.g., "2024-W03")

**What is NOT stored:**
- Merchant names
- Transaction amounts
- Raw transaction IDs
- Transaction timestamps

### 2. Classification Pipeline

```
┌─────────────────────────────────────────────────────────────────┐
│  Transaction (in memory only, NOT stored)                       │
│  - TransactionID (hashed immediately)                           │
│  - ProviderCategory (used for classification, then discarded)   │
│  - ProviderCategoryID (used for classification, then discarded) │
│  - PaymentChannel (used for classification, then discarded)     │
│  - MerchantName (NEVER used)                                    │
│  - Amount (NEVER used)                                          │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│  Transaction Classification (internal/financetxscan/)           │
│  - Priority 1: ProviderCategory match (high confidence)         │
│  - Priority 2: MCC code match (medium confidence)               │
│  - Priority 3: PaymentChannel inference (low confidence)        │
│  - Build evidence hash from ABSTRACT tokens only                │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│  Observation Building (internal/financetxscan/)                 │
│  - Aggregate signals by category                                │
│  - Select top 3 categories (deterministic priority)             │
│  - Convert counts to frequency buckets                          │
│  - Map confidence to stability buckets                          │
│  - Set Source = SourceFinanceTrueLayer                          │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│  Persistence (existing Phase 31 store)                          │
│  - Append-only storage                                          │
│  - Hash-only (no raw data)                                      │
│  - 30-day bounded retention                                     │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│  /mirror/commerce (existing Phase 31 page)                      │
│  - "Seen, quietly." title                                       │
│  - Max 3 category chips                                         │
│  - 1-2 calm lines                                               │
│  - NO counts, merchants, or amounts                             │
└─────────────────────────────────────────────────────────────────┘
```

### 3. Classification Confidence Levels

| Source             | Confidence | Maps to Stability |
|-------------------|------------|-------------------|
| ProviderCategory  | High       | Stable            |
| MCC Code          | Medium     | Drifting          |
| PaymentChannel    | Low        | Volatile          |

### 4. MCC Code Mappings (Subset)

| MCC Range   | Category        |
|-------------|-----------------|
| 5811-5814   | food_delivery   |
| 4111-4131   | transport       |
| 4121        | transport       |
| 5541-5542   | transport       |
| 5311        | retail          |
| 5815-5818   | subscriptions   |
| 4900        | utilities       |
| 6300-6399   | utilities       |

### 5. SourceKind for Audit Trail

A new `SourceKind` enum distinguishes observation origins:

```go
type SourceKind string

const (
    SourceGmailReceipt     SourceKind = "gmail_receipt"
    SourceFinanceTrueLayer SourceKind = "finance_truelayer"
)
```

CommerceObservation now includes Source field for audit trails.

## Integration Point

Phase 31.2 hooks into the TrueLayer sync handler (`/run/truelayer-sync`):

```go
// After sync completes and FinanceSyncReceipt is created:
if s.financeTxScanEngine != nil {
    // Extract transaction data (ProviderCategory, MCC, PaymentChannel ONLY)
    transactionData := extractTransactionData(transactions)

    // Build observations
    result := engine.BuildFromTransactions(circleID, period, syncHash, transactionData)

    // Persist to existing Phase 31 store
    for _, obs := range result.Observations {
        store.PersistObservation(circleID, &obs)
    }

    // Emit Phase 31.2 events
}
```

## Consequences

### Positive

- Bank transactions flow through Commerce Observer pipeline
- Multi-source observations provide better coverage
- Same privacy guarantees as Phase 31.1
- Deterministic, auditable classification
- Source attribution enables debugging

### Negative

- Rule-based classification has false positives/negatives
- Some transactions may not match known patterns
- MCC code coverage is incomplete

### Mitigations

- Classification errors are acceptable (observation, not execution)
- Unknown transactions classified as "other"
- False negatives preferred over false positives

## Implementation

### Files Created

- `internal/financetxscan/model.go` - Transaction scan types
- `internal/financetxscan/rules.go` - Classification rules
- `internal/financetxscan/engine.go` - Ingest engine
- `scripts/guardrails/commerce_from_finance_enforced.sh` - CI enforcement
- `internal/demo_phase31_2_finance_commerce_observer/demo_test.go` - Tests

### Files Modified

- `pkg/domain/commerceobserver/types.go` - Added SourceKind enum
- `cmd/quantumlife-web/main.go` - TrueLayer sync integration
- `pkg/events/events.go` - Phase 31.2 events

### Events

- `phase31_2.transaction_scan.started` - Scan began
- `phase31_2.transaction_scan.completed` - Scan finished
- `phase31_2.commerce_observations.persisted` - Observations stored

## Hard Constraints (CI-Enforced)

- stdlib only (no external deps)
- NO goroutines
- NO time.Now() (clock injection only)
- Deterministic output: same inputs + same clock => same hashes
- NO merchant names stored or used
- NO amounts stored or used
- NO raw timestamps stored
- Max 3 categories per observation set
- Pipe-delimited canonical strings (not JSON)

## References

- ADR-0062: Phase 31 - Commerce Observers (Silent by Default)
- ADR-0063: Phase 31.1 - Gmail Receipt Observers
- ADR-0052: Phase 29 - TrueLayer Finance Read
- Phase 31.2 specification in canon
