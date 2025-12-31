# QuantumLife Interruption Contract v1

## Document Status

| Field | Value |
|-------|-------|
| Version | 1.0 |
| Status | Draft |
| Author | QuantumLife Core Team |
| Date | 2025-01-01 |
| Related | QUANTUMLIFE_END_STATE_V1.md, ARCHITECTURE_LIFE_OS_V1.md |

---

## 1. Purpose

This document defines the **Interruption Contract** — the formal specification of when, why, and how QuantumLife may interrupt the user. An interruption is any system-initiated communication that demands user attention.

**Principle**: Every interruption must be *earned* by preventing future regret. If the system interrupts unnecessarily, it has failed.

---

## 2. Definitions

| Term | Definition |
|------|------------|
| **Interruption** | System-initiated communication to the user |
| **Regret Score** | P(user regrets not seeing this item) ∈ [0.0, 1.0] |
| **Attention Horizon** | Time window within which user can meaningfully act |
| **Interrupt Threshold** | Minimum regret score required to interrupt (per-circle) |
| **Rate Limit** | Maximum interruptions per time period |
| **Suppression** | Decision to not interrupt for a qualifying item |

---

## 3. Interruption Levels

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        INTERRUPTION LEVELS                                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   Level 0: SILENT                                                            │
│   ──────────────────                                                         │
│   - Item logged to audit trail                                              │
│   - Included in weekly digest                                               │
│   - Never shown proactively                                                 │
│   - User can find via search/filter                                         │
│   - Example: Marketing email, low-priority notification                     │
│                                                                              │
│   Level 1: AMBIENT                                                           │
│   ──────────────────                                                         │
│   - Appears in circle summary when user opens circle                        │
│   - No notification, no badge                                               │
│   - Contributes to circle "dot" (has items) indicator                       │
│   - Example: FYI email from colleague, routine transaction                  │
│                                                                              │
│   Level 2: QUEUED                                                            │
│   ──────────────────                                                         │
│   - Appears in "Needs You" list on home screen                              │
│   - Circle shows count badge                                                │
│   - No push notification                                                    │
│   - Example: Email requiring response, upcoming deadline                    │
│                                                                              │
│   Level 3: NOTIFY                                                            │
│   ──────────────────                                                         │
│   - Push notification sent to device                                        │
│   - Appears in "Needs You" with highlight                                   │
│   - Respects Do Not Disturb settings                                        │
│   - Example: Time-sensitive request, payment due today                      │
│                                                                              │
│   Level 4: URGENT                                                            │
│   ──────────────────                                                         │
│   - Breaks through Do Not Disturb                                           │
│   - Critical alert visual treatment                                         │
│   - Reserved for catastrophic events only                                   │
│   - Example: Fraud alert, health emergency, account breach                  │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 4. Interruption Decision Algorithm

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    INTERRUPTION DECISION FLOW                                │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   Input: Item, CirclePolicy, UserContext, Clock                             │
│                                                                              │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │   STEP 1: Calculate Regret Score                                     │   │
│   │                                                                      │   │
│   │   regret_score = f(                                                  │   │
│   │       sender_importance,      // Known sender? Boss? Family?         │   │
│   │       content_urgency,        // Deadline? Action required?          │   │
│   │       historical_pattern,     // User acted on similar items?        │   │
│   │       explicit_markers,       // "URGENT" in subject? Priority flag? │   │
│   │       item_age,               // How long since arrival?             │   │
│   │       deadline_proximity      // Days until any deadline             │   │
│   │   )                                                                  │   │
│   │                                                                      │   │
│   │   Output: regret_score ∈ [0.0, 1.0]                                  │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                            │                                                 │
│                            ▼                                                 │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │   STEP 2: Check Threshold                                            │   │
│   │                                                                      │   │
│   │   IF regret_score < circle.interrupt_threshold:                      │   │
│   │       RETURN Level.SILENT (log: "below_threshold")                   │   │
│   │                                                                      │   │
│   │   Default Thresholds:                                                │   │
│   │   - Work: 0.30 (lower bar, more interrupts OK during work hours)    │   │
│   │   - Family: 0.50 (medium bar)                                       │   │
│   │   - Finance: 0.70 (high bar, only important financial items)        │   │
│   │   - Health: 0.60 (medium-high bar)                                  │   │
│   │   - Kids-School: 0.40 (lower bar, school stuff is often urgent)     │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                            │                                                 │
│                            ▼                                                 │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │   STEP 3: Check Time Relevance                                       │   │
│   │                                                                      │   │
│   │   deadline = item.extracted_deadline OR item.expiry                  │   │
│   │   time_to_deadline = deadline - now                                  │   │
│   │                                                                      │   │
│   │   IF no deadline AND no action_required:                             │   │
│   │       RETURN Level.AMBIENT (log: "no_deadline_no_action")            │   │
│   │                                                                      │   │
│   │   IF time_to_deadline > 7 days:                                      │   │
│   │       RETURN Level.AMBIENT (log: "deadline_far")                     │   │
│   │                                                                      │   │
│   │   IF time_to_deadline > 24 hours:                                    │   │
│   │       RETURN Level.QUEUED (log: "deadline_approaching")              │   │
│   │                                                                      │   │
│   │   IF time_to_deadline <= 24 hours:                                   │   │
│   │       CONTINUE to next step (candidate for NOTIFY)                   │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                            │                                                 │
│                            ▼                                                 │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │   STEP 4: Check Rate Limit                                           │   │
│   │                                                                      │   │
│   │   today_notifies = count(interrupts WHERE level >= NOTIFY            │   │
│   │                          AND date = today)                           │   │
│   │                                                                      │   │
│   │   IF today_notifies >= circle.max_daily_notifies:                    │   │
│   │       RETURN Level.QUEUED (log: "rate_limited")                      │   │
│   │                                                                      │   │
│   │   Default: max_daily_notifies = 5                                    │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                            │                                                 │
│                            ▼                                                 │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │   STEP 5: Check Deduplication                                        │   │
│   │                                                                      │   │
│   │   item_hash = hash(item.source_id, item.content_hash)                │   │
│   │                                                                      │   │
│   │   IF already_interrupted(item_hash, window=24h):                     │   │
│   │       RETURN Level.SILENT (log: "duplicate")                         │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                            │                                                 │
│                            ▼                                                 │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │   STEP 6: Check Circle Schedule                                      │   │
│   │                                                                      │   │
│   │   IF NOT circle.interrupt_schedule.allows(now):                      │   │
│   │       schedule_for = circle.interrupt_schedule.next_window()         │   │
│   │       RETURN Level.QUEUED with delay (log: "outside_schedule")       │   │
│   │                                                                      │   │
│   │   Default Schedules:                                                 │   │
│   │   - Work: Mon-Fri 09:00-18:00 local                                 │   │
│   │   - Family: Always allowed                                          │   │
│   │   - Finance: Mon-Fri 09:00-17:00 local (bank hours)                 │   │
│   │   - Health: Always allowed                                          │   │
│   │   - Kids-School: Mon-Fri 08:00-20:00 local                          │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                            │                                                 │
│                            ▼                                                 │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │   STEP 7: Determine Final Level                                      │   │
│   │                                                                      │   │
│   │   IF regret_score >= 0.95 AND item.is_security_critical:            │   │
│   │       RETURN Level.URGENT (log: "critical_security")                 │   │
│   │                                                                      │   │
│   │   IF regret_score >= 0.80 AND time_to_deadline <= 4 hours:          │   │
│   │       RETURN Level.NOTIFY (log: "high_regret_imminent")              │   │
│   │                                                                      │   │
│   │   IF regret_score >= threshold AND time_to_deadline <= 24 hours:    │   │
│   │       RETURN Level.NOTIFY (log: "deadline_tomorrow")                 │   │
│   │                                                                      │   │
│   │   RETURN Level.QUEUED (log: "default_queued")                        │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 5. Regret Score Calculation

