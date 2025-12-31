# Satish Circle Taxonomy v1

## Document Status

| Field | Value |
|-------|-------|
| Version | 1.0 |
| Status | Draft |
| Author | QuantumLife Core Team |
| Date | 2025-01-01 |
| Supersedes | None |
| Related | QUANTUMLIFE_END_STATE_V1.md, ARCHITECTURE_LIFE_OS_V1.md, INTERRUPTION_CONTRACT_V1.md |

---

## 1. Overview

This document defines the complete circle taxonomy for Satish's QuantumLife instance. Circles are the primary organizational primitive — each represents a domain of responsibility with its own data sources, policies, and interruption thresholds.

**Design Principle**: Circles are defined by *responsibility*, not by *data source*. A single email can touch multiple circles. A calendar event belongs to the circle whose obligation it represents.

---

## 2. Circle Hierarchy

```
QuantumLife
├── WORK
│   ├── work.employer          # Day job at employer
│   ├── work.quantumlife       # QuantumLife building
│   └── work.consulting        # Any side consulting
├── FAMILY
│   ├── family.immediate       # Wife + kids
│   ├── family.extended.uk     # UK-based family
│   ├── family.extended.india  # India-based family
│   └── family.extended.us     # US-based family
├── FINANCE
│   ├── finance.personal       # Satish's personal accounts
│   ├── finance.joint          # Joint accounts with wife
│   ├── finance.business       # QuantumLife business accounts
│   └── finance.india          # India accounts (remittances)
├── HEALTH
│   ├── health.fitness         # Exercise, workouts
│   ├── health.sleep           # Sleep tracking
│   ├── health.vitals          # HR, HRV, BP
│   └── health.medical         # Appointments, prescriptions
├── HOME
│   ├── home.maintenance       # Repairs, services
│   ├── home.utilities         # Bills, providers
│   └── home.purchases         # Household shopping
└── KIDS
    ├── kids.child1            # First child (Year 10)
    │   ├── kids.child1.school
    │   ├── kids.child1.activities
    │   └── kids.child1.health
    └── kids.child2            # Second child (Year 8)
        ├── kids.child2.school
        ├── kids.child2.activities
        └── kids.child2.health
```

---

## 3. Primary Circles

### 3.1 WORK Circle

**Definition**: All obligations arising from professional employment and business building.

#### Sub-Circles

| Sub-Circle | Description | Data Sources |
|------------|-------------|--------------|
| `work.employer` | Day job responsibilities | Outlook (work), Slack, GitHub (work org), Linear |
| `work.quantumlife` | QuantumLife development | GitHub (ql org), Notion, Clerk, personal Gmail |
| `work.consulting` | Side consulting (if any) | Personal Gmail, dedicated project tools |

#### Policies

```yaml
work:
  interruption:
    default_level: QUEUED
    urgent_threshold: 0.85
    notify_threshold: 0.70

  schedules:
    work.employer:
      active_hours: "09:00-18:00"
      active_days: ["Mon", "Tue", "Wed", "Thu", "Fri"]
      timezone: "Europe/London"
    work.quantumlife:
      active_hours: "06:00-08:00,20:00-23:00"  # Before/after day job
      active_days: ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"]
      timezone: "Europe/London"

  classification_rules:
    - sender_domain: "employer.com" -> work.employer
    - sender_domain: "quantumlife.dev" -> work.quantumlife
    - github_org: "employer-org" -> work.employer
    - github_org: "quantumlife-canon" -> work.quantumlife
    - slack_workspace: "employer" -> work.employer
```

#### Regret Weights

| Signal | Weight | Rationale |
|--------|--------|-----------|
| From: direct manager | +0.3 | Manager requests have professional consequences |
| Contains: "urgent", "asap" | +0.2 | Explicit urgency signal |
| PR blocking release | +0.25 | Team velocity impact |
| Meeting in < 2h, no prep done | +0.35 | Preparation deadline |
| Mentioned in Slack thread | +0.1 | Social expectation to respond |

---

### 3.2 FAMILY Circle

**Definition**: All obligations related to immediate and extended family relationships.

#### Sub-Circles

| Sub-Circle | Description | Data Sources |
|------------|-------------|--------------|
| `family.immediate` | Wife + daily family life | Wife's shared calendar, WhatsApp family group, iMessage |
| `family.extended.uk` | UK-based relatives | WhatsApp, email |
| `family.extended.india` | India-based family | WhatsApp, email |
| `family.extended.us` | US-based family | WhatsApp, email |

#### Policies

