package identity

import (
	"errors"
	"strings"
	"sync"
)

// Repository errors.
var (
	ErrEntityNotFound    = errors.New("entity not found")
	ErrEntityExists      = errors.New("entity already exists")
	ErrInvalidEntityType = errors.New("invalid entity type")
	ErrUnificationFailed = errors.New("unification failed")
)

// Repository provides storage and retrieval of identity graph entities.
type Repository interface {
	// Store saves an entity. Returns ErrEntityExists if ID already exists.
	Store(entity Entity) error

	// Get retrieves an entity by ID. Returns ErrEntityNotFound if not found.
	Get(id EntityID) (Entity, error)

	// GetByType retrieves all entities of a given type.
	GetByType(entityType EntityType) ([]Entity, error)

	// Exists checks if an entity exists.
	Exists(id EntityID) bool

	// Delete removes an entity.
	Delete(id EntityID) error

	// Count returns total entity count.
	Count() int

	// CountByType returns count for a specific type.
	CountByType(entityType EntityType) int
}

// UnificationRepository extends Repository with unification capabilities.
type UnificationRepository interface {
	Repository

	// FindPersonByEmail finds a person by any of their email addresses.
	FindPersonByEmail(email string) (*Person, error)

	// FindOrganizationByDomain finds an org by domain.
	FindOrganizationByDomain(domain string) (*Organization, error)

	// FindOrganizationByMerchant finds an org by normalized merchant name.
	FindOrganizationByMerchant(merchantName string) (*Organization, error)

	// LinkEmailToPerson associates an email account with a person.
	LinkEmailToPerson(emailID EntityID, personID EntityID) error

	// MergePersons unifies two person entities into one.
	// All references to secondaryID are updated to point to primaryID.
	MergePersons(primaryID, secondaryID EntityID) error

	// GetPersonEmails returns all email accounts linked to a person.
	GetPersonEmails(personID EntityID) ([]*EmailAccount, error)
}

// EdgeRepository provides storage and retrieval of edges.
type EdgeRepository interface {
	// StoreEdge saves an edge. Returns ErrEntityExists if ID already exists.
	StoreEdge(edge *Edge) error

	// GetEdge retrieves an edge by ID.
	GetEdge(id EntityID) (*Edge, error)

	// GetEdgesFrom returns all edges originating from an entity.
	GetEdgesFrom(entityID EntityID) ([]*Edge, error)

	// GetEdgesTo returns all edges pointing to an entity.
	GetEdgesTo(entityID EntityID) ([]*Edge, error)

	// GetEdgesByType returns all edges of a specific type.
	GetEdgesByType(edgeType EdgeType) ([]*Edge, error)

	// DeleteEdge removes an edge.
	DeleteEdge(id EntityID) error

	// EdgeCount returns total edge count.
	EdgeCount() int
}

// InMemoryRepository is a thread-safe in-memory implementation of UnificationRepository.
type InMemoryRepository struct {
	mu       sync.RWMutex
	entities map[EntityID]Entity
	edges    map[EntityID]*Edge

	// Indexes for fast lookup
	emailToPerson   map[string]EntityID     // normalized email -> person ID
	domainToOrg     map[string]EntityID     // normalized domain -> org ID
	merchantToOrg   map[string]EntityID     // normalized merchant -> org ID
	emailToAccount  map[string]EntityID     // normalized email -> email account ID
	personToEmails  map[EntityID][]EntityID // person ID -> email account IDs
	phoneToPerson   map[string]EntityID     // normalized phone -> person ID
	phoneToEntity   map[string]EntityID     // normalized phone -> PhoneNumber entity ID
	householdByName map[string]EntityID     // normalized name -> household ID

	// Edge indexes
	edgesFromEntity map[EntityID][]EntityID // entity ID -> edge IDs originating from it
	edgesToEntity   map[EntityID][]EntityID // entity ID -> edge IDs pointing to it
	edgesByType     map[EdgeType][]EntityID // edge type -> edge IDs
}

