# iOS UI Mapping v1

**Status**: Canonical
**Phase**: 18
**Last Updated**: 2025-01-15

---

## Purpose

This document maps QuantumLife design tokens and components to SwiftUI equivalents. The goal is 1:1 portability: the same design system expressed in a different rendering surface.

**No iOS code is written yet.** This document ensures the web implementation uses patterns that translate directly to SwiftUI.

---

## 1. Typography Mapping

### Fonts

| Token | CSS Value | SwiftUI Equivalent |
|-------|-----------|-------------------|
| `--font-sans` | System fonts | `.body` / Font.system() |
| `--font-mono` | System mono | `.monospaced` |

### Type Scale

| Token | CSS | SwiftUI |
|-------|-----|---------|
| `--text-xs` | 11px | `.caption2` |
| `--text-sm` | 13px | `.caption` / `.footnote` |
| `--text-base` | 15px | `.body` |
| `--text-lg` | 17px | `.body` with weight |
| `--text-xl` | 21px | `.title3` |
| `--text-2xl` | 28px | `.title2` |
| `--text-3xl` | 36px | `.largeTitle` |

### Font Weight

| Token | CSS | SwiftUI |
|-------|-----|---------|
| `--font-normal` | 400 | `.regular` |
| `--font-medium` | 500 | `.medium` |
| `--font-semibold` | 600 | `.semibold` |

---

## 2. Spacing Mapping

| Token | Value | SwiftUI |
|-------|-------|---------|
| `--space-1` | 4pt | 4 |
| `--space-2` | 8pt | 8 |
| `--space-3` | 12pt | 12 |
| `--space-4` | 16pt | 16 |
| `--space-6` | 24pt | 24 |
| `--space-8` | 32pt | 32 |
| `--space-12` | 48pt | 48 |
| `--space-16` | 64pt | 64 |

SwiftUI uses points (pt), matching our pixel values at 1x scale.

---

## 3. Color Mapping

### Core Colors

| Token | SwiftUI (Light) | SwiftUI (Dark) |
|-------|-----------------|----------------|
| `--color-bg` | Color(UIColor.systemGroupedBackground) | Auto |
| `--color-surface` | Color(UIColor.systemBackground) | Auto |
| `--color-text-primary` | .primary | Auto |
| `--color-text-secondary` | .secondary | Auto |
| `--color-text-tertiary` | Color(UIColor.tertiaryLabel) | Auto |
| `--color-border` | Color(UIColor.separator) | Auto |

### Custom Colors

For colors without system equivalents, define in Asset Catalog:

```swift
extension Color {
    static let levelAmbient = Color("LevelAmbient")
    static let levelNeedsYou = Color("LevelNeedsYou")
    static let levelUrgent = Color("LevelUrgent")
}
```

---

## 4. Radius Mapping

| Token | Value | SwiftUI |
|-------|-------|---------|
| `--radius-sm` | 4pt | .cornerRadius(4) |
| `--radius-md` | 8pt | .cornerRadius(8) |
| `--radius-lg` | 12pt | .cornerRadius(12) |
| `--radius-full` | 9999pt | .clipShape(Capsule()) |

---

## 5. Shadow Mapping

| Token | SwiftUI |
|-------|---------|
| `--shadow-sm` | .shadow(color: .black.opacity(0.04), radius: 1, y: 1) |
| `--shadow-md` | .shadow(color: .black.opacity(0.06), radius: 4, y: 2) |
| `--shadow-lg` | .shadow(color: .black.opacity(0.08), radius: 8, y: 4) |

---

## 6. Motion Mapping

| Token | SwiftUI |
|-------|---------|
| `--duration-fast` | .easeOut(duration: 0.1) |
| `--duration-normal` | .easeInOut(duration: 0.2) |
| `--duration-slow` | .easeInOut(duration: 0.3) |
| `--easing-default` | .easeInOut |
| `--easing-enter` | .easeOut |
| `--easing-exit` | .easeIn |

---

## 7. Component Mapping

### CircleCard

**Web (HTML)**:
```html
<div class="circle-card">
  <div class="circle-card-title">Home</div>
  <div class="circle-card-meta">3 needs you</div>
</div>
```

**SwiftUI**:
```swift
struct CircleCard: View {
    let circle: Circle

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(circle.name)
                .font(.body)
                .fontWeight(.semibold)
            Text(circle.needsYouLabel)
                .font(.caption)
                .foregroundColor(.secondary)
        }
        .padding(16)
        .background(Color(UIColor.systemBackground))
        .cornerRadius(8)
        .shadow(color: .black.opacity(0.06), radius: 4, y: 2)
    }
}
```

---

### NeedsYouItem

**Web (HTML)**:
```html
<div class="needs-you-item needs-you-item--urgent">
  <div class="needs-you-item-title">Energy bill payment</div>
  <div class="needs-you-item-meta">Due tomorrow</div>
</div>
```

