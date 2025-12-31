package identity

import (
	"testing"
	"time"
)

var testTime = time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

func TestDeterministicIDGeneration(t *testing.T) {
	gen := NewGenerator()

	// Same input should produce same ID
	p1 := gen.PersonFromEmail("test@example.com", testTime)
	p2 := gen.PersonFromEmail("test@example.com", testTime)

	if p1.ID() != p2.ID() {
		t.Errorf("determinism failed: got %s and %s", p1.ID(), p2.ID())
	}

	// Different input should produce different ID
	p3 := gen.PersonFromEmail("other@example.com", testTime)
	if p1.ID() == p3.ID() {
		t.Errorf("collision: different inputs produced same ID %s", p1.ID())
	}
}

func TestEmailNormalization(t *testing.T) {
	gen := NewGenerator()

	tests := []struct {
		name   string
		emails []string // All should produce same ID
	}{
		{
			name:   "gmail dot insensitivity",
			emails: []string{"john.doe@gmail.com", "johndoe@gmail.com", "j.o.h.n.d.o.e@gmail.com"},
		},
		{
			name:   "gmail plus addressing",
			emails: []string{"john@gmail.com", "john+shopping@gmail.com", "john+work@gmail.com"},
		},
		{
			name:   "case insensitivity",
			emails: []string{"John@Example.COM", "john@example.com", "JOHN@EXAMPLE.COM"},
		},
		{
			name:   "googlemail normalization",
			emails: []string{"john@gmail.com", "john@googlemail.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.emails) < 2 {
				t.Skip("need at least 2 emails")
			}

			firstID := gen.PersonFromEmail(tt.emails[0], testTime).ID()
			for _, email := range tt.emails[1:] {
				id := gen.PersonFromEmail(email, testTime).ID()
				if id != firstID {
					t.Errorf("normalization failed: %s and %s produced different IDs: %s vs %s",
						tt.emails[0], email, firstID, id)
				}
			}
		})
	}
}

func TestMerchantNormalization(t *testing.T) {
	gen := NewGenerator()

	tests := []struct {
		name      string
		merchants []string
	}{
		{
			name:      "case and whitespace",
			merchants: []string{"Amazon", "AMAZON", "  amazon  "},
		},
		{
			name:      "noise tokens",
			merchants: []string{"tesco", "POS TESCO", "CARD TESCO", "CONTACTLESS TESCO"},
		},
		{
			name:      "trailing store IDs",
			merchants: []string{"sainsburys", "SAINSBURYS 1234", "SAINSBURYS #5678"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.merchants) < 2 {
				t.Skip("need at least 2 merchants")
			}

			firstID := gen.OrganizationFromMerchant(tt.merchants[0], testTime).ID()
			for _, merchant := range tt.merchants[1:] {
				id := gen.OrganizationFromMerchant(merchant, testTime).ID()
				if id != firstID {
					t.Errorf("normalization failed: %s and %s produced different IDs: %s vs %s",
						tt.merchants[0], merchant, firstID, id)
				}
			}
		})
	}
}

func TestEntityIDFormat(t *testing.T) {
	gen := NewGenerator()

	tests := []struct {
		name     string
		entity   Entity
		wantType EntityType
	}{
		{
			name:     "person",
			entity:   gen.PersonFromEmail("test@example.com", testTime),
			wantType: EntityTypePerson,
		},
		{
			name:     "email account",
			entity:   gen.EmailAccountFromAddress("test@gmail.com", "gmail", testTime),
			wantType: EntityTypeEmailAccount,
		},
		{
			name:     "organization",
			entity:   gen.OrganizationFromDomain("example.com", testTime),
			wantType: EntityTypeOrganization,
		},
		{
			name:     "finance account",
			entity:   gen.FinanceAccountFromDetails("truelayer", "Barclays", "checking", "****1234", "GBP", testTime),
			wantType: EntityTypeFinAccount,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := tt.entity.ID()

			// Check type extraction
			if id.Type() != tt.wantType {
				t.Errorf("type extraction: got %s, want %s", id.Type(), tt.wantType)
			}

			// Check hash is non-empty and reasonable length
			hash := id.Hash()
			if len(hash) != 16 {
				t.Errorf("hash length: got %d, want 16", len(hash))
			}
		})
	}
}

