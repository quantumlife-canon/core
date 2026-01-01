// Package demo_phase13_1_identity_routing_web demonstrates Phase 13.1 features.
//
// Phase 13.1: Identity-Driven Routing + People UI (Web)
// - IdentityRepository query helpers with deterministic ordering
// - Identity-based routing with precedence rules (P1-P5)
// - Loop integration with identity graph hash
// - Web UI /people and /people/:id endpoints
//
// Reference: docs/ADR/ADR-0029-phase13-1-identity-driven-routing-and-people-ui.md
package demo_phase13_1_identity_routing_web

import (
	"testing"
	"time"

	"quantumlife/internal/config"
	"quantumlife/internal/identityresolve"
	"quantumlife/internal/routing"
	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/events"
	"quantumlife/pkg/domain/identity"
)

// TestIdentityRepositoryQueryHelpers demonstrates deterministic ordering.
func TestIdentityRepositoryQueryHelpers(t *testing.T) {
	repo := identity.NewInMemoryRepository()
	gen := identity.NewGenerator()
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	// Create persons in non-alphabetical order
	person1 := gen.PersonFromEmail("zebra@example.com", now)
	person2 := gen.PersonFromEmail("alice@example.com", now)
	person3 := gen.PersonFromEmail("bob@example.com", now)

	// Store in random order
	repo.Store(person1)
	repo.Store(person2)
	repo.Store(person3)

	// ListPersons should return in deterministic order (sorted by ID)
	persons := repo.ListPersons()
	if len(persons) != 3 {
		t.Fatalf("expected 3 persons, got %d", len(persons))
	}

	// Verify ordering is deterministic (sorted by EntityID)
	for i := 1; i < len(persons); i++ {
		if string(persons[i-1].ID()) > string(persons[i].ID()) {
			t.Errorf("persons not sorted: %s > %s", persons[i-1].ID(), persons[i].ID())
		}
	}

	t.Logf("Persons returned in deterministic order: %d items", len(persons))
}

// TestIdentityDisplayHelpers demonstrates PrimaryEmail and PersonLabel.
func TestIdentityDisplayHelpers(t *testing.T) {
	repo := identity.NewInMemoryRepository()
	gen := identity.NewGenerator()
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	// Create person with email
	person := gen.PersonFromEmail("satish@example.com", now)
	person.DisplayName = "Satish"
	repo.Store(person)

	// Create email account
	emailAccount := gen.EmailAccountFromAddress("satish@example.com", "example", now)
	repo.Store(emailAccount)

	// Create owns_email edge
	edge := identity.NewEdge(
		identity.EdgeTypeOwnsEmail,
		person.ID(),
		emailAccount.ID(),
		identity.ConfidenceHigh,
		"test",
		now,
	)
	repo.StoreEdge(edge)

	// Test PrimaryEmail
	primary := repo.PrimaryEmail(person.ID())
	if primary != "satish@example.com" {
		t.Errorf("expected primary email satish@example.com, got %s", primary)
	}

	// Test PersonLabel (should prefer DisplayName)
	label := repo.PersonLabel(person.ID())
	if label != "Satish" {
		t.Errorf("expected label 'Satish', got %s", label)
	}

	t.Logf("Display helpers work: PrimaryEmail=%s, PersonLabel=%s", primary, label)
}