// NewInMemoryRepository creates a new in-memory repository.
func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		entities:        make(map[EntityID]Entity),
		edges:           make(map[EntityID]*Edge),
		emailToPerson:   make(map[string]EntityID),
		domainToOrg:     make(map[string]EntityID),
		merchantToOrg:   make(map[string]EntityID),
		emailToAccount:  make(map[string]EntityID),
		personToEmails:  make(map[EntityID][]EntityID),
		phoneToPerson:   make(map[string]EntityID),
		phoneToEntity:   make(map[string]EntityID),
		householdByName: make(map[string]EntityID),
		edgesFromEntity: make(map[EntityID][]EntityID),
		edgesToEntity:   make(map[EntityID][]EntityID),
		edgesByType:     make(map[EdgeType][]EntityID),
	}
}

func (r *InMemoryRepository) Store(entity Entity) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.entities[entity.ID()]; exists {
		return ErrEntityExists
	}

	r.entities[entity.ID()] = entity

	// Update indexes based on entity type
	switch e := entity.(type) {
	case *Person:
		if e.PrimaryEmail != "" {
			r.emailToPerson[e.PrimaryEmail] = e.ID()
		}
		for _, alias := range e.Aliases {
			r.emailToPerson[normalizeEmail(alias)] = e.ID()
		}

	case *EmailAccount:
		r.emailToAccount[e.Address] = e.ID()
		if e.OwnerID != "" {
			r.personToEmails[e.OwnerID] = append(r.personToEmails[e.OwnerID], e.ID())
		}

	case *Organization:
		if e.Domain != "" {
			r.domainToOrg[e.Domain] = e.ID()
		}
		if e.NormalizedName != "" {
			r.merchantToOrg[e.NormalizedName] = e.ID()
		}
		for _, alias := range e.Aliases {
			r.merchantToOrg[normalizeMerchant(alias)] = e.ID()
		}

	case *PhoneNumber:
		r.phoneToEntity[e.Number] = e.ID()
		if e.OwnerID != "" {
			r.phoneToPerson[e.Number] = e.OwnerID
		}

	case *Household:
		r.householdByName[strings.ToLower(e.Name)] = e.ID()
	}

	return nil
}

func (r *InMemoryRepository) Get(id EntityID) (Entity, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entity, exists := r.entities[id]
	if !exists {
		return nil, ErrEntityNotFound
	}
	return entity, nil
}

func (r *InMemoryRepository) GetByType(entityType EntityType) ([]Entity, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []Entity
	for _, entity := range r.entities {
		if entity.Type() == entityType {
			result = append(result, entity)
		}
	}
	return result, nil
}

func (r *InMemoryRepository) Exists(id EntityID) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.entities[id]
	return exists
}

func (r *InMemoryRepository) Delete(id EntityID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entity, exists := r.entities[id]
	if !exists {
		return ErrEntityNotFound
	}

	// Clean up indexes
	switch e := entity.(type) {
	case *Person:
		if e.PrimaryEmail != "" {
			delete(r.emailToPerson, e.PrimaryEmail)
		}
		for _, alias := range e.Aliases {
			delete(r.emailToPerson, normalizeEmail(alias))
		}
		delete(r.personToEmails, e.ID())

	case *EmailAccount:
		delete(r.emailToAccount, e.Address)

	case *Organization:
		if e.Domain != "" {
			delete(r.domainToOrg, e.Domain)
		}
		if e.NormalizedName != "" {
			delete(r.merchantToOrg, e.NormalizedName)
		}
	}

	delete(r.entities, id)
	return nil
}

func (r *InMemoryRepository) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.entities)
}

func (r *InMemoryRepository) CountByType(entityType EntityType) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, entity := range r.entities {
		if entity.Type() == entityType {
			count++
		}
	}
	return count
}

func (r *InMemoryRepository) FindPersonByEmail(email string) (*Person, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	normalized := normalizeEmail(email)
	personID, exists := r.emailToPerson[normalized]
	if !exists {
		return nil, ErrEntityNotFound
	}

	entity, exists := r.entities[personID]
	if !exists {
		return nil, ErrEntityNotFound
	}

	person, ok := entity.(*Person)
	if !ok {
		return nil, ErrInvalidEntityType
	}

	return person, nil
}

