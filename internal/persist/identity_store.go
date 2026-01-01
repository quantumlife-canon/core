package persist

import (
	"fmt"
	"strings"
	"time"

	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/storelog"
)

// IdentityStore provides persistent storage for identity graph entities and edges.
// All operations are append-only and support replay for recovery.
//
// GUARDRAIL: No goroutines. No time.Now(). All operations synchronous.
type IdentityStore struct {
	log  storelog.AppendOnlyLog
	repo *identity.InMemoryRepository
}

// NewIdentityStore creates a new identity store with replay from the log.
func NewIdentityStore(log storelog.AppendOnlyLog) (*IdentityStore, error) {
	store := &IdentityStore{
		log:  log,
		repo: identity.NewInMemoryRepository(),
	}

	// Replay existing records
	if err := store.replay(); err != nil {
		return nil, fmt.Errorf("replay failed: %w", err)
	}

	return store, nil
}

// replay rebuilds the in-memory state from the log.
func (s *IdentityStore) replay() error {
	// Get entity records
	entityRecords, err := s.log.ListByType(storelog.RecordTypeIdentityEntity)
	if err != nil {
		return err
	}

	for _, record := range entityRecords {
		entity, err := parseEntityPayload(record.Payload, record.Timestamp)
		if err != nil {
			// Skip invalid records during replay
			continue
		}
		// Store ignores duplicates, so safe to replay
		_ = s.repo.Store(entity)
	}

	// Get edge records
	edgeRecords, err := s.log.ListByType(storelog.RecordTypeIdentityEdge)
	if err != nil {
		return err
	}

	for _, record := range edgeRecords {
		edge, err := parseEdgePayload(record.Payload, record.Timestamp)
		if err != nil {
			// Skip invalid records during replay
			continue
		}
		// Store ignores duplicates, so safe to replay
		_ = s.repo.StoreEdge(edge)
	}

	return nil
}

// StoreEntity persists an entity to the log and in-memory repository.
func (s *IdentityStore) StoreEntity(entity identity.Entity, ts time.Time) error {
	payload := entityToPayload(entity)

	record := storelog.NewRecord(storelog.RecordTypeIdentityEntity, ts, "", payload)
	if err := s.log.Append(record); err != nil {
		if err != storelog.ErrRecordExists {
			return err
		}
		// Record exists means entity already persisted
	}

	// Update in-memory (ignore exists error)
	_ = s.repo.Store(entity)

	return nil
}

// StoreEdge persists an edge to the log and in-memory repository.
func (s *IdentityStore) StoreEdge(edge *identity.Edge, ts time.Time) error {
	payload := edgeToPayload(edge)

	record := storelog.NewRecord(storelog.RecordTypeIdentityEdge, ts, "", payload)
	if err := s.log.Append(record); err != nil {
		if err != storelog.ErrRecordExists {
			return err
		}
		// Record exists means edge already persisted
	}

	// Update in-memory (ignore exists error)
	_ = s.repo.StoreEdge(edge)

	return nil
}

// GetEntity retrieves an entity by ID.
func (s *IdentityStore) GetEntity(id identity.EntityID) (identity.Entity, error) {
	return s.repo.Get(id)
}

// GetEdge retrieves an edge by ID.
func (s *IdentityStore) GetEdge(id identity.EntityID) (*identity.Edge, error) {
	return s.repo.GetEdge(id)
}

// FindPersonByEmail finds a person by email.
func (s *IdentityStore) FindPersonByEmail(email string) (*identity.Person, error) {
	return s.repo.FindPersonByEmail(email)
}

// FindOrganizationByDomain finds an organization by domain.
func (s *IdentityStore) FindOrganizationByDomain(domain string) (*identity.Organization, error) {
	return s.repo.FindOrganizationByDomain(domain)
}

// FindOrganizationByMerchant finds an organization by merchant name.
func (s *IdentityStore) FindOrganizationByMerchant(name string) (*identity.Organization, error) {
	return s.repo.FindOrganizationByMerchant(name)
}

// FindHouseholdByName finds a household by name.
func (s *IdentityStore) FindHouseholdByName(name string) (*identity.Household, error) {
	return s.repo.FindHouseholdByName(name)
}

// GetEdgesFrom returns all edges originating from an entity.
func (s *IdentityStore) GetEdgesFrom(entityID identity.EntityID) ([]*identity.Edge, error) {
	return s.repo.GetEdgesFrom(entityID)
}

// GetEdgesTo returns all edges pointing to an entity.
func (s *IdentityStore) GetEdgesTo(entityID identity.EntityID) ([]*identity.Edge, error) {
	return s.repo.GetEdgesTo(entityID)
}

// GetEdgesByType returns all edges of a specific type.
func (s *IdentityStore) GetEdgesByType(edgeType identity.EdgeType) ([]*identity.Edge, error) {
	return s.repo.GetEdgesByType(edgeType)
}

