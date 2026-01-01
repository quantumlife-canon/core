# Design Tokens v1

**Status**: Canonical
**Phase**: 18
**Last Updated**: 2025-01-15

---

## Purpose

Design tokens are the atomic units of the QuantumLife visual system. Every visual decision references tokens, never raw values. This ensures consistency across web, iOS, and future surfaces.

---

## 1. Typography

### Font Stack

QuantumLife uses system fonts exclusively. No custom fonts, no loading, no FOUT.

```css
--font-sans: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto,
             "Helvetica Neue", Arial, sans-serif;
--font-mono: ui-monospace, "SF Mono", SFMono-Regular, Menlo,
             Monaco, Consolas, monospace;
```

### Type Scale

| Token | Size | Use Case |
|-------|------|----------|
| `--text-xs` | 11px | Metadata, timestamps |
| `--text-sm` | 13px | Secondary text, captions |
| `--text-base` | 15px | Body text, default |
| `--text-lg` | 17px | Card titles, emphasis |
| `--text-xl` | 21px | Section headers |
| `--text-2xl` | 28px | Page titles |
| `--text-3xl` | 36px | Hero headlines |

### Line Height

| Token | Value | Use Case |
|-------|-------|----------|
| `--leading-tight` | 1.2 | Headlines |
| `--leading-normal` | 1.5 | Body text |
| `--leading-relaxed` | 1.7 | Long-form reading |

### Font Weight

| Token | Value | Use Case |
|-------|-------|----------|
| `--font-normal` | 400 | Body text |
| `--font-medium` | 500 | Emphasis, buttons |
| `--font-semibold` | 600 | Titles, headers |

---

## 2. Spacing Scale

All spacing uses a 4px base unit. These are the only allowed spacing values.

| Token | Value | Use Case |
|-------|-------|----------|
| `--space-1` | 4px | Inline spacing, icon gaps |
| `--space-2` | 8px | Tight component padding |
| `--space-3` | 12px | Standard padding |
| `--space-4` | 16px | Card padding, gaps |
| `--space-6` | 24px | Section spacing |
| `--space-8` | 32px | Large gaps |
| `--space-12` | 48px | Section margins |
| `--space-16` | 64px | Page margins |

---

## 3. Color Palette

QuantumLife uses a restrained, calm palette. Colors are intentionally muted.

### Core Colors

| Token | Light Mode | Dark Mode | Use Case |
|-------|------------|-----------|----------|
| `--color-bg` | #FAFAFA | #121212 | Page background |
| `--color-surface` | #FFFFFF | #1E1E1E | Card backgrounds |
| `--color-surface-raised` | #FFFFFF | #252525 | Elevated surfaces |
| `--color-text-primary` | #1A1A1A | #EBEBEB | Primary text |
| `--color-text-secondary` | #666666 | #A0A0A0 | Secondary text |
| `--color-text-tertiary` | #999999 | #707070 | Metadata, hints |
| `--color-border` | #E5E5E5 | #333333 | Dividers, borders |
| `--color-border-subtle` | #F0F0F0 | #282828 | Subtle separators |

### Interactive Colors

| Token | Value | Use Case |
|-------|-------|----------|
| `--color-focus` | #0066CC | Focus rings |
| `--color-link` | #0066CC | Text links |
| `--color-link-hover` | #004499 | Link hover state |

### Action Colors

| Token | Value | Use Case |
|-------|-------|----------|
| `--color-action-primary` | #1A1A1A | Primary buttons |
| `--color-action-primary-hover` | #333333 | Primary button hover |
| `--color-action-secondary` | transparent | Secondary buttons |
| `--color-action-secondary-border` | #CCCCCC | Secondary button border |

### Interruption Level Colors

These colors are intentionally muted. Even "urgent" is calm.

| Token | Value | Use Case |
|-------|-------|----------|
| `--color-level-silent` | transparent | Not displayed |
| `--color-level-ambient` | #F5F5F5 | Background items |
| `--color-level-needs-you` | #FFF9E6 | Standard attention (warm cream) |
| `--color-level-urgent` | #FFF0F0 | Time-sensitive (soft rose) |

### Semantic Colors

| Token | Value | Use Case |
|-------|-------|----------|
| `--color-success` | #2E7D32 | Confirmations (muted green) |
| `--color-error` | #C62828 | Errors (muted red) |
| `--color-warning` | #F9A825 | Warnings (muted amber) |