### 5.1 Input Features

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        REGRET SCORE FEATURES                                 │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   SENDER IMPORTANCE (weight: 0.25)                                           │
│   ────────────────────────────────                                           │
│   - 1.0: Spouse, children, parents                                          │
│   - 0.9: Direct manager, key client                                         │
│   - 0.7: Close colleagues, close friends                                    │
│   - 0.5: Known contacts                                                     │
│   - 0.3: Organization (school, bank, utility)                               │
│   - 0.1: Unknown sender                                                     │
│   - 0.0: Known spam/marketing                                               │
│                                                                              │
│   CONTENT URGENCY (weight: 0.30)                                            │
│   ────────────────────────────────                                           │
│   - 1.0: Explicit "URGENT", security alert, fraud                           │
│   - 0.8: Deadline mentioned, action required                                │
│   - 0.6: Question asked, response expected                                  │
│   - 0.4: FYI with relevance                                                 │
│   - 0.2: Informational, no action                                           │
│   - 0.0: Marketing, newsletter                                              │
│                                                                              │
│   DEADLINE PROXIMITY (weight: 0.25)                                          │
│   ─────────────────────────────────                                          │
│   - 1.0: Deadline today or overdue                                          │
│   - 0.8: Deadline tomorrow                                                  │
│   - 0.6: Deadline this week                                                 │
│   - 0.4: Deadline next week                                                 │
│   - 0.2: Deadline this month                                                │
│   - 0.0: No deadline or > 1 month                                           │
│                                                                              │
│   HISTORICAL PATTERN (weight: 0.15)                                          │
│   ─────────────────────────────────                                          │
│   - 1.0: User always acts on similar items                                  │
│   - 0.7: User usually acts on similar items                                 │
│   - 0.5: User sometimes acts on similar items                               │
│   - 0.3: User rarely acts on similar items                                  │
│   - 0.0: User never acts on similar items                                   │
│                                                                              │
│   CIRCLE BOOST (weight: 0.05)                                                │
│   ────────────────────────────                                               │
│   - Finance items during bill cycle: +0.2                                   │
│   - School items during term time: +0.1                                     │
│   - Health items after anomaly detected: +0.2                               │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 5.2 Calculation Formula

