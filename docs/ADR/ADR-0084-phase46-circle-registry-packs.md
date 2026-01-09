# ADR-0084: Phase 46 - Circle Registry + Packs (Marketplace v0)

## Status
Accepted

## Context

Phase 45 introduced Circle Semantics & Necessity Declaration - a meaning-only layer that
explains what kind of circle something is and what urgency model applies. This phase
extends that concept by introducing a "marketplace" of curated Packs that bundle
semantics presets and observer binding intents.

**Core principle**: Packs are meaning-only and observer-intent-only. They MUST NOT
grant permission to SURFACE, INTERRUPT, DELIVER, or EXECUTE.

## Decision

### Invariant: effect_no_power Always

Every pack, binding, and record MUST have `Effect = effect_no_power`. This is the
ONLY valid effect in Phase 46. Packs provide meaning and intent but do NOT grant
permission.

### Invariant: Observer Bindings Are Intent-Only

Observer bindings in packs represent the user's *intent* to bind an observer to
a circle pattern. No actual wiring occurs in Phase 46. These bindings are recorded
for:
1. Documentation/transparency
2. Future phases that may implement actual wiring
3. Proof pages showing what the user has declared

### Invariant: Hash-Only Storage

All stored records use hash-only identifiers:
- `PackSlugHash` - SHA256 hash of pack slug
- `VersionHash` - Hash of pack contents
- `StatusHash` - Hash of record state
- `CirclePatternHash` - Hash of circle pattern

No raw identifiers, no secrets, no personal data in storage.

### Invariant: No Decision Package Imports

This package MUST NOT import from decision-making packages:
- `pressuredecision`
- `interruptpolicy`
- `interruptpreview`
- `execrouter`
- `execexecutor`

Packs are meaning-only. They do not participate in decision-making.

### Invariant: No Goroutines

No goroutines in pkg/ or internal/marketplace. All operations are synchronous.
Clock injection is used instead of time.Now().

### Invariant: Bounded Retention

Stores use FIFO eviction with bounds:
- Maximum 200 records per store
- Maximum 30 days retention
- Oldest records evicted first when bounds exceeded

## Architecture

### Domain Types (pkg/domain/marketplace/)

1. **PackSlug** - URL-safe pack identifier with validation
2. **PackKind** - Category: semantics, observer_binding, combined
3. **PackTier** - Trust tier: curated, community, custom
4. **PackVisibility** - Visibility: public, unlisted, private
5. **PackStatus** - Installation status: available, installed, pending_apply
6. **BindingKind** - Observer binding type: observe_only, annotate, enrich
7. **PackEffect** - MUST be effect_no_power (only valid value)

Structs:
- **SemanticsPreset** - Default semantics for a circle pattern
- **ObserverBinding** - Intent to bind an observer (no real wiring)
- **PackTemplate** - Full pack definition with presets and bindings
- **PackInstallRecord** - Stored installation record
- **PackRemovalRecord** - Stored removal record

UI Models:
- **MarketplaceHomePage** - Home page with available/installed packs
- **PackCard** - Pack summary for lists
- **PackDetailPage** - Full pack detail view
- **MarketplaceProofPage** - Proof of installed/removed packs

### Registry (internal/marketplace/)

Static catalog of available pack templates. Includes curated default packs:
- Family & Friends - Human circle semantics
- Essential Services - Bank/healthcare semantics
- Quiet Marketing - Low-priority service semantics
- Commerce Observer - Observer intent for commerce
- Calendar Awareness - Combined semantics + observer
- Inbox Enrichment - Inbox observer intent

### Engine (internal/marketplace/)

Pure deterministic logic for:
- Building home/detail/proof pages
- Creating install intents and records
- Building removal records
- Computing marketplace cue

All methods are deterministic given same inputs. No time.Now() - clock injected.

### Persistence (internal/persist/)

Three stores with bounded retention:
1. **MarketplaceInstallStore** - Pack installation records
2. **MarketplaceRemovalStore** - Pack removal records
3. **MarketplaceAckStore** - Proof acknowledgments

### Web Routes

| Route | Method | Description |
|-------|--------|-------------|
| /marketplace | GET | Marketplace home |
| /marketplace/pack/{slug} | GET | Pack detail |
| /marketplace/install | POST | Install pack |
| /marketplace/remove | POST | Remove pack |
| /proof/marketplace | GET | Proof page |
| /proof/marketplace/dismiss | POST | Dismiss proof |

### Events

- `phase46.marketplace.home.viewed`
- `phase46.marketplace.pack.detail.viewed`
- `phase46.marketplace.pack.installed`
- `phase46.marketplace.pack.removed`
- `phase46.marketplace.proof.rendered`
- `phase46.marketplace.proof.dismissed`
- `phase46.marketplace.cue.computed`

### Storelog Records

- `PACK_INSTALL` - Pack installed
- `PACK_REMOVAL` - Pack removed
- `MARKETPLACE_ACK` - Proof acknowledged

## Consequences

### Positive

1. Users can browse and install curated packs
2. Semantics and observer intents are bundled logically
3. Full proof/transparency of what is installed
4. Hash-only storage maintains privacy
5. effect_no_power enforced at every level

### Negative

1. Observer bindings are intent-only (no actual wiring yet)
2. Packs cannot grant any permissions
3. No custom pack creation UI (registry-only)

### Neutral

1. Marketplace is read-from-registry only
2. No pack updates/upgrades yet
3. No pack dependencies/conflicts

## Implementation Notes

1. All pack templates are registered in DefaultRegistry()
2. Pack detail page shows all presets and bindings
3. Install/remove are POST-only operations
4. Templates use SlugHash for links (hash-only routing)
5. Proof page shows both installed and removed packs

## Security Considerations

1. No raw identifiers in storage
2. No secrets in pack definitions
3. POST-only for mutations
4. Hash-only URLs for pack detail
5. effect_no_power prevents permission escalation

## References

- Phase 45: ADR-0083-phase45-circle-semantics-necessity.md
- Phase 31: ADR-0065-phase31-commerce-observers.md
- Phase 33: ADR-0067-phase33-interrupt-permission-contract.md
