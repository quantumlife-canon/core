# ADR-0029: Phase 13.1 - Identity-Driven Routing + People UI (Web)

## Status

Accepted

## Context

Phase 13 established the identity graph with Person, Organization, Household, and PhoneNumber entities. Phase 13.1 extends this to:
1. Drive routing decisions using identity graph data
2. Provide deterministic query helpers for UI rendering
3. Add People UI to the web interface

## Decision

### 1. IdentityRepository Query Helpers

Extended `pkg/domain/identity/repository.go` with deterministic ordering:

```go
// ListPersons returns all persons sorted by EntityID
func (r *InMemoryRepository) ListPersons() []*Person

// ListOrganizations returns all orgs sorted by EntityID
func (r *InMemoryRepository) ListOrganizations() []*Organization

// ListHouseholds returns all households sorted by EntityID
func (r *InMemoryRepository) ListHouseholds() []*Household

// GetPersonEdgesSorted returns edges sorted by EdgeType, then ToID
func (r *InMemoryRepository) GetPersonEdgesSorted(personID EntityID) []*Edge

// GetAllEdgesSorted returns all edges in deterministic order
func (r *InMemoryRepository) GetAllEdgesSorted() []*Edge
```

### 2. Display Helpers

```go
// PrimaryEmail returns the primary email for a person
func (r *InMemoryRepository) PrimaryEmail(personID EntityID) string

// PersonLabel returns display label (DisplayName, then email local-part)
func (r *InMemoryRepository) PersonLabel(personID EntityID) string

// IsHouseholdMember checks if person belongs to any household
func (r *InMemoryRepository) IsHouseholdMember(personID EntityID) bool

// GetPersonHouseholds returns households a person belongs to
func (r *InMemoryRepository) GetPersonHouseholds(personID EntityID) []*Household

// GetPersonOrganizations returns orgs a person works at
func (r *InMemoryRepository) GetPersonOrganizations(personID EntityID) []*Organization
```

### 3. Identity-Based Routing Precedence

Router extended with identity graph support (`internal/routing/router.go`):

```go
type IdentityRouter interface {
    FindPersonByEmail(email string) (*Person, error)
    FindOrganizationByDomain(domain string) (*Organization, error)
    IsHouseholdMember(personID EntityID) bool
    GetPersonOrganizations(personID EntityID) []*Organization
}

func (r *Router) SetIdentityRepository(repo IdentityRouter)
```

**Routing Precedence (P1-P5)**:
- **P1**: Receiver email bound to circle integration → that circle
- **P2**: Sender resolves to PersonID in Household → family circle
- **P3**: Sender resolves to works_at OrgID in work_domains → work circle
- **P4**: Sender domain in personal_domains → personal circle
- **P5**: Fallback → default circle

Each method has a `WithReason` variant for audit:
```go
func (r *Router) RouteEmailToCircleWithReason(event *events.EmailMessageEvent) (EntityID, string)
```

### 4. Loop Integration

MultiCircleRunner extended with identity components:

```go
type MultiCircleRunner struct {
    // ... existing fields
    IdentityRepo     *identity.InMemoryRepository
    IdentityResolver *identityresolve.Resolver
}

func (r *MultiCircleRunner) WithIdentity(repo, resolver) *MultiCircleRunner
```

**MultiCircleRunResult** includes:
- `IdentityGraphHash`: Deterministic hash of identity graph state
- `IdentityStats`: Entity and edge counts

**Identity Graph Hash** computed from:
- Sorted persons (by ID, with PrimaryEmail)
- Sorted organizations (by ID, with Domain)
- Sorted households (by ID, with Name)
- Sorted edges (by EdgeType, then ToID)

### 5. Web UI

**Routes added** (`cmd/quantumlife-web/main.go`):
- `GET /people` - List all persons in deterministic order
- `GET /people/:id` - Show person details with edges

**Navigation** updated to include "People" link.

**Template data** extended with:
```go
type personInfo struct {
    ID             string
    Label          string
    PrimaryEmail   string
    IsVIP          bool
    IsHousehold    bool
    EdgeCount      int
    Organizations  []string
    Households     []string
}
```

### 6. Guardrails

`scripts/guardrails/identity_routing_web_enforced.sh` verifies:
1. Query helpers with deterministic ordering
2. Display helpers (PrimaryEmail, PersonLabel)
3. Routing with identity precedence (P1-P5)
4. Loop with identity graph hash
5. Web UI /people routes
6. No goroutines in Phase 13.1 packages
7. Demo tests exist

## Consequences

### Positive

- Routing decisions now consider relationship context
- Household members auto-route to family circle
- Work colleagues auto-route to work circle
- Deterministic ordering enables stable UI and hashing
- Identity graph hash enables replay verification

### Negative

- Additional lookup overhead for routing decisions
- Identity repo must be connected to router manually

### Neutral

- Sorting uses bubble sort (stdlib only, no sort package import)
- VIP tagging via config, not identity graph

## Implementation

### Package Structure

```
pkg/domain/identity/
  repository.go       # Extended with query helpers + display helpers

internal/routing/
  router.go           # Extended with IdentityRouter interface

internal/loop/
  multi_circle.go     # Extended with identity integration

cmd/quantumlife-web/
  main.go             # Extended with /people routes

internal/demo_phase13_1_identity_routing_web/
  demo_test.go        # Demonstration tests

scripts/guardrails/
  identity_routing_web_enforced.sh  # Phase 13.1 guardrail
```

### Makefile Targets

- `make demo-phase13-1` - Run Phase 13.1 demo tests
- `make check-identity-routing-web` - Run Phase 13.1 guardrail

## References

- Canon v1: "QuantumLife never executes without explicit human approval"
- Phase 13: Identity + Contact Graph Unification (ADR-0028)
- Phase 11: Multi-Circle Ingestion (ADR-0026)

## Checklist

- [x] IdentityRepository query helpers with deterministic ordering
- [x] Display helpers (PrimaryEmail, PersonLabel)
- [x] IdentityRouter interface for routing
- [x] Routing precedence rules (P1-P5)
- [x] WithReason variants for audit
- [x] MultiCircleRunner identity integration
- [x] IdentityGraphHash computation
- [x] IdentityStats in run result
- [x] Web UI /people route
- [x] Web UI /people/:id route
- [x] Navigation link
- [x] No goroutines
- [x] Demo tests
- [x] Guardrail script
- [x] Makefile targets
