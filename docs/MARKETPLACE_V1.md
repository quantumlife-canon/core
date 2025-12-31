# Connector Marketplace v1

## Document Status

| Field | Value |
|-------|-------|
| Version | 1.0 |
| Status | Draft |
| Author | QuantumLife Core Team |
| Date | 2025-01-01 |
| Supersedes | None |
| Related | TECHNICAL_ARCHITECTURE_V1.md, CANONICAL_CAPABILITIES_V1.md |

---

## 1. Overview

The QuantumLife Connector Marketplace solves the **vendor explosion problem**: supporting hundreds of vendors across multiple regions (UK, India, US) without polluting core with vendor-specific logic.

**Key Insight**: Users don't care about "Plaid" or "TrueLayer" — they care about seeing their bank balance. The marketplace abstracts vendors behind capabilities.

---

## 2. Design Principles

### 2.1 Capability-First, Not Vendor-First

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    USER-CENTRIC CAPABILITY VIEW                         │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  What User Sees                    What System Does                     │
│  ──────────────                    ────────────────                     │
│                                                                         │
│  "Connect Bank Account"     →      Lists available connectors for       │
│                                    capability: finance.balance          │
│                                    filtered by user's region            │
│                                                                         │
│  "Connect Email"            →      Lists: Gmail, Outlook, Yahoo         │
│                                    for capability: email.read           │
│                                                                         │
│  "Connect Calendar"         →      Lists: Google, Apple, Outlook        │
│                                    for capability: calendar.read        │
│                                                                         │
│  User never needs to know about Plaid vs TrueLayer vs Setu             │
│  System selects based on region and availability                        │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 2.2 Region-Aware Routing

```yaml
# When user adds a bank account:
region_routing:
  UK:
    primary: truelayer-uk
    fallback: plaid-uk
    note: "TrueLayer has better UK coverage"

  US:
    primary: plaid-us
    fallback: yodlee-us
    note: "Plaid dominates US market"

  IN:
    primary: setu-in
    fallback: finbox-in
    note: "Account Aggregator framework"

  EU:
    primary: truelayer-eu
    fallback: tink-eu
    note: "PSD2 compliance required"
```

### 2.3 Zero Core Pollution

**Invariant**: Core engine has zero knowledge of specific vendors.

```go
// CORRECT: Core processes canonical events
func (e *Engine) ProcessEvent(event CanonicalEvent) error {
    // Works the same for Gmail, Outlook, or ProtonMail
    // Core sees only EmailMessageEvent
}

// WRONG: Core knowing about vendors
func (e *Engine) ProcessEvent(event CanonicalEvent) error {
    if event.Vendor() == "gmail" {
        // NO! Vendor logic in core is forbidden
    }
}
```

---

## 3. Marketplace Architecture

### 3.1 Component Overview

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      MARKETPLACE ARCHITECTURE                           │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                    CONNECTOR REGISTRY                            │   │
│  │                                                                   │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐              │   │
│  │  │ Connector   │  │ Capability  │  │  Region     │              │   │
│  │  │ Metadata    │  │ Mappings    │  │  Routing    │              │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘              │   │
│  └──────────────────────────────┬──────────────────────────────────┘   │
│                                 │                                       │
│  ┌──────────────────────────────┼──────────────────────────────────┐   │
│  │                              ▼                                   │   │
│  │                    CONNECTOR RUNTIME                             │   │
│  │                                                                   │   │
│  │  ┌──────────────────────────────────────────────────────────┐   │   │
│  │  │                  Connector Instances                      │   │   │
│  │  │                                                            │   │   │
│  │  │  ┌────────┐  ┌────────┐  ┌────────┐  ┌────────┐          │   │   │
│  │  │  │ Gmail  │  │TrueLayer│  │ Plaid  │  │ Setu   │  ...    │   │   │
│  │  │  └────────┘  └────────┘  └────────┘  └────────┘          │   │   │
│  │  │                                                            │   │   │
│  │  └──────────────────────────────────────────────────────────┘   │   │
│  │                              │                                   │   │
│  │                              │ Canonical Events                  │   │
│  │                              ▼                                   │   │
│  │  ┌──────────────────────────────────────────────────────────┐   │   │
│  │  │                    Event Queue                            │   │   │
│  │  └──────────────────────────────────────────────────────────┘   │   │
│  │                                                                   │   │
│  └───────────────────────────────────────────────────────────────────┘   │
│                                                                         │
│  ┌───────────────────────────────────────────────────────────────────┐   │
│  │                    CREDENTIAL VAULT                               │   │
│  │                                                                   │   │
│  │  Per-user, per-connector OAuth tokens, API keys, refresh tokens   │   │
│  │  Encrypted at rest, region-aware storage                          │   │
│  │                                                                   │   │
│  └───────────────────────────────────────────────────────────────────┘   │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 3.2 Connector Lifecycle