**SwiftUI**:
```swift
struct NeedsYouItem: View {
    let item: NeedsYouData

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(item.title)
                .font(.body)
            Text(item.meta)
                .font(.caption)
                .foregroundColor(.secondary)
        }
        .padding(12)
        .background(backgroundColor(for: item.level))
        .cornerRadius(8)
    }

    private func backgroundColor(for level: InterruptionLevel) -> Color {
        switch level {
        case .ambient: return Color.levelAmbient
        case .needsYou: return Color.levelNeedsYou
        case .urgent: return Color.levelUrgent
        default: return Color.clear
        }
    }
}
```

---

### DraftCard

**Web (HTML)**:
```html
<div class="draft-card">
  <div class="draft-card-action">Pay £127.50 to British Gas</div>
  <div class="draft-card-meta">Due in 3 days</div>
  <div class="draft-card-actions">
    <button class="btn btn-primary">Approve</button>
    <button class="btn btn-secondary">Reject</button>
  </div>
</div>
```

**SwiftUI**:
```swift
struct DraftCard: View {
    let draft: Draft
    let onApprove: () -> Void
    let onReject: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text(draft.actionDescription)
                .font(.body)
                .fontWeight(.medium)
            Text(draft.metaLabel)
                .font(.caption)
                .foregroundColor(.secondary)
            HStack(spacing: 12) {
                Button("Approve", action: onApprove)
                    .buttonStyle(.borderedProminent)
                Button("Reject", action: onReject)
                    .buttonStyle(.bordered)
            }
        }
        .padding(16)
        .background(Color(UIColor.systemBackground))
        .cornerRadius(8)
        .shadow(color: .black.opacity(0.06), radius: 4, y: 2)
    }
}
```

---

### ExplainPanel

**Web (HTML)**:
```html
<div class="explain-panel">
  <div class="explain-panel-title">Why am I seeing this?</div>
  <p class="explain-panel-body">This needs your approval because it exceeds your auto-approve threshold.</p>
</div>
```

**SwiftUI**:
```swift
struct ExplainPanel: View {
    let explanation: String
    @State private var isExpanded = false

    var body: some View {
        DisclosureGroup("Why am I seeing this?", isExpanded: $isExpanded) {
            Text(explanation)
                .font(.caption)
                .foregroundColor(.secondary)
                .padding(.top, 8)
        }
        .padding(12)
        .background(Color(UIColor.secondarySystemBackground))
        .cornerRadius(8)
    }
}
```

---

### EmptyState

**Web (HTML)**:
```html
<div class="empty-state">
  <div class="empty-state-title">Nothing needs you.</div>
  <div class="empty-state-body">QuantumLife handled 12 items this week.</div>
</div>
```

**SwiftUI**:
```swift
struct EmptyState: View {
    let handledCount: Int

    var body: some View {
        VStack(spacing: 16) {
            Text("Nothing needs you.")
                .font(.title2)
                .fontWeight(.semibold)
            Text("QuantumLife handled \(handledCount) items this week.")
                .font(.body)
                .foregroundColor(.secondary)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .padding(32)
    }
}
```

---

## 8. Layout Patterns

### Screen Container

**Web**:
```html
<main class="container">
  <!-- Content -->
</main>
```

**SwiftUI**:
```swift
struct ScreenContainer<Content: View>: View {
    let content: Content

    var body: some View {
        ScrollView {
            content
                .padding(.horizontal, 16)
                .padding(.vertical, 24)
        }
    }
}
```

### Circles Row

**Web**:
```html
<div class="circles-row">
  <!-- CircleCards -->
</div>
```

**SwiftUI**:
```swift
ScrollView(.horizontal, showsIndicators: false) {
    HStack(spacing: 12) {
        ForEach(circles) { circle in
            CircleCard(circle: circle)
        }
    }
    .padding(.horizontal, 16)
}
```

---

## 9. Navigation Mapping

| Web Pattern | SwiftUI |
|-------------|---------|
| Tab bar | TabView |
| Push navigation | NavigationStack |
| Modal | .sheet() |
| Dropdown | Menu |

---

## 10. Accessibility Mapping

| Web | SwiftUI |
|-----|---------|
| `aria-label` | .accessibilityLabel() |
| `aria-hidden` | .accessibilityHidden() |
| `role="button"` | Button (automatic) |
| Focus ring | .focused() |

---

## 11. Implementation Notes

### When Building iOS

1. Create Asset Catalog with custom colors
2. Create TokensEnum.swift with spacing/sizing constants
3. Create Components/ folder mirroring web components
4. Use SwiftUI previews for parity testing

### Parity Checklist

For each component:
- [ ] Same visual appearance
- [ ] Same spacing
- [ ] Same typography
- [ ] Same interaction model
- [ ] Same accessibility
- [ ] Same dark mode behavior

---

## Changelog

| Version | Date | Changes |
|---------|------|---------|
| v1 | 2025-01-15 | Initial canonical version |