func (r *InMemoryRepository) FindOrganizationByDomain(domain string) (*Organization, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	normalized := normalizeDomain(domain)
	orgID, exists := r.domainToOrg[normalized]
	if !exists {
		return nil, ErrEntityNotFound
	}

	entity, exists := r.entities[orgID]
	if !exists {
		return nil, ErrEntityNotFound
	}

	org, ok := entity.(*Organization)
	if !ok {
		return nil, ErrInvalidEntityType
	}

	return org, nil
}

func (r *InMemoryRepository) FindOrganizationByMerchant(merchantName string) (*Organization, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	normalized := normalizeMerchant(merchantName)
	orgID, exists := r.merchantToOrg[normalized]
	if !exists {
		return nil, ErrEntityNotFound
	}

	entity, exists := r.entities[orgID]
	if !exists {
		return nil, ErrEntityNotFound
	}

	org, ok := entity.(*Organization)
	if !ok {
		return nil, ErrInvalidEntityType
	}

	return org, nil
}

func (r *InMemoryRepository) LinkEmailToPerson(emailID EntityID, personID EntityID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Get email account
	emailEntity, exists := r.entities[emailID]
	if !exists {
		return ErrEntityNotFound
	}
	emailAccount, ok := emailEntity.(*EmailAccount)
	if !ok {
		return ErrInvalidEntityType
	}

	// Get person
	personEntity, exists := r.entities[personID]
	if !exists {
		return ErrEntityNotFound
	}
	person, ok := personEntity.(*Person)
	if !ok {
		return ErrInvalidEntityType
	}

	// Update email account owner
	emailAccount.OwnerID = personID

	// Update person's email accounts list
	person.EmailAccounts = append(person.EmailAccounts, emailID)

	// Update indexes
	r.emailToPerson[emailAccount.Address] = personID
	r.personToEmails[personID] = append(r.personToEmails[personID], emailID)

	return nil
}

func (r *InMemoryRepository) MergePersons(primaryID, secondaryID EntityID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Get both persons
	primaryEntity, exists := r.entities[primaryID]
	if !exists {
		return ErrEntityNotFound
	}
	primary, ok := primaryEntity.(*Person)
	if !ok {
		return ErrInvalidEntityType
	}

	secondaryEntity, exists := r.entities[secondaryID]
	if !exists {
		return ErrEntityNotFound
	}
	secondary, ok := secondaryEntity.(*Person)
	if !ok {
		return ErrInvalidEntityType
	}

	// Merge aliases
	primary.Aliases = append(primary.Aliases, secondary.Aliases...)

	// Merge email accounts
	primary.EmailAccounts = append(primary.EmailAccounts, secondary.EmailAccounts...)

	// Update email account owners
	for _, emailID := range secondary.EmailAccounts {
		if emailEntity, exists := r.entities[emailID]; exists {
			if emailAccount, ok := emailEntity.(*EmailAccount); ok {
				emailAccount.OwnerID = primaryID
			}
		}
	}

	// Update indexes: point secondary's emails to primary
	for _, alias := range secondary.Aliases {
		r.emailToPerson[normalizeEmail(alias)] = primaryID
	}

	// Merge personToEmails
	r.personToEmails[primaryID] = append(r.personToEmails[primaryID], r.personToEmails[secondaryID]...)
	delete(r.personToEmails, secondaryID)

	// Delete secondary person
	delete(r.entities, secondaryID)

	return nil
}

func (r *InMemoryRepository) GetPersonEmails(personID EntityID) ([]*EmailAccount, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	emailIDs, exists := r.personToEmails[personID]
	if !exists {
		return nil, nil // No emails, not an error
	}

	var result []*EmailAccount
	for _, emailID := range emailIDs {
		if entity, exists := r.entities[emailID]; exists {
			if emailAccount, ok := entity.(*EmailAccount); ok {
				result = append(result, emailAccount)
			}
		}
	}

	return result, nil
}

// FindPersonByPhone finds a person by any of their phone numbers.
func (r *InMemoryRepository) FindPersonByPhone(phone string) (*Person, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	normalized := normalizePhone(phone)
	personID, exists := r.phoneToPerson[normalized]
	if !exists {
		return nil, ErrEntityNotFound
	}

	entity, exists := r.entities[personID]
	if !exists {
		return nil, ErrEntityNotFound
	}

	person, ok := entity.(*Person)
	if !ok {
		return nil, ErrInvalidEntityType
	}

	return person, nil
}

