# ADR-0028: Phase 13 - Identity + Contact Graph Unification

## Status

Accepted

## Context

QuantumLife ingests data from 20+ email accounts, multiple calendars, and various commerce sources. The same person (e.g., "Alice") may appear across multiple accounts with different email addresses, display names, or phone numbers. Without identity unification, QuantumLife cannot provide a coherent view of relationships.

This ADR defines the identity graph that enables:
1. Deterministic identity resolution across all data sources
2. Family/household relationship tracking
3. Organization/vendor identification from email domains and merchant names
4. Persistence and replay of identity data

## Decision

### 1. Identity Entity Types

Extended entity types in `pkg/domain/identity/`:

| Entity Type | Description | Canonical String Format |
|-------------|-------------|------------------------|
| Person | A human being | `person:email:{normalized_email}` |
| Organization | A company/institution | `organization:domain:{domain}` or `organization:merchant:{name}` |
| EmailAccount | An email address | `email_account:{normalized_email}` |
| PhoneNumber | A phone number | `phone_number|{e164_number}` |
| Household | A family unit | `household|{name}` |

### 2. Edge Types

Relationships between entities:

| Edge Type | From | To | Description |
|-----------|------|-----|-------------|
| owns_email | Person | EmailAccount | Person owns this email |
| owns_phone | Person | PhoneNumber | Person owns this phone |
| works_at | Person | Organization | Person works at org |
| member_of_org | Person | Organization | Person is member of org |
| spouse_of | Person | Person | Spousal relationship |
| parent_of | Person | Person | Parent-child relationship |
| child_of | Person | Person | Child-parent relationship |
| member_of_hh | Person | Household | Person is household member |
| vendor_of | Organization | Person | Org is vendor to person |
| alias_of | Entity | Entity | Same real-world entity |

### 3. Confidence Levels

All edges include a confidence level:

```go
type Confidence string

const (
    ConfidenceHigh   Confidence = "high"   // Explicit config or exact match
    ConfidenceMedium Confidence = "medium" // Strong heuristic match
    ConfidenceLow    Confidence = "low"    // Weak heuristic match
)
```

### 4. Identity Resolution Engine

The `internal/identityresolve/` package provides rule-based (NOT ML) identity resolution:

```go
type Resolver struct {
    generator *identity.Generator
    config    *Config
}

func (r *Resolver) ProcessEvent(event CanonicalEvent) []IdentityUpdate
```

Resolution rules:
1. **Exact email match** → same person (high confidence)
2. **Gmail normalization** → dots and plus-addressing removed
3. **Work email domain** → works_at organization
4. **Config-based family** → explicit household membership
5. **Merchant name** → organization from commerce events

### 5. Persistence Integration

New record types in storelog:

```go
const (
    RecordTypeIdentityEntity = "IDENTITY_ENTITY_UPSERT"
    RecordTypeIdentityEdge   = "IDENTITY_EDGE_UPSERT"
)
```

Identity store in `internal/persist/`:
- Supports replay from log on startup
- Maintains in-memory indexes for fast lookup
- Provides stats (entity counts, edge counts)

### 6. Canonical String Format

All canonical strings use **pipe-delimited format** (NOT JSON):

```
edge|works_at|person_abc123|organization_def456
person|person_abc123|person:email:satish@gmail.com|satish@gmail.com|Satish|
```

This ensures:
- Deterministic serialization
- Human readability
- No JSON encoding overhead

### 7. Guardrails Enforced

The `identity_graph_enforced.sh` script verifies:
1. Identity entity types defined
2. Edge types defined
3. Identity resolution engine exists
4. Persistence with replay
5. No goroutines
6. No time.Now()
7. Pipe-delimited canonical strings (NOT JSON)
8. Confidence enum defined
9. Demo tests exist

## Consequences

### Positive

- Unified view of people across 20+ accounts
- Family relationships explicitly modeled
- Vendor/organization tracking from commerce data
- Deterministic, replayable identity resolution
- Config-based family member identification

### Negative

- Identity graph grows with each ingested event
- Manual configuration needed for family relationships
- No ML-based fuzzy matching (by design)

### Neutral

- Identity resolution runs synchronously with event processing
- Merge operations require careful handling to preserve provenance

## Implementation

### Package Structure

```
pkg/domain/identity/
  types.go          # EntityType, EdgeType, Edge, Confidence
  generator.go      # Entity factory methods, normalization
  repository.go     # Repository interfaces, InMemoryRepository

internal/identityresolve/
  resolver.go       # Resolver, CanonicalEvent, IdentityUpdate
  resolver_test.go

internal/persist/
  identity_store.go # Persistent identity storage with replay

internal/demo_phase13_identity_graph/
  demo_test.go      # Demonstration tests
```

### Configuration

Family members configured via:

```go
type Config struct {
    FamilyMembers map[string]FamilyMemberConfig
    KnownAliases  map[string]string
    OwnerEmails   []string
}
```

### Gmail Normalization

Gmail addresses are normalized by:
1. Removing dots from local part
2. Removing plus-addressing
3. Converting googlemail.com to gmail.com

Example: `sa.ti.sh+work@gmail.com` → `satish@gmail.com`

## References

- Canon v1: "QuantumLife never executes without explicit human approval"
- Phase 12: Persistence + Deterministic Replay
- Phase 11: Multi-Circle Ingestion

## Checklist

- [x] Identity entity types (Person, Organization, EmailAccount, PhoneNumber, Household)
- [x] Edge types (owns_email, works_at, spouse_of, parent_of, member_of_hh)
- [x] Confidence levels (high, medium, low)
- [x] Identity resolution engine with rule-based matching
- [x] Persistence integration with storelog
- [x] Replay support for identity store
- [x] No goroutines (synchronous only)
- [x] No time.Now() (injected clock)
- [x] Pipe-delimited canonical strings
- [x] Demo tests passing
- [x] Guardrail script
- [x] Makefile targets
