// Package demo_phase13_identity_graph demonstrates the identity graph functionality.
//
// Phase 13: Identity + Contact Graph Unification
//
// This demo shows:
// 1. Identity entity creation with deterministic IDs
// 2. Edge creation for relationships
// 3. Identity resolution from events
// 4. Persistence and replay
// 5. Family/household configuration
//
// GUARDRAIL: No goroutines. No time.Now(). All operations synchronous.
//
// Reference: docs/ADR/ADR-0028-phase13-identity-graph.md
package demo_phase13_identity_graph

import (
	"testing"
	"time"

	"quantumlife/internal/identityresolve"
	"quantumlife/internal/persist"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/storelog"
)

func TestDemo_IdentityEntityCreation(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	gen := identity.NewGenerator()

	// Create person from email
	person := gen.PersonFromEmail("satish@gmail.com", now)

	t.Logf("Person ID: %s", person.ID())
	t.Logf("Person CanonicalString: %s", person.CanonicalString())

	if person.ID() == "" {
		t.Error("person ID should not be empty")
	}

	// Create organization from domain
	org := gen.OrganizationFromDomain("anthropic.com", now)

	t.Logf("Org ID: %s", org.ID())
	t.Logf("Org CanonicalString: %s", org.CanonicalString())

	if org.ID() == "" {
		t.Error("org ID should not be empty")
	}

	// Create household
	household := gen.HouseholdFromName("Rajan Household", now)

	t.Logf("Household ID: %s", household.ID())
	t.Logf("Household CanonicalString: %s", household.CanonicalString())

	if household.ID() == "" {
		t.Error("household ID should not be empty")
	}
}

func TestDemo_DeterministicIDs(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	gen := identity.NewGenerator()

	// Same email should always produce same ID
	person1 := gen.PersonFromEmail("satish@gmail.com", now)
	person2 := gen.PersonFromEmail("satish@gmail.com", now)

	if person1.ID() != person2.ID() {
		t.Errorf("deterministic ID failed: %s != %s", person1.ID(), person2.ID())
	}

	// Gmail normalization: dots and plus-addressing
	personWithDots := gen.PersonFromEmail("sa.ti.sh@gmail.com", now)
	personWithPlus := gen.PersonFromEmail("satish+work@gmail.com", now)

	if person1.ID() != personWithDots.ID() {
		t.Errorf("Gmail dot normalization failed: %s != %s", person1.ID(), personWithDots.ID())
	}

	if person1.ID() != personWithPlus.ID() {
		t.Errorf("Gmail plus normalization failed: %s != %s", person1.ID(), personWithPlus.ID())
	}
}

func TestDemo_EdgeCreation(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	gen := identity.NewGenerator()

	person := gen.PersonFromEmail("alice@example.com", now)
	org := gen.OrganizationFromDomain("example.com", now)

	// Create works_at edge
	edge := identity.NewEdge(
		identity.EdgeTypeWorksAt,
		person.ID(),
		org.ID(),
		identity.ConfidenceMedium,
		"demo:email_domain",
		now,
	)

	t.Logf("Edge ID: %s", edge.ID())
	t.Logf("Edge Type: %s", edge.EdgeType)
	t.Logf("Edge From: %s", edge.FromID)
	t.Logf("Edge To: %s", edge.ToID)

	if edge.ID() == "" {
		t.Error("edge ID should not be empty")
	}

	if edge.EdgeType != identity.EdgeTypeWorksAt {
		t.Errorf("edge type mismatch: %s", edge.EdgeType)
	}
}

func TestDemo_IdentityResolution(t *testing.T) {
	config := &identityresolve.Config{
		FamilyMembers: map[string]identityresolve.FamilyMemberConfig{
			"spouse@gmail.com": {
				HouseholdName: "Demo Household",
				Relationship:  identity.EdgeTypeSpouseOf,
				DisplayName:   "Spouse",
			},
		},
		KnownAliases: make(map[string]string),
		OwnerEmails:  []string{"owner@gmail.com"},
	}

	resolver := identityresolve.NewResolver(config)

	event := identityresolve.CanonicalEvent{
		EventType:  identityresolve.EventTypeEmail,
		EventID:    "msg-1",
		Timestamp:  time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		CircleID:   "personal",
		OwnerEmail: "owner@gmail.com",
		Participants: []identityresolve.Participant{
			{Email: "bob@company.com", DisplayName: "Bob", Role: "from"},
			{Email: "spouse@gmail.com", DisplayName: "Spouse", Role: "to"},
		},
	}

	updates := resolver.ProcessEvent(event)

	t.Logf("Generated %d identity updates", len(updates))

	// Count update types
	entityCount := 0
	edgeCount := 0
	for _, u := range updates {
		switch u.UpdateType {
		case identityresolve.UpdateTypeEntityUpsert:
			entityCount++
			t.Logf("Entity: %s (%s)", u.EntityID, u.Entity.Type())
		case identityresolve.UpdateTypeEdgeUpsert:
			edgeCount++
			t.Logf("Edge: %s (%s)", u.EdgeID, u.Edge.EdgeType)
		}
	}

	// Should have entities for owner, bob, spouse, orgs, email accounts, household
	if entityCount < 5 {
		t.Errorf("expected at least 5 entity updates, got %d", entityCount)
	}

	// Should have edges for owns_email, works_at, member_of_hh
	if edgeCount < 3 {
		t.Errorf("expected at least 3 edge updates, got %d", edgeCount)
	}
}