```yaml
family:
  interruption:
    default_level: QUEUED
    urgent_threshold: 0.80
    notify_threshold: 0.60

  schedules:
    family.immediate:
      always_active: true  # Family is always relevant
    family.extended.*:
      # Respect timezone differences for non-urgent items
      active_hours: "08:00-22:00"
      timezone: "Europe/London"

  special_rules:
    wife_messages:
      always_notify: false  # Only if regret score high
      priority_boost: +0.2  # But always boost priority
    birthday_reminders:
      lead_time_days: 7
      escalation_days: 3
```

#### Regret Weights

| Signal | Weight | Rationale |
|--------|--------|-----------|
| From: wife | +0.2 | Spouse communication priority |
| Birthday in < 3 days | +0.4 | Gift/card deadline |
| Death/illness mention | +0.5 | Family emergency |
| Travel coordination | +0.3 | Logistics with deadlines |
| Anniversary/special date | +0.35 | Relationship milestone |

---

### 3.3 FINANCE Circle

**Definition**: All obligations related to money management, payments, and financial health.

#### Sub-Circles

| Sub-Circle | Description | Data Sources |
|------------|-------------|--------------|
| `finance.personal` | Satish's personal accounts | Barclays, Monzo (personal) |
| `finance.joint` | Joint household accounts | NatWest joint, Amex joint |
| `finance.business` | QuantumLife business | Stripe, business bank |
| `finance.india` | India accounts | HDFC, ICICI |

#### Policies

```yaml
finance:
  interruption:
    default_level: QUEUED
    urgent_threshold: 0.90  # Higher threshold - finance is stressful
    notify_threshold: 0.75

  execution:
    requires_approval: always  # v9+ canon: no auto-execution

    caps:
      finance.personal:
        single_tx_max: 500_00  # £500 in pence
        daily_max: 1000_00
        approval_mode: SINGLE_PARTY

      finance.joint:
        single_tx_max: 200_00  # £200 requires both parties
        daily_max: 500_00
        approval_mode: MULTI_PARTY
        required_approvers: ["satish", "wife"]

      finance.business:
        single_tx_max: 1000_00
        daily_max: 5000_00
        approval_mode: SINGLE_PARTY  # Satish is sole director

      finance.india:
        single_tx_max: 50000_00  # ₹50,000
        daily_max: 100000_00
        approval_mode: SINGLE_PARTY

  payee_registry:
    mode: CLOSED  # Only registered payees allowed
    auto_register: false  # Manual registration required
```

#### Regret Weights

| Signal | Weight | Rationale |
|--------|--------|-----------|
| Fraud alert from bank | +0.5 | Immediate financial risk |
| Bill due in < 24h | +0.4 | Late payment consequences |
| Unusual transaction | +0.3 | Potential fraud |
| Low balance warning | +0.25 | Overdraft prevention |
| Direct debit failed | +0.35 | Service disruption risk |

---

### 3.4 HEALTH Circle

**Definition**: All obligations related to physical and mental wellbeing.

#### Sub-Circles

| Sub-Circle | Description | Data Sources |
|------------|-------------|--------------|
| `health.fitness` | Exercise tracking | Peloton, Concept2, Apple Watch Activity |
| `health.sleep` | Sleep quality | Apple Watch Sleep, manual logs |
| `health.vitals` | Health metrics | Apple Watch HR/HRV, BP monitor |
| `health.medical` | Healthcare | NHS app, GP portal, pharmacy |

#### Policies

```yaml
health:
  interruption:
    default_level: SILENT  # Health insights don't interrupt
    urgent_threshold: 0.95  # Only medical emergencies
    notify_threshold: 0.85

  digest:
    frequency: WEEKLY
    day: "Sunday"
    time: "09:00"
    include:
      - weekly_averages
      - trend_analysis
      - goal_progress

  alerts:
    # Only these conditions trigger notifications
    emergency_only:
      - resting_hr_spike: "> 30% above baseline"
      - irregular_rhythm_detected: true
      - fall_detected: true

  no_nagging:
    # These are informational only, never interrupt
    - missed_workout
    - sleep_deficit
    - step_count_low
    - calories_over_target
```

#### Regret Weights

| Signal | Weight | Rationale |
|--------|--------|-----------|
| Irregular heart rhythm | +0.5 | Medical emergency |
| GP appointment tomorrow | +0.3 | Preparation needed |
| Prescription running out | +0.35 | Health continuity |
| 5+ day sleep deficit | +0.15 | Worth mentioning in digest |
| Vaccination due | +0.2 | Preventive care |

---

### 3.5 HOME Circle

