# Canonical Capabilities v1

## Document Status

| Field | Value |
|-------|-------|
| Version | 1.0 |
| Status | Draft |
| Author | QuantumLife Core Team |
| Date | 2025-01-01 |
| Supersedes | None |
| Related | TECHNICAL_ARCHITECTURE_V1.md, TECH_SPEC_V1.md, MARKETPLACE_V1.md |

---

## 1. Overview

This document defines the complete catalog of **Canonical Capabilities** — the abstraction layer between QuantumLife's core engine and the diverse ecosystem of vendors and services.

**Design Philosophy**: The core engine knows only capabilities and canonical events. Vendor diversity is handled entirely at the connector layer. This separation enables:

1. **Vendor Independence**: Add new vendors without modifying core
2. **Regional Flexibility**: UK, India, US vendors map to same capabilities
3. **Future-Proofing**: New services fit into existing capability taxonomy
4. **Testability**: Core can be tested with mock capabilities

---

## 2. Capability Taxonomy

### 2.1 Capability Hierarchy

```
capabilities/
├── email/
│   ├── email.read            # Read email messages
│   ├── email.send            # Send email messages
│   └── email.labels          # Manage labels/folders
│
├── calendar/
│   ├── calendar.read         # Read calendar events
│   ├── calendar.write        # Create/modify events
│   └── calendar.availability # Check free/busy
│
├── finance/
│   ├── finance.balance       # Read account balances
│   ├── finance.transactions  # Read transaction history
│   ├── finance.payment       # Initiate payments
│   └── finance.standing      # Manage standing orders
│
├── messaging/
│   ├── messaging.read        # Read messages
│   └── messaging.send        # Send messages
│
├── health/
│   ├── health.activity       # Activity/steps data
│   ├── health.sleep          # Sleep data
│   ├── health.vitals         # Heart rate, HRV, etc.
│   └── health.workouts       # Workout sessions
│
├── commerce/
│   ├── commerce.orders       # Purchase orders
│   ├── commerce.shipments    # Package tracking
│   └── commerce.subscriptions # Recurring subscriptions
│
├── transport/
│   ├── transport.rides       # Taxi/rideshare
│   ├── transport.trains      # Rail bookings
│   └── transport.flights     # Flight bookings
│
├── documents/
│   ├── documents.read        # Read documents
│   └── documents.write       # Write documents
│
├── school/
│   ├── school.notifications  # School communications
│   ├── school.calendar       # School calendar
│   └── school.forms          # Forms and consents
│
└── identity/
    ├── identity.contacts     # Contact information
    └── identity.profile      # Profile data
```

### 2.2 Capability Naming Convention

```
{domain}.{action}

Examples:
- email.read       # Domain: email, Action: read
- finance.payment  # Domain: finance, Action: payment
- health.vitals    # Domain: health, Action: vitals (read implied)
```

---

## 3. Capability Specifications

### 3.1 Email Capabilities

#### email.read

Read email messages from any email provider.

```yaml
capability:
  id: email.read
  domain: email
  type: READ

  # What this capability produces
  canonical_events:
    - EmailMessageEvent

  # Connectors implementing this capability
  implementations:
    - connector: gmail
      vendor: google
      regions: [GLOBAL]
      auth: oauth2
      polling_interval: 5m
      webhook_support: true

    - connector: outlook
      vendor: microsoft
      regions: [GLOBAL]
      auth: oauth2
      polling_interval: 5m
      webhook_support: true

    - connector: yahoo
      vendor: yahoo
      regions: [GLOBAL]
      auth: oauth2
      polling_interval: 15m
      webhook_support: false

    - connector: protonmail
      vendor: proton
      regions: [GLOBAL]
      auth: oauth2
      polling_interval: 15m
      webhook_support: false

  # Data extraction capabilities
  features:
    thread_support: true
    attachment_metadata: true
    attachment_download: true
    full_body: true
    html_body: true
    labels: true
    folders: true

  # Privacy considerations
  privacy:
    body_stored: false  # Only preview stored
    attachments_stored: false
    sender_indexed: true
    subject_indexed: true
```

**Interface**:

