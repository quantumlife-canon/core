# Guided Demo Script v1

**Status**: Canonical
**Phase**: 18
**Last Updated**: 2025-01-15

---

## Purpose

This is the 60-second narrative walkthrough of QuantumLife. It can be delivered in person, recorded as a video, or used as the structure for interactive demos.

The demo uses **deterministic seeded data**. Same seed = same demo, always.

---

## The 60-Second Script

### Opening (10 seconds)

> "Let me show you what QuantumLife would have handled for you this week."

*Screen: App home showing "Nothing needs you"*

> "You're looking at the home screen. It says 'Nothing needs you.' This is success."

---

### The Handling (15 seconds)

> "Behind the scenes, QuantumLife handled 12 items."

*Scroll to "Handled this week" section*

> "Your energy bill came in. Matched your policy. Auto-approved. Paid."

*Point to energy bill line*

> "Your car insurance renewal notification. Parsed. Due date extracted. Added to your obligations."

*Point to insurance line*

> "A subscription you forgot about tried to charge you. Matched your 'flag unknown charges' policy. Surfaced."

*Point to subscription line — this one is in "Needs You"*

---

### The Decision (20 seconds)

> "That subscription? It surfaced because QuantumLife didn't have a policy for it."

*Click into the draft*

> "Here's the draft. 'Spotify Family - £16.99 monthly.' You can see why it surfaced: 'No policy covers this charge.'"

*Point to explainability section*

> "You have three options: Approve, making this automatic going forward. Reject, blocking it. Or mark for review later."

*Point to action buttons*

> "You tap Approve. Done. Next time, it handles silently."

*Tap Approve, show confirmation*

---

### The Close (15 seconds)

*Return to home screen, now truly empty*

> "That's it. One decision. The rest was handled."

*Pause*

> "QuantumLife isn't a todo list. It's not an inbox. It's an operating layer for your life. When it's working, you don't notice."

*Screen shows "Nothing needs you"*

> "And when you see this? You've won."

---

## Demo Data Specification

### Seed Value

```
demo-seed-v1-2025
```

### Circles

| Circle | People | Items Handled | Items Surfaced |
|--------|--------|---------------|----------------|
| Home | 2 | 5 | 1 |
| Finance | 1 | 4 | 0 |
| Health | 1 | 2 | 0 |
| Work | 3 | 1 | 0 |

### Handled Items

| Item | Circle | Action | Why Handled |
|------|--------|--------|-------------|
| Energy bill (£87.50) | Home | Paid | Matched auto-pay policy |
| Water bill (£34.20) | Home | Paid | Matched auto-pay policy |
| Council tax (£156.00) | Home | Paid | Matched auto-pay policy |
| Phone bill (£45.00) | Finance | Paid | Matched auto-pay policy |
| Car insurance renewal | Finance | Logged | Due date extracted |
| Dentist appointment | Health | Confirmed | Auto-confirmed |
| GP reminder | Health | Acknowledged | Standard acknowledgment |
| Gym membership | Finance | Renewed | Matched renewal policy |
| Calendar sync | Work | Completed | Background sync |
| Email digest | Work | Processed | Routine processing |

### Surfaced Items

| Item | Circle | Why Surfaced | Urgency |
|------|--------|--------------|---------|
| Spotify Family (£16.99) | Home | No policy | Needs You |

### Explainability for Surfaced Item

```
Why am I seeing this?

This charge from Spotify Family (£16.99/month) doesn't match any
existing policy. This is the first time we've seen a charge from
this merchant.

Options:
- Approve: Allow this charge and future charges from Spotify Family
- Reject: Block this charge and flag future attempts
- Review later: Keep this pending
```

---

## Extended Demo (5 minutes)

For longer demos, add these sections:

### Circles Deep Dive (60 seconds)

> "Let's look at how circles work."

*Navigate to Home circle*

> "Each circle represents an area of your life. Home has Sarah — that's your partner. Policies here govern household decisions."

*Show people, show policies*

### Policy Configuration (60 seconds)

> "Policies are rules you set."

*Show a policy*

> "This one says: auto-approve recurring bills under £200 from known vendors. That's why your energy bill was handled silently."

### Audit Trail (60 seconds)

> "Everything is auditable."

*Navigate to audit*

> "Every action, every decision, every auto-approval. You can see exactly what happened and why."

### Intersections (60 seconds)

> "Intersections are where circles overlap."

*Show intersection with Sarah*

> "When a decision affects both of you — like a shared bill — it surfaces in the intersection. Both parties must approve."

---

## Demo Environment Setup

### Requirements

1. Demo uses in-memory stores
2. Demo uses fixed clock (2025-01-15 10:00 UTC)
3. Demo uses deterministic seed
4. Demo data is pre-populated

### Reproducibility

Running the demo twice with the same seed produces identical output. This is tested:

```go
func TestDemoReproducibility(t *testing.T) {
    demo1 := RunDemo("demo-seed-v1-2025")
    demo2 := RunDemo("demo-seed-v1-2025")

    if demo1.Hash() != demo2.Hash() {
        t.Fatal("demo is not deterministic")
    }
}
```

---

## Objection Handling

### "This is just another todo app"

> "Todo apps are lists you process. QuantumLife has no list. It handles. You see only what couldn't be handled."

### "How do I trust it to pay my bills?"

> "You approve the policy. Each execution requires that policy to match. You can revoke any time. And you see everything in the audit trail."

### "What if it makes a mistake?"

> "Drafts, not actions. QuantumLife never executes without your explicit approval. If something looks wrong, you reject it."

### "Why would I pay for silence?"

> "You're not paying for silence. You're paying to not think about bills, renewals, and subscriptions. The absence of mental load is the product."

---

## Changelog

| Version | Date | Changes |
|---------|------|---------|
| v1 | 2025-01-15 | Initial canonical version |
