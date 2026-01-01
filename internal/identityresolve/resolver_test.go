package identityresolve

import (
	"testing"
	"time"

	"quantumlife/pkg/domain/identity"
)

func TestResolver_ProcessEmailEvent(t *testing.T) {
	config := &Config{
		FamilyMembers: make(map[string]FamilyMemberConfig),
		KnownAliases:  make(map[string]string),
		OwnerEmails:   []string{"satish@gmail.com"},
	}
	resolver := NewResolver(config)

	event := CanonicalEvent{
		EventType:  EventTypeEmail,
		EventID:    "msg-1",
		Timestamp:  time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		CircleID:   "work",
		OwnerEmail: "satish@gmail.com",
		Participants: []Participant{
			{Email: "bob@example.com", DisplayName: "Bob Smith", Role: "from"},
		},
	}

	updates := resolver.ProcessEvent(event)

	if len(updates) == 0 {
		t.Fatal("expected identity updates")
	}

	// Should have updates for owner + participant
	entityUpdates := 0
	edgeUpdates := 0
	for _, u := range updates {
		switch u.UpdateType {
		case UpdateTypeEntityUpsert:
			entityUpdates++
		case UpdateTypeEdgeUpsert:
			edgeUpdates++
		}
	}

	if entityUpdates < 4 {
		t.Errorf("expected at least 4 entity updates (2 persons, 2 email accounts), got %d", entityUpdates)
	}
	if edgeUpdates < 2 {
		t.Errorf("expected at least 2 edge updates (owns_email edges), got %d", edgeUpdates)
	}
}

func TestResolver_ProcessGmailNormalization(t *testing.T) {
	resolver := NewResolver(nil)

	// Gmail with dots and plus-addressing should normalize to same person
	event1 := CanonicalEvent{
		EventType: EventTypeEmail,
		EventID:   "msg-1",
		Timestamp: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		CircleID:  "personal",
		Participants: []Participant{
			{Email: "john.doe@gmail.com", DisplayName: "John", Role: "from"},
		},
	}

	event2 := CanonicalEvent{
		EventType: EventTypeEmail,
		EventID:   "msg-2",
		Timestamp: time.Date(2025, 1, 15, 11, 30, 0, 0, time.UTC),
		CircleID:  "personal",
		Participants: []Participant{
			{Email: "johndoe+work@gmail.com", DisplayName: "John Doe", Role: "from"},
		},
	}

	updates1 := resolver.ProcessEvent(event1)
	updates2 := resolver.ProcessEvent(event2)

	// Find person entity IDs
	var personID1, personID2 identity.EntityID
	for _, u := range updates1 {
		if u.Entity != nil && u.Entity.Type() == identity.EntityTypePerson {
			personID1 = u.EntityID
			break
		}
	}
	for _, u := range updates2 {
		if u.Entity != nil && u.Entity.Type() == identity.EntityTypePerson {
			personID2 = u.EntityID
			break
		}
	}

	if personID1 != personID2 {
		t.Errorf("Gmail normalization failed: %s != %s", personID1, personID2)
	}
}

func TestResolver_ProcessWorkEmail(t *testing.T) {
	resolver := NewResolver(nil)

	event := CanonicalEvent{
		EventType: EventTypeEmail,
		EventID:   "msg-1",
		Timestamp: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		CircleID:  "work",
		Participants: []Participant{
			{Email: "alice@acme.com", DisplayName: "Alice", Role: "from"},
		},
	}

	updates := resolver.ProcessEvent(event)

	// Should have organization entity
	foundOrg := false
	foundWorksAt := false
	for _, u := range updates {
		if u.Entity != nil && u.Entity.Type() == identity.EntityTypeOrganization {
			foundOrg = true
		}
		if u.Edge != nil && u.Edge.EdgeType == identity.EdgeTypeWorksAt {
			foundWorksAt = true
		}
	}

	if !foundOrg {
		t.Error("expected organization entity for work email")
	}
	if !foundWorksAt {
		t.Error("expected works_at edge for work email")
	}
}

