# ADR-0060: Phase 29 — TrueLayer Read-Only Connect (UK Sandbox) + Finance Mirror Proof

## Status

Accepted

## Context

QuantumLife has built a comprehensive trust foundation through Phases 1-28. Financial data observation is critical for a personal sovereign agent, but requires the highest level of privacy protection.

The core challenge is: **Can we connect to real financial data and prove we saw it without storing any identifiable information?**

## Decision

Phase 29 implements read-only TrueLayer connection (UK sandbox) with a privacy-first finance mirror proof page.

### What This Phase Does

1. **TrueLayer OAuth (Read-Only)**
   - Connect via OAuth with read-only scopes ONLY
   - Scopes: accounts, balance, transactions, offline_access
   - NO payment scopes allowed - hard blocked
   - UK sandbox environment only (Phase 29)

2. **Bounded Sync**
   - Explicit sync only (POST /run/truelayer-sync)
   - Maximum 25 items total
   - Last 7 days only
   - Single request, no retries

3. **Privacy Transformation**
   - Raw data → Abstract magnitude buckets
   - NO amounts, NO merchants, NO bank names, NO account numbers
   - Evidence stored as hashes only
   - Privacy guard blocks any identifiable patterns

4. **Finance Mirror Proof Page**
   - GET /mirror/finance
   - Title: "Seen, quietly."
   - Calm line based on magnitude (not counts)
   - Up to 3 category signals (abstract only)
   - Reassurance: "Nothing was stored. Only the fact that it was seen."

### What This Phase Does NOT Do

- Execute payments or transfers
- Store raw financial data
- Display amounts, merchants, or account details
- Use live/production TrueLayer environment
- Auto-sync in background
- Retry failed syncs

## Domain Model

### MagnitudeBucket

```go
type MagnitudeBucket string

const (
    MagnitudeNothing  MagnitudeBucket = "nothing"   // 0 items
    MagnitudeAFew     MagnitudeBucket = "a_few"     // 1-3 items
    MagnitudeSeveral  MagnitudeBucket = "several"   // 4-10 items
    MagnitudeMany     MagnitudeBucket = "many"      // 11+ items
)
```

Raw counts are converted to buckets immediately. Never stored.

### CategoryBucket

```go
type CategoryBucket string

const (
    CategoryLiquidity        CategoryBucket = "liquidity"
    CategoryObligations      CategoryBucket = "obligations"
    CategoryUpcomingPressure CategoryBucket = "upcoming_pressure"
    CategorySpendPattern     CategoryBucket = "spend_pattern"
)
```

### FinanceSyncReceipt

```go
type FinanceSyncReceipt struct {
    ReceiptID             string           // Deterministic hash
    CircleID              string
    Provider              string           // "truelayer"
    TimeBucket            time.Time        // 5-minute granularity
    PeriodBucket          string           // "2025-01-15"
    AccountsMagnitude     MagnitudeBucket  // NOT raw count
    TransactionsMagnitude MagnitudeBucket  // NOT raw count
    EvidenceHash          string           // Hash of sorted abstract tokens
    Success               bool
    FailReason            string           // Generic, no PII
    StatusHash            string
}
```

Hash-only. Never raw identifiers.

### FinanceMirrorPage

```go
type FinanceMirrorPage struct {
    Title          string              // "Seen, quietly."
    CalmLine       string              // Based on magnitude, not counts
    Categories     []CategorySignal    // Max 3, abstract only
    Reassurance    string              // Privacy reassurance
    LastSyncBucket string              // Abstract time
    Connected      bool
    StatusHash     string
}
```

## TrueLayer OAuth

### Allowed Scopes (Read-Only)

- `accounts` - Read account information
- `balance` - Read account balances
- `transactions` - Read transaction history
- `offline_access` - Allow refresh tokens

### Forbidden Scope Patterns

Any scope containing these patterns is rejected:
- payment, payments, pay
- transfer
- write
- initiate
- standing_order
- direct_debit
- beneficiar
- mandate

### OAuth Flow

1. GET /connect/truelayer/start?circle_id=...
   - Generates CSRF-protected state
   - Redirects to TrueLayer auth

2. GET /connect/truelayer/callback?code=...&state=...
   - Validates state
   - Exchanges code for tokens
   - Validates scopes are read-only
   - Stores connection hash (never raw tokens)

3. POST /disconnect/truelayer?circle_id=...
   - Removes local connection
   - Idempotent

## Sync Process

### Bounded Parameters

| Parameter | Limit |
|-----------|-------|
| Max accounts | 25 |
| Max transactions | 25 |
| Date range | 7 days |
| Requests | Single, no retry |

### Privacy Transformation

```
Raw Data           →   Evidence Token      →   Receipt
-----------------      -----------------       -----------------
Account Type       →   hash("account|checking") → EvidenceHash
Transaction Cat    →   hash("tx|essentials")    →
```

### Privacy Guard

Blocks any field containing:
- Email patterns
- Phone patterns
- IBAN patterns
- Sort code patterns
- Account numbers (8+ digits)
- Currency symbols followed by numbers
- More than 2 contiguous digits