// ListEntitiesByType returns all entities of a given type.
func (s *IdentityStore) ListEntitiesByType(entityType identity.EntityType) ([]identity.Entity, error) {
	return s.repo.GetByType(entityType)
}

// EntityCount returns the number of entities.
func (s *IdentityStore) EntityCount() int {
	return s.repo.Count()
}

// EdgeCount returns the number of edges.
func (s *IdentityStore) EdgeCount() int {
	return s.repo.EdgeCount()
}

// Stats returns identity graph statistics.
func (s *IdentityStore) Stats() IdentityStats {
	return IdentityStats{
		PersonCount:       s.repo.CountByType(identity.EntityTypePerson),
		OrganizationCount: s.repo.CountByType(identity.EntityTypeOrganization),
		EmailAccountCount: s.repo.CountByType(identity.EntityTypeEmailAccount),
		HouseholdCount:    s.repo.CountByType(identity.EntityTypeHousehold),
		PhoneNumberCount:  s.repo.CountByType(identity.EntityTypePhoneNumber),
		TotalEntityCount:  s.repo.Count(),
		EdgeCount:         s.repo.EdgeCount(),
	}
}

// IdentityStats contains counts for identity graph entities.
type IdentityStats struct {
	PersonCount       int
	OrganizationCount int
	EmailAccountCount int
	HouseholdCount    int
	PhoneNumberCount  int
	TotalEntityCount  int
	EdgeCount         int
}

// Payload format: type|id|canonical_string|field1|field2|...
// Uses pipe-delimited format (NOT JSON) for determinism.

func entityToPayload(entity identity.Entity) string {
	parts := []string{
		string(entity.Type()),
		string(entity.ID()),
		entity.CanonicalString(),
	}

	// Add type-specific fields
	switch e := entity.(type) {
	case *identity.Person:
		parts = append(parts, e.PrimaryEmail, e.DisplayName, e.PhoneNumber)
	case *identity.Organization:
		parts = append(parts, e.Name, e.Domain, e.NormalizedName)
	case *identity.EmailAccount:
		parts = append(parts, e.Address, e.Provider, boolToStr(e.IsWork))
	case *identity.PhoneNumber:
		parts = append(parts, e.Number, e.Provider, boolToStr(e.IsVerified))
	case *identity.Household:
		parts = append(parts, e.Name, e.Address)
	}

	return strings.Join(parts, "|")
}

func edgeToPayload(edge *identity.Edge) string {
	parts := []string{
		string(edge.EdgeType),
		string(edge.ID()),
		edge.CanonicalString(),
		string(edge.FromID),
		string(edge.ToID),
		string(edge.Confidence),
		edge.Provenance,
	}
	return strings.Join(parts, "|")
}

func parseEntityPayload(payload string, ts time.Time) (identity.Entity, error) {
	parts := strings.Split(payload, "|")
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid entity payload: too few parts")
	}

	entityType := identity.EntityType(parts[0])
	// For replay, we use the generator to recreate with canonical string
	gen := identity.NewGenerator()

	switch entityType {
	case identity.EntityTypePerson:
		if len(parts) >= 4 && parts[3] != "" {
			return gen.PersonFromEmail(parts[3], ts), nil
		}
	case identity.EntityTypeOrganization:
		if len(parts) >= 5 && parts[4] != "" {
			return gen.OrganizationFromDomain(parts[4], ts), nil
		}
		if len(parts) >= 4 && parts[3] != "" {
			return gen.OrganizationFromMerchant(parts[3], ts), nil
		}
	case identity.EntityTypeEmailAccount:
		if len(parts) >= 5 {
			return gen.EmailAccountFromAddress(parts[3], parts[4], ts), nil
		}
	case identity.EntityTypePhoneNumber:
		if len(parts) >= 4 {
			return gen.PhoneNumberFromNumber(parts[3], ts), nil
		}
	case identity.EntityTypeHousehold:
		if len(parts) >= 4 {
			return gen.HouseholdFromName(parts[3], ts), nil
		}
	}

	return nil, fmt.Errorf("cannot parse entity type: %s", entityType)
}

func parseEdgePayload(payload string, ts time.Time) (*identity.Edge, error) {
	parts := strings.Split(payload, "|")
	if len(parts) < 7 {
		return nil, fmt.Errorf("invalid edge payload: too few parts")
	}

	edgeType := identity.EdgeType(parts[0])
	fromID := identity.EntityID(parts[3])
	toID := identity.EntityID(parts[4])
	confidence := identity.Confidence(parts[5])
	provenance := parts[6]

	return identity.NewEdge(edgeType, fromID, toID, confidence, provenance, ts), nil
}

func boolToStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