---

## 4. Radius

Minimal, consistent border radius.

| Token | Value | Use Case |
|-------|-------|----------|
| `--radius-sm` | 4px | Buttons, inputs |
| `--radius-md` | 8px | Cards |
| `--radius-lg` | 12px | Modals, panels |
| `--radius-full` | 9999px | Pills, avatars |

---

## 5. Shadow

Subtle shadows only. No dramatic elevation.

| Token | Value | Use Case |
|-------|-------|----------|
| `--shadow-sm` | 0 1px 2px rgba(0,0,0,0.04) | Subtle depth |
| `--shadow-md` | 0 2px 8px rgba(0,0,0,0.06) | Cards |
| `--shadow-lg` | 0 4px 16px rgba(0,0,0,0.08) | Modals, dropdowns |

---

## 6. Motion

Motion tokens are defined for consistency, even if not all are used on web.

| Token | Value | Use Case |
|-------|-------|----------|
| `--duration-instant` | 0ms | Immediate feedback |
| `--duration-fast` | 100ms | Micro-interactions |
| `--duration-normal` | 200ms | Standard transitions |
| `--duration-slow` | 300ms | Modal open/close |
| `--easing-default` | cubic-bezier(0.4, 0, 0.2, 1) | General purpose |
| `--easing-enter` | cubic-bezier(0, 0, 0.2, 1) | Elements entering |
| `--easing-exit` | cubic-bezier(0.4, 0, 1, 1) | Elements leaving |

---

## 7. Breakpoints

Mobile-first responsive design.

| Token | Value | Use Case |
|-------|-------|----------|
| `--breakpoint-sm` | 640px | Small tablets |
| `--breakpoint-md` | 768px | Tablets |
| `--breakpoint-lg` | 1024px | Laptops |
| `--breakpoint-xl` | 1280px | Desktops |

---

## 8. Z-Index Scale

Predictable layering.

| Token | Value | Use Case |
|-------|-------|----------|
| `--z-base` | 0 | Default |
| `--z-raised` | 10 | Sticky headers |
| `--z-dropdown` | 100 | Dropdowns, popovers |
| `--z-modal` | 200 | Modals, dialogs |
| `--z-toast` | 300 | Toast notifications |

---

## 9. Component-Specific Tokens

### Cards

| Token | Value |
|-------|-------|
| `--card-padding` | var(--space-4) |
| `--card-radius` | var(--radius-md) |
| `--card-shadow` | var(--shadow-md) |
| `--card-border` | 1px solid var(--color-border-subtle) |

### Buttons

| Token | Value |
|-------|-------|
| `--button-padding-x` | var(--space-4) |
| `--button-padding-y` | var(--space-2) |
| `--button-radius` | var(--radius-sm) |
| `--button-font-size` | var(--text-sm) |
| `--button-font-weight` | var(--font-medium) |

### Inputs

| Token | Value |
|-------|-------|
| `--input-padding-x` | var(--space-3) |
| `--input-padding-y` | var(--space-2) |
| `--input-radius` | var(--radius-sm) |
| `--input-border` | 1px solid var(--color-border) |
| `--input-focus-ring` | 0 0 0 2px var(--color-focus) |

---

## 10. Implementation

### CSS Custom Properties

All tokens are implemented as CSS custom properties in `tokens.css`.

```css
:root {
  --text-base: 15px;
  --space-4: 16px;
  --color-bg: #FAFAFA;
  /* ... */
}
```

### Usage Rules

1. **Never use raw values** — Always reference tokens
2. **Never create new tokens inline** — Add to tokens.css
3. **Never override tokens per-component** — Use semantic tokens instead

### Valid Usage

```css
/* CORRECT */
.card {
  padding: var(--space-4);
  background: var(--color-surface);
  border-radius: var(--radius-md);
}

/* INCORRECT */
.card {
  padding: 16px;
  background: #FFFFFF;
  border-radius: 8px;
}
```

---

## 11. Dark Mode

Dark mode is achieved by redefining tokens, not by adding new styles.

```css
@media (prefers-color-scheme: dark) {
  :root {
    --color-bg: #121212;
    --color-surface: #1E1E1E;
    --color-text-primary: #EBEBEB;
    /* ... */
  }
}
```

---

## Changelog

| Version | Date | Changes |
|---------|------|---------|
| v1 | 2025-01-15 | Initial canonical version |