```go
type EmailReadCapability interface {
    // Poll for new messages since cursor
    Poll(ctx context.Context, cursor Cursor) (*EmailPollResult, error)

    // Get a specific message by ID
    GetMessage(ctx context.Context, messageID string) (*EmailMessageEvent, error)

    // List messages with filters
    ListMessages(ctx context.Context, filter EmailFilter, page Pagination) (*EmailListResult, error)

    // Get attachment
    GetAttachment(ctx context.Context, messageID string, attachmentID string) (*Attachment, error)
}

type EmailFilter struct {
    Folder      string
    From        string
    Subject     string
    After       time.Time
    Before      time.Time
    HasAttachment *bool
    IsUnread    *bool
}
```

#### email.send

Send email messages through any email provider.

```yaml
capability:
  id: email.send
  domain: email
  type: WRITE

  # What actions this capability can execute
  canonical_actions:
    - SendEmailAction
    - ReplyEmailAction
    - ForwardEmailAction

  # v9+ enforcement
  enforcement:
    requires_approval: true
    policy_binding: true
    view_binding: true
    forced_pause: 3s
    audit_level: FULL

  implementations:
    - connector: gmail
      vendor: google
      regions: [GLOBAL]

    - connector: outlook
      vendor: microsoft
      regions: [GLOBAL]

  features:
    html_body: true
    attachments: true
    cc_bcc: true
    reply_to: true
    scheduling: false  # Not supported yet
```

**Interface**:

```go
type EmailSendCapability interface {
    // Send a new email
    Send(ctx context.Context, action SendEmailAction) (*SendResult, error)

    // Reply to an existing email
    Reply(ctx context.Context, action ReplyEmailAction) (*SendResult, error)

    // Forward an email
    Forward(ctx context.Context, action ForwardEmailAction) (*SendResult, error)
}

type SendEmailAction struct {
    BaseAction

    To          []EmailAddress
    Cc          []EmailAddress
    Bcc         []EmailAddress
    Subject     string
    BodyPlain   string
    BodyHTML    string
    Attachments []Attachment
    ReplyTo     *EmailAddress
}
```

---

### 3.2 Calendar Capabilities

#### calendar.read

Read calendar events from any calendar provider.

```yaml
capability:
  id: calendar.read
  domain: calendar
  type: READ

  canonical_events:
    - CalendarEventEvent

  implementations:
    - connector: google-calendar
      vendor: google
      regions: [GLOBAL]
      auth: oauth2
      polling_interval: 5m
      webhook_support: true

    - connector: outlook-calendar
      vendor: microsoft
      regions: [GLOBAL]
      auth: oauth2
      polling_interval: 5m
      webhook_support: true

    - connector: apple-calendar
      vendor: apple
      regions: [GLOBAL]
      auth: oauth2
      polling_interval: 15m
      webhook_support: false

    - connector: caldav
      vendor: generic
      regions: [GLOBAL]
      auth: basic
      polling_interval: 15m
      webhook_support: false

  features:
    recurring_events: true
    attendees: true
    reminders: true
    attachments: true
    conference_links: true
    multiple_calendars: true
```

**Interface**:

```go
type CalendarReadCapability interface {
    // List calendars
    ListCalendars(ctx context.Context) ([]Calendar, error)

    // Poll for changes since cursor
    Poll(ctx context.Context, calendarID string, cursor Cursor) (*CalendarPollResult, error)

    // Get events in date range
    GetEvents(ctx context.Context, calendarID string, start, end time.Time) ([]CalendarEventEvent, error)

    // Get single event
    GetEvent(ctx context.Context, calendarID string, eventID string) (*CalendarEventEvent, error)

    // Check free/busy
    FreeBusy(ctx context.Context, calendarIDs []string, start, end time.Time) (*FreeBusyResult, error)
}

type Calendar struct {
    ID          string
    Name        string
    Description string
    Color       string
    Primary     bool
    AccessRole  CalendarAccessRole
}
```

#### calendar.write

Create and modify calendar events.

```yaml
capability:
  id: calendar.write
  domain: calendar
  type: WRITE

  canonical_actions:
    - CreateEventAction
    - UpdateEventAction
    - DeleteEventAction
    - RSVPAction

  enforcement:
    requires_approval: true
    policy_binding: true
    view_binding: true
    forced_pause: 2s
    audit_level: FULL

  implementations:
    - connector: google-calendar
      vendor: google
      regions: [GLOBAL]

    - connector: outlook-calendar
      vendor: microsoft
      regions: [GLOBAL]

  features:
    create: true
    update: true
    delete: true
    rsvp: true
    invite_attendees: true
```

