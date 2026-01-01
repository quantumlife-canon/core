# Web Flows v1

**Status**: Canonical
**Phase**: 18
**Last Updated**: 2025-01-15

---

## Purpose

This document defines the information architecture and user flows for the QuantumLife web application. Each screen is defined by its intent, not its visual treatment.

---

## Screen Inventory

| Route | Screen | Purpose |
|-------|--------|---------|
| `/` | Public Landing | Explain the category, invite exploration |
| `/demo` | Public Demo | Deterministic demonstration |
| `/app` | App Home | Show "Nothing Needs You" or items needing attention |
| `/app/circle/:id` | Circle Detail | Deep view of a single circle |
| `/app/drafts` | Drafts List | All pending drafts |
| `/app/draft/:id` | Draft Detail | Review a specific draft |
| `/app/people` | People | Identity graph view |
| `/app/policies` | Policies | Circle policy management |

---

## 1. Public Landing (`/`)

### Primary User Question

> "What is this? Is it for me?"

### Intent

Explain that QuantumLife is a new category — not a task manager, not an inbox, not an assistant. Communicate the emotional outcome (feeling lighter) without feature lists.

### Content Structure

1. **Hero** — "Nothing Needs You" + category definition
2. **Problem** — The invisible weight of life administration
3. **Solution** — A system that handles, surfaces only what needs you
4. **Proof** — Demo CTA
5. **Entry** — Login/Enter App CTA

### What Is NOT Shown

- Feature lists
- Pricing
- Screenshots of busy dashboards
- Testimonials
- Metrics or statistics

### Empty State

N/A — Landing page is never empty.

### Emotional Register

Calm confidence. Intrigue without hype.

---

## 2. Public Demo (`/demo`)

### Primary User Question

> "How does this actually work?"

### Intent

Show a deterministic week of QuantumLife operation. Same seed = same output, always. User sees circles, what needed them, what was handled.

### Content Structure

1. **Demo header** — Clear "Demo" badge, "This is simulated" notice
2. **Circles row** — Visual representation of demo circles
3. **Needs You** — Items that surfaced in this demo week
4. **Handled** — Items the system resolved automatically
5. **Explainability** — "Why" available for every item

### What Is NOT Shown

- Real user data
- Live integrations
- Account creation prompts mid-demo
- Feature tour modals

### Empty State

N/A — Demo is never empty; it shows a scripted week.

### Emotional Register

Educational. "See how calm this is."

---

## 3. App Home (`/app`)

### Primary User Question

> "Does anything need me right now?"

### Intent

The home screen answers one question: do I need to make any decisions? If not, show "Nothing Needs You" prominently. If yes, show the items clearly.

### Content Structure

**When empty**:
1. **"Nothing Needs You"** — Large, centered, calm
2. **Handled count** — "QuantumLife handled 12 items this week"
3. **Circles row** — Quick access to circles

**When items exist**:
1. **Needs You count** — "{N} items need you"
2. **Needs You list** — Prioritized by urgency, then deadline
3. **Circles row** — With badges showing pending items

### What Is NOT Shown

- Dashboards
- Charts
- Activity feeds
- Gamification elements
- Time-in-app metrics

### Empty State

"Nothing Needs You" is not an error state. It is the success state. The visual treatment should feel like relief and accomplishment.

### Emotional Register

Relief when empty. Focused calm when items exist.

---

## 4. Circle Detail (`/app/circle/:id`)

### Primary User Question

> "What's happening in this area of my life?"

### Intent

Deep view of a single circle (Home, Work, Health, etc.). Shows people in this circle, policies governing it, and any pending items.

### Content Structure

1. **Circle header** — Name, description
2. **Needs You** — Items in this circle needing decisions
3. **People** — Identities associated with this circle
4. **Policies** — Rules governing this circle
5. **Audit** — Recent history in this circle

### What Is NOT Shown

- Cross-circle items (those appear on home)
- Historical data beyond recent
- Complex policy editors inline

### Empty State

"This circle is quiet. Nothing requires your attention."

