# ADR-0032: Phase 16 - Notification Projection

## Status
ACCEPTED

## Context
QuantumLife needs to convert computed interruptions (Phase 3/14) into deterministic, policy-driven outbound notifications. The system must:
- Respect quiet hours and daily limits
- Enforce privacy boundaries (personal circles never leak to spouse)
- Use web as primary UI with email as first outbound channel
- Support push/SMS as interfaces only (mocks in Phase 16)
- Never spam or create engagement loops

### Real-World Scenario
Satish receives urgent email at 11 PM. The notification system:
1. Computes notification from interruption
2. Checks quiet hours policy (10 PM - 7 AM)
3. Since not urgent-allowed, downgrades from push to web_badge
4. Stores badge for morning display
5. Logs decision with full audit trail

## Decision

### Notification Domain Model (`pkg/domain/notify/`)
```go
type Notification struct {
    NotificationID  string
    InterruptionID  string
    CircleID        identity.EntityID
    IntersectionID  string
    Level           interrupt.Level
    Channel         Channel      // web_badge, email_digest, email_alert, push, sms
    Trigger         interrupt.Trigger
    Audience        Audience     // owner_only, spouse_only, both, intersection
    PersonIDs       []identity.EntityID
    Summary         string
    Template        string
    PlannedAt       time.Time
    DeliveredAt     *time.Time
    ExpiresAt       time.Time
    Status          NotificationStatus
    SuppressionReason SuppressionReason
    OriginalChannel Channel
    Hash            string
}
```

Channels ordered by intrusiveness:
- `web_badge` (1) - Passive badge count
- `email_digest` (2) - Weekly summary
- `email_alert` (3) - Immediate email
- `push` (4) - Mobile push
- `sms` (5) - SMS message

### Notification Policy (`pkg/domain/policy/`)
```go
type NotificationPolicy struct {
    CircleID        string
    QuietHours      QuietHoursPolicy  // start/end minutes, allow_urgent, downgrade_to
    DailyLimits     DailyLimits       // per-channel daily caps
    LevelChannels   LevelChannels     // level -> allowed channels
    DigestSchedule  DigestSchedule    // day, hour, auto_send (false in Phase 16)
    AllowDigestSend bool
    IsPrivate       bool              // never shared with intersection
}
```

Default quiet hours: 10 PM - 7 AM UTC, urgent allowed, downgrade to web_badge.

### Notification Planner (`internal/notifyplan/`)
```go
func (p *Planner) Plan(input PlannerInput) *PlannerOutput {
    // 1. Skip silent level
    // 2. Check user suppressions
    // 3. Select channel based on level + policy
    // 4. Apply quiet hours downgrade
    // 5. Apply daily quota downgrade
    // 6. Resolve audience from intersection rules
    // 7. Enforce privacy boundaries
    // 8. Compute deterministic plan hash
}
```

### Notification Executor (`internal/notifyexec/`)
```go
type NotificationEnvelope struct {
    EnvelopeID         string
    Notification       *notify.Notification
    PolicySnapshotHash string
    ViewSnapshotHash   string
    IdempotencyKey     string
    TraceID            string
    Status             EnvelopeStatus
}

func (e *Executor) Execute(env *NotificationEnvelope) (*NotificationEnvelope, error) {
    // 1. Check idempotency
    // 2. Verify policy snapshot
    // 3. Verify view snapshot
    // 4. Execute based on channel:
    //    - web_badge: store in BadgeStore
    //    - email_*: create EmailDraft (not auto-sent)
    //    - push/sms: mock only, blocked in real mode
}
```

### Channel Behavior
| Channel | Behavior |
|---------|----------|
| web_badge | Store for UI rendering, unlimited quota |
| email_digest | Create draft, requires explicit send |
| email_alert | Create draft, goes through approval |
| push | Mock only, ErrChannelBlocked in real mode |
| sms | Mock only, ErrChannelBlocked in real mode |

### Persistence (`internal/persist/notification_store.go`)
Uses Phase 12 storelog with record types:
- `NOTIFICATION_PLANNED` - Planned notification
- `NOTIFICATION_DELIVERED` - Delivery confirmation
- `NOTIFICATION_SUPPRESSED` - Suppression with reason
- `NOTIFY_ENVELOPE` - Execution envelope
- `NOTIFY_BADGE` - Web badge state
- `NOTIFY_DIGEST` - Digest plan

### Events (`pkg/events/events.go`)
Phase 16 events cover:
- Notification plan lifecycle
- Envelope execution
- Channel delivery (badge, email, push, sms)
- Policy enforcement (quiet hours, quota, privacy)
- Store operations

## Absolute Constraints
1. **stdlib only** - No external dependencies
2. **No goroutines** - Synchronous operations only
3. **No time.Now()** - Clock injection required
4. **Deterministic** - Same inputs + clock → same outputs
5. **No auto-retry** - Single attempt only
6. **Push/SMS mock only** - ErrNotSupported in real mode
7. **Email via drafts** - Never auto-send user-visible content

## Consequences

### Positive
- Web-first approach ensures no spam
- Quiet hours prevent after-hours interruptions
- Daily limits prevent notification fatigue
- Privacy boundaries protect personal circles
- Deterministic planning enables testing
- Full audit trail via storelog

### Negative
- Push/SMS not available until external providers integrated
- Digest send requires manual trigger (no scheduler yet)
- Email delivery still requires approval flow

### Neutral
- Web badges can accumulate (user clears manually)
- Policy changes don't affect already-planned notifications

## Audience Resolution
| Scenario | Audience |
|----------|----------|
| Personal circle | owner_only (always) |
| Family circle, intersection | based on rule (both, owner, spouse) |
| Private circle flag | owner_only (override intersection) |
| Work circle, no intersection | owner_only |

## Level → Channel Mapping (Default)
| Level | Allowed Channels |
|-------|------------------|
| silent | (none) |
| ambient | web_badge |
| queued | web_badge, email_digest |
| notify | web_badge, email_alert |
| urgent | web_badge, email_alert, push |

## Files Created
```
pkg/domain/notify/
├── types.go           # Notification model
└── types_test.go      # Determinism tests

pkg/domain/policy/     # (extended)
└── types.go           # + NotificationPolicy

internal/notifyplan/
├── planner.go         # Notification planner
└── planner_test.go    # Planning tests

internal/notifyexec/
├── executor.go        # Notification executor
└── interfaces.go      # Stores, providers

internal/persist/
└── notification_store.go  # Storelog persistence

internal/demo_phase16_notifications/
└── demo_test.go       # Demo scenarios S1-S6

scripts/guardrails/
└── notification_projection_enforced.sh

docs/ADR/
└── ADR-0032-phase16-notification-projection.md
```

## Guardrail Script
`scripts/guardrails/notification_projection_enforced.sh` validates:
1. Deterministic hashing in notify domain
2. Quiet hours in policy
3. No goroutines in notify packages
4. No time.Now() in notify packages
5. Planner with deterministic patterns
6. Executor with envelope pattern
7. Push/SMS blocked in real mode
8. Phase 16 events defined
9. Storelog record types defined
10. Demo tests exist
11. No auto-retry patterns
12. Web badge is primary channel

## References
- Phase 3: Interruption Engine (interrupt domain)
- Phase 14: Circle Policies (policy domain)
- Phase 12: Persistence & Replay (storelog)
- Phase 7: Email Execution Boundary (email drafts)
