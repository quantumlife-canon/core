# Technical Specification v1

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

This document specifies the technical interfaces, data models, and protocols for QuantumLife. It serves as the contract between components.

**Key Design Principle**: Canonical event models abstract vendor diversity. Core engine processes only canonical events, never vendor-specific formats.

---

## 2. Canonical Event Models

All data entering the core engine must be transformed into canonical events. This section defines the complete canonical event taxonomy.

### 2.1 Base Event Structure

```go
// CanonicalEvent is the base interface for all events.
type CanonicalEvent interface {
    // Identity
    EventID() EventID              // Globally unique, deterministic
    EventType() EventType          // e.g., "transaction", "email_message"
    EventVersion() int             // Schema version

    // Provenance
    SourceConnector() ConnectorID  // Which connector produced this
    SourceVendor() VendorID        // Which vendor the data came from
    SourceID() string              // Original ID in source system
    IngestedAt() time.Time         // When we received it
    OccurredAt() time.Time         // When it actually happened

    // Classification hints (set by connector, refined by core)
    SuggestedCircles() []CircleID
    SuggestedEntities() []EntityID

    // Serialization
    MarshalCanonical() ([]byte, error)
}

// BaseEvent provides common fields.
type BaseEvent struct {
    ID              EventID       `json:"event_id"`
    Type            EventType     `json:"event_type"`
    Version         int           `json:"version"`
    Connector       ConnectorID   `json:"source_connector"`
    Vendor          VendorID      `json:"source_vendor"`
    SourceID        string        `json:"source_id"`
    IngestedAt      time.Time     `json:"ingested_at"`
    OccurredAt      time.Time     `json:"occurred_at"`
    Circles         []CircleID    `json:"suggested_circles,omitempty"`
    Entities        []EntityID    `json:"suggested_entities,omitempty"`
}
```

### 2.2 Financial Events

#### TransactionEvent

```go
// TransactionEvent represents a financial transaction from any bank/provider.
type TransactionEvent struct {
    BaseEvent

    // Account identification
    AccountID       AccountID           `json:"account_id"`
    AccountType     AccountType         `json:"account_type"`     // CHECKING, SAVINGS, CREDIT

    // Transaction details
    TransactionType TransactionType     `json:"transaction_type"` // DEBIT, CREDIT
    Kind            TransactionKind     `json:"kind"`             // PURCHASE, REFUND, TRANSFER, FEE
    Status          TransactionStatus   `json:"status"`           // PENDING, POSTED, REVERSED

    // Amount (always in minor units)
    AmountCents     int64               `json:"amount_cents"`
    Currency        Currency            `json:"currency"`         // ISO 4217

    // Counterparty (normalized)
    MerchantName    string              `json:"merchant_name"`         // Normalized name
    MerchantRaw     string              `json:"merchant_name_raw"`     // Original from bank
    MerchantID      *MerchantID         `json:"merchant_id,omitempty"` // If identified
    PayeeID         *PayeeID            `json:"payee_id,omitempty"`    // If registered payee

    // Categorization
    Category        TransactionCategory `json:"category"`         // GROCERIES, UTILITIES, etc.
    CategorySource  CategorySource      `json:"category_source"`  // BANK, RULES, ML

    // Timestamps
    TransactionDate time.Time           `json:"transaction_date"` // When txn occurred
    PostedDate      *time.Time          `json:"posted_date,omitempty"`

    // Metadata
    Reference       string              `json:"reference,omitempty"`
    Notes           string              `json:"notes,omitempty"`
}

// TransactionKind classifies the nature of a transaction (v8.5).
type TransactionKind string
const (
    KindPurchase    TransactionKind = "PURCHASE"
    KindRefund      TransactionKind = "REFUND"
    KindReversal    TransactionKind = "REVERSAL"
    KindChargeback  TransactionKind = "CHARGEBACK"
    KindFee         TransactionKind = "FEE"
    KindTransfer    TransactionKind = "TRANSFER"
    KindInterest    TransactionKind = "INTEREST"
    KindATM         TransactionKind = "ATM"
)

// TransactionCategory for spending analysis.
type TransactionCategory string
const (
    CategoryGroceries      TransactionCategory = "GROCERIES"
    CategoryUtilities      TransactionCategory = "UTILITIES"
    CategoryTransport      TransactionCategory = "TRANSPORT"
    CategoryEntertainment  TransactionCategory = "ENTERTAINMENT"
    CategoryDining         TransactionCategory = "DINING"
    CategoryShopping       TransactionCategory = "SHOPPING"
    CategoryHealthcare     TransactionCategory = "HEALTHCARE"
    CategoryEducation      TransactionCategory = "EDUCATION"
    CategoryTravel         TransactionCategory = "TRAVEL"
    CategorySubscription   TransactionCategory = "SUBSCRIPTION"
    CategoryTransfer       TransactionCategory = "TRANSFER"
    CategoryIncome         TransactionCategory = "INCOME"
    CategoryOther          TransactionCategory = "OTHER"
)
```

#### BalanceEvent