```
┌─────────────────────────────────────────────────────────────────────────┐
│                     CONNECTOR LIFECYCLE                                 │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  1. REGISTRATION (Build-time)                                           │
│     ┌──────────────────────────────────────────────────────────────┐   │
│     │  Developer submits connector package:                         │   │
│     │  - Connector manifest (capabilities, regions, auth type)      │   │
│     │  - Transformer code (vendor → canonical)                      │   │
│     │  - Test suite (required to pass)                              │   │
│     │  - Documentation                                              │   │
│     └──────────────────────────────────────────────────────────────┘   │
│                          │                                              │
│                          ▼                                              │
│  2. VALIDATION (CI/CD)                                                  │
│     ┌──────────────────────────────────────────────────────────────┐   │
│     │  Automated checks:                                            │   │
│     │  - Schema compliance (canonical event output)                 │   │
│     │  - Security audit (no data leaks, proper auth handling)       │   │
│     │  - Performance benchmarks (latency, memory)                   │   │
│     │  - Test coverage (>80% required)                              │   │
│     └──────────────────────────────────────────────────────────────┘   │
│                          │                                              │
│                          ▼                                              │
│  3. PUBLICATION (Registry)                                              │
│     ┌──────────────────────────────────────────────────────────────┐   │
│     │  Published to connector registry:                             │   │
│     │  - Listed with capabilities and regions                       │   │
│     │  - Available for user installation                            │   │
│     │  - Version tracked                                            │   │
│     └──────────────────────────────────────────────────────────────┘   │
│                          │                                              │
│                          ▼                                              │
│  4. INSTALLATION (User-initiated)                                       │
│     ┌──────────────────────────────────────────────────────────────┐   │
│     │  User connects account:                                       │   │
│     │  - OAuth flow initiated                                       │   │
│     │  - Tokens stored in vault                                     │   │
│     │  - Connector instance created                                 │   │
│     │  - Initial sync triggered                                     │   │
│     └──────────────────────────────────────────────────────────────┘   │
│                          │                                              │
│                          ▼                                              │
│  5. OPERATION (Runtime)                                                 │
│     ┌──────────────────────────────────────────────────────────────┐   │
│     │  Continuous operation:                                        │   │
│     │  - Scheduled polling (or webhook listening)                   │   │
│     │  - Transform vendor data → canonical events                   │   │
│     │  - Enqueue events for core processing                         │   │
│     │  - Token refresh as needed                                    │   │
│     │  - Health monitoring                                          │   │
│     └──────────────────────────────────────────────────────────────┘   │
│                          │                                              │
│                          ▼                                              │
│  6. DISCONNECTION (User-initiated)                                      │
│     ┌──────────────────────────────────────────────────────────────┐   │
│     │  User disconnects:                                            │   │
│     │  - Tokens revoked (if API supports)                           │   │
│     │  - Credentials deleted from vault                             │   │
│     │  - Connector instance terminated                              │   │
│     │  - Historical data retained (user choice)                     │   │
│     └──────────────────────────────────────────────────────────────┘   │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 4. Connector Specification

### 4.1 Connector Manifest

Every connector must provide a manifest file:

```yaml
# connector.manifest.yaml
connector:
  id: truelayer-uk
  name: TrueLayer UK Banking
  vendor: truelayer
  version: 1.2.0

  # What this connector does
  capabilities:
    - capability: finance.balance
      read: true
      write: false
      polling_interval: 15m

    - capability: finance.transactions
      read: true
      write: false
      polling_interval: 15m
      historical_depth: 90d

    - capability: finance.payment
      read: false
      write: true
      requires_approval: true

  # Where this connector works
  regions:
    - UK
    - IE

  # Authentication
  auth:
    type: oauth2
    authorization_url: https://auth.truelayer.com/
    token_url: https://auth.truelayer.com/connect/token
    scopes:
      - info
      - accounts
      - balance
      - transactions
      - offline_access
    refresh_strategy: automatic

  # Requirements
  requirements:
    api_version: ">=1.0.0"
    sdk_version: ">=0.5.0"

  # Metadata
  metadata:
    documentation_url: https://docs.truelayer.com/
    support_email: support@truelayer.com
    logo_url: https://assets.truelayer.com/logo.png

  # Health check
  health:
    endpoint: /v1/health
    interval: 5m
    timeout: 10s