**Interface**:

```go
type CalendarWriteCapability interface {
    // Create a new event
    CreateEvent(ctx context.Context, action CreateEventAction) (*EventResult, error)

    // Update an existing event
    UpdateEvent(ctx context.Context, action UpdateEventAction) (*EventResult, error)

    // Delete an event
    DeleteEvent(ctx context.Context, action DeleteEventAction) (*EventResult, error)

    // RSVP to an event
    RSVP(ctx context.Context, action RSVPAction) (*RSVPResult, error)
}

type CreateEventAction struct {
    BaseAction

    CalendarID  string
    Title       string
    Description string
    Location    string
    Start       time.Time
    End         time.Time
    Timezone    string
    AllDay      bool
    Attendees   []EventAttendee
    Reminders   []Reminder
}
```

---

### 3.3 Finance Capabilities

#### finance.balance

Read account balances from any financial provider.

```yaml
capability:
  id: finance.balance
  domain: finance
  type: READ

  canonical_events:
    - BalanceEvent

  implementations:
    - connector: truelayer-uk
      vendor: truelayer
      regions: [UK, IE]
      auth: oauth2
      polling_interval: 15m
      banks_supported: 50+

    - connector: plaid-us
      vendor: plaid
      regions: [US, CA]
      auth: link_token
      polling_interval: 15m
      banks_supported: 11000+

    - connector: setu-in
      vendor: setu
      regions: [IN]
      auth: oauth2
      polling_interval: 30m
      banks_supported: 20+
      note: "Account Aggregator framework"

    - connector: tink-eu
      vendor: tink
      regions: [EU]
      auth: oauth2
      polling_interval: 15m
      note: "PSD2 compliant"

  # Direct bank APIs (faster, more reliable)
  direct_implementations:
    - connector: monzo-uk
      vendor: monzo
      regions: [UK]
      auth: oauth2
      polling_interval: 5m
      note: "Native API, real-time webhooks"

    - connector: starling-uk
      vendor: starling
      regions: [UK]
      auth: oauth2
      polling_interval: 5m
      note: "Native API"

  features:
    current_balance: true
    available_balance: true
    pending_balance: true
    credit_limit: true  # For credit accounts
    multiple_accounts: true
    account_details: true
```

**Interface**:

```go
type FinanceBalanceCapability interface {
    // List all accounts
    ListAccounts(ctx context.Context) ([]Account, error)

    // Get balance for specific account
    GetBalance(ctx context.Context, accountID string) (*BalanceEvent, error)

    // Get balances for all accounts
    GetAllBalances(ctx context.Context) ([]BalanceEvent, error)
}

type Account struct {
    AccountID     string
    AccountType   AccountType
    AccountName   string
    Currency      Currency
    Institution   string
    AccountNumber string  // Masked: ****1234
    SortCode      string  // UK only
    IBAN          string  // EU only
}
```

#### finance.transactions

Read transaction history from any financial provider.

```yaml
capability:
  id: finance.transactions
  domain: finance
  type: READ

  canonical_events:
    - TransactionEvent

  implementations:
    - connector: truelayer-uk
      vendor: truelayer
      regions: [UK, IE]
      historical_depth: 90d
      pending_transactions: true

    - connector: plaid-us
      vendor: plaid
      regions: [US, CA]
      historical_depth: 730d  # 2 years
      pending_transactions: true

    - connector: setu-in
      vendor: setu
      regions: [IN]
      historical_depth: 365d

  features:
    pending_transactions: true
    merchant_enrichment: true
    category_enrichment: true
    recurring_detection: true
    pagination: true
```

**Interface**:

```go
type FinanceTransactionsCapability interface {
    // Poll for new transactions
    Poll(ctx context.Context, accountID string, cursor Cursor) (*TransactionPollResult, error)

    // Get transactions in date range
    GetTransactions(ctx context.Context, accountID string, filter TransactionFilter) ([]TransactionEvent, error)

    // Get single transaction
    GetTransaction(ctx context.Context, accountID string, transactionID string) (*TransactionEvent, error)
}

type TransactionFilter struct {
    From        time.Time
    To          time.Time
    Type        *TransactionType    // DEBIT, CREDIT
    Status      *TransactionStatus  // PENDING, POSTED
    MinAmount   *int64
    MaxAmount   *int64
    Category    *TransactionCategory
    Merchant    string              // Search by merchant name
}
```

