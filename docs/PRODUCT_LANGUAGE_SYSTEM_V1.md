# Product Language System v1

**Status**: Canonical
**Phase**: 18
**Last Updated**: 2025-01-15

---

## Purpose

This document defines the language, voice, and behavioral principles that govern every QuantumLife surface. All UI copy, documentation, and user communication must conform to these rules.

QuantumLife is not software to be used. It is a system that handles life administration so you do not have to think about it.

---

## 1. Brand Essence

### Core Phrases

These phrases define QuantumLife. They appear in the product, in marketing, and in internal discussion.

| Phrase | Meaning |
|--------|---------|
| **Nothing Needs You** | The success state. When the system is working, this is what the user sees. |
| **Interruptions are earned** | The system must justify every surfaced item. Nothing appears by default. |
| **Drafts, not actions** | QuantumLife proposes. The human decides. No autonomous execution. |
| **You stay sovereign** | The user controls what happens. The system never acts without consent. |

### What QuantumLife Is NOT

QuantumLife rejects the patterns of existing software categories:

| Anti-Pattern | Why We Reject It |
|--------------|------------------|
| **Not a feed** | Feeds exploit attention. We protect it. |
| **Not a task manager** | Task managers are lists to process. We eliminate the list. |
| **Not an assistant** | Assistants act on your behalf. We never act without approval. |
| **Not engagement-driven** | Success is measured by silence, not time-in-app. |
| **Not an inbox** | Inboxes collect. We handle. |
| **Not a dashboard** | Dashboards display. We resolve. |

### The North Star Metric

**Time NOT spent in the app.**

If the user opens QuantumLife frequently, the system is failing. The ideal user opens the app once per week, sees "Nothing Needs You", and closes it.

---

## 2. Voice & Tone Rules

### Principles

1. **Calm** — Never urgent. Never pushy. Never alarming unless truly necessary.
2. **Precise** — Say exactly what is meant. No ambiguity.
3. **Respectful** — The user's time is sacred. Every word must earn its place.
4. **Non-salesy** — No marketing language in the product. No upsells. No growth hacks.

### Writing Rules

| Rule | Example |
|------|---------|
| Short sentences | "This draft expires tomorrow." not "Just a friendly reminder that this draft is set to expire tomorrow!" |
| Minimal punctuation | Avoid exclamation marks. One period per sentence. |
| No hype | "Ready for review" not "Awesome! Your draft is ready!" |
| No emojis | Never. Not even in casual contexts. |
| No gamification | No streaks. No points. No badges. No celebrations. |
| Explainable | Every surfaced item has a "Why" available. |

### Forbidden Words

These words never appear in QuantumLife:

- Amazing, Awesome, Exciting, Great
- Boost, Optimize, Maximize, Supercharge
- Smart, Intelligent, AI-powered (as marketing)
- Notification, Alert, Reminder (use "surfaced", "needs you")
- Task, Todo, Action item (use "draft", "need")
- Sync, Update, Refresh (invisible to user)

### Allowed Emotional Register

| Emotion | When Allowed |
|---------|--------------|
| Calm confidence | Always |
| Quiet satisfaction | Empty states |
| Gentle urgency | Time-sensitive items only |
| Neutral acknowledgment | After approvals |

---

## 3. Behavioral UX Principles

### Emptiness Is Success

The empty state is not an error. It is the goal. When QuantumLife shows "Nothing Needs You", the system has succeeded in handling life administration.

**Empty states must feel like relief, not abandonment.**

### Attention Is Scarce

Every interruption has a cost. The system must justify surfacing anything. The threshold for interruption is high.

**If in doubt, do not surface.**

### Silence Is The Default

The default state of QuantumLife is silence. No notifications. No badges. No alerts. Items are surfaced only when:

1. A human decision is required
2. The decision window is closing
3. No reasonable default exists

### Everything Is Explainable

Every surfaced item must answer "Why am I seeing this?" The explanation must be:

- One sentence
- Factual
- Traceable to a specific trigger

**Example**: "You're seeing this because your energy bill payment is due in 3 days and requires your approval."

### The User Is Always In Control

QuantumLife never:

- Acts without explicit approval
- Hides options
- Creates urgency artificially
- Manipulates through design

The user can always:

- Dismiss anything
- Understand why something appeared
- Undo any approval
- Revoke any execution

---

## 4. Vocabulary Contract