```go
// BalanceEvent represents an account balance snapshot.
type BalanceEvent struct {
    BaseEvent

    AccountID       AccountID   `json:"account_id"`
    AccountType     AccountType `json:"account_type"`

    // Balances (all in minor units)
    CurrentCents    int64       `json:"current_cents"`
    AvailableCents  int64       `json:"available_cents"`
    Currency        Currency    `json:"currency"`

    // Credit-specific (optional)
    CreditLimitCents *int64     `json:"credit_limit_cents,omitempty"`
    PendingCents     *int64     `json:"pending_cents,omitempty"`

    // Timestamp
    AsOf            time.Time   `json:"as_of"`
}
```

### 2.3 Commerce Events

#### OrderEvent

```go
// OrderEvent represents a purchase order from any e-commerce platform.
type OrderEvent struct {
    BaseEvent

    // Order identification
    OrderNumber     string              `json:"order_number"`
    OrderStatus     OrderStatus         `json:"order_status"`

    // Merchant
    MerchantName    string              `json:"merchant_name"`
    MerchantID      *MerchantID         `json:"merchant_id,omitempty"`
    MerchantURL     string              `json:"merchant_url,omitempty"`

    // Items
    Items           []OrderItem         `json:"items"`
    ItemCount       int                 `json:"item_count"`

    // Amounts
    SubtotalCents   int64               `json:"subtotal_cents"`
    ShippingCents   int64               `json:"shipping_cents"`
    TaxCents        int64               `json:"tax_cents"`
    DiscountCents   int64               `json:"discount_cents"`
    TotalCents      int64               `json:"total_cents"`
    Currency        Currency            `json:"currency"`

    // Dates
    OrderDate       time.Time           `json:"order_date"`
    EstimatedDelivery *time.Time        `json:"estimated_delivery,omitempty"`

    // Payment
    PaymentMethod   string              `json:"payment_method,omitempty"`
    PaymentStatus   PaymentStatus       `json:"payment_status"`
}

type OrderItem struct {
    Name            string  `json:"name"`
    Quantity        int     `json:"quantity"`
    UnitPriceCents  int64   `json:"unit_price_cents"`
    SKU             string  `json:"sku,omitempty"`
    URL             string  `json:"url,omitempty"`
}

type OrderStatus string
const (
    OrderPending     OrderStatus = "PENDING"
    OrderConfirmed   OrderStatus = "CONFIRMED"
    OrderProcessing  OrderStatus = "PROCESSING"
    OrderShipped     OrderStatus = "SHIPPED"
    OrderDelivered   OrderStatus = "DELIVERED"
    OrderCancelled   OrderStatus = "CANCELLED"
    OrderReturned    OrderStatus = "RETURNED"
)
```

#### ShipmentEvent

```go
// ShipmentEvent represents a package shipment from any carrier.
type ShipmentEvent struct {
    BaseEvent

    // Shipment identification
    TrackingNumber  string              `json:"tracking_number"`
    Carrier         CarrierID           `json:"carrier"`          // ROYAL_MAIL, DPD, UPS, etc.
    CarrierName     string              `json:"carrier_name"`

    // Related order (if known)
    OrderID         *EventID            `json:"order_event_id,omitempty"`
    OrderNumber     string              `json:"order_number,omitempty"`
    MerchantName    string              `json:"merchant_name,omitempty"`

    // Status
    Status          ShipmentStatus      `json:"status"`
    StatusDetail    string              `json:"status_detail,omitempty"`

    // Locations
    Origin          *Address            `json:"origin,omitempty"`
    Destination     *Address            `json:"destination,omitempty"`
    CurrentLocation *Location           `json:"current_location,omitempty"`

    // Dates
    ShippedDate     *time.Time          `json:"shipped_date,omitempty"`
    EstimatedDelivery *time.Time        `json:"estimated_delivery,omitempty"`
    DeliveredDate   *time.Time          `json:"delivered_date,omitempty"`

    // Tracking history
    TrackingEvents  []TrackingEvent     `json:"tracking_events,omitempty"`
}

type ShipmentStatus string
const (
    ShipmentLabelCreated  ShipmentStatus = "LABEL_CREATED"
    ShipmentPickedUp      ShipmentStatus = "PICKED_UP"
    ShipmentInTransit     ShipmentStatus = "IN_TRANSIT"
    ShipmentOutForDelivery ShipmentStatus = "OUT_FOR_DELIVERY"
    ShipmentDelivered     ShipmentStatus = "DELIVERED"
    ShipmentException     ShipmentStatus = "EXCEPTION"
    ShipmentReturned      ShipmentStatus = "RETURNED"
)

type TrackingEvent struct {
    Timestamp   time.Time `json:"timestamp"`
    Status      string    `json:"status"`
    Location    *Location `json:"location,omitempty"`
    Description string    `json:"description"`
}
```

#### RideEvent

