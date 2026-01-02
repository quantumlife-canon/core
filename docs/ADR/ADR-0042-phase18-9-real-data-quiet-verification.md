# ADR-0042: Phase 18.9 Real Data Quiet Verification

## Status

Accepted

## Context

Phase 18.8 implemented real Gmail OAuth with read-only scopes and deterministic receipts. Before proceeding with additional data sources, we need to verify that real Gmail data integrates quietly without breaking the core promise: "Nothing needs you — unless it truly does."

This is a VERIFICATION phase, not a feature phase. The goal is to prove the quiet loop remains calm when processing real external data.

### Key Questions to Answer

1. Does real data feel quiet? Do syncs stay invisible unless explicitly shown?
2. Does the user trust what we read, and believe what we refuse to store?
3. Does Today remain quiet after a real sync completes?
4. Do Gmail-derived obligations respect the hold-by-default principle?

## Decision

We implement Phase 18.9 as a verification and restraint phase with the following components:

### 1. Gmail Consent Page with Restraint-First Copy

The consent page (`/connect/gmail`) displays clear restraint messaging before OAuth:

**Header:**
- "Read-only. Revocable. Nothing stored."

**Three Promise Sections:**

| Section | What we DO | What we DON'T |
|---------|-----------|---------------|
| What we read | Message headers only. Sender domains, timestamps, labels. | Not: email bodies, attachments, contact details. |
| What we store | Hashes, buckets, derived signals. Abstract shapes. | Not: subject lines, sender names, message content. |
| What we never do | No auto-sync. No background polling. Only when you ask. | Revoke anytime. Immediate effect. No trace remains. |

### 2. Sync Results Display Only Magnitude Buckets

The `MagnitudeBucket` function converts raw counts to abstract buckets:

| Count Range | Bucket |
|-------------|--------|
| 0 | "none" |
| 1-5 | "handful" |
| 6-20 | "several" |
| 21+ | "many" |

CRITICAL: Raw counts are NEVER exposed in UI. Only buckets.

### 3. Gmail Obligation Restraint Rules (`internal/obligations/rules_gmail.go`)

```go
type GmailRestraintConfig struct {
    DefaultToHold          bool    // Always true
    MaxDailyObligations    int     // Capped at 3
    StalenessThresholdDays int     // Ignore messages > 3 days old
    BaseRegret             float64 // 0.15 (very low)
    MaxRegret              float64 // 0.35 (below surface threshold)
    RequireExplicitAction  bool    // Always true
}
```

Key constraints:
- `MaxRegret` (0.35) is ALWAYS below surface threshold (0.5)
- This ensures no Gmail obligation ever auto-surfaces
- All obligations require explicit user action to surface
- Stale messages (> 3 days) are ignored
- At most 3 obligations per day can be created

### 4. Abstract-Only Evidence

Gmail obligations store only abstract evidence:

| Key | Example Value | Description |
|-----|---------------|-------------|
| `domain_bucket` | "personal" | Abstract sender category |
| `label_bucket` | "important" | Abstract label category |
| `requires_explicit_action` | "true" | Flags need for user action |

NEVER stored: sender, subject, body, name, email address

### 5. Mirror Page Restraint Language

Mirror shows what was NOT stored with reassuring copy:

```
Source: email
- ReadSuccessfully: true
- NotStored: [messages, senders, subjects]
- Observed: a few messages (recently)
```

Restraint statements:
- "We chose not to interrupt you."
- "Quiet is a feature, not a gap."

### 6. Today Page Stays Quiet

After a Gmail sync completes, the Today page:
- Uses the single-whisper rule (at most ONE cue)
- Shows surface cue OR proof cue, never both
- Does NOT auto-show new Gmail content
- Remains calm and quiet

### 7. Disconnection Confirmation

The `/gmail-disconnected` page reassures:
- "Nothing further is read."
- "Access was revoked with Google."
- "Local tokens removed."

## Guardrail Script

`scripts/guardrails/quiet_verification_enforced.sh` validates:

1. Gmail consent route exists
2. Restraint tagline present
3. "Not stored" explanations present
4. MagnitudeBucket function exists
5. Gmail rules file exists
6. DefaultToHold = true
7. RequireExplicitAction = true
8. MaxRegret < 0.5 (surface threshold)
9. No sender/subject/body stored
10. Abstract buckets used
11. MaxDailyObligations < 10
12. Demo tests exist and pass
13. Validate function exists
14. No goroutines
15. Revocation handler exists
16. Disconnect confirmation exists
17. Mirror shows NotStored for email
18. ShouldHold returns true
19. No auto-surface logic

## Demo Tests

`internal/demo_phase18_9_quiet_verification/demo_test.go` verifies:

1. `TestGmailRestraintPolicyValidation` - Default policy maintains quietness
2. `TestGmailObligationsDefaultToHold` - All obligations are held
3. `TestMirrorShowsNotStoredReassurance` - Mirror displays "not stored" items
4. `TestSyncReceiptsUseMagnitudeBucketsOnly` - No raw counts exposed
5. `TestMirrorOutcomeUsesQuietLanguage` - Calm, reassuring language
6. `TestDailyObligationCap` - Max obligations per day enforced
7. `TestStaleMessagesIgnored` - Old messages don't create obligations
8. `TestAbstractOnlyEvidence` - No identifiable information stored

## Consequences

### Positive

1. Real Gmail data integrates without breaking quiet promise
2. Users see exactly what is/isn't stored before connecting
3. Obligations default to HOLD, never auto-surface
4. Abstract buckets prevent information leakage
5. Revocation is immediate and reassuring
6. Guardrail script catches violations before merge

### Negative

1. More complex consent flow may slow user onboarding
2. Conservative restraint means some "important" emails may not surface
3. Abstract buckets hide detail that some users might want

### Neutral

1. This is a verification phase - no new capabilities added
2. Establishes pattern for future data source integrations
3. Proves quiet loop works with real external data

## Verification Checklist

- [ ] User connects Gmail at `/connect/gmail`
- [ ] Consent page clearly shows "Read-only. Revocable. Nothing stored."
- [ ] OAuth flow completes with `gmail.readonly` scope only
- [ ] Sync button triggers explicit sync (no auto-sync)
- [ ] Sync results show magnitude bucket ("several messages noticed")
- [ ] Mirror shows what was NOT stored (messages, senders, subjects)
- [ ] Today page remains quiet after sync
- [ ] No Gmail content auto-surfaces
- [ ] Revocation removes access immediately
- [ ] Disconnect confirmation is reassuring

## Related ADRs

- ADR-0041: Phase 18.8 Real OAuth (Gmail Read-Only)
- ADR-0039: Phase 18.7 Mirror Proof
- ADR-0038: Phase 18.6 First Connect
- ADR-0036: Phase 18.3 Held Quietly
