# Auth Subsystem

This package implements the Token Broker pattern for managing OAuth credentials
and minting access tokens for calendar provider operations.

## Canon & Technical Split Alignment

Reference documents:
- Canon: docs/QUANTUMLIFE_CANON_V1.md
- Technical Split: docs/TECHNICAL_SPLIT_V1.md §3.5 Action Execution Layer
- Human Guarantees: docs/HUMAN_GUARANTEES.md

### Core Principles

1. **Credentials owned by Circle**: Each Circle owns its OAuth credentials.
   The Circle initiates the OAuth flow and stores the resulting refresh token.

2. **Usage authorized via Intersection**: Access tokens are only minted when
   an Intersection authorizes the operation via an AuthorizationProof.

3. **Scope mapping enforced**: QuantumLife scopes (e.g., `calendar:read`) are
   mapped to provider-specific scopes (e.g., Google's `calendar.readonly`).

4. **READ-ONLY in v5**: Only read scopes can be minted. Write operations
   will fail with a clear error.

## Components

### TokenBroker Interface

The `TokenBroker` interface provides:
- `BeginOAuth`: Generate an OAuth authorization URL
- `ExchangeCode`: Exchange an authorization code for tokens
- `MintAccessToken`: Mint an access token for a specific operation

### Scope Mapping

QuantumLife scopes are mapped to provider-specific scopes:

| QuantumLife Scope | Google Calendar | Microsoft Graph |
|-------------------|-----------------|-----------------|
| `calendar:read`   | `calendar.readonly` | `Calendars.Read` |
| `calendar:write`  | NOT ALLOWED (v5) | NOT ALLOWED (v5) |

### Security

- Refresh tokens are encrypted at rest (placeholder encryption in demo)
- Access tokens have short TTL and are not stored
- All token minting requires AuthorizationProof
- Tokens are redacted in logs

## Usage

```go
broker := impl_inmem.NewBroker(store, authorityEngine, clock)

// Generate OAuth URL (user follows this)
authURL, err := broker.BeginOAuth(auth.ProviderGoogle, redirectURI, state, scopes)

// After user authorizes, exchange code (via CLI in v5)
handle, err := broker.ExchangeCode(ctx, auth.ProviderGoogle, code, redirectURI)

// Mint access token for operation
envelope := primitives.ExecutionEnvelope{...}
token, err := broker.MintAccessToken(ctx, envelope, auth.ProviderGoogle, []string{"calendar:read"})
```

## Environment Variables

See `config.go` for required environment variables:
- `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`
- `MICROSOFT_CLIENT_ID`, `MICROSOFT_CLIENT_SECRET`, `MICROSOFT_TENANT_ID`
- `TOKEN_ENC_KEY` (placeholder for envelope encryption key)
