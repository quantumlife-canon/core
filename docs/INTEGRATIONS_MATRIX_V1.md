# QuantumLife Integrations Matrix v1

## Document Status

| Field | Value |
|-------|-------|
| Version | 1.0 |
| Status | Draft |
| Author | QuantumLife Core Team |
| Date | 2025-01-01 |
| Related | ARCHITECTURE_LIFE_OS_V1.md, SATISH_CIRCLES_TAXONOMY_V1.md |

---

## 1. Overview

This document catalogs all external integrations for QuantumLife, specifying:
- Read/Write feasibility
- Authentication method
- Data model
- Rate limits
- Policy enforcement points (v9+ canon alignment)

**Principle**: Every integration must respect v9.7 (no background execution in core). Sync operations are triggered externally or by user action.

---

## 2. Integration Summary

| Integration | Read | Write | Auth | Circle | Priority |
|-------------|------|-------|------|--------|----------|
| Gmail | Yes | Yes | OAuth 2.0 | Work, Personal | P0 |
| Outlook | Yes | Yes | OAuth 2.0 | Work | P1 |
| Google Calendar | Yes | Yes | OAuth 2.0 | Work, Family | P0 |
| Apple Calendar | Yes | Yes | OAuth 2.0 | Family | P1 |
| Plaid | Yes | No | OAuth 2.0 | Finance | P0 |
| TrueLayer | No | Yes | OAuth 2.0 | Finance | P0 (exists) |
| WhatsApp Business | Yes | Yes | API Key | Family, Social | P2 |
| Apple Health | Yes | No | HealthKit | Health | P1 |
| GitHub | Yes | Yes | OAuth 2.0 | Work | P1 |
| School Portals | Yes | Yes | Scrape/API | Kids-School | P2 |
| Stripe | Yes | No | API Key | Work | P2 |

---

## 3. Email: Gmail

### 3.1 Overview

| Field | Value |
|-------|-------|
| Provider | Google |
| API | Gmail API v1 |
| Auth | OAuth 2.0 |
| Circles | Work (work account), Personal (personal accounts) |

### 3.2 Read Capabilities

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         GMAIL READ CAPABILITIES                              │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   Endpoint                     Data Retrieved                               │
│   ────────                     ──────────────                               │
│   messages.list                Message IDs, thread IDs, labels              │
│   messages.get                 Full message content, headers, attachments   │
│   threads.list                 Thread metadata                              │
│   threads.get                  Full thread with all messages                │
│   labels.list                  User labels for classification              │
│   users.getProfile             Email address, history ID                    │
│                                                                              │
│   Sync Strategy:                                                             │
│   - Initial: Last 30 days of messages                                       │
│   - Incremental: Use history ID for delta sync                              │
│   - Trigger: External scheduler OR user pull-to-refresh                     │
│                                                                              │
│   Data Model:                                                                │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │   EmailItem {                                                        │   │
│   │       item_id: "email_" + message_id                                │   │
│   │       source: "gmail"                                               │   │
│   │       source_account: account_id                                    │   │
│   │       thread_id: string                                             │   │
│   │       from: PersonRef (resolved via identity graph)                 │   │
│   │       to: []PersonRef                                               │   │
│   │       cc: []PersonRef                                               │   │
│   │       subject: string                                               │   │
│   │       body_preview: string (first 500 chars)                        │   │
│   │       body_full: string (encrypted at rest)                         │   │
│   │       labels: []string                                              │   │
│   │       received_at: timestamp                                        │   │
│   │       has_attachments: bool                                         │   │
│   │       is_read: bool                                                 │   │
│   │       is_starred: bool                                              │   │
│   │   }                                                                 │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 3.3 Write Capabilities

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         GMAIL WRITE CAPABILITIES                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   Action                       Endpoint                  v9+ Enforcement    │
│   ──────                       ────────                  ────────────────   │
│   Send email                   messages.send             Full envelope      │
│   Reply to thread              messages.send (threadId)  Full envelope      │
│   Archive message              messages.modify           Draft approval     │
│   Add label                    messages.modify           Draft approval     │
│   Mark as read                 messages.modify           Auto (no approval) │
│                                                                              │
│   ExecutionEnvelope for Email Send:                                         │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │   EmailExecutionEnvelope {                                           │   │
│   │       envelope_id: string                                           │   │
│   │       action_class: "email_send"                                    │   │
│   │       circle_id: string                                             │   │
│   │       policy_snapshot_hash: string (v9.12)                          │   │
│   │       view_snapshot_hash: string (v9.13)                            │   │
│   │       action_spec: {                                                │   │
│   │           account_id: string                                        │   │
│   │           thread_id: string (for reply)                             │   │
│   │           to: []string                                              │   │
│   │           cc: []string                                              │   │
│   │           subject: string                                           │   │
│   │           body: string                                              │   │
│   │           attachments: []AttachmentRef                              │   │
│   │       }                                                             │   │
│   │       approval_threshold: 1 (single-party for personal)             │   │
│   │       approvals: []ApprovalArtifact                                 │   │
│   │   }                                                                 │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 3.4 Rate Limits