// FindHouseholdByName finds a household by name.
func (r *InMemoryRepository) FindHouseholdByName(name string) (*Household, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	normalized := strings.ToLower(strings.TrimSpace(name))
	householdID, exists := r.householdByName[normalized]
	if !exists {
		return nil, ErrEntityNotFound
	}

	entity, exists := r.entities[householdID]
	if !exists {
		return nil, ErrEntityNotFound
	}

	household, ok := entity.(*Household)
	if !ok {
		return nil, ErrInvalidEntityType
	}

	return household, nil
}

// LinkPhoneToPerson associates a phone number with a person.
func (r *InMemoryRepository) LinkPhoneToPerson(phoneID EntityID, personID EntityID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	phoneEntity, exists := r.entities[phoneID]
	if !exists {
		return ErrEntityNotFound
	}
	phone, ok := phoneEntity.(*PhoneNumber)
	if !ok {
		return ErrInvalidEntityType
	}

	_, exists = r.entities[personID]
	if !exists {
		return ErrEntityNotFound
	}

	phone.OwnerID = personID
	r.phoneToPerson[phone.Number] = personID

	return nil
}

// StoreEdge saves an edge to the repository.
func (r *InMemoryRepository) StoreEdge(edge *Edge) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.edges[edge.ID()]; exists {
		return ErrEntityExists
	}

	r.edges[edge.ID()] = edge

	// Update indexes
	r.edgesFromEntity[edge.FromID] = append(r.edgesFromEntity[edge.FromID], edge.ID())
	r.edgesToEntity[edge.ToID] = append(r.edgesToEntity[edge.ToID], edge.ID())
	r.edgesByType[edge.EdgeType] = append(r.edgesByType[edge.EdgeType], edge.ID())

	return nil
}

// GetEdge retrieves an edge by ID.
func (r *InMemoryRepository) GetEdge(id EntityID) (*Edge, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	edge, exists := r.edges[id]
	if !exists {
		return nil, ErrEntityNotFound
	}
	return edge, nil
}

// GetEdgesFrom returns all edges originating from an entity.
func (r *InMemoryRepository) GetEdgesFrom(entityID EntityID) ([]*Edge, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	edgeIDs := r.edgesFromEntity[entityID]
	result := make([]*Edge, 0, len(edgeIDs))
	for _, id := range edgeIDs {
		if edge, exists := r.edges[id]; exists {
			result = append(result, edge)
		}
	}
	return result, nil
}

// GetEdgesTo returns all edges pointing to an entity.
func (r *InMemoryRepository) GetEdgesTo(entityID EntityID) ([]*Edge, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	edgeIDs := r.edgesToEntity[entityID]
	result := make([]*Edge, 0, len(edgeIDs))
	for _, id := range edgeIDs {
		if edge, exists := r.edges[id]; exists {
			result = append(result, edge)
		}
	}
	return result, nil
}

// GetEdgesByType returns all edges of a specific type.
func (r *InMemoryRepository) GetEdgesByType(edgeType EdgeType) ([]*Edge, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	edgeIDs := r.edgesByType[edgeType]
	result := make([]*Edge, 0, len(edgeIDs))
	for _, id := range edgeIDs {
		if edge, exists := r.edges[id]; exists {
			result = append(result, edge)
		}
	}
	return result, nil
}

// DeleteEdge removes an edge.
func (r *InMemoryRepository) DeleteEdge(id EntityID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	edge, exists := r.edges[id]
	if !exists {
		return ErrEntityNotFound
	}

	// Remove from indexes
	r.edgesFromEntity[edge.FromID] = removeFromSlice(r.edgesFromEntity[edge.FromID], id)
	r.edgesToEntity[edge.ToID] = removeFromSlice(r.edgesToEntity[edge.ToID], id)
	r.edgesByType[edge.EdgeType] = removeFromSlice(r.edgesByType[edge.EdgeType], id)

	delete(r.edges, id)
	return nil
}

// EdgeCount returns total edge count.
func (r *InMemoryRepository) EdgeCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.edges)
}

