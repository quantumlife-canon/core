# ADR-0075: Phase 38 - Mobile Notification Metadata Observer

## Status
Accepted

## Context

Phase 31.4 introduced External Pressure Circles derived from commerce observations.
These capture abstract pressure from things like deliveries and subscriptions.

However, real-world urgency often arrives through mobile notifications:
- "Your driver is arriving" (Uber, Lyft)
- "Your order is out for delivery" (Amazon, food delivery)
- "Your appointment is in 30 minutes" (healthcare)
- "School pickup in 15 minutes" (institutions)

These represent genuine "someone is waiting" situations that the system should
be aware of - but WITHOUT:
- Reading notification content
- Identifying apps, people, merchants, or locations
- Delivering interruptions
- Making decisions
- Changing behavior

## Decision

Introduce a notification metadata observer that:

1. **Accepts ONLY metadata, never content**
   - App category (provided by OS, not app name)
   - Notification count (bucketed before input)
   - Delivery timing bucket (now/soon/later)

2. **Produces abstract pressure signals**
   - NotificationPressureSignal with only buckets and hashes
   - No free-text fields allowed
   - Max 1 signal per app class per period

3. **Integrates with existing pressure pipeline**
   - Converts to ExternalPressureCircle (Phase 31.4)
   - Feeds into Phase 32 Pressure Decision Gate
   - Does NOT alter any downstream phase

4. **Stores hash-only evidence**
   - Append-only, 30-day bounded retention
   - No lookup by app, device, or user
   - Storelog integration for replay

## Why Notification Content is Forbidden

Notification content contains:
- Names of people ("John is waiting")
- Specific times ("arriving at 3:47 PM")
- Addresses and locations
- Financial amounts
- Medical information

Even "helpful" parsing would:
- Require app-specific logic
- Create maintenance burden
- Risk privacy leaks
- Create anxiety through specificity

Abstract buckets preserve privacy and calm.

## Why Urgency is Abstracted

Instead of "Your Uber arrives in 3 minutes", we observe:
- App class: transport
- Magnitude: a_few (1-3 notifications)
- Horizon: now

This tells us "something transport-related needs attention soon"
without exposing who, where, or exactly when.

## Why This Phase Cannot Interrupt

This phase ONLY produces pressure signals. It cannot:
- Deliver notifications
- Trigger sounds or vibrations
- Change phone state
- Display UI elements

Whether a signal becomes an interruption is determined by:
- Phase 32: Pressure Decision Gate
- Phase 33: Interrupt Permission Contract
- Phase 34: Permitted Interrupt Preview
- Phase 35-36: Sealed delivery with undo

## How This Solves "Someone is Waiting" Without Anxiety

Traditional notification systems create anxiety through:
- Constant interruptions
- Specific countdown timers
- "Your driver is waiting" guilt messages

Our approach:
1. System observes that transport-category pressure exists
2. Signal enters pressure pipeline as abstract bucket
3. User sees "External, contained." on pressure proof page
4. User decides if/when to check their phone
5. No guilt, no urgency, no manipulation

The system knows "something external is pressing" without knowing
what, who, or where - and without acting unless explicitly permitted.

## App Class Categories

| AppClass | Examples (never exposed) | Interpretation |
|----------|-------------------------|----------------|
| transport | Uber, Lyft, rideshare | Movement-related pressure |
| health | Medical, pharmacy, wellness | Health-related pressure |
| institution | School, government, banks | Official/institutional pressure |
| commerce | Delivery, retail, payments | Transaction-related pressure |
| unknown | Everything else | Abstract external pressure |

## Magnitude Buckets

| Bucket | Count Range | Interpretation |
|--------|------------|----------------|
| nothing | 0 | No pressure |
| a_few | 1-3 | Minimal pressure |
| several | 4+ | Moderate pressure |

## Horizon Buckets

| Bucket | Meaning |
|--------|---------|
| now | Immediate attention may be warranted |
| soon | Attention within hours |
| later | No immediate urgency |

## Critical Invariants

1. **No notification content** - Only OS-provided category metadata
2. **No app names** - Only abstract class buckets
3. **No device identifiers** - Hash-only storage
4. **No time.Now()** - Clock injection only
5. **No goroutines** - Synchronous processing
6. **No network calls** - Pure local observation
7. **No decision logic** - Observation only
8. **No delivery triggers** - Cannot send notifications
9. **stdlib only** - No external dependencies
10. **hash-only storage** - No raw data persistence

## Consequences

### Positive
- System aware of real-world pressure
- Privacy preserved through abstraction
- No anxiety-inducing specificity
- Integrates cleanly with existing pipeline
- User remains in control

### Negative
- Cannot provide app-specific context
- Cannot differentiate urgent from non-urgent within category
- Requires iOS/Android permission integration (future phases)

## Related

- ADR-0067: Phase 31.4 External Pressure Circles
- ADR-0068: Phase 32 Pressure Decision Gate
- ADR-0069: Phase 33 Interrupt Permission Contract
- ADR-0071: Phase 35 Push Permission + Sealed Notification