```go
// RideEvent represents a ride/taxi from any provider.
type RideEvent struct {
    BaseEvent

    // Ride identification
    RideID          string              `json:"ride_id"`
    Provider        RideProvider        `json:"provider"`  // UBER, BOLT, LYFT

    // Status
    Status          RideStatus          `json:"status"`

    // Route
    PickupLocation  Location            `json:"pickup_location"`
    DropoffLocation Location            `json:"dropoff_location"`
    DistanceMeters  int                 `json:"distance_meters,omitempty"`
    DurationSeconds int                 `json:"duration_seconds,omitempty"`

    // Timing
    RequestedAt     time.Time           `json:"requested_at"`
    PickupAt        *time.Time          `json:"pickup_at,omitempty"`
    DropoffAt       *time.Time          `json:"dropoff_at,omitempty"`

    // Pricing
    FareCents       int64               `json:"fare_cents"`
    Currency        Currency            `json:"currency"`
    SurgePricing    bool                `json:"surge_pricing"`
    TipCents        int64               `json:"tip_cents,omitempty"`

    // Vehicle/Driver (anonymized)
    VehicleType     string              `json:"vehicle_type,omitempty"`
    DriverRating    *float64            `json:"driver_rating,omitempty"`
}

type RideStatus string
const (
    RideRequested   RideStatus = "REQUESTED"
    RideAccepted    RideStatus = "ACCEPTED"
    RideEnRoute     RideStatus = "EN_ROUTE"
    RideArrived     RideStatus = "ARRIVED"
    RideInProgress  RideStatus = "IN_PROGRESS"
    RideCompleted   RideStatus = "COMPLETED"
    RideCancelled   RideStatus = "CANCELLED"
)
```

### 2.4 Subscription Events

#### SubscriptionEvent

```go
// SubscriptionEvent represents a recurring subscription from any provider.
type SubscriptionEvent struct {
    BaseEvent

    // Subscription identification
    SubscriptionID  string                  `json:"subscription_id"`
    Provider        string                  `json:"provider"`
    ProductName     string                  `json:"product_name"`

    // Status
    Status          SubscriptionStatus      `json:"status"`
    StatusChangedAt time.Time               `json:"status_changed_at"`

    // Billing
    AmountCents     int64                   `json:"amount_cents"`
    Currency        Currency                `json:"currency"`
    BillingPeriod   BillingPeriod           `json:"billing_period"`
    NextBillingDate *time.Time              `json:"next_billing_date,omitempty"`

    // Trial (if applicable)
    TrialEnd        *time.Time              `json:"trial_end,omitempty"`
    IsInTrial       bool                    `json:"is_in_trial"`

    // Cancellation
    CancelledAt     *time.Time              `json:"cancelled_at,omitempty"`
    CancelReason    string                  `json:"cancel_reason,omitempty"`
    ExpiresAt       *time.Time              `json:"expires_at,omitempty"`
}

type SubscriptionStatus string
const (
    SubscriptionActive      SubscriptionStatus = "ACTIVE"
    SubscriptionTrialing    SubscriptionStatus = "TRIALING"
    SubscriptionPastDue     SubscriptionStatus = "PAST_DUE"
    SubscriptionCancelled   SubscriptionStatus = "CANCELLED"
    SubscriptionExpired     SubscriptionStatus = "EXPIRED"
    SubscriptionPaused      SubscriptionStatus = "PAUSED"
)

type BillingPeriod string
const (
    BillingWeekly   BillingPeriod = "WEEKLY"
    BillingMonthly  BillingPeriod = "MONTHLY"
    BillingQuarterly BillingPeriod = "QUARTERLY"
    BillingAnnual   BillingPeriod = "ANNUAL"
)
```

#### InvoiceEvent

```go
// InvoiceEvent represents a bill/invoice from any provider.
type InvoiceEvent struct {
    BaseEvent

    // Invoice identification
    InvoiceNumber   string              `json:"invoice_number"`
    Provider        string              `json:"provider"`
    ProviderID      *MerchantID         `json:"provider_id,omitempty"`

    // Status
    Status          InvoiceStatus       `json:"status"`

    // Amounts
    SubtotalCents   int64               `json:"subtotal_cents"`
    TaxCents        int64               `json:"tax_cents"`
    TotalCents      int64               `json:"total_cents"`
    Currency        Currency            `json:"currency"`
    AmountDueCents  int64               `json:"amount_due_cents"`
    AmountPaidCents int64               `json:"amount_paid_cents"`

    // Dates
    InvoiceDate     time.Time           `json:"invoice_date"`
    DueDate         time.Time           `json:"due_date"`
    PaidDate        *time.Time          `json:"paid_date,omitempty"`

    // Period (for recurring bills)
    PeriodStart     *time.Time          `json:"period_start,omitempty"`
    PeriodEnd       *time.Time          `json:"period_end,omitempty"`

    // Line items
    LineItems       []InvoiceLineItem   `json:"line_items,omitempty"`

    // Payment
    PaymentMethod   string              `json:"payment_method,omitempty"`
    AutoPay         bool                `json:"auto_pay"`
}

type InvoiceStatus string
const (
    InvoiceDraft    InvoiceStatus = "DRAFT"
    InvoiceOpen     InvoiceStatus = "OPEN"
    InvoicePaid     InvoiceStatus = "PAID"
    InvoiceOverdue  InvoiceStatus = "OVERDUE"
    InvoiceVoid     InvoiceStatus = "VOID"
)

type InvoiceLineItem struct {
    Description     string  `json:"description"`
    Quantity        float64 `json:"quantity"`
    UnitPriceCents  int64   `json:"unit_price_cents"`
    TotalCents      int64   `json:"total_cents"`
}
```