#### finance.payment

Initiate payments through financial providers.

```yaml
capability:
  id: finance.payment
  domain: finance
  type: WRITE

  canonical_actions:
    - PaymentAction

  enforcement:
    requires_approval: always
    policy_binding: true
    view_binding: true
    forced_pause: 5s  # Longer pause for payments
    audit_level: FULL

    # Additional v9+ enforcement
    v9_compliance:
      payee_registry_lock: true   # v9.10
      provider_registry_lock: true # v9.9
      no_auto_retry: true          # v9.8
      single_trace: true           # v9.8
      idempotency: true            # v9.6

  implementations:
    - connector: truelayer-uk
      vendor: truelayer
      regions: [UK, IE]
      payment_types: [DOMESTIC, SEPA]
      instant_payment: true
      note: "UK Faster Payments, SEPA Instant"

    - connector: stripe
      vendor: stripe
      regions: [GLOBAL]
      payment_types: [CARD]
      note: "For business payments only"

  features:
    domestic_payments: true
    international_payments: false  # Not in v1
    scheduled_payments: false      # Not in v1
    recurring_payments: false      # Not in v1
    payee_validation: true
```

**Interface**:

```go
type FinancePaymentCapability interface {
    // Initiate a payment
    Initiate(ctx context.Context, action PaymentAction) (*PaymentInitResult, error)

    // Get payment status
    GetStatus(ctx context.Context, paymentID string) (*PaymentStatus, error)
}

type PaymentAction struct {
    BaseAction

    // Source
    SourceAccountID string

    // Destination
    PayeeID         PayeeID           // From PayeeRegistry
    PayeeName       string            // Display name
    PayeeAccount    PayeeAccountDetails

    // Amount
    AmountCents     int64
    Currency        Currency
    Reference       string            // Payment reference

    // v9+ binding (set by ExecutionEnvelope)
    IdempotencyKey      string
    PolicySnapshotHash  string
    ViewSnapshotHash    string
}

type PayeeAccountDetails struct {
    // UK
    SortCode    string  `json:"sort_code,omitempty"`
    AccountNumber string `json:"account_number,omitempty"`

    // EU
    IBAN        string  `json:"iban,omitempty"`
    BIC         string  `json:"bic,omitempty"`

    // US
    RoutingNumber string `json:"routing_number,omitempty"`
    AccountNumber string `json:"account_number_us,omitempty"`

    // India
    IFSC        string  `json:"ifsc,omitempty"`
    AccountNumber string `json:"account_number_in,omitempty"`
}
```

---

### 3.4 Messaging Capabilities

#### messaging.read

Read messages from messaging platforms.

```yaml
capability:
  id: messaging.read
  domain: messaging
  type: READ

  canonical_events:
    - MessageEvent

  implementations:
    - connector: whatsapp-business
      vendor: meta
      regions: [GLOBAL]
      auth: api_key
      note: "Business API only, limited personal use"
      limitations:
        - "Cannot read personal chats without business account"
        - "User must initiate conversation"

    - connector: slack
      vendor: slack
      regions: [GLOBAL]
      auth: oauth2
      features:
        channels: true
        direct_messages: true
        threads: true
        reactions: true

    - connector: telegram
      vendor: telegram
      regions: [GLOBAL]
      auth: bot_token
      note: "Bot API, requires user to add bot"

  features:
    text_messages: true
    media_messages: true
    reactions: true
    threads: true
    read_receipts: true

  privacy:
    message_content_stored: false  # Privacy-first
    sender_indexed: true
    timestamp_indexed: true
```

**Interface**:

```go
type MessagingReadCapability interface {
    // List conversations
    ListConversations(ctx context.Context) ([]Conversation, error)

    // Poll for new messages
    Poll(ctx context.Context, cursor Cursor) (*MessagePollResult, error)

    // Get messages in conversation
    GetMessages(ctx context.Context, conversationID string, filter MessageFilter) ([]MessageEvent, error)
}

type Conversation struct {
    ID              string
    Type            ConversationType  // DIRECT, GROUP
    Name            string            // Group name or contact name
    Participants    []Participant
    LastMessage     *time.Time
    UnreadCount     int
}
```