```
regret_score = (
    sender_importance * 0.25 +
    content_urgency * 0.30 +
    deadline_proximity * 0.25 +
    historical_pattern * 0.15 +
    circle_boost * 0.05
)

// Clamped to [0.0, 1.0]
regret_score = max(0.0, min(1.0, regret_score))
```

---

## 6. Circle-Specific Policies

### 6.1 Work Circle

```yaml
circle_id: "work"
interrupt_threshold: 0.30
max_daily_notifies: 7
schedule:
  days: [mon, tue, wed, thu, fri]
  start: "09:00"
  end: "18:00"
  timezone: "Europe/London"
urgent_override: true  # Fraud/security can break schedule
```

**Rationale**: Work has lower threshold because missing work items has professional consequences. Limited to work hours to protect personal time.

### 6.2 Family Circle

```yaml
circle_id: "family"
interrupt_threshold: 0.50
max_daily_notifies: 5
schedule:
  days: [mon, tue, wed, thu, fri, sat, sun]
  start: "00:00"
  end: "23:59"
  timezone: "Europe/London"
urgent_override: true
```

**Rationale**: Family can interrupt anytime, but with higher threshold. Only truly important family matters should interrupt.

### 6.3 Finance Circle

```yaml
circle_id: "finance"
interrupt_threshold: 0.70
max_daily_notifies: 3
schedule:
  days: [mon, tue, wed, thu, fri]
  start: "09:00"
  end: "17:00"
  timezone: "Europe/London"
urgent_override: true  # Fraud alerts always
```

**Rationale**: Finance has highest threshold. Most financial items are routine. Only payment deadlines, fraud, and unusual activity warrant interruption.

### 6.4 Health Circle

```yaml
circle_id: "health"
interrupt_threshold: 0.60
max_daily_notifies: 2
schedule:
  days: [mon, tue, wed, thu, fri, sat, sun]
  start: "08:00"
  end: "22:00"
  timezone: "Europe/London"
urgent_override: true  # Health emergencies always
```

**Rationale**: Health insights are rarely urgent. Weekly digest is usually sufficient. Only anomalies or appointment reminders warrant interruption.

### 6.5 Kids-School Circle

```yaml
circle_id: "kids_school"
interrupt_threshold: 0.40
max_daily_notifies: 4
schedule:
  days: [mon, tue, wed, thu, fri]
  start: "08:00"
  end: "20:00"
  timezone: "Europe/London"
urgent_override: false  # School rarely has true emergencies
```

**Rationale**: School items often have tight deadlines. Lower threshold ensures forms and events aren't missed. Weekend interrupts only if deadline is Monday.

---

## 7. Suppression Rules