### 2.5 Communication Events

#### EmailMessageEvent

```go
// EmailMessageEvent represents an email from any provider.
type EmailMessageEvent struct {
    BaseEvent

    // Message identification
    MessageID       string              `json:"message_id"`
    ThreadID        string              `json:"thread_id,omitempty"`

    // Account
    AccountEmail    string              `json:"account_email"`
    Folder          string              `json:"folder"`           // INBOX, SENT, etc.

    // Participants
    From            EmailAddress        `json:"from"`
    To              []EmailAddress      `json:"to"`
    Cc              []EmailAddress      `json:"cc,omitempty"`
    Bcc             []EmailAddress      `json:"bcc,omitempty"`
    ReplyTo         *EmailAddress       `json:"reply_to,omitempty"`

    // Content
    Subject         string              `json:"subject"`
    BodyPreview     string              `json:"body_preview"`     // First 500 chars
    BodyPlain       string              `json:"body_plain,omitempty"`
    BodyHTML        string              `json:"body_html,omitempty"`
    HasAttachments  bool                `json:"has_attachments"`
    AttachmentCount int                 `json:"attachment_count"`

    // Flags
    IsRead          bool                `json:"is_read"`
    IsStarred       bool                `json:"is_starred"`
    IsImportant     bool                `json:"is_important"`
    Labels          []string            `json:"labels,omitempty"`

    // Dates
    SentAt          time.Time           `json:"sent_at"`
    ReceivedAt      time.Time           `json:"received_at"`

    // Classification hints
    SenderDomain    string              `json:"sender_domain"`
    IsAutomated     bool                `json:"is_automated"`     // Newsletters, notifications
    IsTransactional bool                `json:"is_transactional"` // Receipts, confirmations
}

type EmailAddress struct {
    Address string `json:"address"`
    Name    string `json:"name,omitempty"`
}
```

#### MessageEvent

```go
// MessageEvent represents a message from any messaging platform.
type MessageEvent struct {
    BaseEvent

    // Platform
    Platform        MessagingPlatform   `json:"platform"`     // WHATSAPP, SLACK, IMESSAGE

    // Conversation
    ConversationID  string              `json:"conversation_id"`
    ConversationType ConversationType   `json:"conversation_type"` // DIRECT, GROUP

    // Participants
    SenderID        string              `json:"sender_id"`
    SenderName      string              `json:"sender_name"`
    SenderPhone     string              `json:"sender_phone,omitempty"`
    GroupName       string              `json:"group_name,omitempty"`
    GroupSize       int                 `json:"group_size,omitempty"`

    // Content
    MessageType     MessageType         `json:"message_type"` // TEXT, IMAGE, VOICE, etc.
    TextContent     string              `json:"text_content,omitempty"`
    MediaURL        string              `json:"media_url,omitempty"`
    Caption         string              `json:"caption,omitempty"`

    // Metadata
    IsFromMe        bool                `json:"is_from_me"`
    IsForwarded     bool                `json:"is_forwarded"`
    ReplyToID       string              `json:"reply_to_id,omitempty"`

    // Dates
    SentAt          time.Time           `json:"sent_at"`
    DeliveredAt     *time.Time          `json:"delivered_at,omitempty"`
    ReadAt          *time.Time          `json:"read_at,omitempty"`
}

type MessagingPlatform string
const (
    PlatformWhatsApp  MessagingPlatform = "WHATSAPP"
    PlatformSlack     MessagingPlatform = "SLACK"
    PlatformIMessage  MessagingPlatform = "IMESSAGE"
    PlatformTelegram  MessagingPlatform = "TELEGRAM"
)

type MessageType string
const (
    MsgTypeText     MessageType = "TEXT"
    MsgTypeImage    MessageType = "IMAGE"
    MsgTypeVideo    MessageType = "VIDEO"
    MsgTypeVoice    MessageType = "VOICE"
    MsgTypeDocument MessageType = "DOCUMENT"
    MsgTypeLocation MessageType = "LOCATION"
    MsgTypeContact  MessageType = "CONTACT"
    MsgTypeSticker  MessageType = "STICKER"
)
```

### 2.6 Calendar Events

#### CalendarEventEvent

