# ADR-0066: Phase 31.3b — Real TrueLayer Sync (Accounts + Transactions → Finance Mirror + Commerce Observer)

## Status

**Accepted**

## Context

Phase 31.3 established that only real TrueLayer API responses may be processed for finance ingest — mock data is rejected. However, the actual API integration was not yet implemented.

Phase 31.3b completes this work by:
1. Wiring REAL TrueLayer sandbox/production API responses into the existing OAuth flow
2. Building Finance Mirror proof pages from REAL sync receipts
3. Running the Commerce-from-Finance pipeline (Phase 31.2/31.3) ONLY on real data
4. Ensuring NO mock transaction path exists in production codepaths
5. Using httptest for CI-safe testing without real credentials

## Decision

### Read-Only Access Only

TrueLayer integration remains strictly READ-ONLY:
- `accounts` scope: List connected bank accounts
- `balance` scope: Read account balances
- `transactions` scope: Read transaction history
- `offline_access` scope: Refresh tokens without re-authentication

**Payment scopes are architecturally forbidden** — the code rejects any scope containing payment/transfer/write patterns at multiple validation layers.

### Bounded Sync Limits

To prevent abuse and ensure predictable performance:

| Limit | Value | Rationale |
|-------|-------|-----------|
| `MaxAccounts` | 25 | Sufficient for personal finance, prevents bulk scraping |
| `MaxTransactionsPerAccount` | 25 | Recent activity only, prevents historical bulk export |
| `SyncWindowDays` | 7 | One week of data, sufficient for commerce observation |

These limits are enforced at the sync service layer before data reaches the handler.

### Privacy Redaction

The `TransactionClassification` struct contains ONLY fields needed for commerce observation:

```go
type TransactionClassification struct {
    TransactionID      string  // Will be hashed, never stored raw
    ProviderCategory   string  // Bank-assigned category (e.g., "FOOD_AND_DRINK")
    ProviderCategoryID string  // MCC code or similar (e.g., "5812")
    PaymentChannel     string  // Payment type (e.g., "debit", "credit")
}
```

**Explicitly excluded** (even though TrueLayer provides them):
- `amount` — Never extracted or stored
- `merchant_name` — Never extracted or stored
- `timestamp` — Never extracted or stored
- `description` — Never extracted or stored

### Clock Injection

All time-dependent operations use injected clock functions:

```go
type SyncService struct {
    client *Client
    clock  func() time.Time  // Injected, never time.Now()
}
```

This ensures:
1. **Determinism**: Same inputs + clock = same outputs
2. **Testability**: Tests can control time precisely
3. **Canon compliance**: No `time.Now()` in internal/ or pkg/

### Token Storage

OAuth tokens are stored in-memory only:

```go
type TrueLayerTokenStore struct {
    mu     sync.RWMutex
    tokens map[string]*TrueLayerTokenEntry
    clock  func() time.Time
}
```

**Security properties**:
- Tokens never persisted to disk
- Tokens never logged (SENSITIVE markers throughout)
- Token hash stored for audit correlation
- Automatic expiration handling

### httptest Strategy

All tests use `httptest.Server` with deterministic JSON fixtures:

```go
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    if r.URL.Path == "/data/v1/accounts" {
        json.NewEncoder(w).Encode(accountsFixture)
        return
    }
    // ...
}))
defer server.Close()

client.SetBaseURL(server.URL)
```

This ensures:
- **CI-safe**: No real credentials needed
- **Deterministic**: Same fixtures = same results
- **Fast**: No network latency
- **Isolated**: No external dependencies

## Architecture

### Component Diagram

```
┌────────────────────────────────────────────────────────────────────┐
│                        cmd/quantumlife-web/main.go                  │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │                 handleTrueLayerSync()                        │   │
│  │  1. Emit Phase31_3bTrueLayerSyncStarted                     │   │
│  │  2. Get token from TrueLayerTokenStore                      │   │
│  │  3. Call SyncService.Sync()                                 │   │
│  │  4. Build FinanceSyncReceipt                                │   │
│  │  5. Store receipt in FinanceMirrorStore                     │   │
│  │  6. Convert to TransactionData (ProviderTrueLayer)          │   │
│  │  7. Call financetxscan.BuildFromTransactions()              │   │
│  │  8. Persist observations to CommerceObserverStore           │   │
│  │  9. Emit Phase31_3bTrueLayerIngestCompleted                 │   │
│  └─────────────────────────────────────────────────────────────┘   │
└────────────────────────────────────────────────────────────────────┘
        │                        │                          │
        ▼                        ▼                          ▼
┌──────────────┐     ┌───────────────────┐     ┌──────────────────────┐
│ TrueLayer    │     │ SyncService       │     │ financetxscan        │
│ TokenStore   │     │ (sync.go)         │     │ Engine               │
│              │     │                   │     │                      │
│ • StoreToken │     │ • Sync()          │     │ • BuildFromTx()      │
│ • GetToken   │     │ • MaxAccounts=25  │     │ • ValidateProvider() │
│ • HasValid() │     │ • MaxTx=25        │     │ • ProviderTrueLayer  │
│ • TokenHash  │     │ • SyncWindow=7d   │     │ • RejectMock         │
└──────────────┘     └───────────────────┘     └──────────────────────┘
                              │
                              ▼
                     ┌───────────────────┐
                     │ TrueLayer Client  │
                     │ (client.go)       │
                     │                   │
                     │ • GetAccounts()   │
                     │ • GetTransactions │
                     │ • stdlib net/http │
                     └───────────────────┘
                              │
                              ▼
                     ┌───────────────────┐
                     │ TrueLayer API     │
                     │ (real/sandbox)    │
                     └───────────────────┘
```