**Definition**: All obligations related to household management.

#### Sub-Circles

| Sub-Circle | Description | Data Sources |
|------------|-------------|--------------|
| `home.maintenance` | Repairs, tradespeople | Email, calendar |
| `home.utilities` | Bills, service providers | Email, bank statements |
| `home.purchases` | Household shopping | Amazon, email receipts |

#### Policies

```yaml
home:
  interruption:
    default_level: QUEUED
    urgent_threshold: 0.80
    notify_threshold: 0.65

  recurring_obligations:
    - name: "Council Tax"
      frequency: MONTHLY
      day: 1
      account: finance.joint

    - name: "Electricity"
      frequency: MONTHLY
      variable: true
      account: finance.joint

    - name: "Gas"
      frequency: MONTHLY
      variable: true
      account: finance.joint

    - name: "Water"
      frequency: MONTHLY
      account: finance.joint

    - name: "Broadband"
      frequency: MONTHLY
      account: finance.joint

    - name: "TV License"
      frequency: ANNUAL
      account: finance.joint

  tradespeople:
    # Registered service providers
    - name: "Cleaner"
      frequency: WEEKLY
      day: "Thursday"
      payment: finance.joint
      amount: 80_00
```

#### Regret Weights

| Signal | Weight | Rationale |
|--------|--------|-----------|
| Utility bill overdue | +0.4 | Service disconnection risk |
| Tradesperson arriving today | +0.3 | Coordination needed |
| Boiler service due | +0.25 | Seasonal urgency |
| Package delivery today | +0.15 | Low priority awareness |
| Insurance renewal due | +0.35 | Coverage continuity |

---

### 3.6 KIDS Circle

**Definition**: All obligations related to children's education, activities, and wellbeing.

#### Sub-Circles

| Sub-Circle | Description | Data Sources |
|------------|-------------|--------------|
| `kids.child1` | First child (Year 10) | School portal, email |
| `kids.child1.school` | School-related | School portal, parent emails |
| `kids.child1.activities` | Extracurriculars | Activity provider emails/calendars |
| `kids.child1.health` | Health matters | NHS, GP |
| `kids.child2` | Second child (Year 8) | School portal, email |
| `kids.child2.school` | School-related | School portal, parent emails |
| `kids.child2.activities` | Extracurriculars | Activity provider emails/calendars |
| `kids.child2.health` | Health matters | NHS, GP |

#### Policies

```yaml
kids:
  interruption:
    default_level: QUEUED
    urgent_threshold: 0.75  # Lower threshold - kids obligations are critical
    notify_threshold: 0.55

  school_deadlines:
    # Forms and payments require lead time
    lead_time_days: 3
    escalation_threshold: 24h

  term_dates:
    # Loaded from school portal
    sync_frequency: WEEKLY

  parent_evenings:
    priority: HIGH
    lead_time_days: 7
    booking_reminder: true

  trips_and_events:
    permission_forms:
      extract: true
      deadline_tracking: true
      auto_fill: true  # Pre-fill with stored info
      requires_approval: true  # Never auto-submit
```

#### Regret Weights

| Signal | Weight | Rationale |
|--------|--------|-----------|
| School form due tomorrow | +0.5 | Child exclusion risk |
| Parent evening booking open | +0.35 | Time slots fill fast |
| School fee payment due | +0.4 | Financial obligation |
| Trip permission outstanding | +0.45 | Child participation |
| Report card available | +0.15 | Informational, not urgent |
| School closure notice | +0.3 | Childcare coordination |

---

## 4. Intersections

Intersections are policy overlays where two or more circles require coordinated decision-making.

### 4.1 Defined Intersections

| Intersection ID | Circles | Description |
|-----------------|---------|-------------|
| `intersection.joint_finance` | FAMILY + FINANCE | Joint account decisions |
| `intersection.kids_school` | KIDS + FAMILY | School matters involving wife |
| `intersection.work_family` | WORK + FAMILY | Calendar conflicts |
| `intersection.health_family` | HEALTH + FAMILY | Medical decisions |

### 4.2 Intersection Policies

