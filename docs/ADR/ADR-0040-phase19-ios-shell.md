# ADR-0040: Phase 19.0 — iOS Shell (Zero New Concepts)

## Status

Accepted

## Context

QuantumLife has established its core experience through the web interface, demonstrating:
- Abstract-only data presentation (magnitude buckets, horizon buckets)
- Hash-only storage (never raw content)
- Whisper-level interactions (minimal, optional, restrained)
- Mirror proof (evidence of reading without storing)
- Connection consent flow

To extend this experience to mobile devices, we need an iOS application that reproduces the exact same flows and tone without introducing any new product concepts.

## Decision

Implement an iOS shell that mirrors the web experience exactly:

### Architecture

```
ios/QuantumLife/
├── QuantumLife.xcodeproj/
├── QuantumLife/
│   ├── QuantumLifeApp.swift          # App entry point
│   ├── Core/
│   │   ├── Models/Models.swift       # Domain models (mirrors server)
│   │   ├── Mock/SeededData.swift     # Deterministic mock generation
│   │   └── Store/LocalStore.swift    # Hash-only local stores
│   ├── UI/
│   │   ├── Theme/DesignTokens.swift  # Design tokens from CSS
│   │   ├── Components/Components.swift
│   │   └── Screens/
│   │       ├── LandingScreen.swift
│   │       ├── TodayScreen.swift
│   │       ├── HeldScreen.swift
│   │       ├── SurfaceScreen.swift
│   │       ├── ProofScreen.swift
│   │       ├── StartScreen.swift
│   │       ├── ConnectionsScreen.swift
│   │       └── MirrorScreen.swift
│   └── Assets.xcassets/
└── QuantumLifeTests/
    └── DeterminismTests.swift
```

### Key Constraints

1. **Zero New Concepts**: No new screens, flows, or behaviors not in web
2. **Design Token Portability**: Export tokens.css to Swift for identical styling
3. **Deterministic Mocks**: Same seed produces identical output (SHA256-based)
4. **Hash-Only Storage**: Never store raw content, only hashes
5. **Abstract-Only Display**: Magnitude buckets, horizon buckets, no raw counts

### Design Token Mapping

| CSS | Swift |
|-----|-------|
| `--text-sm` | `DesignTokens.Typography.textSM` |
| `--space-4` | `DesignTokens.Spacing.space4` |
| `--adaptive-text-primary` | `DesignTokens.Colors.adaptiveTextPrimary` |
| `--radius-sm` | `DesignTokens.Radius.sm` |

### Screens Implemented

1. **Landing** - Moments (arrival, recognition, promise, permission)
2. **Today** - Quiet greeting with whisper cue
3. **Held** - Abstract summary of what's being watched
4. **Surface** - Single item with explain/hold actions
5. **Proof** - Evidence of restraint
6. **Start** - Consent flow
7. **Connections** - Mock connection management
8. **Mirror** - Abstract evidence of reading

### Determinism Requirements

```swift
// Same seed MUST produce identical output
let gen1 = SeededDataGenerator(seed: "test")
let gen2 = SeededDataGenerator(seed: "test")
assert(gen1.generateTodayPage().hash256() == gen2.generateTodayPage().hash256())
```

### Local Storage Pattern

```swift
// Hash-only storage
func recordHold(itemKeyHash: String) {
    let canonical = "HOLD|\(itemKeyHash)|\(timestamp)"
    let hash = SHA256.hash(data: Data(canonical.utf8))
    // Store hash, never itemKeyHash content
}
```

## Consequences

### Benefits

1. **Consistent Experience**: iOS users get exact same flows as web
2. **Tone Preservation**: Whisper-level restraint maintained on mobile
3. **Testable Determinism**: Same seed = same output, verifiable
4. **Privacy by Design**: Hash-only storage pattern enforced
5. **No Concept Drift**: Web and iOS stay in sync

### Constraints

1. iOS 17.0+ required (SwiftUI latest features)
2. No real OAuth (mock connections only)
3. No networking (seeded data only)
4. No push notifications (contrary to philosophy)

## Implementation Notes

### Navigation

State-binding navigation (not NavigationStack) for simplicity:

```swift
@State private var currentDestination: AppDestination? = .landing

switch currentDestination {
case .today: TodayScreen(navigateTo: $currentDestination)
case .held: HeldScreen(navigateTo: $currentDestination)
// ...
}
```

### Dark Mode Support

Automatic via adaptive colors:

```swift
static var adaptiveTextPrimary: Color {
    Color(UIColor { traits in
        traits.userInterfaceStyle == .dark ? darkTextPrimary : lightTextPrimary
    })
}
```

### Test Strategy

1. **Determinism Tests**: Verify same seed = same output
2. **Hash Stability Tests**: Verify hash format and consistency
3. **Abstract-Only Tests**: Verify no identifiers leak through
4. **Single Whisper Tests**: Verify at most one cue

## Related

- ADR-0033: Phase 13 — Today, Quietly
- ADR-0035: Phase 15 — Household Approvals
- ADR-0037: Phase 17 — Finance Execution Boundary
- ADR-0039: Phase 18.7 — Mirror Proof

## References

- [Apple Human Interface Guidelines](https://developer.apple.com/design/human-interface-guidelines/)
- [SwiftUI Documentation](https://developer.apple.com/documentation/swiftui/)