```

### 4.2 Connector Interface

```go
// Connector is the interface all connectors must implement.
type Connector interface {
    // Identity
    Manifest() *ConnectorManifest

    // Lifecycle
    Initialize(ctx context.Context, config ConnectorConfig) error
    Shutdown(ctx context.Context) error

    // Health
    HealthCheck(ctx context.Context) (*HealthStatus, error)

    // Capabilities - implementations vary by connector
    // Read capabilities return CanonicalEvent slices
    // Write capabilities accept CanonicalAction and return results
}

// ReadConnector can poll for data.
type ReadConnector interface {
    Connector

    // Poll retrieves new data since the given cursor.
    Poll(ctx context.Context, cursor Cursor) (*PollResult, error)
}

// WriteConnector can execute actions.
type WriteConnector interface {
    Connector

    // Execute performs an action.
    Execute(ctx context.Context, action CanonicalAction) (*ExecutionResult, error)
}

// WebhookConnector can receive push notifications.
type WebhookConnector interface {
    Connector

    // HandleWebhook processes an incoming webhook.
    HandleWebhook(ctx context.Context, payload []byte, headers map[string]string) ([]CanonicalEvent, error)

    // WebhookEndpoint returns the expected webhook path.
    WebhookEndpoint() string
}

// PollResult contains the results of a poll operation.
type PollResult struct {
    Events      []CanonicalEvent
    NextCursor  Cursor
    HasMore     bool
    PollAgainIn time.Duration // Backoff hint
}

// Cursor tracks polling position.
type Cursor struct {
    Position    string    // Opaque position string
    LastSync    time.Time
    Metadata    map[string]string
}
```

### 4.3 Transformer Contract

```go
// Transformer converts vendor-specific data to canonical events.
type Transformer interface {
    // Transform converts vendor data to canonical event(s).
    // One vendor record may produce multiple canonical events.
    Transform(vendorData []byte) ([]CanonicalEvent, error)

    // SourceEventType returns what vendor event type this handles.
    SourceEventType() string

    // TargetEventTypes returns what canonical events this produces.
    TargetEventTypes() []EventType
}

// Example: TrueLayer transaction transformer
type TrueLayerTransactionTransformer struct{}