```go
// CalendarEventEvent represents a calendar event from any provider.
type CalendarEventEvent struct {
    BaseEvent

    // Calendar
    CalendarID      string              `json:"calendar_id"`
    CalendarName    string              `json:"calendar_name"`
    AccountEmail    string              `json:"account_email"`

    // Event identification
    EventUID        string              `json:"event_uid"`        // iCal UID
    RecurrenceID    string              `json:"recurrence_id,omitempty"`

    // Content
    Title           string              `json:"title"`
    Description     string              `json:"description,omitempty"`
    Location        string              `json:"location,omitempty"`
    LocationGeo     *Location           `json:"location_geo,omitempty"`

    // Timing
    StartTime       time.Time           `json:"start_time"`
    EndTime         time.Time           `json:"end_time"`
    IsAllDay        bool                `json:"is_all_day"`
    Timezone        string              `json:"timezone"`

    // Recurrence
    IsRecurring     bool                `json:"is_recurring"`
    RecurrenceRule  string              `json:"recurrence_rule,omitempty"` // RRULE
    RecurrenceEnd   *time.Time          `json:"recurrence_end,omitempty"`

    // Attendees
    Organizer       *CalendarAttendee   `json:"organizer,omitempty"`
    Attendees       []CalendarAttendee  `json:"attendees,omitempty"`
    AttendeeCount   int                 `json:"attendee_count"`

    // Status
    Status          CalendarEventStatus `json:"status"`
    MyResponseStatus RSVPStatus         `json:"my_response_status"`
    IsCancelled     bool                `json:"is_cancelled"`

    // Metadata
    Visibility      Visibility          `json:"visibility"`
    IsBusy          bool                `json:"is_busy"`
    Reminders       []Reminder          `json:"reminders,omitempty"`
    ConferenceURL   string              `json:"conference_url,omitempty"`
    Attachments     []string            `json:"attachments,omitempty"`
}

type CalendarAttendee struct {
    Email           string     `json:"email"`
    Name            string     `json:"name,omitempty"`
    ResponseStatus  RSVPStatus `json:"response_status"`
    IsOptional      bool       `json:"is_optional"`
    IsOrganizer     bool       `json:"is_organizer"`
}

type RSVPStatus string
const (
    RSVPNeedsAction RSVPStatus = "NEEDS_ACTION"
    RSVPAccepted    RSVPStatus = "ACCEPTED"
    RSVPDeclined    RSVPStatus = "DECLINED"
    RSVPTentative   RSVPStatus = "TENTATIVE"
)
```

### 2.7 Health Events

#### ActivityEvent

```go
// ActivityEvent represents activity data from any health platform.
type ActivityEvent struct {
    BaseEvent

    // Period
    Date            time.Time           `json:"date"`
    PeriodStart     time.Time           `json:"period_start"`
    PeriodEnd       time.Time           `json:"period_end"`

    // Steps
    StepCount       int                 `json:"step_count"`
    StepGoal        int                 `json:"step_goal,omitempty"`

    // Distance
    DistanceMeters  int                 `json:"distance_meters"`

    // Calories
    ActiveCalories  int                 `json:"active_calories"`
    TotalCalories   int                 `json:"total_calories,omitempty"`
    CalorieGoal     int                 `json:"calorie_goal,omitempty"`

    // Active time
    ActiveMinutes   int                 `json:"active_minutes"`
    ExerciseMinutes int                 `json:"exercise_minutes,omitempty"`
    StandHours      int                 `json:"stand_hours,omitempty"`

    // Floors
    FloorsClimbed   int                 `json:"floors_climbed,omitempty"`
}
```

#### SleepEvent

```go
// SleepEvent represents sleep data from any health platform.
type SleepEvent struct {
    BaseEvent

    // Period
    Date            time.Time           `json:"date"`           // Date of sleep (when went to bed)
    SleepStart      time.Time           `json:"sleep_start"`
    SleepEnd        time.Time           `json:"sleep_end"`

    // Duration
    TotalMinutes    int                 `json:"total_minutes"`
    InBedMinutes    int                 `json:"in_bed_minutes"`
    AwakeMinutes    int                 `json:"awake_minutes"`

    // Stages (if available)
    DeepMinutes     int                 `json:"deep_minutes,omitempty"`
    LightMinutes    int                 `json:"light_minutes,omitempty"`
    REMMinutes      int                 `json:"rem_minutes,omitempty"`
    AwakeCount      int                 `json:"awake_count,omitempty"`

    // Quality
    SleepScore      int                 `json:"sleep_score,omitempty"`      // 0-100
    Efficiency      int                 `json:"efficiency,omitempty"`       // 0-100

    // Goals
    SleepGoalMinutes int                `json:"sleep_goal_minutes,omitempty"`
}
```

#### WorkoutEvent