---

### 3.5 Health Capabilities

#### health.activity

Read activity data (steps, calories, distance).

```yaml
capability:
  id: health.activity
  domain: health
  type: READ

  canonical_events:
    - ActivityEvent

  implementations:
    - connector: apple-health
      vendor: apple
      regions: [GLOBAL]
      auth: healthkit
      platform: iOS
      data_sources: [Apple Watch, iPhone]

    - connector: google-fit
      vendor: google
      regions: [GLOBAL]
      auth: oauth2
      platform: Android
      data_sources: [Wear OS, Android phone]

    - connector: fitbit
      vendor: google
      regions: [GLOBAL]
      auth: oauth2
      data_sources: [Fitbit devices]

  features:
    steps: true
    distance: true
    calories: true
    active_minutes: true
    floors: true
    intraday: true  # Minute-by-minute data
```

**Interface**:

```go
type HealthActivityCapability interface {
    // Get activity summary for date
    GetActivity(ctx context.Context, date time.Time) (*ActivityEvent, error)

    // Get activity summaries for date range
    GetActivityRange(ctx context.Context, from, to time.Time) ([]ActivityEvent, error)

    // Get intraday data (if available)
    GetIntraday(ctx context.Context, date time.Time, metric string) ([]IntradayPoint, error)
}

type IntradayPoint struct {
    Time  time.Time
    Value int
}
```

#### health.sleep

Read sleep data.

```yaml
capability:
  id: health.sleep
  domain: health
  type: READ

  canonical_events:
    - SleepEvent

  implementations:
    - connector: apple-health
      vendor: apple
      regions: [GLOBAL]
      features:
        sleep_stages: true
        sleep_score: false

    - connector: oura
      vendor: oura
      regions: [GLOBAL]
      auth: oauth2
      features:
        sleep_stages: true
        sleep_score: true
        hrv_during_sleep: true

    - connector: whoop
      vendor: whoop
      regions: [GLOBAL]
      auth: oauth2
      features:
        sleep_stages: true
        sleep_score: true
        recovery_score: true
```

#### health.workouts

Read workout/exercise sessions.

```yaml
capability:
  id: health.workouts
  domain: health
  type: READ

  canonical_events:
    - WorkoutEvent

  implementations:
    - connector: apple-health
      vendor: apple
      regions: [GLOBAL]
      workout_types: [all]

    - connector: peloton
      vendor: peloton
      regions: [GLOBAL]
      auth: oauth2
      workout_types: [CYCLING, RUNNING, STRENGTH, YOGA]
      features:
        instructor_data: true
        class_data: true
        leaderboard: false  # Privacy

    - connector: concept2
      vendor: concept2
      regions: [GLOBAL]
      auth: oauth2
      workout_types: [ROWING, BIKEERG, SKIERG]
      features:
        stroke_data: true
        split_times: true
```

**Interface**:

```go
type HealthWorkoutsCapability interface {
    // Poll for new workouts
    Poll(ctx context.Context, cursor Cursor) (*WorkoutPollResult, error)

    // Get workouts in date range
    GetWorkouts(ctx context.Context, from, to time.Time) ([]WorkoutEvent, error)

    // Get single workout with details
    GetWorkout(ctx context.Context, workoutID string) (*WorkoutEvent, error)
}
```

---

### 3.6 Commerce Capabilities

#### commerce.orders

Read purchase orders from e-commerce platforms.

```yaml
capability:
  id: commerce.orders
  domain: commerce
  type: READ

  canonical_events:
    - OrderEvent

  implementations:
    - connector: amazon-orders
      vendor: amazon
      regions: [UK, US, DE, FR]
      auth: oauth2
      features:
        order_history: true
        order_items: true
        delivery_tracking: true

    - connector: ebay-orders
      vendor: ebay
      regions: [GLOBAL]
      auth: oauth2

  # Email-based extraction (fallback)
  email_extraction:
    enabled: true
    patterns:
      - vendor: amazon
        subject_pattern: "Your Amazon.* order"
      - vendor: ebay
        subject_pattern: "Order confirmed"
      - vendor: generic
        subject_pattern: "(order|receipt|confirmation)"
```