func TestResolver_ProcessFamilyMember(t *testing.T) {
	config := &Config{
		FamilyMembers: map[string]FamilyMemberConfig{
			"spouse@gmail.com": {
				HouseholdName: "Rajan Household",
				Relationship:  identity.EdgeTypeSpouseOf,
				DisplayName:   "Spouse",
			},
		},
		KnownAliases: make(map[string]string),
		OwnerEmails:  []string{"satish@gmail.com"},
	}
	resolver := NewResolver(config)

	event := CanonicalEvent{
		EventType: EventTypeEmail,
		EventID:   "msg-1",
		Timestamp: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		CircleID:  "personal",
		Participants: []Participant{
			{Email: "spouse@gmail.com", DisplayName: "Spouse", Role: "to"},
		},
	}

	updates := resolver.ProcessEvent(event)

	// Should have household entity
	foundHousehold := false
	foundMemberOfHH := false
	for _, u := range updates {
		if u.Entity != nil && u.Entity.Type() == identity.EntityTypeHousehold {
			foundHousehold = true
		}
		if u.Edge != nil && u.Edge.EdgeType == identity.EdgeTypeMemberOfHH {
			foundMemberOfHH = true
		}
	}

	if !foundHousehold {
		t.Error("expected household entity for family member")
	}
	if !foundMemberOfHH {
		t.Error("expected member_of_hh edge for family member")
	}
}

func TestResolver_ProcessMerchant(t *testing.T) {
	resolver := NewResolver(nil)

	event := CanonicalEvent{
		EventType:     EventTypeCommerce,
		EventID:       "receipt-1",
		Timestamp:     time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		CircleID:      "personal",
		MerchantName:  "AMAZON.CO.UK",
		MerchantEmail: "noreply@amazon.co.uk",
	}

	updates := resolver.ProcessEvent(event)

	// Should have organization entity(s)
	orgCount := 0
	for _, u := range updates {
		if u.Entity != nil && u.Entity.Type() == identity.EntityTypeOrganization {
			orgCount++
		}
	}

	if orgCount == 0 {
		t.Error("expected organization entity for merchant")
	}
}

func TestResolver_KnownAlias(t *testing.T) {
	config := &Config{
		FamilyMembers: make(map[string]FamilyMemberConfig),
		KnownAliases: map[string]string{
			"satish+work@gmail.com": "satish@gmail.com",
		},
		OwnerEmails: []string{"satish@gmail.com"},
	}
	resolver := NewResolver(config)

	event1 := CanonicalEvent{
		EventType: EventTypeEmail,
		EventID:   "msg-1",
		Timestamp: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		CircleID:  "personal",
		Participants: []Participant{
			{Email: "satish@gmail.com", Role: "from"},
		},
	}

	event2 := CanonicalEvent{
		EventType: EventTypeEmail,
		EventID:   "msg-2",
		Timestamp: time.Date(2025, 1, 15, 11, 30, 0, 0, time.UTC),
		CircleID:  "work",
		Participants: []Participant{
			{Email: "satish+work@gmail.com", Role: "from"},
		},
	}

	updates1 := resolver.ProcessEvent(event1)
	updates2 := resolver.ProcessEvent(event2)

	// Both should resolve to the same person (Gmail normalization removes +work)
	var personID1, personID2 identity.EntityID
	for _, u := range updates1 {
		if u.Entity != nil && u.Entity.Type() == identity.EntityTypePerson {
			personID1 = u.EntityID
			break
		}
	}
	for _, u := range updates2 {
		if u.Entity != nil && u.Entity.Type() == identity.EntityTypePerson {
			personID2 = u.EntityID
			break
		}
	}

	if personID1 != personID2 {
		t.Errorf("alias resolution failed: %s != %s", personID1, personID2)
	}
}

func TestCanonicalEvent_CanonicalString(t *testing.T) {
	event := CanonicalEvent{
		EventType:  EventTypeEmail,
		EventID:    "msg-1",
		Timestamp:  time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		CircleID:   "work",
		OwnerEmail: "satish@gmail.com",
		Participants: []Participant{
			{Email: "bob@example.com", DisplayName: "Bob", Role: "from"},
		},
	}

	canonical := event.CanonicalString()

	// Should be pipe-delimited
	if len(canonical) == 0 {
		t.Error("expected non-empty canonical string")
	}
	if canonical[0] == '{' {
		t.Error("canonical string should not be JSON")
	}
	if !contains(canonical, "|") {
		t.Error("canonical string should be pipe-delimited")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