```go
// WorkoutEvent represents a workout session from any platform.
type WorkoutEvent struct {
    BaseEvent

    // Workout type
    WorkoutType     WorkoutType         `json:"workout_type"`
    WorkoutName     string              `json:"workout_name,omitempty"`

    // Platform-specific
    PlatformName    string              `json:"platform_name,omitempty"`   // "Peloton", "Concept2"
    ClassName       string              `json:"class_name,omitempty"`
    InstructorName  string              `json:"instructor_name,omitempty"`

    // Timing
    StartTime       time.Time           `json:"start_time"`
    EndTime         time.Time           `json:"end_time"`
    DurationMinutes int                 `json:"duration_minutes"`

    // Effort
    CaloriesBurned  int                 `json:"calories_burned,omitempty"`
    AvgHeartRate    int                 `json:"avg_heart_rate,omitempty"`
    MaxHeartRate    int                 `json:"max_heart_rate,omitempty"`

    // Type-specific metrics
    DistanceMeters  int                 `json:"distance_meters,omitempty"`
    Pace            string              `json:"pace,omitempty"`             // "5:30/km"
    SpeedKPH        float64             `json:"speed_kph,omitempty"`

    // Rowing-specific
    Strokes         int                 `json:"strokes,omitempty"`
    StrokesPerMinute float64            `json:"strokes_per_minute,omitempty"`
    Split500m       string              `json:"split_500m,omitempty"`       // "2:05"

    // Cycling-specific
    OutputWatts     int                 `json:"output_watts,omitempty"`
    AvgCadence      int                 `json:"avg_cadence,omitempty"`
    AvgResistance   int                 `json:"avg_resistance,omitempty"`
}

type WorkoutType string
const (
    WorkoutRunning      WorkoutType = "RUNNING"
    WorkoutCycling      WorkoutType = "CYCLING"
    WorkoutRowing       WorkoutType = "ROWING"
    WorkoutSwimming     WorkoutType = "SWIMMING"
    WorkoutStrength     WorkoutType = "STRENGTH"
    WorkoutYoga         WorkoutType = "YOGA"
    WorkoutHIIT         WorkoutType = "HIIT"
    WorkoutWalking      WorkoutType = "WALKING"
    WorkoutOther        WorkoutType = "OTHER"
)
```

#### VitalsEvent

```go
// VitalsEvent represents health vitals from any platform.
type VitalsEvent struct {
    BaseEvent

    // Measurement type
    VitalType       VitalType           `json:"vital_type"`

    // Timestamp
    MeasuredAt      time.Time           `json:"measured_at"`

    // Heart rate
    HeartRateBPM    int                 `json:"heart_rate_bpm,omitempty"`
    RestingHRBPM    int                 `json:"resting_hr_bpm,omitempty"`

    // HRV
    HRVMS           int                 `json:"hrv_ms,omitempty"`           // Milliseconds

    // Blood pressure
    SystolicMmHg    int                 `json:"systolic_mmhg,omitempty"`
    DiastolicMmHg   int                 `json:"diastolic_mmhg,omitempty"`

    // Blood oxygen
    SpO2Percent     int                 `json:"spo2_percent,omitempty"`

    // Temperature
    TempCelsius     float64             `json:"temp_celsius,omitempty"`

    // Weight
    WeightKg        float64             `json:"weight_kg,omitempty"`
    BodyFatPercent  float64             `json:"body_fat_percent,omitempty"`

    // Source
    MeasurementSource string            `json:"measurement_source,omitempty"` // "automatic", "manual"
}

type VitalType string
const (
    VitalHeartRate      VitalType = "HEART_RATE"
    VitalRestingHR      VitalType = "RESTING_HR"
    VitalHRV            VitalType = "HRV"
    VitalBloodPressure  VitalType = "BLOOD_PRESSURE"
    VitalSpO2           VitalType = "SPO2"
    VitalTemperature    VitalType = "TEMPERATURE"
    VitalWeight         VitalType = "WEIGHT"
)
```

### 2.8 School/Education Events

#### SchoolNotificationEvent

```go
// SchoolNotificationEvent represents a notification from a school portal.
type SchoolNotificationEvent struct {
    BaseEvent

    // School identification
    SchoolName      string              `json:"school_name"`
    SchoolID        string              `json:"school_id,omitempty"`

    // Student
    StudentName     string              `json:"student_name"`
    StudentID       string              `json:"student_id,omitempty"`
    YearGroup       string              `json:"year_group,omitempty"`

    // Notification
    NotificationType SchoolNotificationType `json:"notification_type"`
    Title           string              `json:"title"`
    Body            string              `json:"body"`
    Priority        Priority            `json:"priority"`

    // Action required
    RequiresAction  bool                `json:"requires_action"`
    ActionType      *SchoolActionType   `json:"action_type,omitempty"`
    ActionDeadline  *time.Time          `json:"action_deadline,omitempty"`

    // Attachments
    HasAttachment   bool                `json:"has_attachment"`
    AttachmentURL   string              `json:"attachment_url,omitempty"`
    AttachmentType  string              `json:"attachment_type,omitempty"` // "PDF", "FORM"

    // Related event (if applicable)
    EventDate       *time.Time          `json:"event_date,omitempty"`
    EventName       string              `json:"event_name,omitempty"`
}

type SchoolNotificationType string
const (
    SchoolNotifGeneral      SchoolNotificationType = "GENERAL"
    SchoolNotifEvent        SchoolNotificationType = "EVENT"
    SchoolNotifTrip         SchoolNotificationType = "TRIP"
    SchoolNotifPayment      SchoolNotificationType = "PAYMENT"
    SchoolNotifReport       SchoolNotificationType = "REPORT"
    SchoolNotifAbsence      SchoolNotificationType = "ABSENCE"
    SchoolNotifParentEvening SchoolNotificationType = "PARENT_EVENING"
    SchoolNotifHomework     SchoolNotificationType = "HOMEWORK"
)

type SchoolActionType string
const (
    SchoolActionForm        SchoolActionType = "COMPLETE_FORM"
    SchoolActionPayment     SchoolActionType = "MAKE_PAYMENT"
    SchoolActionConsent     SchoolActionType = "GIVE_CONSENT"
    SchoolActionBooking     SchoolActionType = "MAKE_BOOKING"
    SchoolActionReply       SchoolActionType = "REPLY_REQUIRED"
)
```