## Web Routes

| Route | Method | Purpose |
|-------|--------|---------|
| /connect/truelayer/start | GET | Start OAuth flow |
| /connect/truelayer/callback | GET | Handle OAuth callback |
| /disconnect/truelayer | POST | Revoke connection |
| /run/truelayer-sync | POST | Explicit sync |
| /mirror/finance | GET | Finance mirror proof page |

## Events

```go
// TrueLayer OAuth lifecycle
Phase29TrueLayerOAuthStart    = "phase29.truelayer.oauth.start"
Phase29TrueLayerOAuthCallback = "phase29.truelayer.oauth.callback"
Phase29TrueLayerOAuthRevoke   = "phase29.truelayer.oauth.revoke"

// Sync lifecycle
Phase29TrueLayerSyncRequested = "phase29.truelayer.sync.requested"
Phase29TrueLayerSyncCompleted = "phase29.truelayer.sync.completed"
Phase29TrueLayerSyncFailed    = "phase29.truelayer.sync.failed"
Phase29TrueLayerSyncPersisted = "phase29.truelayer.sync.persisted"

// Finance mirror page
Phase29FinanceMirrorRendered  = "phase29.finance_mirror.rendered"
Phase29FinanceMirrorViewed    = "phase29.finance_mirror.viewed"
Phase29FinanceMirrorAcked     = "phase29.finance_mirror.acked"
```

All events contain hashes only, never identifiers.

## Guardrails

30+ checks enforcing:

1. **Read-only scopes only** (5 checks)
   - No payment scope patterns
   - Only allowed scopes
   - Forbidden scope validation

2. **Bounded sync** (4 checks)
   - MaxAccountsToFetch = 25
   - MaxTransactionsToFetch = 25
   - MaxSyncDays = 7
   - No retry patterns

3. **Privacy guard** (6 checks)
   - No raw amounts in receipts
   - No merchant names
   - No account numbers
   - No identifiers in UI
   - Privacy guard exists
   - Evidence hash only

4. **No background work** (3 checks)
   - No goroutines in engine
   - No goroutines in store
   - No goroutines in handlers

5. **No time.Now()** (3 checks)
   - Not in domain
   - Not in engine
   - Not in store

6. **Hash-only storage** (4 checks)
   - No raw tokens stored
   - Connection hash only
   - Evidence hash only
   - Canonical strings are abstract

7. **Web route checks** (5 checks)
   - /connect/truelayer/start exists
   - /connect/truelayer/callback exists
   - /disconnect/truelayer exists
   - /run/truelayer-sync exists
   - /mirror/finance exists

## Persistence

### FinanceMirrorStore

```go
type FinanceMirrorStore struct {
    syncReceipts         map[string]*FinanceSyncReceipt
    syncReceiptsByCircle map[string][]string
    syncReceiptsByPeriod map[string]string
    acks                 map[string]*FinanceMirrorAck
    connectionHashes     map[string]string  // NOT raw tokens
    maxPeriods           int                // 30 days
}
```

Constraints:
- Hash-only storage
- Bounded retention (30 days)
- Append-only with storelog integration
- No raw tokens or data

## UI

### Finance Mirror Proof Page

```
┌─────────────────────────────────────────┐
│  Seen, quietly.                         │
│                                         │
│  "A few things passed through.          │
│   Quietly noted."                       │
│                                         │
│  ┌─────────────────────────────────┐   │
│  │ Funds available: a few          │   │
│  │ Recent activity: several        │   │
│  └─────────────────────────────────┘   │
│                                         │
│  Last sync: Jan 15 10:30               │
│                                         │
│  "Nothing was stored. Only the fact    │
│   that it was seen."                   │
│                                         │
│  Hash: abc123...                       │
└─────────────────────────────────────────┘
```

**No:** amounts, merchant names, account numbers, bank names

## Configuration

Environment variables (cmd layer only):
- TRUELAYER_ENV=uk_sandbox
- TRUELAYER_CLIENT_ID
- TRUELAYER_CLIENT_SECRET
- TRUELAYER_REDIRECT_URI

Never print secrets.

## Consequences

### Positive

- Proves financial data can be observed safely
- Establishes privacy-first pattern for financial features
- Provides foundation for future financial observations
- Demonstrates restraint in data handling

### Negative

- Limited to sandbox environment (by design)
- No detailed financial insights (by design)
- Abstract buckets only (by design)

### Neutral

- Future phases could add more providers
- Pattern is established for privacy-safe finance

## Related ADRs

- ADR-0001: Canon v1 (foundation)
- ADR-0059: Phase 28 Trust Kept
- ADR-0044: Phase 20 Trust Accrual

## Final Constraint

After Phase 29 completes:
- User can connect TrueLayer (sandbox)
- User can run explicit sync
- User sees abstract proof only
- NO raw financial data is ever stored or displayed

The system should be able to say truthfully:

> "We saw your finances. We stored nothing. Here's proof."

And show only abstract buckets and hashes.