func TestDemo_PersistenceAndReplay(t *testing.T) {
	log := storelog.NewInMemoryLog()
	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	gen := identity.NewGenerator()

	// Create first store and add entities
	store1, err := persist.NewIdentityStore(log)
	if err != nil {
		t.Fatalf("NewIdentityStore failed: %v", err)
	}

	person := gen.PersonFromEmail("persist@example.com", now)
	store1.StoreEntity(person, now)

	org := gen.OrganizationFromDomain("example.com", now)
	store1.StoreEntity(org, now)

	edge := identity.NewEdge(
		identity.EdgeTypeWorksAt,
		person.ID(),
		org.ID(),
		identity.ConfidenceHigh,
		"demo:persist",
		now,
	)
	store1.StoreEdge(edge, now)

	stats1 := store1.Stats()
	t.Logf("Store 1 - Entities: %d, Edges: %d", stats1.TotalEntityCount, stats1.EdgeCount)

	// Create new store from same log - should replay
	store2, err := persist.NewIdentityStore(log)
	if err != nil {
		t.Fatalf("NewIdentityStore (replay) failed: %v", err)
	}

	stats2 := store2.Stats()
	t.Logf("Store 2 (after replay) - Entities: %d, Edges: %d", stats2.TotalEntityCount, stats2.EdgeCount)

	// Verify entity count matches
	if stats1.TotalEntityCount != stats2.TotalEntityCount {
		t.Errorf("entity count mismatch after replay: %d != %d",
			stats1.TotalEntityCount, stats2.TotalEntityCount)
	}

	// Verify can find entities
	foundPerson, err := store2.FindPersonByEmail("persist@example.com")
	if err != nil {
		t.Fatalf("FindPersonByEmail after replay failed: %v", err)
	}

	if foundPerson.ID() != person.ID() {
		t.Errorf("person ID mismatch after replay")
	}
}

func TestDemo_CanonicalStringFormat(t *testing.T) {
	event := identityresolve.CanonicalEvent{
		EventType:  identityresolve.EventTypeEmail,
		EventID:    "msg-123",
		Timestamp:  time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		CircleID:   "work",
		OwnerEmail: "owner@example.com",
		Participants: []identityresolve.Participant{
			{Email: "alice@example.com", DisplayName: "Alice", Role: "from"},
		},
	}

	canonical := event.CanonicalString()

	t.Logf("Canonical string: %s", canonical)

	// Must be pipe-delimited (NOT JSON)
	if len(canonical) > 0 && canonical[0] == '{' {
		t.Error("canonical string should NOT be JSON")
	}

	// Must contain pipe delimiters
	hasPipe := false
	for _, c := range canonical {
		if c == '|' {
			hasPipe = true
			break
		}
	}

	if !hasPipe {
		t.Error("canonical string should be pipe-delimited")
	}
}

func TestDemo_GraphStatistics(t *testing.T) {
	log := storelog.NewInMemoryLog()
	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	gen := identity.NewGenerator()

	store, _ := persist.NewIdentityStore(log)

	// Add various entity types
	store.StoreEntity(gen.PersonFromEmail("alice@example.com", now), now)
	store.StoreEntity(gen.PersonFromEmail("bob@example.com", now), now)
	store.StoreEntity(gen.OrganizationFromDomain("example.com", now), now)
	store.StoreEntity(gen.OrganizationFromMerchant("ACME Corp", now), now)
	store.StoreEntity(gen.EmailAccountFromAddress("alice@example.com", "other", now), now)
	store.StoreEntity(gen.HouseholdFromName("Test Household", now), now)
	store.StoreEntity(gen.PhoneNumberFromNumber("+44123456789", now), now)

	stats := store.Stats()

	t.Logf("Identity Graph Statistics:")
	t.Logf("  Persons: %d", stats.PersonCount)
	t.Logf("  Organizations: %d", stats.OrganizationCount)
	t.Logf("  Email Accounts: %d", stats.EmailAccountCount)
	t.Logf("  Households: %d", stats.HouseholdCount)
	t.Logf("  Phone Numbers: %d", stats.PhoneNumberCount)
	t.Logf("  Total Entities: %d", stats.TotalEntityCount)
	t.Logf("  Edges: %d", stats.EdgeCount)

	if stats.PersonCount != 2 {
		t.Errorf("PersonCount = %d, want 2", stats.PersonCount)
	}
	if stats.OrganizationCount != 2 {
		t.Errorf("OrganizationCount = %d, want 2", stats.OrganizationCount)
	}
	if stats.TotalEntityCount != 7 {
		t.Errorf("TotalEntityCount = %d, want 7", stats.TotalEntityCount)
	}
}