```yaml
intersections:
  joint_finance:
    circles: ["family.immediate", "finance.joint"]
    approval_mode: MULTI_PARTY
    required_approvers:
      - satish
      - wife
    notification_policy:
      # Both parties see all transactions
      notify_both: true
      # Either can initiate, both must approve
      initiation: ANY
      approval: ALL

  kids_school:
    circles: ["kids.*", "family.immediate"]
    approval_mode: EITHER_PARTY
    required_approvers:
      - satish
      - wife
    notification_policy:
      # Whoever sees it first can handle it
      notify_both: true
      initiation: ANY
      approval: ANY  # Either parent can approve school forms

  work_family:
    circles: ["work.*", "family.immediate"]
    conflict_detection: true
    resolution_mode: PROPOSE_ALTERNATIVES
    notification_policy:
      # Calendar conflicts shown to Satish only
      notify_both: false
      requires_review: true

  health_family:
    circles: ["health.medical", "family.immediate"]
    approval_mode: INFORM_PARTNER
    notification_policy:
      # Major medical decisions inform wife
      inform_threshold: NOTIFY  # Appointments, procedures
      emergency_escalate: true
```

---

## 5. Data Source Mappings

### 5.1 Email Routing

```yaml
email_routing:
  # Work emails
  - match:
      to: "satish@employer.com"
    route_to: work.employer

  - match:
      to: "satish@quantumlife.dev"
    route_to: work.quantumlife

  # Personal emails - require classification
  - match:
      to: "satish.personal@gmail.com"
    classification:
      - sender_domain: "school1.sch.uk" -> kids.child1.school
      - sender_domain: "school2.sch.uk" -> kids.child2.school
      - sender_domain: "*.bank.co.uk" -> finance.*  # Sub-classify by account
      - sender_domain: "nhs.uk" -> health.medical
      - sender_contains: "cleaner" -> home.maintenance
      - sender_contains: "utility" -> home.utilities
      - default: INBOX_REVIEW  # Manual classification needed
```

### 5.2 Calendar Routing

```yaml
calendar_routing:
  - calendar: "Work (Outlook)"
    route_to: work.employer

  - calendar: "Personal (Google)"
    classification:
      - title_contains: "Parent" -> kids.*.school
      - title_contains: "School" -> kids.*.school
      - title_contains: "GP", "Doctor" -> health.medical
      - organizer_domain: "employer.com" -> work.employer
      - default: family.immediate

  - calendar: "Wife Shared"
    route_to: family.immediate
    intersection: work_family  # For conflict detection

  - calendar: "Child1 School"
    route_to: kids.child1.school

  - calendar: "Child2 School"
    route_to: kids.child2.school
```

### 5.3 Messaging Routing

```yaml
messaging_routing:
  whatsapp:
    - chat_name: "Family Group"
      route_to: family.immediate

    - chat_name: "India Family"
      route_to: family.extended.india

    - chat_name: "School Parents Y10"
      route_to: kids.child1.school

    - chat_name: "School Parents Y8"
      route_to: kids.child2.school

    - participant: "Wife"
      route_to: family.immediate
      intersection: joint_finance  # Money discussions

  slack:
    - workspace: "employer"
      route_to: work.employer

  imessage:
    - contact_group: "Family"
      route_to: family.immediate
```

### 5.4 Financial Routing

```yaml
financial_routing:
  accounts:
    - account_id: "barclays_personal"
      route_to: finance.personal

    - account_id: "monzo_personal"
      route_to: finance.personal

    - account_id: "natwest_joint"
      route_to: finance.joint
      intersection: joint_finance

    - account_id: "amex_joint"
      route_to: finance.joint
      intersection: joint_finance

    - account_id: "stripe_business"
      route_to: finance.business

    - account_id: "hdfc_india"
      route_to: finance.india

    - account_id: "icici_india"
      route_to: finance.india
```

---

## 6. View Definitions

Each circle has a defined View that determines what the user sees when they open that circle.

### 6.1 Work View

```yaml
work_view:
  primary_display:
    - pending_actions:
        - emails_requiring_response
        - prs_awaiting_review
        - meetings_requiring_prep
    - upcoming:
        - next_meeting: time, attendees, prep_status
        - deadlines_this_week

  secondary_display:
    - processed_today:
        - emails_archived: count
        - emails_delegated: count
        - prs_merged: count
    - calendar_week_view

  digest_items:
    - weekly_email_volume
    - meeting_load_trend
    - focus_time_available
```

### 6.2 Family View

```yaml
family_view:
  primary_display:
    - pending_actions:
        - messages_requiring_response
        - upcoming_birthdays
        - unresolved_logistics
    - shared_calendar_today

  secondary_display:
    - family_whereabouts:  # If location sharing enabled
        - wife: "Work" / "Home" / "Traveling"
    - upcoming_events_week

  digest_items:
    - birthdays_next_30_days
    - travel_coordination_needed
    - family_communication_summary
```

### 6.3 Finance View