---

## 3. Common Types

### 3.1 Location Types

```go
type Location struct {
    Latitude    float64 `json:"latitude"`
    Longitude   float64 `json:"longitude"`
    Accuracy    float64 `json:"accuracy_meters,omitempty"`
    Description string  `json:"description,omitempty"`
}

type Address struct {
    Line1       string `json:"line1"`
    Line2       string `json:"line2,omitempty"`
    City        string `json:"city"`
    County      string `json:"county,omitempty"`
    PostCode    string `json:"post_code"`
    Country     string `json:"country"`         // ISO 3166-1 alpha-2
    CountryName string `json:"country_name,omitempty"`
}
```

### 3.2 Identity Types

```go
type EventID string      // Format: "evt_{ulid}"
type EntityID string     // Format: "ent_{type}_{hash}"
type AccountID string    // Format: "acc_{provider}_{hash}"
type CircleID string     // Format: "circle.subcircle"
type MerchantID string   // Format: "mch_{hash}"
type PayeeID string      // Format: "pay_{hash}"
type ConnectorID string  // Format: "connector-name"
type VendorID string     // Format: "vendor-name"
type CapabilityID string // Format: "category.capability"

type Currency string     // ISO 4217: "GBP", "USD", "INR"
type Region string       // "UK", "US", "IN", "GLOBAL"
```

### 3.3 Common Enums

```go
type Priority string
const (
    PriorityLow      Priority = "LOW"
    PriorityNormal   Priority = "NORMAL"
    PriorityHigh     Priority = "HIGH"
    PriorityUrgent   Priority = "URGENT"
)

type Visibility string
const (
    VisibilityPublic    Visibility = "PUBLIC"
    VisibilityPrivate   Visibility = "PRIVATE"
    VisibilityConfidential Visibility = "CONFIDENTIAL"
)

type ConversationType string
const (
    ConvDirect ConversationType = "DIRECT"
    ConvGroup  ConversationType = "GROUP"
)
```

---

## 4. API Specifications

### 4.1 Internal Event Queue API

Events flow from ingestion to core via a queue interface.

```go
// EventQueue defines the interface for event delivery.
type EventQueue interface {
    // Enqueue adds an event to the queue (called by ingestion workers).
    Enqueue(ctx context.Context, event CanonicalEvent) error

    // Dequeue blocks until an event is available (called by core engine).
    // Returns the event and an acknowledgment function.
    Dequeue(ctx context.Context) (CanonicalEvent, AckFunc, error)

    // Peek returns events without removing them (for inspection).
    Peek(ctx context.Context, limit int) ([]CanonicalEvent, error)

    // Depth returns the current queue depth.
    Depth(ctx context.Context) (int64, error)
}

type AckFunc func(ctx context.Context, success bool) error
```

### 4.2 Proposal API

```go
// ProposalStore manages pending proposals awaiting user approval.
type ProposalStore interface {
    // Create stores a new proposal.
    Create(ctx context.Context, proposal *Proposal) error

    // Get retrieves a proposal by ID.
    Get(ctx context.Context, id ProposalID) (*Proposal, error)

    // ListPending returns all pending proposals for a user.
    ListPending(ctx context.Context, userID UserID) ([]*Proposal, error)

    // ListByCircle returns pending proposals for a specific circle.
    ListByCircle(ctx context.Context, userID UserID, circle CircleID) ([]*Proposal, error)

    // Approve marks a proposal as approved by a user.
    Approve(ctx context.Context, id ProposalID, approverID UserID, signature []byte) error

    // Reject marks a proposal as rejected.
    Reject(ctx context.Context, id ProposalID, approverID UserID, reason string) error

    // MarkExecuted marks a proposal as executed.
    MarkExecuted(ctx context.Context, id ProposalID, result ExecutionResult) error

    // Expire marks stale proposals as expired.
    Expire(ctx context.Context, olderThan time.Time) (int, error)
}

// Proposal represents a proposed action awaiting approval.
type Proposal struct {
    ID                  ProposalID          `json:"proposal_id"`
    UserID              UserID              `json:"user_id"`
    Circle              CircleID            `json:"circle"`
    Intersections       []IntersectionID    `json:"intersections,omitempty"`

    // The action
    ActionType          ActionType          `json:"action_type"`
    ActionPayload       json.RawMessage     `json:"action_payload"`
    ActionSummary       string              `json:"action_summary"`

    // v9+ binding
    PolicySnapshotHash  string              `json:"policy_snapshot_hash"`
    ViewSnapshotHash    string              `json:"view_snapshot_hash"`
    IdempotencyKey      string              `json:"idempotency_key"`

    // Approval state
    RequiredApprovers   []UserID            `json:"required_approvers"`
    ApprovalThreshold   int                 `json:"approval_threshold"`
    Approvals           []Approval          `json:"approvals"`
    Status              ProposalStatus      `json:"status"`

    // Timestamps
    CreatedAt           time.Time           `json:"created_at"`
    ExpiresAt           time.Time           `json:"expires_at"`
    ApprovedAt          *time.Time          `json:"approved_at,omitempty"`
    ExecutedAt          *time.Time          `json:"executed_at,omitempty"`

    // Execution result (if executed)
    ExecutionResult     *ExecutionResult    `json:"execution_result,omitempty"`
}

type ProposalStatus string
const (
    ProposalPending   ProposalStatus = "PENDING"
    ProposalApproved  ProposalStatus = "APPROVED"
    ProposalRejected  ProposalStatus = "REJECTED"
    ProposalExpired   ProposalStatus = "EXPIRED"
    ProposalExecuting ProposalStatus = "EXECUTING"
    ProposalCompleted ProposalStatus = "COMPLETED"
    ProposalFailed    ProposalStatus = "FAILED"
)
```