### Data Flow (Privacy Boundary)

```
TrueLayer API Response              After Privacy Redaction
─────────────────────              ────────────────────────
{                                   TransactionClassification {
  "transaction_id": "tx-123",  →      TransactionID: "tx-123",  // Will be hashed
  "amount": -15.99,            ✗      // NEVER extracted
  "merchant_name": "Tesco",    ✗      // NEVER extracted
  "timestamp": "2026-01-06T...",✗     // NEVER extracted
  "description": "CARD...",    ✗      // NEVER extracted
  "transaction_category":      →      ProviderCategory: "FOOD_AND_DRINK",
    "FOOD_AND_DRINK",
  "transaction_classification":→      ProviderCategoryID: "5812",
    ["5812"],
  "transaction_type": "DEBIT"  →      PaymentChannel: "debit"
}                                   }
```

## Events

| Event | Trigger | Metadata |
|-------|---------|----------|
| `phase31_3b.truelayer.sync.started` | Sync initiated | `circle_id` |
| `phase31_3b.truelayer.sync.completed` | Sync successful | `receipt_hash`, `accounts_magnitude`, `transactions_magnitude` |
| `phase31_3b.truelayer.sync.failed` | Sync failed | `fail_reason` |
| `phase31_3b.truelayer.ingest.started` | Commerce ingest started | `transaction_count` |
| `phase31_3b.truelayer.ingest.completed` | Commerce ingest done | `observations_count`, `overall_magnitude` |
| `phase31_3b.truelayer.token.stored` | Token stored after OAuth | `token_hash` (safe to log) |
| `phase31_3b.truelayer.token.refreshed` | Token refreshed | `token_hash` |
| `phase31_3b.truelayer.token.expired` | Token expired | - |

## Configuration

Environment variables (not committed to repo):

```bash
# Required for real TrueLayer integration
TRUELAYER_CLIENT_ID=sandbox-client-id
TRUELAYER_CLIENT_SECRET=sandbox-secret-never-commit
TRUELAYER_REDIRECT_URI=http://localhost:8080/connect/truelayer/callback

# Optional
TRUELAYER_ENV=sandbox  # or "live"
```

## Testing

### Running Demo Tests (CI-Safe)

```bash
go test ./internal/demo_phase31_3b_truelayer_real_sync/... -v
```

### Running Guardrails

```bash
./scripts/guardrails/truelayer_real_sync_enforced.sh
```

### Running with Real Sandbox Credentials

```bash
# Set environment variables
export TRUELAYER_CLIENT_ID=your-sandbox-client-id
export TRUELAYER_CLIENT_SECRET=your-sandbox-secret
export TRUELAYER_REDIRECT_URI=http://localhost:8080/connect/truelayer/callback

# Start the server
go run cmd/quantumlife-web/main.go

# Navigate to http://localhost:8080/connections
# Click "Connect Bank (TrueLayer)"
# Complete OAuth flow with TrueLayer sandbox bank
# Click "Sync" to perform real sync
```

## Consequences

### Positive

1. **Real data flow**: Finance Mirror and Commerce Observer now work with actual bank data
2. **Privacy preserved**: No amounts, merchants, or timestamps ever stored
3. **Deterministic**: Clock injection ensures reproducible behavior
4. **CI-safe**: All tests use httptest, no real credentials needed
5. **Bounded**: Sync limits prevent abuse and ensure performance

### Negative

1. **Token volatility**: In-memory token storage means re-auth after server restart
2. **No automatic refresh**: User must re-authenticate when tokens expire

### Neutral

1. **Sandbox-first**: Default to sandbox environment for safety
2. **Single attempt**: No retries on API failure (fail gracefully)

## References

- Phase 29: TrueLayer Read-Only Connect (ADR-0060)
- Phase 31: Commerce Observers (ADR-0062)
- Phase 31.1: Gmail Receipt Observers (ADR-0063)
- Phase 31.2: Commerce from Finance (ADR-0064)
- Phase 31.3: Real Finance Only (ADR-0065)
- TrueLayer Data API: https://docs.truelayer.com/docs/data-api-overview
