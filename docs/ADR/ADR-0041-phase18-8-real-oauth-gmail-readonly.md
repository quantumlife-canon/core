# ADR-0041: Phase 18.8 Real OAuth (Gmail Read-Only)

## Status

Accepted

## Context

Phase 18.6 introduced "First Connect" with simulated mock connections. Phase 18.7 added Mirror Proof for verifying what was observed. Now we need to turn the simulated email connection into a real Google OAuth flow for Gmail with actual data fetching, while maintaining strict read-only constraints and full revocability.

### Requirements

1. **Real OAuth flow** - Actual Google OAuth 2.0 authorization for Gmail
2. **Read-only only** - Only `gmail.readonly` scope, never write scopes
3. **Full revocability** - Both Google-side revocation and local token removal
4. **Deterministic audit** - Canonical receipts with reproducible hashes
5. **No background polling** - Syncs only when explicitly triggered by human
6. **CSRF protection** - Time-bucketed HMAC-signed state tokens

## Decision

We implement Phase 18.8 with the following components:

### 1. OAuth State Management (`internal/oauth/state.go`)

```go
type State struct {
    CircleID       string
    Nonce          string    // 16 bytes hex, cryptographically random
    IssuedAtBucket int64     // Unix timestamp floored to 5-minute bucket
    Signature      string    // HMAC-SHA256 of canonical form
}
```

State tokens:
- Are time-bucketed (5-minute granularity) for deterministic validation
- Expire after 10 minutes
- Use HMAC-SHA256 for CSRF protection
- Bind to a specific circle

### 2. Gmail OAuth Handler (`internal/oauth/gmail.go`)

```go
type GmailHandler struct {
    stateManager *StateManager
    broker       auth.TokenBroker
    httpClient   *http.Client
    redirectBase string
    clock        func() time.Time
}
```

Operations:
- `Start(circleID)` - Generates auth URL with CSRF state
- `Callback(ctx, code, state)` - Validates state, exchanges code, stores token
- `Revoke(ctx, circleID)` - Revokes at Google + removes local token
- `HasConnection(ctx, circleID)` - Checks if token exists

### 3. Connection Receipts (`internal/oauth/receipts.go`)

All receipts have canonical string representation for deterministic hashing:

```go
type ConnectionReceipt struct {
    CircleID    string
    Provider    Provider    // "google"
    Product     Product     // "gmail"
    Action      ReceiptAction
    Success     bool
    FailReason  string      // Only set if Success=false
    At          time.Time
    StateHash   string      // For correlation
    TokenHandle string      // Opaque handle, NOT the token
}
```

CRITICAL: Receipts NEVER contain actual tokens or secrets.

### 4. Web Routes

| Route | Method | Description |
|-------|--------|-------------|
| `/connect/gmail/start` | GET | Initiates OAuth, redirects to Google |
| `/connect/gmail/callback` | GET | Handles Google callback |
| `/disconnect/gmail` | POST | Revokes connection |
| `/run/gmail-sync` | GET/POST | Explicit sync trigger |

### 5. Scope Enforcement

```go
var GmailScopes = []string{"gmail.readonly"}
```

- Only read-only scope is ever requested
- Callback validates returned scopes
- Write scopes trigger immediate token revocation

### 6. Revocation Behavior

Revocation is **idempotent**:
1. Call Google's revocation endpoint (best effort)
2. Remove local token (always)
3. Return success even if already disconnected

### 7. Persistence Records

For replay and audit:
- `OAuthStateRecord` - State lifecycle
- `OAuthTokenHandleRecord` - Token handle metadata
- `GmailSyncReceiptRecord` - Sync outcomes
- `OAuthRevokeReceiptRecord` - Revocation outcomes

All records have canonical string form and SHA256 hash.

### 8. Phase 18.8 Events

```go
Phase18_8OAuthStarted          // OAuth flow initiated
Phase18_8OAuthCallback         // Callback received
Phase18_8OAuthTokenMinted      // Token minted for sync
Phase18_8OAuthRevokeRequested  // Revocation requested
Phase18_8OAuthRevokeCompleted  // Revocation completed
Phase18_8GmailSyncStarted      // Sync initiated
Phase18_8GmailSyncCompleted    // Sync succeeded
Phase18_8GmailSyncFailed       // Sync failed
```

## Constraints

### Hard Rejections

1. **No goroutines** in OAuth code
2. **No write scopes** ever
3. **No background polling** - only explicit human-triggered syncs
4. **No tokens in events/receipts** - only hashes and handles
5. **No time.Now()** - injected clock only

### Security Measures

1. CSRF state with HMAC signature
2. State expiration (10 minutes)
3. Scope validation on callback
4. Immediate revocation if write scopes received
5. Separate revocation of Google and local tokens

## Consequences

### Positive

1. Real Gmail data instead of mocks
2. Full OAuth 2.0 compliance
3. Complete revocability
4. Deterministic audit trail
5. CSRF-protected flow

### Negative

1. Requires Google Cloud Console setup
2. Token refresh complexity
3. Network dependency for sync

### Neutral

1. Mirror proof unchanged (already abstract)
2. Connection store uses same intent model
3. Fits existing deterministic architecture

## Dependencies

- Phase 18.6: Connection intents and store
- Phase 18.7: Mirror proof (for observability)
- `internal/connectors/auth`: TokenBroker interface
- `internal/integrations/gmail_read`: Real Gmail adapter

## Verification

Run guardrails:
```bash
make check-oauth-gmail
```

Run demo tests:
```bash
make demo-phase18-8
```

Full CI:
```bash
make ci
```

## References

- Phase 18.8 Spec (in conversation)
- [Google OAuth 2.0 for Web Server Applications](https://developers.google.com/identity/protocols/oauth2/web-server)
- [Gmail API Scopes](https://developers.google.com/gmail/api/auth/scopes)