These terms are sacred. They have precise meanings and must be used consistently across all surfaces.

### Core Vocabulary

| Term | Definition | Usage |
|------|------------|-------|
| **Circle** | A context or role the user operates in (e.g., "Home", "Work", "Health"). Contains people, policies, and obligations. | "Your Home circle" |
| **Intersection** | The overlap between two or more people's circles. Where shared decisions happen. | "The intersection between you and Sarah" |
| **Needs You** | An item that requires human decision. Surfaced because no default applies. | "3 items need you" |
| **Digest** | A periodic summary of what the system handled. Sent weekly by default. | "Your weekly digest" |
| **Draft** | A proposed action awaiting approval. Never executed automatically. | "Review this draft" |
| **Approval** | Explicit human consent to execute a draft. | "Approve" / "Reject" |
| **Execution Envelope** | A sealed, auditable container for an approved action. Immutable once created. | Internal terminology |
| **Audit Trail** | The complete history of decisions and executions. Always available. | "View audit trail" |

### Secondary Vocabulary

| Term | Definition |
|------|------------|
| **Policy** | A rule that governs a circle's behavior |
| **Obligation** | A recurring commitment (bill, subscription, etc.) |
| **Identity** | A person or entity the system knows about |
| **Explainability** | The ability to understand why something was surfaced |
| **Revocation** | Canceling an approved-but-not-executed action |

### Forbidden Synonyms

Do not use these terms. Use the canonical vocabulary instead.

| Forbidden | Use Instead |
|-----------|-------------|
| Task | Need / Draft |
| Notification | Surfaced item |
| Alert | Need |
| Reminder | Surfaced item |
| Contact | Identity / Person |
| Group | Circle |
| Permission | Policy |
| Action | Draft / Execution |

---

## 5. Interruption Levels

QuantumLife has exactly four interruption levels. Each maps to specific visual and behavioral treatment.

| Level | Name | When Used | Visual Treatment |
|-------|------|-----------|------------------|
| 0 | **Silent** | Handled automatically. User never sees. | Not displayed |
| 1 | **Ambient** | Included in digest. No active surfacing. | Muted, background |
| 2 | **Needs You** | Requires decision. Surfaced in app. | Standard prominence |
| 3 | **Urgent** | Time-critical. Active notification. | Elevated, but calm |

**There is no Level 4.** Nothing is more urgent than urgent. Panic is not a design state.

---

## 6. Component Naming

UI components have semantic names that match their purpose.

| Component | Purpose |
|-----------|---------|
| `CircleCard` | Displays a circle's summary |
| `NeedsYouItem` | A single item requiring decision |
| `DraftCard` | A proposed action for review |
| `ExplainPanel` | Answers "Why am I seeing this?" |
| `DigestSection` | A portion of the weekly digest |
| `EmptyState` | The "Nothing Needs You" display |
| `ApprovalButtons` | Approve / Reject controls |
| `AuditRow` | A single entry in audit trail |

---

## 7. Copy Formulas

### Empty States

```
Pattern: [Positive statement]. [What the system did].

Example: "Nothing needs you. QuantumLife handled 12 items this week."
```

### Surfaced Items

```
Pattern: [What] — [Why] — [When]

Example: "Energy bill payment — requires your approval — due in 3 days"
```

### Drafts

```
Pattern: [Action verb] [amount/target] [context]

Example: "Pay £127.50 to British Gas for March electricity"
```

### Approvals

```
Approve: "Approve"
Reject: "Reject"
Defer: "Decide later"
```

No additional copy. No "Are you sure?" dialogs unless irreversible.

---

## 8. Anti-Patterns

### Never Do These

1. **Fake urgency** — "Act now!" / "Limited time!"
2. **Social proof** — "10,000 users love this!"
3. **Guilt** — "You haven't checked in for 5 days"
4. **Gamification** — Points, streaks, badges, levels
5. **Dark patterns** — Hidden options, confusing flows
6. **Over-communication** — Multiple channels for one item
7. **Celebration** — Confetti, animations for basic actions
8. **Mystery** — Unexplained AI decisions

---

## 9. Success Criteria

The product language is successful when:

1. Users feel calmer after using the app
2. Users open the app less over time
3. Users can explain how decisions were made
4. Users feel in control, not managed
5. The empty state feels like accomplishment

---

## Changelog

| Version | Date | Changes |
|---------|------|---------|
| v1 | 2025-01-15 | Initial canonical version |