func TestInMemoryRepository(t *testing.T) {
	repo := NewInMemoryRepository()
	gen := NewGenerator()

	// Store entities
	person := gen.PersonFromEmail("satish@example.com", testTime)
	if err := repo.Store(person); err != nil {
		t.Fatalf("Store person: %v", err)
	}

	email := gen.EmailAccountFromAddress("satish@gmail.com", "gmail", testTime)
	if err := repo.Store(email); err != nil {
		t.Fatalf("Store email: %v", err)
	}

	org := gen.OrganizationFromDomain("amazon.co.uk", testTime)
	if err := repo.Store(org); err != nil {
		t.Fatalf("Store org: %v", err)
	}

	// Test counts
	if repo.Count() != 3 {
		t.Errorf("Count: got %d, want 3", repo.Count())
	}
	if repo.CountByType(EntityTypePerson) != 1 {
		t.Errorf("CountByType(Person): got %d, want 1", repo.CountByType(EntityTypePerson))
	}

	// Test Get
	retrieved, err := repo.Get(person.ID())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if retrieved.ID() != person.ID() {
		t.Errorf("Get returned wrong entity")
	}

	// Test Exists
	if !repo.Exists(person.ID()) {
		t.Error("Exists returned false for existing entity")
	}
	if repo.Exists("nonexistent_abc123") {
		t.Error("Exists returned true for nonexistent entity")
	}

	// Test duplicate store
	if err := repo.Store(person); err != ErrEntityExists {
		t.Errorf("duplicate Store: got %v, want ErrEntityExists", err)
	}

	// Test Delete
	if err := repo.Delete(org.ID()); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if repo.Count() != 2 {
		t.Errorf("Count after delete: got %d, want 2", repo.Count())
	}
}

func TestFindByEmail(t *testing.T) {
	repo := NewInMemoryRepository()
	gen := NewGenerator()

	person := gen.PersonFromEmail("satish@example.com", testTime)
	if err := repo.Store(person); err != nil {
		t.Fatalf("Store: %v", err)
	}

	// Find by exact email
	found, err := repo.FindPersonByEmail("satish@example.com")
	if err != nil {
		t.Fatalf("FindPersonByEmail: %v", err)
	}
	if found.ID() != person.ID() {
		t.Error("FindPersonByEmail returned wrong person")
	}

	// Find by normalized variant
	found, err = repo.FindPersonByEmail("SATISH@EXAMPLE.COM")
	if err != nil {
		t.Fatalf("FindPersonByEmail (uppercase): %v", err)
	}
	if found.ID() != person.ID() {
		t.Error("FindPersonByEmail (uppercase) returned wrong person")
	}

	// Not found
	_, err = repo.FindPersonByEmail("unknown@example.com")
	if err != ErrEntityNotFound {
		t.Errorf("FindPersonByEmail (unknown): got %v, want ErrEntityNotFound", err)
	}
}

func TestLinkEmailToPerson(t *testing.T) {
	repo := NewInMemoryRepository()
	gen := NewGenerator()

	// Create person and email account
	person := gen.PersonFromEmail("satish@example.com", testTime)
	if err := repo.Store(person); err != nil {
		t.Fatalf("Store person: %v", err)
	}

	email := gen.EmailAccountFromAddress("satish.work@company.com", "outlook", testTime)
	if err := repo.Store(email); err != nil {
		t.Fatalf("Store email: %v", err)
	}

	// Link email to person
	if err := repo.LinkEmailToPerson(email.ID(), person.ID()); err != nil {
		t.Fatalf("LinkEmailToPerson: %v", err)
	}

	// Verify link
	emails, err := repo.GetPersonEmails(person.ID())
	if err != nil {
		t.Fatalf("GetPersonEmails: %v", err)
	}
	if len(emails) != 1 {
		t.Errorf("GetPersonEmails: got %d emails, want 1", len(emails))
	}
	if emails[0].ID() != email.ID() {
		t.Error("GetPersonEmails returned wrong email")
	}

	// Verify owner is set
	retrieved, _ := repo.Get(email.ID())
	retrievedEmail := retrieved.(*EmailAccount)
	if retrievedEmail.OwnerID != person.ID() {
		t.Error("Email OwnerID not set after linking")
	}
}