func (t *TrueLayerTransactionTransformer) Transform(data []byte) ([]CanonicalEvent, error) {
    var tlTxn TrueLayerTransaction
    if err := json.Unmarshal(data, &tlTxn); err != nil {
        return nil, err
    }

    // Map TrueLayer-specific fields to canonical
    event := &TransactionEvent{
        BaseEvent: BaseEvent{
            ID:        generateEventID("txn", tlTxn.TransactionID),
            Type:      EventTypeTransaction,
            Version:   1,
            Connector: "truelayer-uk",
            Vendor:    "truelayer",
            SourceID:  tlTxn.TransactionID,
            IngestedAt: time.Now(),
            OccurredAt: tlTxn.Timestamp,
        },
        AccountID:       AccountID(tlTxn.AccountID),
        TransactionType: mapTransactionType(tlTxn.TransactionType),
        AmountCents:     int64(tlTxn.Amount * 100),
        Currency:        Currency(tlTxn.Currency),
        MerchantName:    normalizeMerchant(tlTxn.Description),
        MerchantRaw:     tlTxn.Description,
        Category:        mapCategory(tlTxn.Category),
        TransactionDate: tlTxn.Timestamp,
    }

    return []CanonicalEvent{event}, nil
}
```

---

## 5. Connector Catalog

### 5.1 Financial Connectors

| Connector ID | Vendor | Region | Capabilities | Status |
|-------------|--------|--------|--------------|--------|
| `truelayer-uk` | TrueLayer | UK, IE | balance, transactions, payment | Active |
| `truelayer-eu` | TrueLayer | EU | balance, transactions, payment | Active |
| `plaid-us` | Plaid | US, CA | balance, transactions | Active |
| `plaid-uk` | Plaid | UK | balance, transactions | Beta |
| `setu-in` | Setu | IN | balance, transactions | Active |
| `finbox-in` | Finbox | IN | balance, transactions | Planned |
| `yodlee-us` | Yodlee | US | balance, transactions | Planned |
| `tink-eu` | Tink | EU | balance, transactions, payment | Planned |
| `monzo-uk` | Monzo | UK | balance, transactions | Direct |
| `starling-uk` | Starling | UK | balance, transactions | Direct |

### 5.2 Email Connectors

| Connector ID | Vendor | Region | Capabilities | Status |
|-------------|--------|--------|--------------|--------|
| `gmail` | Google | Global | read, send, labels | Active |
| `outlook` | Microsoft | Global | read, send, folders | Active |
| `yahoo` | Yahoo | Global | read | Planned |
| `protonmail` | Proton | Global | read | Planned |
| `icloud-mail` | Apple | Global | read | Planned |

### 5.3 Calendar Connectors

| Connector ID | Vendor | Region | Capabilities | Status |
|-------------|--------|--------|--------------|--------|
| `google-calendar` | Google | Global | read, write | Active |
| `outlook-calendar` | Microsoft | Global | read, write | Active |
| `apple-calendar` | Apple | Global | read | Planned |
| `caldav` | Generic | Global | read, write | Planned |

### 5.4 Messaging Connectors

| Connector ID | Vendor | Region | Capabilities | Status |
|-------------|--------|--------|--------------|--------|
| `whatsapp-business` | Meta | Global | read | Limited |
| `slack` | Slack | Global | read | Active |
| `telegram` | Telegram | Global | read | Planned |
| `imessage` | Apple | Global | read | Limited |

### 5.5 Health Connectors

| Connector ID | Vendor | Region | Capabilities | Status |
|-------------|--------|--------|--------------|--------|
| `apple-health` | Apple | Global | activity, sleep, vitals, workouts | Active |
| `google-fit` | Google | Global | activity, sleep | Planned |
| `fitbit` | Google | Global | activity, sleep, vitals | Planned |
| `oura` | Oura | Global | sleep, activity | Planned |
| `peloton` | Peloton | Global | workouts | Active |
| `concept2` | Concept2 | Global | workouts | Active |
| `whoop` | Whoop | Global | sleep, recovery | Planned |

### 5.6 E-Commerce Connectors

| Connector ID | Vendor | Region | Capabilities | Status |
|-------------|--------|--------|--------------|--------|
| `amazon-orders` | Amazon | UK, US | orders, shipments | Planned |
| `ebay-orders` | eBay | Global | orders | Planned |
| `royal-mail` | Royal Mail | UK | shipments | Planned |
| `ups` | UPS | Global | shipments | Planned |
| `dpd` | DPD | UK, EU | shipments | Planned |
| `fedex` | FedEx | Global | shipments | Planned |

### 5.7 Transport Connectors

| Connector ID | Vendor | Region | Capabilities | Status |
|-------------|--------|--------|--------------|--------|
| `uber` | Uber | Global | rides | Planned |
| `bolt` | Bolt | UK, EU | rides | Planned |
| `lyft` | Lyft | US | rides | Planned |
| `trainline` | Trainline | UK, EU | bookings | Planned |
| `skyscanner` | Skyscanner | Global | bookings | Planned |

### 5.8 School Portal Connectors

| Connector ID | Vendor | Region | Capabilities | Status |
|-------------|--------|--------|--------------|--------|
| `parentmail` | ParentMail | UK | notifications, forms | Planned |
| `arbor` | Arbor | UK | notifications | Planned |
| `classcharts` | ClassCharts | UK | notifications | Planned |
| `schoolcomms` | SchoolComms | UK | notifications | Planned |
| `bromcom` | Bromcom | UK | notifications | Planned |

---

## 6. Connector Development

### 6.1 SDK Overview

```go
// SDK provides utilities for connector development.
package sdk