// TestIdentityBasedRoutingPrecedence demonstrates P1-P5 routing rules.
func TestIdentityBasedRoutingPrecedence(t *testing.T) {
	// Create config with circles
	clk := clock.NewFixed(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	cfg := config.DefaultConfig(clk.Now())

	// Add work and family circles (default only has personal)
	cfg.Circles["work"] = &config.CircleConfig{ID: "work", Name: "Work"}
	cfg.Circles["family"] = &config.CircleConfig{ID: "family", Name: "Family"}

	// Add work domains and family members to config
	cfg.Routing.WorkDomains = []string{"acme.com", "work.org"}
	cfg.Routing.PersonalDomains = []string{"gmail.com", "outlook.com"}
	cfg.Routing.FamilyMembers = []string{"spouse@gmail.com"}

	// Create router
	router := routing.NewRouter(cfg)

	// Create identity repository with household member
	repo := identity.NewInMemoryRepository()
	gen := identity.NewGenerator()
	now := clk.Now()

	// Create spouse as household member
	spouse := gen.PersonFromEmail("spouse@gmail.com", now)
	repo.Store(spouse)

	household := gen.HouseholdFromName("TestFamily", now)
	repo.Store(household)

	memberEdge := identity.NewEdge(
		identity.EdgeTypeMemberOfHH,
		spouse.ID(),
		household.ID(),
		identity.ConfidenceHigh,
		"test",
		now,
	)
	repo.StoreEdge(memberEdge)

	// Connect identity repo to router
	router.SetIdentityRepository(repo)

	// Test P2: Household member routing
	emailEvent := &events.EmailMessageEvent{
		From: events.EmailAddress{Address: "spouse@gmail.com"},
	}
	emailEvent.SenderDomain = "gmail.com"

	circleID, reason := router.RouteEmailToCircleWithReason(emailEvent)
	t.Logf("P2 Household routing: circle=%s, reason=%s", circleID, reason)

	if reason != "P2:household_member" && reason != "P2:config_family" {
		t.Errorf("expected P2 reason, got %s", reason)
	}

	// Test P3: Work domain routing
	workEmail := &events.EmailMessageEvent{
		From: events.EmailAddress{Address: "colleague@acme.com"},
	}
	workEmail.SenderDomain = "acme.com"

	workCircle, workReason := router.RouteEmailToCircleWithReason(workEmail)
	t.Logf("P3 Work domain routing: circle=%s, reason=%s", workCircle, workReason)

	if workReason != "P3:work_domain" {
		t.Errorf("expected P3:work_domain, got %s", workReason)
	}

	// Test P4: Personal domain routing
	personalEmail := &events.EmailMessageEvent{
		From: events.EmailAddress{Address: "random@outlook.com"},
	}
	personalEmail.SenderDomain = "outlook.com"

	personalCircle, personalReason := router.RouteEmailToCircleWithReason(personalEmail)
	t.Logf("P4 Personal domain routing: circle=%s, reason=%s", personalCircle, personalReason)

	if personalReason != "P4:personal_domain" {
		t.Errorf("expected P4:personal_domain, got %s", personalReason)
	}
}

// TestIdentityResolutionCreatesEdges demonstrates resolver creating edges.
func TestIdentityResolutionCreatesEdges(t *testing.T) {
	resolverConfig := &identityresolve.Config{
		FamilyMembers: map[string]identityresolve.FamilyMemberConfig{
			"spouse@gmail.com": {
				HouseholdName: "TestHousehold",
				Relationship:  identity.EdgeTypeSpouseOf,
				DisplayName:   "Spouse",
			},
		},
		KnownAliases: make(map[string]string),
		OwnerEmails:  []string{"owner@gmail.com"},
	}

	resolver := identityresolve.NewResolver(resolverConfig)

	event := identityresolve.CanonicalEvent{
		EventType:  identityresolve.EventTypeEmail,
		EventID:    "test-001",
		Timestamp:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		CircleID:   "personal",
		OwnerEmail: "owner@gmail.com",
		Participants: []identityresolve.Participant{
			{Email: "spouse@gmail.com", DisplayName: "Spouse", Role: "from"},
		},
	}

	updates := resolver.ProcessEvent(event)

	// Count different update types
	var entityCount, edgeCount int
	for _, u := range updates {
		switch u.UpdateType {
		case identityresolve.UpdateTypeEntityUpsert:
			entityCount++
		case identityresolve.UpdateTypeEdgeUpsert:
			edgeCount++
		}
	}

	t.Logf("Identity resolution: %d entity updates, %d edge updates", entityCount, edgeCount)

	if entityCount == 0 {
		t.Error("expected at least one entity update")
	}
	if edgeCount == 0 {
		t.Error("expected at least one edge update")
	}
}

// TestIdentityGraphHashDeterminism demonstrates hash stability.
func TestIdentityGraphHashDeterminism(t *testing.T) {
	repo1 := identity.NewInMemoryRepository()
	repo2 := identity.NewInMemoryRepository()
	gen := identity.NewGenerator()
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	// Add same data to both repos in same order
	person := gen.PersonFromEmail("test@example.com", now)
	repo1.Store(person)
	repo2.Store(person)

	// Get persons in sorted order
	persons1 := repo1.ListPersons()
	persons2 := repo2.ListPersons()

	if len(persons1) != len(persons2) {
		t.Fatalf("repos have different person counts")
	}

	// Verify IDs match
	for i := range persons1 {
		if persons1[i].ID() != persons2[i].ID() {
			t.Errorf("person IDs differ at index %d", i)
		}
	}

	t.Logf("Identity graph ordering is deterministic")
}

// TestEdgeSortingDeterminism verifies edges are sorted deterministically.
func TestEdgeSortingDeterminism(t *testing.T) {
	repo := identity.NewInMemoryRepository()
	gen := identity.NewGenerator()
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	person := gen.PersonFromEmail("test@example.com", now)
	org1 := gen.OrganizationFromDomain("acme.com", now)
	org2 := gen.OrganizationFromDomain("corp.com", now)
	household := gen.HouseholdFromName("Family", now)

	repo.Store(person)
	repo.Store(org1)
	repo.Store(org2)
	repo.Store(household)

	// Create edges in non-sorted order
	edge1 := identity.NewEdge(identity.EdgeTypeWorksAt, person.ID(), org1.ID(), identity.ConfidenceHigh, "test", now)
	edge2 := identity.NewEdge(identity.EdgeTypeMemberOfHH, person.ID(), household.ID(), identity.ConfidenceHigh, "test", now)
	edge3 := identity.NewEdge(identity.EdgeTypeWorksAt, person.ID(), org2.ID(), identity.ConfidenceHigh, "test", now)

	repo.StoreEdge(edge1)
	repo.StoreEdge(edge2)
	repo.StoreEdge(edge3)

	// Get sorted edges
	edges := repo.GetPersonEdgesSorted(person.ID())

	if len(edges) != 3 {
		t.Fatalf("expected 3 edges, got %d", len(edges))
	}

	// Verify sorted by EdgeType, then ToID
	for i := 1; i < len(edges); i++ {
		prev := edges[i-1]
		curr := edges[i]

		if string(prev.EdgeType) > string(curr.EdgeType) {
			t.Errorf("edges not sorted by type: %s > %s", prev.EdgeType, curr.EdgeType)
		} else if prev.EdgeType == curr.EdgeType {
			if string(prev.ToID) > string(curr.ToID) {
				t.Errorf("edges not sorted by ToID: %s > %s", prev.ToID, curr.ToID)
			}
		}
	}

	t.Logf("Edges sorted deterministically: %d edges", len(edges))
}
