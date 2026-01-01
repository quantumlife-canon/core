# ADR-0034: Phase 18 - Product Language System and Web Shell

## Status
ACCEPTED

## Context
QuantumLife is a Personal Operating System for life administration. Before building more features, we must define the product category's LANGUAGE, BEHAVIOR, and VISUAL SYSTEM. This is not "build a web page" - it's establishing a new product category where success is measured by SILENCE.

### Core Philosophy
> "When QuantumLife is working, you don't notice."

The success state is "Nothing Needs You" - the absence of mental load is the product.

### Problems with Existing Solutions
| Solution | Failure Mode |
|----------|--------------|
| Todo Apps | Create lists you process. QuantumLife has no list. |
| Inbox Zero | You manage a queue. QuantumLife manages obligations. |
| AI Assistants | Wait for commands. QuantumLife acts within policies. |
| Automation Tools | Require configuration. QuantumLife learns from approvals. |
| Calendar Apps | Show everything. QuantumLife shows only what needs you. |

## Decision

### Product Language System
Establish a vocabulary contract that spans all surfaces:

**Core Terms (Never Substitute)**
- **Circle**: Area of life context (not "folder", "category", "workspace")
- **Intersection**: Where circles overlap (not "shared", "joint")
- **Needs You**: Items requiring attention (not "pending", "todo", "unread")
- **Draft**: Prepared action awaiting approval (not "suggestion", "recommendation")
- **Approval**: Explicit consent that enables future silent handling
- **Handled**: Completed within policy (not "done", "finished")
- **Policy**: Rule governing silent handling (not "preference", "setting")

**Interruption Levels (4-Level System)**
| Level | Description | UI Treatment |
|-------|-------------|--------------|
| Silent (0) | Handled, never shown | No UI |
| Ambient (1) | Available if sought | Subtle background |
| Needs You (2) | Requires attention | Warm highlight |
| Urgent (3) | Time-sensitive | Soft alert |

### Design Tokens
CSS custom properties for cross-platform portability:

```css
:root {
  /* Typography */
  --font-sans: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, ...;
  --text-base: 0.9375rem;

  /* Spacing (4px base) */
  --space-4: 1rem;

  /* Colors */
  --color-bg: #FAFAFA;
  --color-surface: #FFFFFF;
  --color-text-primary: #1A1A1A;

  /* Interruption Levels */
  --color-level-silent: transparent;
  --color-level-ambient: #F5F5F5;
  --color-level-needs-you: #FFF9E6;
  --color-level-urgent: #FFF0F0;
}
```

### Web Reference Implementation
Stdlib-only stack (no React, Tailwind, or JavaScript):
- `net/http` for routing
- `html/template` for rendering
- External CSS files referencing design tokens

File structure:
```
cmd/quantumlife-web/
├── main.go              # Server + handlers + templates
└── static/
    ├── tokens.css       # Design tokens
    ├── reset.css        # CSS reset
    └── app.css          # Component styles
```

Routes:
| Route | Purpose |
|-------|---------|
| `/` | Landing page |
| `/demo` | Demo mode entry |
| `/app` | App home (success state) |
| `/app/circle/:id` | Circle detail |
| `/app/drafts` | All drafts |
| `/app/draft/:id` | Single draft |
| `/app/people` | People/contacts |
| `/app/policies` | Policy management |

### Voice and Tone Rules
1. **Calm, Never Urgent** - No exclamation marks, no "action required"
2. **Factual, Never Persuasive** - State what happened, not why it matters
3. **Brief, Never Elaborate** - One sentence preferred
4. **Confident, Never Hedging** - No "we think" or "you might want"
5. **Neutral, Never Emotional** - No "great news" or "unfortunately"

### Copy Patterns

**Empty State (Success):**
```
Nothing needs you
[Secondary: This week: 12 handled, 3 require decisions]
```

**Item Surfaced:**
```
Spotify Family · £16.99
Monthly · No policy covers this charge
[Approve] [Reject] [Later]
```

**Explainability (On Demand):**
```
Why am I seeing this?
This charge doesn't match any existing policy.
This is the first time we've seen this merchant.
```

### iOS Handoff Contract
Token-to-SwiftUI mapping for future iOS implementation:

| CSS Token | SwiftUI Equivalent |
|-----------|-------------------|
| `--font-sans` | `.system()` |
| `--text-base` | `Font.body` |
| `--color-text-primary` | `Color.primary` |
| `--color-level-needs-you` | `Color.yellow.opacity(0.15)` |

### Demo Specification
Deterministic demo with seed `demo-seed-v1-2025`:
- Fixed clock: 2025-01-15 10:00 UTC
- 12 items handled silently
- 1 item surfaced (Spotify Family - no policy)
- Same seed = same output, always

## Consequences

### Positive
- Consistent vocabulary across all surfaces
- Portable design system (CSS → iOS → Android)
- Stdlib-only web stack (no build tools, no dependencies)
- Reproducible demos for investor presentations
- Clear success metric (silence = success)

### Negative
- Limited interactivity without JavaScript
- No hot reload during development
- Manual CSS maintenance (no preprocessor)

### Constraints
- All CSS must reference tokens (never raw values)
- All copy must use vocabulary contract terms
- Templates must render without JavaScript
- Demo must be deterministic (no time.Now())
- No external dependencies in pkg/internal

## Files Changed
```
docs/PRODUCT_LANGUAGE_SYSTEM_V1.md           (NEW)
docs/DESIGN_TOKENS_V1.md                      (NEW)
docs/COPY_DECK_V1.md                          (NEW)
docs/WEB_FLOWS_V1.md                          (NEW)
docs/IOS_UI_MAPPING_V1.md                     (NEW)
docs/CATEGORY_MANIFESTO_V1.md                 (NEW)
docs/GUIDED_DEMO_SCRIPT_V1.md                 (NEW)
cmd/quantumlife-web/main.go                   (MODIFIED)
cmd/quantumlife-web/static/tokens.css         (NEW)
cmd/quantumlife-web/static/reset.css          (NEW)
cmd/quantumlife-web/static/app.css            (NEW)
internal/demo_phase18_product_language/       (NEW)
Makefile                                       (MODIFIED)
docs/ADR/ADR-0034-phase18-product-language-and-web-shell.md (NEW)
```

## References
- Canon v1: docs/QUANTUMLIFE_CANON_V1.md
- Technical Split v9: docs/TECHNICAL_SPLIT_V9_EXECUTION.md
- Phase 17 Finance: docs/ADR/ADR-0033-phase17-finance-execution-boundary.md