// NewConnector creates a new connector instance.
func NewConnector(manifest *ConnectorManifest) (*ConnectorBuilder, error)

// ConnectorBuilder helps build connectors.
type ConnectorBuilder struct {
    manifest *ConnectorManifest
}

// WithReadCapability adds a read capability.
func (b *ConnectorBuilder) WithReadCapability(cap CapabilityID, impl ReadCapabilityImpl) *ConnectorBuilder

// WithWriteCapability adds a write capability.
func (b *ConnectorBuilder) WithWriteCapability(cap CapabilityID, impl WriteCapabilityImpl) *ConnectorBuilder

// WithTransformer registers a transformer.
func (b *ConnectorBuilder) WithTransformer(t Transformer) *ConnectorBuilder

// WithOAuth2 configures OAuth2 authentication.
func (b *ConnectorBuilder) WithOAuth2(config OAuth2Config) *ConnectorBuilder

// Build creates the connector.
func (b *ConnectorBuilder) Build() (Connector, error)
```

### 6.2 Example: Building a Simple Connector

```go
package gmail

import (
    "context"
    sdk "github.com/quantumlife/connector-sdk"
)

func NewGmailConnector() (sdk.Connector, error) {
    manifest := &sdk.ConnectorManifest{
        ID:      "gmail",
        Name:    "Gmail",
        Vendor:  "google",
        Version: "1.0.0",
        Capabilities: []sdk.CapabilitySpec{
            {ID: "email.read", Read: true},
            {ID: "email.send", Write: true, RequiresApproval: true},
        },
        Regions: []string{"GLOBAL"},
    }

    return sdk.NewConnector(manifest).
        WithOAuth2(sdk.OAuth2Config{
            AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
            TokenURL:     "https://oauth2.googleapis.com/token",
            Scopes:       []string{"https://www.googleapis.com/auth/gmail.readonly"},
            RefreshAuto:  true,
        }).
        WithReadCapability("email.read", &gmailReader{}).
        WithWriteCapability("email.send", &gmailSender{}).
        WithTransformer(&gmailMessageTransformer{}).
        Build()
}

type gmailReader struct{}

func (r *gmailReader) Poll(ctx context.Context, cursor sdk.Cursor) (*sdk.PollResult, error) {
    // Implementation: call Gmail API, transform to canonical events
    // ...
}

type gmailMessageTransformer struct{}

func (t *gmailMessageTransformer) Transform(data []byte) ([]sdk.CanonicalEvent, error) {
    // Implementation: convert Gmail message to EmailMessageEvent
    // ...
}
```

### 6.3 Testing Requirements

```go
// All connectors must pass these test suites.
package testing

// SchemaComplianceTest verifies canonical event output.
func SchemaComplianceTest(c Connector) TestResult

// SecurityAuditTest checks for common vulnerabilities.
func SecurityAuditTest(c Connector) TestResult

// PerformanceTest benchmarks connector performance.
func PerformanceTest(c Connector) TestResult

// EdgeCaseTest verifies handling of malformed input.
func EdgeCaseTest(c Connector) TestResult

// TokenRefreshTest verifies OAuth token refresh.
func TokenRefreshTest(c Connector) TestResult