// removeFromSlice removes an element from a slice of EntityIDs.
func removeFromSlice(slice []EntityID, id EntityID) []EntityID {
	result := make([]EntityID, 0, len(slice))
	for _, item := range slice {
		if item != id {
			result = append(result, item)
		}
	}
	return result
}

// ============================================================================
// Query Helpers with Deterministic Ordering (Phase 13.1)
// ============================================================================

// ListPersons returns all persons in deterministic order (sorted by PersonID).
func (r *InMemoryRepository) ListPersons() []*Person {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var persons []*Person
	for _, entity := range r.entities {
		if person, ok := entity.(*Person); ok {
			persons = append(persons, person)
		}
	}

	// Sort by ID for determinism
	sortPersonsByID(persons)
	return persons
}

// ListOrganizations returns all organizations in deterministic order (sorted by OrgID).
func (r *InMemoryRepository) ListOrganizations() []*Organization {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var orgs []*Organization
	for _, entity := range r.entities {
		if org, ok := entity.(*Organization); ok {
			orgs = append(orgs, org)
		}
	}

	// Sort by ID for determinism
	sortOrganizationsByID(orgs)
	return orgs
}

// ListHouseholds returns all households in deterministic order (sorted by HouseholdID).
func (r *InMemoryRepository) ListHouseholds() []*Household {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var households []*Household
	for _, entity := range r.entities {
		if household, ok := entity.(*Household); ok {
			households = append(households, household)
		}
	}

	// Sort by ID for determinism
	sortHouseholdsByID(households)
	return households
}

// GetPersonEdgesSorted returns all edges for a person in deterministic order.
// Sorted by EdgeType, then by ToID.
func (r *InMemoryRepository) GetPersonEdgesSorted(personID EntityID) []*Edge {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var edges []*Edge

	// Get edges from this person
	fromEdgeIDs := r.edgesFromEntity[personID]
	for _, id := range fromEdgeIDs {
		if edge, exists := r.edges[id]; exists {
			edges = append(edges, edge)
		}
	}

	// Get edges to this person
	toEdgeIDs := r.edgesToEntity[personID]
	for _, id := range toEdgeIDs {
		if edge, exists := r.edges[id]; exists {
			edges = append(edges, edge)
		}
	}

	// Sort deterministically: by EdgeType, then ToID
	sortEdgesDeterministically(edges)
	return edges
}

// GetAllEdgesSorted returns all edges in deterministic order.
func (r *InMemoryRepository) GetAllEdgesSorted() []*Edge {
	r.mu.RLock()
	defer r.mu.RUnlock()

	edges := make([]*Edge, 0, len(r.edges))
	for _, edge := range r.edges {
		edges = append(edges, edge)
	}

	sortEdgesDeterministically(edges)
	return edges
}

// Sorting helpers (stdlib sort, no external deps)

func sortPersonsByID(persons []*Person) {
	for i := 0; i < len(persons); i++ {
		for j := i + 1; j < len(persons); j++ {
			if string(persons[i].ID()) > string(persons[j].ID()) {
				persons[i], persons[j] = persons[j], persons[i]
			}
		}
	}
}

func sortOrganizationsByID(orgs []*Organization) {
	for i := 0; i < len(orgs); i++ {
		for j := i + 1; j < len(orgs); j++ {
			if string(orgs[i].ID()) > string(orgs[j].ID()) {
				orgs[i], orgs[j] = orgs[j], orgs[i]
			}
		}
	}
}

func sortHouseholdsByID(households []*Household) {
	for i := 0; i < len(households); i++ {
		for j := i + 1; j < len(households); j++ {
			if string(households[i].ID()) > string(households[j].ID()) {
				households[i], households[j] = households[j], households[i]
			}
		}
	}
}

func sortEdgesDeterministically(edges []*Edge) {
	for i := 0; i < len(edges); i++ {
		for j := i + 1; j < len(edges); j++ {
			// Sort by EdgeType first
			if string(edges[i].EdgeType) > string(edges[j].EdgeType) {
				edges[i], edges[j] = edges[j], edges[i]
			} else if edges[i].EdgeType == edges[j].EdgeType {
				// Then by ToID
				if string(edges[i].ToID) > string(edges[j].ToID) {
					edges[i], edges[j] = edges[j], edges[i]
				}
			}
		}
	}
}