### 4.3 External API (Mobile/Web)

```yaml
openapi: 3.0.3
info:
  title: QuantumLife API
  version: 1.0.0

paths:
  /v1/circles:
    get:
      summary: List all circles and their current state
      responses:
        200:
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: '#/components/schemas/CircleState'

  /v1/circles/{circleId}:
    get:
      summary: Get circle detail view
      parameters:
        - name: circleId
          in: path
          required: true
          schema:
            type: string
      responses:
        200:
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/CircleDetailView'

  /v1/proposals:
    get:
      summary: List pending proposals
      parameters:
        - name: circle
          in: query
          schema:
            type: string
      responses:
        200:
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: '#/components/schemas/Proposal'

  /v1/proposals/{proposalId}/approve:
    post:
      summary: Approve a proposal
      parameters:
        - name: proposalId
          in: path
          required: true
          schema:
            type: string
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/ApprovalRequest'
      responses:
        200:
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ExecutionResult'

  /v1/proposals/{proposalId}/reject:
    post:
      summary: Reject a proposal
      parameters:
        - name: proposalId
          in: path
          required: true
          schema:
            type: string
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/RejectionRequest'
      responses:
        200:
          description: Proposal rejected

  /v1/digest:
    get:
      summary: Get weekly digest
      parameters:
        - name: week
          in: query
          schema:
            type: string
            format: date
      responses:
        200:
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/WeeklyDigest'

  /v1/audit:
    get:
      summary: Query audit log
      parameters:
        - name: from
          in: query
          schema:
            type: string
            format: date-time
        - name: to
          in: query
          schema:
            type: string
            format: date-time
        - name: circle
          in: query
          schema:
            type: string
        - name: eventType
          in: query
          schema:
            type: string
      responses:
        200:
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: '#/components/schemas/AuditEntry'

components:
  schemas:
    CircleState:
      type: object
      properties:
        circleId:
          type: string
        name:
          type: string
        pendingCount:
          type: integer
        hasUrgent:
          type: boolean
        lastUpdated:
          type: string
          format: date-time

    Proposal:
      type: object
      properties:
        proposalId:
          type: string
        circle:
          type: string
        actionType:
          type: string
        actionSummary:
          type: string
        status:
          type: string
        requiredApprovers:
          type: array
          items:
            type: string
        currentApprovals:
          type: integer
        createdAt:
          type: string
          format: date-time
        expiresAt:
          type: string
          format: date-time

    ApprovalRequest:
      type: object
      properties:
        signature:
          type: string
          format: byte
          description: Cryptographic signature of approval

    ExecutionResult:
      type: object
      properties:
        success:
          type: boolean
        executedAt:
          type: string
          format: date-time
        externalReference:
          type: string
        error:
          type: string
```

---

## 5. Protocol Specifications

### 5.1 Event Serialization

All canonical events are serialized as JSON with the following envelope:

```json
{
  "schema_version": 1,
  "event_type": "transaction",
  "event_id": "evt_01HXYZ...",
  "ingested_at": "2025-01-01T12:00:00Z",
  "payload": {
    // Event-specific fields
  }
}
```

### 5.2 Audit Log Format

```json
{
  "audit_id": "aud_01HXYZ...",
  "timestamp": "2025-01-01T12:00:00.123456Z",
  "user_id": "usr_satish",
  "session_id": "ses_...",
  "event_type": "proposal.approved",
  "circle": "finance.joint",
  "resource_type": "proposal",
  "resource_id": "prop_...",
  "action": "approve",
  "actor": "usr_satish",
  "context": {
    "policy_snapshot_hash": "abc123...",
    "view_snapshot_hash": "def456...",
    "approval_count": 2,
    "approval_threshold": 2
  },
  "outcome": "success",
  "ip_address": "192.168.1.1",
  "user_agent": "QuantumLife/1.0 iOS/17.0"
}
```

---

## 6. Document History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2025-01-01 | Core Team | Initial version |