// Required coverage: >80%
// Required pass rate: 100% (no failures allowed)
```

---

## 7. Security Model

### 7.1 Credential Isolation

```
┌─────────────────────────────────────────────────────────────────────────┐
│                     CREDENTIAL ISOLATION MODEL                          │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  USER A's CREDENTIALS                USER B's CREDENTIALS              │
│  ────────────────────                ────────────────────              │
│                                                                         │
│  ┌─────────────────────┐            ┌─────────────────────┐            │
│  │ Vault Namespace: A  │            │ Vault Namespace: B  │            │
│  │                     │            │                     │            │
│  │ gmail/oauth_token   │            │ gmail/oauth_token   │            │
│  │ truelayer/token     │            │ plaid/token         │            │
│  │ ...                 │            │ ...                 │            │
│  └─────────────────────┘            └─────────────────────┘            │
│                                                                         │
│  - Users cannot access each other's credentials                         │
│  - Connectors can only access their own credentials                     │
│  - Credentials encrypted at rest with user-specific key                 │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 7.2 Connector Sandboxing

```yaml
connector_security:
  # Network isolation
  network:
    allowed_domains:
      - "*.googleapis.com"      # For Gmail connector
      - "*.truelayer.com"       # For TrueLayer connector
      # Each connector whitelists its domains
    denied: ["*"]               # Deny all by default

  # Resource limits
  resources:
    max_memory: 256Mi
    max_cpu: 500m
    max_file_descriptors: 100
    max_network_connections: 50

  # Secrets access
  secrets:
    own_credentials: true       # Can access own OAuth tokens
    other_credentials: false    # Cannot access other connectors
    system_secrets: false       # Cannot access system secrets

  # Audit
  audit:
    log_all_api_calls: true
    log_credentials_access: true
    log_network_requests: true
```

### 7.3 Supply Chain Security

```yaml
connector_supply_chain:
  # Code signing
  signing:
    required: true
    algorithm: ED25519
    verify_on_load: true

  # Dependency scanning
  dependencies:
    scan_on_publish: true
    block_known_vulnerabilities: true
    update_policy: automatic_security_patches

  # Provenance
  provenance:
    track_git_commit: true
    track_build_timestamp: true
    track_builder_identity: true
    sbom_required: true  # Software Bill of Materials
```

---

## 8. Operational Model

### 8.1 Connector Health Monitoring

```yaml
monitoring:
  health_checks:
    interval: 5m
    timeout: 10s
    failure_threshold: 3
    recovery_threshold: 2

  alerts:
    connector_unhealthy:
      severity: warning
      action: notify_user

    connector_failed:
      severity: critical
      action: disable_polling

    auth_expired:
      severity: warning
      action: prompt_reauth

  metrics:
    - connector_poll_latency_ms
    - connector_poll_success_rate
    - connector_transform_errors
    - connector_events_produced
    - connector_auth_refresh_count
```

### 8.2 Connector Updates

```yaml
update_policy:
  # Automatic updates for security patches
  security_patches:
    auto_apply: true
    notification: after

  # Manual approval for feature updates
  feature_updates:
    auto_apply: false
    notification: before
    rollback_window: 24h

  # Version pinning available
  version_pinning:
    allowed: true
    max_age: 90d  # Must update within 90 days
```

### 8.3 Rate Limiting

```yaml
rate_limits:
  # Per-connector limits
  per_connector:
    gmail:
      requests_per_minute: 100
      requests_per_day: 10000

    truelayer:
      requests_per_minute: 60
      requests_per_day: 1000

  # Global limits
  global:
    total_requests_per_minute: 500
    total_events_per_day: 100000

  # Backoff strategy
  backoff:
    initial: 1s
    max: 5m
    multiplier: 2
```

---

## 9. User Experience

### 9.1 Connection Flow