```yaml
finance_view:
  primary_display:
    - pending_actions:
        - bills_due_soon
        - payments_awaiting_approval
        - unusual_transactions
    - account_summaries:
        - for_each: [personal, joint, business, india]
        - show: current_balance, pending_transactions

  secondary_display:
    - recent_transactions: last_10
    - spending_this_month: by_category

  digest_items:
    - monthly_spending_trend
    - savings_progress
    - upcoming_recurring_payments

  # v9.13 View Freshness
  freshness:
    max_age_ms: 300000  # 5 minutes
    stale_action: REFRESH_REQUIRED
```

### 6.4 Health View

```yaml
health_view:
  primary_display:
    - pending_actions:
        - appointments_upcoming
        - prescriptions_due
    - today_summary:
        - steps: count, goal_progress
        - active_calories
        - sleep_last_night

  secondary_display:
    - week_trends:
        - sleep_average
        - resting_hr_trend
        - workout_count

  digest_items:
    - weekly_activity_summary
    - health_trends_monthly
    - goal_achievement_rate
```

### 6.5 Home View

```yaml
home_view:
  primary_display:
    - pending_actions:
        - bills_due
        - deliveries_expected
        - maintenance_scheduled
    - utilities_status:
        - for_each: [electric, gas, water, broadband]
        - show: last_bill, next_due

  secondary_display:
    - recurring_services:
        - cleaner: next_visit, payment_status
    - recent_home_spend

  digest_items:
    - monthly_home_costs
    - upcoming_renewals
    - maintenance_due
```

### 6.6 Kids View

```yaml
kids_view:
  primary_display:
    - pending_actions:
        - forms_outstanding
        - payments_due
        - events_requiring_response
    - this_week:
        - school_events
        - activities
        - important_dates

  secondary_display:
    - by_child:
        - child1:
            - school_calendar_week
            - recent_communications
        - child2:
            - school_calendar_week
            - recent_communications

  digest_items:
    - term_overview
    - upcoming_parent_evenings
    - fee_payment_schedule
```

---

## 7. Privacy Boundaries

### 7.1 Data Isolation

```yaml
privacy:
  # Wife can see
  wife_visible:
    - finance.joint  # Full visibility
    - kids.*  # Full visibility
    - family.immediate  # Full visibility
    - work.*  # Calendar only (for conflict detection)
    - health.*  # None (unless emergency)

  # Wife cannot see
  wife_hidden:
    - finance.personal  # Satish's personal accounts
    - finance.business  # Business accounts
    - work.*.content  # Email/Slack content
    - health.*.details  # Health data details

  # No external sharing
  never_shared:
    - health.*  # Medical data never leaves device
    - finance.*.transactions  # Transaction details
    - messaging.*.content  # Message content
```

### 7.2 Device Boundaries

```yaml
device_policy:
  # What stays on device
  local_only:
    - health_data_raw
    - message_content
    - email_bodies
    - financial_transaction_details

  # What syncs to cloud (encrypted)
    synced_encrypted:
    - circle_membership
    - obligation_metadata
    - interruption_history
    - approval_audit_log

  # What's processed server-side
  server_processed:
    - email_classification
    - calendar_conflict_detection
    - regret_score_calculation
```

---

## 8. Evolution Notes

### 8.1 Future Circle Additions

The taxonomy is designed to be extensible. Potential future circles:

| Circle | Trigger | Data Sources |
|--------|---------|--------------|
| TRAVEL | Satish travels frequently | Flight emails, hotel bookings, passport validity |
| SOCIAL | Social obligations increase | Event invitations, RSVPs |
| LEARNING | Structured learning goals | Course platforms, reading lists |
| GIVING | Charitable donations | Charity emails, tax records |

### 8.2 Circle Retirement

Circles may be retired when:
- Children leave home → KIDS circle simplified
- Career change → WORK sub-circles reorganized
- Location change → HOME circle reconfigured

### 8.3 Configuration vs. Code

This taxonomy is **documentation only**. The actual implementation will:
1. Load taxonomy from configuration (YAML/JSON)
2. Validate against schema
3. Allow runtime modification (with audit)
4. Never hardcode circle definitions in Go

---

## 9. Glossary

| Term | Definition |
|------|------------|
| Circle | Primary organizational unit representing a domain of responsibility |
| Sub-Circle | Specialized subdivision within a circle |
| Intersection | Policy overlay where multiple circles require coordination |
| View | What the user sees when opening a circle |
| Routing | Rules for assigning incoming data to circles |
| Regret Weight | Multiplier applied to signals when calculating regret scores |

---

## 10. Document History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2025-01-01 | Core Team | Initial version |