#### commerce.shipments

Track package shipments.

```yaml
capability:
  id: commerce.shipments
  domain: commerce
  type: READ

  canonical_events:
    - ShipmentEvent

  implementations:
    - connector: royal-mail
      vendor: royal-mail
      regions: [UK]
      auth: api_key

    - connector: ups
      vendor: ups
      regions: [GLOBAL]
      auth: oauth2

    - connector: dpd
      vendor: dpd
      regions: [UK, EU]
      auth: api_key

    - connector: fedex
      vendor: fedex
      regions: [GLOBAL]
      auth: oauth2

  # Email-based tracking extraction
  email_extraction:
    enabled: true
    patterns:
      - extract: tracking_number
        from: email_body
        link_to: carrier_api
```

#### commerce.subscriptions

Track recurring subscriptions.

```yaml
capability:
  id: commerce.subscriptions
  domain: commerce
  type: READ

  canonical_events:
    - SubscriptionEvent
    - InvoiceEvent

  implementations:
    # No direct APIs typically - extracted from:
    extraction_sources:
      - email_receipts
      - bank_transactions  # Recurring pattern detection
      - calendar_events    # Annual renewals

  features:
    detect_from_transactions: true
    detect_from_emails: true
    renewal_prediction: true
    cancellation_tracking: true
```

---

### 3.7 School Capabilities

#### school.notifications

Read notifications from school portals.

```yaml
capability:
  id: school.notifications
  domain: school
  type: READ

  canonical_events:
    - SchoolNotificationEvent

  implementations:
    - connector: parentmail
      vendor: parentmail
      regions: [UK]
      auth: credentials
      features:
        notifications: true
        forms: true
        payments: true

    - connector: arbor
      vendor: arbor
      regions: [UK]
      auth: credentials
      features:
        notifications: true
        attendance: true
        grades: true

    - connector: classcharts
      vendor: classcharts
      regions: [UK]
      auth: credentials
      features:
        behaviour: true
        homework: true
        timetable: true

  # Email-based fallback
  email_extraction:
    enabled: true
    patterns:
      - sender_domain: "*.sch.uk"
        extract: [subject, body, attachments]
      - sender_domain: "parentmail.co.uk"
        extract: [subject, body, links]

  features:
    deadline_extraction: true
    form_detection: true
    payment_detection: true
    event_detection: true
```

---

## 4. Capability Resolution

### 4.1 Resolution Algorithm

When a user requests a capability, the system resolves to the best available connector:

```go
func ResolveCapability(
    capabilityID CapabilityID,
    userRegion Region,
    userPreferences ConnectorPreferences,
) (Connector, error) {
    // 1. Get all connectors implementing this capability
    candidates := registry.GetConnectorsForCapability(capabilityID)

    // 2. Filter by region
    candidates = filterByRegion(candidates, userRegion)

    // 3. Filter by user preferences (if any)
    if userPreferences.PreferredVendor != "" {
        candidates = filterByVendor(candidates, userPreferences.PreferredVendor)
    }

    // 4. Sort by priority (configured per region)
    sort.Sort(ByRegionalPriority{candidates, userRegion})

    // 5. Check health/availability
    for _, connector := range candidates {
        if connector.IsHealthy() {
            return connector, nil
        }
    }

    return nil, ErrNoAvailableConnector
}
```

### 4.2 Regional Priority Configuration

```yaml
regional_priorities:
  UK:
    finance.balance:
      - truelayer-uk      # Primary
      - plaid-uk          # Fallback
    finance.payment:
      - truelayer-uk      # Only option currently

  US:
    finance.balance:
      - plaid-us          # Primary
      - yodlee-us         # Fallback
    finance.payment:
      - null              # Not available in v1

  IN:
    finance.balance:
      - setu-in           # Primary (Account Aggregator)
      - finbox-in         # Fallback
    finance.payment:
      - null              # Not available in v1

  GLOBAL:
    email.read:
      - gmail             # Most common
      - outlook           # Enterprise
      - yahoo             # Legacy
```

---

## 5. Canonical Actions

### 5.1 Action Taxonomy

All write capabilities operate through canonical actions:

```
actions/
├── email/
│   ├── SendEmailAction
│   ├── ReplyEmailAction
│   └── ForwardEmailAction
│
├── calendar/
│   ├── CreateEventAction
│   ├── UpdateEventAction
│   ├── DeleteEventAction
│   └── RSVPAction
│
├── finance/
│   └── PaymentAction
│
└── messaging/
    └── SendMessageAction
```

### 5.2 Base Action Structure

```go
// CanonicalAction is the base interface for all write actions.
type CanonicalAction interface {
    // Identity
    ActionID() ActionID
    ActionType() ActionType

    // Targeting
    TargetCapability() CapabilityID
    TargetConnector() ConnectorID  // May be empty (let system choose)

    // v9+ binding
    IdempotencyKey() string
    PolicySnapshotHash() string
    ViewSnapshotHash() string

    // Approval
    RequiresApproval() bool
    ApprovalThreshold() int

    // Serialization
    MarshalAction() ([]byte, error)
}

type BaseAction struct {
    ID                  ActionID    `json:"action_id"`
    Type                ActionType  `json:"action_type"`
    Capability          CapabilityID `json:"capability"`
    Connector           ConnectorID `json:"connector,omitempty"`

    // v9+ fields
    IdempotencyKey      string      `json:"idempotency_key"`
    PolicySnapshotHash  string      `json:"policy_snapshot_hash"`
    ViewSnapshotHash    string      `json:"view_snapshot_hash"`

    // Approval
    RequiresApproval    bool        `json:"requires_approval"`
    ApprovalThreshold   int         `json:"approval_threshold"`
}
```

---

## 6. Capability Testing

### 6.1 Test Categories

Every capability implementation must pass:

```yaml
test_categories:
  schema_compliance:
    description: "Output matches canonical event schema"
    required: true
    coverage: 100%

  data_mapping:
    description: "Vendor fields correctly mapped to canonical"
    required: true
    test_cases:
      - normal_data
      - edge_cases
      - malformed_input
      - empty_fields

  authentication:
    description: "Auth flow works correctly"
    required: true
    test_cases:
      - initial_auth
      - token_refresh
      - token_expiry
      - invalid_credentials

  polling:
    description: "Incremental polling works correctly"
    required: true
    test_cases:
      - first_poll
      - incremental_poll
      - empty_response
      - pagination

  error_handling:
    description: "Errors are handled gracefully"
    required: true
    test_cases:
      - network_error
      - rate_limit
      - server_error
      - invalid_response

  performance:
    description: "Meets performance requirements"
    required: true
    thresholds:
      poll_latency_p99: 5s
      transform_latency_p99: 100ms
      memory_usage: 256MB
```

### 6.2 Mock Capabilities

For testing core engine without real connectors:

```go
// MockEmailReadCapability provides test data for email.read
type MockEmailReadCapability struct {
    Messages []EmailMessageEvent
    Cursor   Cursor
}

func (m *MockEmailReadCapability) Poll(ctx context.Context, cursor Cursor) (*EmailPollResult, error) {
    // Return pre-configured test messages
}

// MockFinanceBalanceCapability provides test data for finance.balance
type MockFinanceBalanceCapability struct {
    Accounts []Account
    Balances map[string]BalanceEvent
}

func (m *MockFinanceBalanceCapability) GetAllBalances(ctx context.Context) ([]BalanceEvent, error) {
    // Return pre-configured test balances
}
```

---

## 7. Capability Evolution

### 7.1 Versioning Strategy

```yaml
versioning:
  # Canonical events are versioned
  event_versions:
    TransactionEvent: 1
    EmailMessageEvent: 1
    CalendarEventEvent: 1

  # Version changes
  version_policy:
    additive_changes: minor_bump
    breaking_changes: major_bump
    deprecation_period: 6_months

  # Backward compatibility
  compatibility:
    # Connectors can produce multiple versions
    multi_version_output: true
    # Core can consume multiple versions
    multi_version_input: true
```

### 7.2 Adding New Capabilities

```
1. Define capability specification (this document)
2. Define canonical events (TECH_SPEC_V1.md)
3. Create capability interface (Go code)
4. Implement mock capability (for testing)
5. Implement first connector
6. Add to registry
7. Add tests
8. Document in marketplace
```

---

## 8. Document History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2025-01-01 | Core Team | Initial version |