```
┌─────────────────────────────────────────────────────────────────────────┐
│                     USER CONNECTION FLOW                                │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  1. User wants to connect a capability                                  │
│     ┌──────────────────────────────────────┐                           │
│     │  "Connect your bank account"          │                           │
│     │                                        │                           │
│     │  [Connect UK Bank]  [Connect US Bank]  │                           │
│     └──────────────────────────────────────┘                           │
│                                                                         │
│  2. System shows available banks (region-filtered)                      │
│     ┌──────────────────────────────────────┐                           │
│     │  Select your bank:                    │                           │
│     │                                        │                           │
│     │  🏦 Barclays                          │                           │
│     │  🏦 NatWest                           │                           │
│     │  🏦 Monzo                             │                           │
│     │  🏦 Starling                          │                           │
│     │  ...                                  │                           │
│     └──────────────────────────────────────┘                           │
│                                                                         │
│  3. OAuth flow with selected bank                                       │
│     ┌──────────────────────────────────────┐                           │
│     │  [Bank's login page]                  │                           │
│     │                                        │                           │
│     │  QuantumLife wants to:                │                           │
│     │  ✓ View your account balances         │                           │
│     │  ✓ View your transactions             │                           │
│     │                                        │                           │
│     │  [Authorize]  [Cancel]                │                           │
│     └──────────────────────────────────────┘                           │
│                                                                         │
│  4. Success + initial sync                                              │
│     ┌──────────────────────────────────────┐                           │
│     │  ✅ Barclays connected!               │                           │
│     │                                        │                           │
│     │  Syncing your accounts...             │                           │
│     │  ████████░░ 80%                       │                           │
│     │                                        │                           │
│     │  Found:                               │                           │
│     │  • Current Account (****1234)         │                           │
│     │  • Savings Account (****5678)         │                           │
│     └──────────────────────────────────────┘                           │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 9.2 Connection Management

```
┌─────────────────────────────────────────────────────────────────────────┐
│                   CONNECTED ACCOUNTS                                    │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  BANKING                                          [+ Add Bank]          │
│  ───────                                                                │
│  🏦 Barclays UK                                                        │
│     Current Account (****1234)     ✅ Synced 5m ago                    │
│     Savings Account (****5678)     ✅ Synced 5m ago                    │
│     [Refresh] [Disconnect]                                              │
│                                                                         │
│  🏦 Monzo                                                               │
│     Personal (****9012)            ✅ Synced 3m ago                    │
│     [Refresh] [Disconnect]                                              │
│                                                                         │
│  EMAIL                                            [+ Add Email]         │
│  ─────                                                                  │
│  📧 Gmail (satish@gmail.com)       ✅ Synced 1m ago                    │
│     [Refresh] [Disconnect]                                              │
│                                                                         │
│  📧 Outlook (satish@employer.com)  ⚠️ Re-auth needed                   │
│     [Re-authorize] [Disconnect]                                         │
│                                                                         │
│  CALENDAR                                         [+ Add Calendar]      │
│  ────────                                                               │
│  📅 Google Calendar                ✅ Synced 2m ago                    │
│     [Refresh] [Disconnect]                                              │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 10. Future Roadmap

### 10.1 Phase 1: Core Connectors (MVP)

| Priority | Connector | Capability | Target |
|----------|-----------|------------|--------|
| P0 | Gmail | email.read | Q1 |
| P0 | Google Calendar | calendar.read | Q1 |
| P0 | TrueLayer UK | finance.balance, finance.transactions | Q1 |
| P1 | Outlook | email.read | Q2 |
| P1 | Apple Health | health.* | Q2 |

### 10.2 Phase 2: Extended Coverage

| Priority | Connector | Capability | Target |
|----------|-----------|------------|--------|
| P1 | TrueLayer UK | finance.payment | Q2 |
| P1 | WhatsApp Business | messaging.read | Q2 |
| P2 | Plaid US | finance.* | Q3 |
| P2 | School portals | school.notifications | Q3 |
| P2 | Amazon | orders, shipments | Q3 |

### 10.3 Phase 3: Global Expansion

| Priority | Connector | Capability | Target |
|----------|-----------|------------|--------|
| P2 | Setu India | finance.* | Q4 |
| P3 | Additional EU banks | finance.* | Q4 |
| P3 | Regional e-commerce | orders, shipments | Q4+ |

---

## 11. Document History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2025-01-01 | Core Team | Initial version |