### 7.1 Automatic Suppression

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                     AUTOMATIC SUPPRESSION RULES                              │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   Rule: DUPLICATE                                                            │
│   ─────────────────                                                          │
│   IF item.content_hash seen within 24 hours:                                │
│       Suppress with reason: "duplicate"                                     │
│                                                                              │
│   Rule: ALREADY_HANDLED                                                      │
│   ─────────────────────                                                      │
│   IF item.thread_id has user reply within 24 hours:                         │
│       Suppress with reason: "already_handled"                               │
│                                                                              │
│   Rule: KNOWN_SPAM                                                           │
│   ────────────────────                                                       │
│   IF item.sender in spam_list OR item.content matches spam_patterns:        │
│       Suppress with reason: "spam"                                          │
│                                                                              │
│   Rule: USER_UNSUBSCRIBED                                                    │
│   ────────────────────────                                                   │
│   IF item.sender in user.unsubscribe_list:                                  │
│       Suppress with reason: "user_unsubscribed"                             │
│                                                                              │
│   Rule: OUTSIDE_CIRCLE_SCOPE                                                 │
│   ────────────────────────────                                               │
│   IF item cannot be assigned to any circle:                                 │
│       Suppress with reason: "no_circle"                                     │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 7.2 User-Initiated Suppression

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                     USER SUPPRESSION ACTIONS                                 │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   Action: "Don't show me this again"                                        │
│   ────────────────────────────────────                                       │
│   - Creates permanent suppression rule for sender OR thread                 │
│   - User chooses: sender vs. this thread only                               │
│   - Logged to audit: user_suppression_created                               │
│                                                                              │
│   Action: "Snooze for X days"                                               │
│   ─────────────────────────────                                              │
│   - Temporarily suppresses item                                             │
│   - Re-evaluates after snooze period                                        │
│   - If still relevant, re-queues                                            │
│                                                                              │
│   Action: "This was not important"                                          │
│   ────────────────────────────────                                           │
│   - Negative feedback signal                                                │
│   - Lowers regret score for similar items                                   │
│   - Does not create permanent rule                                          │
│                                                                              │
│   Action: Adjust threshold                                                   │
│   ────────────────────────────                                               │
│   - User can raise/lower circle threshold                                   │
│   - Affects all future items in that circle                                 │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 8. Audit Requirements

Every interruption decision MUST be logged:

```json
{
  "event_type": "interrupt.evaluated",
  "timestamp": "2025-01-15T09:30:00Z",
  "item_id": "item_email_abc123",
  "circle_id": "work",
  "decision": {
    "level": "QUEUED",
    "reason": "deadline_approaching"
  },
  "scores": {
    "regret_score": 0.65,
    "threshold": 0.30,
    "sender_importance": 0.70,
    "content_urgency": 0.60,
    "deadline_proximity": 0.80,
    "historical_pattern": 0.50
  },
  "checks": {
    "threshold_passed": true,
    "time_relevant": true,
    "rate_limit_ok": true,
    "not_duplicate": true,
    "schedule_allows": true
  },
  "context": {
    "today_notifies": 2,
    "max_daily_notifies": 7,
    "deadline": "2025-01-16T17:00:00Z",
    "time_to_deadline_hours": 31.5
  }
}
```

---

## 9. Failure Modes

### 9.1 False Negatives (Missed Important Items)

**Detection**: User acts on item that was suppressed
**Response**: Log `interrupt.false_negative`, adjust weights
**Mitigation**: Lower threshold for similar items

### 9.2 False Positives (Unnecessary Interrupts)

**Detection**: User dismisses without action, marks "not important"
**Response**: Log `interrupt.false_positive`, adjust weights
**Mitigation**: Raise threshold for similar items

### 9.3 Rate Limit Overflow

**Detection**: High-priority items queued due to rate limit
**Response**: Log `interrupt.rate_limited_high_priority`
**Mitigation**: Review rate limits; consider priority queue

---

## 10. Testing Strategy

### 10.1 Unit Tests

- Regret score calculation with known inputs
- Threshold comparison logic
- Schedule evaluation
- Deduplication logic

### 10.2 Integration Tests

- Full decision flow with mock items
- Rate limiting across multiple items
- Cross-circle interference

### 10.3 Acceptance Tests

- Scenario: Important email arrives outside work hours
- Scenario: Duplicate notification within 24h
- Scenario: Rate limit reached, high-priority item arrives
- Scenario: Deadline crosses into urgent window

---

## 11. Document History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2025-01-01 | Core Team | Initial version |