func TestMergePersons(t *testing.T) {
	repo := NewInMemoryRepository()
	gen := NewGenerator()

	// Create two persons (same real person, different emails)
	person1 := gen.PersonFromEmail("satish.personal@gmail.com", testTime)
	if err := repo.Store(person1); err != nil {
		t.Fatalf("Store person1: %v", err)
	}

	person2 := gen.PersonFromEmail("satish.work@company.com", testTime)
	if err := repo.Store(person2); err != nil {
		t.Fatalf("Store person2: %v", err)
	}

	initialCount := repo.CountByType(EntityTypePerson)
	if initialCount != 2 {
		t.Fatalf("initial person count: got %d, want 2", initialCount)
	}

	// Merge person2 into person1
	if err := repo.MergePersons(person1.ID(), person2.ID()); err != nil {
		t.Fatalf("MergePersons: %v", err)
	}

	// Verify person2 is gone
	if repo.Exists(person2.ID()) {
		t.Error("person2 still exists after merge")
	}

	// Verify person1 has both aliases
	retrieved, _ := repo.Get(person1.ID())
	mergedPerson := retrieved.(*Person)
	if len(mergedPerson.Aliases) < 2 {
		t.Errorf("merged person aliases: got %d, want >= 2", len(mergedPerson.Aliases))
	}

	// Verify person count decreased
	if repo.CountByType(EntityTypePerson) != 1 {
		t.Errorf("person count after merge: got %d, want 1", repo.CountByType(EntityTypePerson))
	}
}

func TestCircleAndIntersection(t *testing.T) {
	gen := NewGenerator()

	// Create owner
	owner := gen.PersonFromEmail("satish@example.com", testTime)

	// Create circles
	work := gen.CircleFromName(owner.ID(), "Work", testTime)
	family := gen.CircleFromName(owner.ID(), "Family", testTime)

	// Circles should have different IDs
	if work.ID() == family.ID() {
		t.Error("different circles have same ID")
	}

	// Same circle name for same owner should be deterministic
	work2 := gen.CircleFromName(owner.ID(), "Work", testTime)
	if work.ID() != work2.ID() {
		t.Error("same circle parameters produced different IDs")
	}

	// Create intersection
	intersection := gen.IntersectionFromCircles("Work-Family", []EntityID{work.ID(), family.ID()}, testTime)

	// Intersection should be deterministic regardless of circle order
	intersection2 := gen.IntersectionFromCircles("Work-Family", []EntityID{family.ID(), work.ID()}, testTime)
	if intersection.ID() != intersection2.ID() {
		t.Error("intersection ID not deterministic with different circle order")
	}
}

func TestCollisionResistance(t *testing.T) {
	gen := NewGenerator()

	// Generate many IDs and check for collisions
	ids := make(map[EntityID]string)
	count := 1000

	for i := 0; i < count; i++ {
		// Generate unique emails using index directly
		email := "user" + itoa(i) + "@domain" + itoa(i/100) + ".com"
		person := gen.PersonFromEmail(email, testTime)

		if existing, exists := ids[person.ID()]; exists {
			t.Errorf("collision: %s and %s produced same ID %s", existing, email, person.ID())
		}
		ids[person.ID()] = email
	}

	// Verify we generated expected number of unique IDs
	if len(ids) != count {
		t.Errorf("generated %d unique IDs, expected %d", len(ids), count)
	}
}

// itoa converts int to string without fmt dependency
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	result := ""
	for i > 0 {
		result = string(rune('0'+i%10)) + result
		i /= 10
	}
	return result
}

func TestEntityRef(t *testing.T) {
	gen := NewGenerator()
	person := gen.PersonFromEmail("satish@example.com", testTime)
	person.DisplayName = "Satish"

	ref := NewEntityRef(person, "Satish")

	if ref.ID != person.ID() {
		t.Error("EntityRef ID mismatch")
	}
	if ref.Type != EntityTypePerson {
		t.Error("EntityRef Type mismatch")
	}
	if ref.Name != "Satish" {
		t.Error("EntityRef Name mismatch")
	}
}

func TestPhoneNormalization(t *testing.T) {
	gen := NewGenerator()

	tests := []struct {
		phones []string
	}{
		{phones: []string{"+447700900123", "+44 7700 900 123", "+44-7700-900-123"}},
		{phones: []string{"07700900123", "07700 900 123", "07700-900-123"}},
	}

	for _, tt := range tests {
		firstID := gen.PersonFromPhone(tt.phones[0], testTime).ID()
		for _, phone := range tt.phones[1:] {
			id := gen.PersonFromPhone(phone, testTime).ID()
			if id != firstID {
				t.Errorf("phone normalization failed: %s and %s produced different IDs", tt.phones[0], phone)
			}
		}
	}
}

func TestWorkEmailDetection(t *testing.T) {
	gen := NewGenerator()

	workEmail := gen.EmailAccountFromAddress("satish@company.com", "outlook", testTime)
	if !workEmail.IsWork {
		t.Error("company email not detected as work")
	}

	personalEmail := gen.EmailAccountFromAddress("satish@gmail.com", "gmail", testTime)
	if personalEmail.IsWork {
		t.Error("gmail detected as work")
	}
	if !personalEmail.IsPersonal {
		t.Error("gmail not detected as personal")
	}
}