Emotional: This circle is running smoothly.

---

## 5. Drafts List (`/app/drafts`)

### Primary User Question

> "What decisions are waiting for me?"

### Intent

Show all pending drafts across all circles. Grouped by urgency, then by circle.

### Content Structure

1. **Drafts count** — "{N} drafts pending"
2. **Draft cards** — Each showing action, amount, deadline, circle
3. **Quick actions** — Approve/Reject inline where appropriate

### What Is NOT Shown

- Executed drafts (those are in audit)
- Complex filtering
- Batch operations (each draft is a decision)

### Empty State

"No drafts. Proposed actions will appear here."

Emotional: Nothing is waiting for you.

---

## 6. Draft Detail (`/app/draft/:id`)

### Primary User Question

> "Should I approve this? What exactly will happen?"

### Intent

Full view of a single draft. Every detail needed to make a decision. Explainability prominent.

### Content Structure

1. **Draft header** — Action type, amount, recipient
2. **Why** — "Why am I seeing this?" always visible
3. **Details** — All relevant information
4. **Policy context** — What policy this falls under
5. **Actions** — Approve / Reject / Decide Later

### What Is NOT Shown

- Other drafts
- Upsells
- Suggested actions

### Empty State

N/A — If draft doesn't exist, redirect to drafts list.

### Emotional Register

Focused decision-making. All information visible. No pressure.

---

## 7. People (`/app/people`)

### Primary User Question

> "Who does the system know about?"

### Intent

View of the identity graph. All people/entities the system recognizes, organized by circle.

### Content Structure

1. **People count** — "{N} people known"
2. **By circle** — Grouped by primary circle
3. **Identity cards** — Name, role, relationship
4. **Intersection indicator** — Shows shared circles

### What Is NOT Shown

- Social features
- Communication tools
- Activity feeds about people

### Empty State

"No people yet. Identities will appear as you connect accounts."

---

## 8. Policies (`/app/policies`)

### Primary User Question

> "What rules govern my circles?"

### Intent

View and manage policies across circles. Policies determine what is auto-approved, what surfaces, what is escalated.

### Content Structure

1. **By circle** — Policies grouped by circle
2. **Policy cards** — Name, description, effect
3. **Thresholds** — Auto-approve limits, etc.

### What Is NOT Shown

- Complex rule builders (Phase 19+)
- AI policy suggestions
- A/B testing of policies

### Empty State

"No policies defined. Default policies apply."

---

## Flow Diagrams

### Landing → Demo → App

```
[/]
 ├── "Try Demo" → [/demo]
 │                   └── "Enter App" → [/app]
 └── "Enter App" → [/app]
```

### App Navigation

```
[/app]
 ├── [Circles Row]
 │    └── Click → [/app/circle/:id]
 ├── [Needs You Item]
 │    └── Click → [/app/draft/:id]
 ├── [Nav: Drafts] → [/app/drafts]
 │                    └── Click → [/app/draft/:id]
 ├── [Nav: People] → [/app/people]
 └── [Nav: Policies] → [/app/policies]
```

### Approval Flow

```
[/app/draft/:id]
 ├── "Approve" → Execute → [/app] with confirmation
 ├── "Reject" → Archive → [/app] with confirmation
 └── "Decide Later" → [/app/drafts]
```

---

## URL Design

| Pattern | Example |
|---------|---------|
| Public | `/`, `/demo` |
| App root | `/app` |
| App section | `/app/drafts`, `/app/people` |
| App detail | `/app/circle/:id`, `/app/draft/:id` |

No query parameters for core navigation. State is in URL path.

---

## Responsive Behavior

| Breakpoint | Layout |
|------------|--------|
| Mobile (<640px) | Single column, stacked |
| Tablet (640-1024px) | Two columns where appropriate |
| Desktop (>1024px) | Max-width container, generous whitespace |

Navigation collapses to hamburger on mobile.

---

## Changelog

| Version | Date | Changes |
|---------|------|---------|
| v1 | 2025-01-15 | Initial canonical version |
