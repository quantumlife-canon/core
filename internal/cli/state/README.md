# CLI State Store

This package provides local state storage for the QuantumLife CLI.

## Purpose

Local developer convenience for CLI-based OAuth flows. This is **NOT** authoritative state.

## Important Distinctions

| Aspect | CLI State Store | Authority Memory Layer |
|--------|-----------------|------------------------|
| Purpose | Developer convenience | Authoritative state |
| Scope | Local machine only | Distributed/persistent |
| Contents | Opaque handle IDs only | Full domain state |
| Security | 0600 file permissions | Encrypted at rest + in transit |
| Durability | Best-effort | Transactional |

## What is Stored

- **Token handle IDs**: Opaque references to tokens stored by the TokenBroker
- **Circle-to-provider mappings**: Which circle has authorized which provider
- **Metadata**: Schema version, timestamps

## What is NOT Stored

- **Raw tokens**: Never stored in CLI state
- **Refresh tokens**: Only stored encrypted by TokenBroker
- **Access tokens**: Never persisted (minted on demand)
- **Secrets**: Never stored

## File Location

Default: `~/.quantumlife/state.json`

The file is created with permissions `0600` (owner read/write only).

## Schema

```json
{
  "schema_version": 1,
  "updated_at": "2025-01-15T10:00:00Z",
  "circles": {
    "circle-1": {
      "providers": {
        "google": {
          "handle_id": "token-1",
          "linked_at": "2025-01-15T10:00:00Z"
        }
      }
    }
  }
}
```

## Usage

```go
import "quantumlife/internal/cli/state"

// Load or create state
s, err := state.Load()

// Link a token handle to a circle+provider
s.SetTokenHandle("circle-1", "google", "token-123")
if err := s.Save(); err != nil {
    return err
}

// Get token handle for a circle+provider
handleID, ok := s.GetTokenHandle("circle-1", "google")
```

## Reference

- docs/TECHNICAL_SPLIT_V1.md §3.5 Action Execution Layer
- internal/connectors/auth/ for TokenBroker