| Quota | Limit | Handling |
|-------|-------|----------|
| Queries per day | 1,000,000,000 | N/A (won't hit) |
| Queries per 100 seconds | 250 | Exponential backoff |
| Messages sent per day | 500 | Track in caps (v9.11 pattern) |

### 3.5 Policy Enforcement Points

| Check | v9 Reference | Implementation |
|-------|--------------|----------------|
| Provider allowlist | v9.9 | `gmail` in allowed providers |
| Recipient allowlist | v9.10 pattern | Known contacts only (configurable) |
| Daily send cap | v9.11 | Max 20 sends/day default |
| Policy snapshot | v9.12 | Hash includes account + caps |
| View snapshot | v9.13 | Hash includes inbox state |

---

## 4. Email: Outlook

### 4.1 Overview

| Field | Value |
|-------|-------|
| Provider | Microsoft |
| API | Microsoft Graph API |
| Auth | OAuth 2.0 (Azure AD) |
| Circles | Work |

### 4.2 Read/Write Capabilities

Similar to Gmail with Microsoft Graph endpoints:
- `GET /me/messages` for read
- `POST /me/sendMail` for write

### 4.3 Rate Limits

| Quota | Limit |
|-------|-------|
| Requests per 10 minutes | 10,000 |
| Messages sent per day | 10,000 (Exchange Online) |

---

## 5. Calendar: Google Calendar

### 5.1 Overview

| Field | Value |
|-------|-------|
| Provider | Google |
| API | Google Calendar API v3 |
| Auth | OAuth 2.0 |
| Circles | Work, Family |

### 5.2 Read Capabilities

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    GOOGLE CALENDAR READ CAPABILITIES                         │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   Endpoint                     Data Retrieved                               │
│   ────────                     ──────────────                               │
│   calendarList.list            All calendars user has access to             │
│   events.list                  Events within date range                     │
│   events.get                   Single event details                         │
│                                                                              │
│   Sync Strategy:                                                             │
│   - Initial: Events from -7 days to +90 days                                │
│   - Incremental: Use syncToken for delta sync                               │
│   - Trigger: External scheduler OR user pull-to-refresh                     │
│                                                                              │
│   Data Model:                                                                │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │   CalendarEventItem {                                                │   │
│   │       item_id: "cal_" + event_id                                    │   │
│   │       source: "google_calendar"                                     │   │
│   │       calendar_id: string                                           │   │
│   │       title: string                                                 │   │
│   │       description: string                                           │   │
│   │       start_time: timestamp                                         │   │
│   │       end_time: timestamp                                           │   │
│   │       all_day: bool                                                 │   │
│   │       location: string                                              │   │
│   │       organizer: PersonRef                                          │   │
│   │       attendees: []AttendeeRef                                      │   │
│   │       response_status: enum (needs_action, accepted, declined,      │   │
│   │                              tentative)                             │   │
│   │       is_recurring: bool                                            │   │
│   │       recurrence_rule: string                                       │   │
│   │       conference_link: string                                       │   │
│   │   }                                                                 │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 5.3 Write Capabilities

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    GOOGLE CALENDAR WRITE CAPABILITIES                        │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   Action                       Endpoint                  v9+ Enforcement    │
│   ──────                       ────────                  ────────────────   │
│   Accept invite                events.patch (status)     Full envelope      │
│   Decline invite               events.patch (status)     Full envelope      │
│   Propose new time             events.patch + comment    Full envelope      │
│   Create event                 events.insert             Full envelope      │
│   Update event                 events.patch              Full envelope      │
│   Delete event                 events.delete             Full envelope      │
│                                                                              │
│   ExecutionEnvelope for Calendar Response:                                  │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │   CalendarExecutionEnvelope {                                        │   │
│   │       envelope_id: string                                           │   │
│   │       action_class: "calendar_response"                             │   │
│   │       circle_id: string                                             │   │
│   │       intersection_id: string (if family calendar)                  │   │
│   │       policy_snapshot_hash: string (v9.12)                          │   │
│   │       view_snapshot_hash: string (v9.13)                            │   │
│   │       action_spec: {                                                │   │
│   │           event_id: string                                          │   │
│   │           calendar_id: string                                       │   │
│   │           response_type: enum (accept, decline, tentative)          │   │
│   │           message: string (optional)                                │   │
│   │           propose_times: []TimeSlot (for counter-proposal)          │   │
│   │       }                                                             │   │
│   │       approval_threshold: 1 (or 2 for family intersection)          │   │
│   │       approvals: []ApprovalArtifact                                 │   │
│   │   }                                                                 │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│   Multi-Party Approval (Family Calendar):                                   │
│   - Events on shared family calendar require spouse approval               │
│   - Conflict detection across all family members' calendars                │
│   - ViewSnapshot includes all relevant calendars                           │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 5.4 Rate Limits

| Quota | Limit |
|-------|-------|
| Queries per day | 1,000,000 |
| Queries per 100 seconds per user | 500 |

### 5.5 Policy Enforcement Points

| Check | v9 Reference | Implementation |
|-------|--------------|----------------|
| Calendar allowlist | v9.9 pattern | Only synced calendars |
| Conflict detection | v9.13 | View includes other calendars |
| Daily action cap | v9.11 | Max 20 calendar actions/day |
| Multi-party | v9.4 | Family calendar intersection |

---

## 6. Finance Read: Plaid

### 6.1 Overview

| Field | Value |
|-------|-------|
| Provider | Plaid |
| API | Plaid API |
| Auth | OAuth 2.0 (Link) |
| Circles | Finance |

### 6.2 Read Capabilities

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         PLAID READ CAPABILITIES                              │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   Endpoint                     Data Retrieved                               │
│   ────────                     ──────────────                               │
│   /accounts/get                Account list, balances, types                │
│   /transactions/get            Transaction history                          │
│   /transactions/sync           Incremental transaction updates              │
│   /identity/get                Account holder identity                      │
│   /institutions/get_by_id      Institution details                          │
│                                                                              │
│   Sync Strategy:                                                             │
│   - Initial: Last 90 days of transactions                                   │
│   - Incremental: Use /transactions/sync with cursor                         │
│   - Trigger: External scheduler (daily) OR user pull-to-refresh             │
│                                                                              │
│   Data Model:                                                                │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │   TransactionItem {                                                  │   │
│   │       item_id: "txn_" + transaction_id                              │   │
│   │       source: "plaid"                                               │   │
│   │       account_id: AccountRef                                        │   │
│   │       amount_cents: int64 (positive = debit, negative = credit)     │   │
│   │       currency: string (ISO 4217)                                   │   │
│   │       merchant_name: string                                         │   │
│   │       merchant_entity: OrgRef (resolved via identity graph)         │   │
│   │       category: []string (Plaid categories)                         │   │
│   │       date: date                                                    │   │
│   │       pending: bool                                                 │   │
│   │       payment_channel: enum (online, in_store, other)               │   │
│   │   }                                                                 │   │
│   │                                                                      │   │
│   │   AccountSnapshot {                                                  │   │
│   │       account_id: string                                            │   │
│   │       institution: OrgRef                                           │   │
│   │       name: string                                                  │   │
│   │       type: enum (depository, credit, loan, investment)             │   │
│   │       subtype: string                                               │   │
│   │       balance_available: int64                                      │   │
│   │       balance_current: int64                                        │   │
│   │       balance_limit: int64 (for credit)                             │   │
│   │       currency: string                                              │   │
│   │       snapshot_at: timestamp                                        │   │
│   │   }                                                                 │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│   CRITICAL: Plaid is READ-ONLY. No write operations.                        │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 6.3 Write Capabilities

**NONE**. Plaid is strictly read-only for transaction/balance data.

### 6.4 Rate Limits

| Quota | Limit |
|-------|-------|
| /transactions/get | 100 requests/minute per item |
| /accounts/balance/get | 30 requests/minute per item |

---

## 7. Finance Write: TrueLayer

### 7.1 Overview

| Field | Value |
|-------|-------|
| Provider | TrueLayer |
| API | TrueLayer Payments API |
| Auth | OAuth 2.0 |
| Circles | Finance |

### 7.2 Capabilities

**This integration already exists as the v9+ canon (v9.3-v9.13).**

See existing implementation in `internal/finance/execution/`.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    TRUELAYER (EXISTING v9+ CANON)                            │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   Read: None (use Plaid for read)                                           │
│                                                                              │
│   Write: Payment initiation                                                 │
│   - Prepare payment                                                         │
│   - Execute payment                                                         │
│   - Check payment status                                                    │
│                                                                              │
│   v9+ Enforcement:                                                           │
│   - v9.3: Single-party execution with caps                                  │
│   - v9.4: Multi-party approval for joint accounts                           │
│   - v9.5: Presentation gate, revocation                                     │
│   - v9.6: Idempotency, replay defense                                       │
│   - v9.7: No background execution                                           │
│   - v9.8: No auto-retry, single trace finalization                          │
│   - v9.9: Provider registry (truelayer-sandbox allowed)                     │
│   - v9.10: Payee registry (no free-text recipients)                         │
│   - v9.11: Daily caps and rate limits                                       │
│   - v9.12: Policy snapshot hash binding                                     │
│   - v9.13: View freshness binding                                           │
│                                                                              │
│   This is the reference implementation for all other write integrations.    │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 8. Messaging: WhatsApp Business

### 8.1 Overview

| Field | Value |
|-------|-------|
| Provider | Meta |
| API | WhatsApp Business Cloud API |
| Auth | API Key (System User Token) |
| Circles | Family, Social |

### 8.2 Read Capabilities

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    WHATSAPP READ CAPABILITIES                                │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   Method: Webhook (incoming messages)                                       │
│                                                                              │
│   Data Retrieved:                                                           │
│   - Incoming text messages                                                  │
│   - Incoming media (images, documents)                                      │
│   - Message status updates (delivered, read)                                │
│   - Contact information                                                     │
│                                                                              │
│   Limitations:                                                               │
│   - Cannot read historical messages (only new incoming)                     │
│   - Requires WhatsApp Business account                                      │
│   - User must initiate conversation first (24h window)                      │
│                                                                              │
│   Data Model:                                                                │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │   MessageItem {                                                      │   │
│   │       item_id: "wa_" + message_id                                   │   │
│   │       source: "whatsapp"                                            │   │
│   │       from: PersonRef (phone number resolved)                       │   │
│   │       to: PersonRef (self)                                          │   │
│   │       body: string                                                  │   │
│   │       media_type: enum (text, image, document, audio, video)        │   │
│   │       media_url: string (if applicable)                             │   │
│   │       received_at: timestamp                                        │   │
│   │       thread_id: string (conversation with contact)                 │   │
│   │   }                                                                 │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 8.3 Write Capabilities

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    WHATSAPP WRITE CAPABILITIES                               │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   Action                       Constraint                v9+ Enforcement    │
│   ──────                       ──────────                ────────────────   │
│   Send template message        Pre-approved templates    Full envelope      │
│   Reply to message             Within 24h window         Full envelope      │
│   Send media                   Image/document/audio      Full envelope      │
│                                                                              │
│   Limitations:                                                               │
│   - Can only message users who messaged first (24h window)                  │
│   - Outside 24h: Only pre-approved template messages                        │
│   - No bulk messaging                                                       │
│                                                                              │
│   ExecutionEnvelope for Message Send:                                       │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │   MessageExecutionEnvelope {                                         │   │
│   │       envelope_id: string                                           │   │
│   │       action_class: "message_send"                                  │   │
│   │       circle_id: string                                             │   │
│   │       policy_snapshot_hash: string (v9.12)                          │   │
│   │       view_snapshot_hash: string (v9.13)                            │   │
│   │       action_spec: {                                                │   │
│   │           platform: "whatsapp"                                      │   │
│   │           to_phone: string (E.164)                                  │   │
│   │           to_person: PersonRef                                      │   │
│   │           message_type: enum (text, template, media)                │   │
│   │           body: string                                              │   │
│   │           template_name: string (if template)                       │   │
│   │           media_url: string (if media)                              │   │
│   │       }                                                             │   │
│   │       approval_threshold: 1                                         │   │
│   │   }                                                                 │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 8.4 Rate Limits

| Quota | Limit |
|-------|-------|
| Messages per phone number per day | 1,000 (Business tier) |
| Template messages per day | Varies by tier |

### 8.5 Policy Enforcement Points

| Check | v9 Reference | Implementation |
|-------|--------------|----------------|
| Contact allowlist | v9.10 pattern | Only known contacts |
| Daily message cap | v9.11 | Max 10 sends/day default |
| 24h window check | Platform | Block if outside window |

---

## 9. Health: Apple Health

### 9.1 Overview

| Field | Value |
|-------|-------|
| Provider | Apple |
| API | HealthKit |
| Auth | HealthKit Authorization |
| Circles | Health |

### 9.2 Read Capabilities

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    APPLE HEALTH READ CAPABILITIES                            │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   Data Types:                                                                │
│   - Steps (HKQuantityTypeIdentifierStepCount)                               │
│   - Heart rate (HKQuantityTypeIdentifierHeartRate)                          │
│   - Resting heart rate                                                      │
│   - Heart rate variability                                                  │
│   - Sleep analysis (HKCategoryTypeIdentifierSleepAnalysis)                  │
│   - Active energy burned                                                    │
│   - Workouts (HKWorkoutType)                                                │
│   - Walking + running distance                                              │
│                                                                              │
│   Sync Strategy:                                                             │
│   - On-device only (HealthKit runs on iPhone)                               │
│   - Background delivery via HealthKit observer                              │
│   - Summary synced to backend daily (aggregated, not raw)                   │
│                                                                              │
│   Data Model (synced summary):                                               │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │   HealthDailySummary {                                               │   │
│   │       date: date                                                    │   │
│   │       steps: int                                                    │   │
│   │       active_calories: int                                          │   │
│   │       resting_heart_rate: int                                       │   │
│   │       hrv_average: float                                            │   │
│   │       sleep_hours: float                                            │   │
│   │       sleep_quality: enum (poor, fair, good, excellent)             │   │
│   │       workouts: []WorkoutSummary                                    │   │
│   │   }                                                                 │   │
│   │                                                                      │   │
│   │   WorkoutSummary {                                                   │   │
│   │       workout_type: string                                          │   │
│   │       duration_minutes: int                                         │   │
│   │       calories: int                                                 │   │
│   │       distance_km: float                                            │   │
│   │       source: string (Apple Watch, Peloton, etc.)                   │   │
│   │   }                                                                 │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 9.3 Write Capabilities

**NONE**. QuantumLife does not write to Apple Health.

### 9.4 Privacy Considerations

- Health data processed on-device where possible
- Only aggregated summaries sent to backend
- No raw heart rate samples stored server-side
- User can revoke HealthKit access at any time

---

## 10. Work: GitHub

### 10.1 Overview

| Field | Value |
|-------|-------|
| Provider | GitHub |
| API | GitHub REST API v3 / GraphQL |
| Auth | OAuth 2.0 |
| Circles | Work |

### 10.2 Read Capabilities

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    GITHUB READ CAPABILITIES                                  │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   Endpoint                     Data Retrieved                               │
│   ────────                     ──────────────                               │
│   /notifications               All notifications (PRs, issues, mentions)    │
│   /repos/:owner/:repo/pulls    Pull requests                                │
│   /repos/:owner/:repo/issues   Issues                                       │
│   /user/repos                  Repositories user has access to              │
│                                                                              │
│   Data Model:                                                                │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │   GitHubNotificationItem {                                           │   │
│   │       item_id: "gh_" + notification_id                              │   │
│   │       source: "github"                                              │   │
│   │       repo: string (owner/repo)                                     │   │
│   │       type: enum (pull_request, issue, commit, release)             │   │
│   │       title: string                                                 │   │
│   │       url: string                                                   │   │
│   │       author: PersonRef                                             │   │
│   │       reason: enum (assign, author, mention, review_requested, ...)  │   │
│   │       unread: bool                                                  │   │
│   │       updated_at: timestamp                                         │   │
│   │   }                                                                 │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 10.3 Write Capabilities

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    GITHUB WRITE CAPABILITIES                                 │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   Action                       Endpoint                  v9+ Enforcement    │
│   ──────                       ────────                  ────────────────   │
│   Approve PR                   /pulls/:id/reviews        Full envelope      │
│   Request changes              /pulls/:id/reviews        Full envelope      │
│   Merge PR                     /pulls/:id/merge          Full envelope      │
│   Comment on PR/issue          /issues/:id/comments      Full envelope      │
│   Close issue                  /issues/:id               Full envelope      │
│                                                                              │
│   ExecutionEnvelope for GitHub Action:                                      │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │   GitHubExecutionEnvelope {                                          │   │
│   │       envelope_id: string                                           │   │
│   │       action_class: "github_action"                                 │   │
│   │       circle_id: "work"                                             │   │
│   │       policy_snapshot_hash: string (v9.12)                          │   │
│   │       view_snapshot_hash: string (v9.13)                            │   │
│   │       action_spec: {                                                │   │
│   │           repo: string                                              │   │
│   │           action_type: enum (approve, request_changes, merge,       │   │
│   │                              comment, close)                        │   │
│   │           pr_number: int                                            │   │
│   │           comment_body: string (if comment)                         │   │
│   │           merge_method: enum (merge, squash, rebase)                │   │
│   │       }                                                             │   │
│   │       approval_threshold: 1                                         │   │
│   │   }                                                                 │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 10.4 Rate Limits

| Quota | Limit |
|-------|-------|
| Authenticated requests | 5,000/hour |
| GraphQL | 5,000 points/hour |

---

## 11. School Portals

### 11.1 Overview

| Field | Value |
|-------|-------|
| Provider | Various (school-specific) |
| API | Web scraping / proprietary |
| Auth | Username/password (stored encrypted) |
| Circles | Kids-School |

### 11.2 Capabilities

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    SCHOOL PORTAL CAPABILITIES                                │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   Common Portals:                                                            │
│   - ParentPay (payments)                                                    │
│   - Arbor (attendance, grades)                                              │
│   - ClassCharts (homework, behavior)                                        │
│   - School-specific portals                                                 │
│                                                                              │
│   Read:                                                                      │
│   - Announcements                                                           │
│   - Homework assignments                                                    │
│   - Events and deadlines                                                    │
│   - Grades and reports                                                      │
│   - Attendance records                                                      │
│                                                                              │
│   Write:                                                                     │
│   - Permission form submission                                              │
│   - Payment for trips/meals                                                 │
│   - Absence notification                                                    │
│                                                                              │
│   Data Model:                                                                │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │   SchoolItem {                                                       │   │
│   │       item_id: "school_" + portal + "_" + item_id                   │   │
│   │       source: portal_name                                           │   │
│   │       child: PersonRef                                              │   │
│   │       type: enum (announcement, homework, event, form, payment)      │   │
│   │       title: string                                                 │   │
│   │       description: string                                           │   │
│   │       deadline: timestamp                                           │   │
│   │       action_required: bool                                         │   │
│   │       action_url: string                                            │   │
│   │   }                                                                 │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│   Implementation Note:                                                       │
│   - Scraping is fragile; portals change frequently                          │
│   - Consider manual form submission via in-app browser initially            │
│   - Prioritize read-only notification of deadlines                          │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 12. v9.7 Compliance Summary

All integrations must comply with v9.7 (no background execution in core):

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    v9.7 COMPLIANCE BY INTEGRATION                            │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   Integration          Sync Trigger                       v9.7 Status       │
│   ───────────          ────────────                       ───────────       │
│   Gmail                External cron / user refresh       Compliant         │
│   Outlook              External cron / user refresh       Compliant         │
│   Google Calendar      External cron / user refresh       Compliant         │
│   Apple Calendar       External cron / user refresh       Compliant         │
│   Plaid                External cron / user refresh       Compliant         │
│   TrueLayer            User-initiated only                Compliant         │
│   WhatsApp             Webhook (request-driven)           Compliant         │
│   Apple Health         HealthKit observer (on-device)     Compliant (*)     │
│   GitHub               Webhook / user refresh             Compliant         │
│   School Portals       External cron / user refresh       Compliant         │
│                                                                              │
│   (*) Apple Health sync runs on device, not in core backend packages        │
│                                                                              │
│   Prohibited in core packages:                                               │
│   - goroutines for polling                                                  │
│   - time.Ticker / time.Timer for scheduled tasks                            │
│   - Background workers embedded in API process                              │
│                                                                              │
│   Allowed:                                                                   │
│   - Synchronous HTTP handlers                                               │
│   - Webhook receivers                                                       │
│   - User-triggered refresh endpoints                                        │
│   - External scheduler calling HTTP endpoints                               │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 13. Document History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2025-01-01 | Core Team | Initial version |