// ============================================================================
// Display Helpers (Phase 13.1)
// ============================================================================

// PrimaryEmail returns the primary email for a person, if known via EdgeTypeOwnsEmail.
// Returns empty string if no email edge exists.
func (r *InMemoryRepository) PrimaryEmail(personID EntityID) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check person entity first
	if entity, exists := r.entities[personID]; exists {
		if person, ok := entity.(*Person); ok {
			if person.PrimaryEmail != "" {
				return person.PrimaryEmail
			}
		}
	}

	// Fall back to edges
	edgeIDs := r.edgesFromEntity[personID]
	var emails []string
	for _, id := range edgeIDs {
		if edge, exists := r.edges[id]; exists {
			if edge.EdgeType == EdgeTypeOwnsEmail {
				if emailEntity, exists := r.entities[edge.ToID]; exists {
					if emailAccount, ok := emailEntity.(*EmailAccount); ok {
						emails = append(emails, emailAccount.Address)
					}
				}
			}
		}
	}

	if len(emails) == 0 {
		return ""
	}

	// Return first email deterministically (sorted)
	for i := 0; i < len(emails); i++ {
		for j := i + 1; j < len(emails); j++ {
			if emails[i] > emails[j] {
				emails[i], emails[j] = emails[j], emails[i]
			}
		}
	}
	return emails[0]
}

// PersonLabel returns a display label for a person.
// Prefers configured name, then DisplayName, then email local-part.
func (r *InMemoryRepository) PersonLabel(personID EntityID) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entity, exists := r.entities[personID]
	if !exists {
		return string(personID)[:16] // Fallback to truncated ID
	}

	person, ok := entity.(*Person)
	if !ok {
		return string(personID)[:16]
	}

	// Prefer DisplayName
	if person.DisplayName != "" {
		return person.DisplayName
	}

	// Fall back to email local-part
	if person.PrimaryEmail != "" {
		return extractLocalPart(person.PrimaryEmail)
	}

	// Final fallback
	return string(personID)[:16]
}

// extractLocalPart extracts the local part of an email address.
func extractLocalPart(email string) string {
	atIdx := strings.Index(email, "@")
	if atIdx < 0 {
		return email
	}
	return email[:atIdx]
}

// IsHouseholdMember checks if a person belongs to any household.
func (r *InMemoryRepository) IsHouseholdMember(personID EntityID) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	edgeIDs := r.edgesFromEntity[personID]
	for _, id := range edgeIDs {
		if edge, exists := r.edges[id]; exists {
			if edge.EdgeType == EdgeTypeMemberOfHH {
				return true
			}
		}
	}
	return false
}

// GetPersonHouseholds returns all households a person belongs to.
func (r *InMemoryRepository) GetPersonHouseholds(personID EntityID) []*Household {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var households []*Household
	edgeIDs := r.edgesFromEntity[personID]
	for _, id := range edgeIDs {
		if edge, exists := r.edges[id]; exists {
			if edge.EdgeType == EdgeTypeMemberOfHH {
				if entity, exists := r.entities[edge.ToID]; exists {
					if household, ok := entity.(*Household); ok {
						households = append(households, household)
					}
				}
			}
		}
	}

	sortHouseholdsByID(households)
	return households
}

// GetPersonOrganizations returns all organizations a person works at.
func (r *InMemoryRepository) GetPersonOrganizations(personID EntityID) []*Organization {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var orgs []*Organization
	edgeIDs := r.edgesFromEntity[personID]
	for _, id := range edgeIDs {
		if edge, exists := r.edges[id]; exists {
			if edge.EdgeType == EdgeTypeWorksAt || edge.EdgeType == EdgeTypeMemberOfOrg {
				if entity, exists := r.entities[edge.ToID]; exists {
					if org, ok := entity.(*Organization); ok {
						orgs = append(orgs, org)
					}
				}
			}
		}
	}

	sortOrganizationsByID(orgs)
	return orgs
}

// Verify interface compliance at compile time.
var (
	_ Repository            = (*InMemoryRepository)(nil)
	_ UnificationRepository = (*InMemoryRepository)(nil)
	_ EdgeRepository        = (*InMemoryRepository)(nil)
)
