package identity

import (
	"errors"
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

// InMemoryRepository is a thread-safe in-memory implementation of UnificationRepository.
type InMemoryRepository struct {
	mu       sync.RWMutex
	entities map[EntityID]Entity

	// Indexes for fast lookup
	emailToPerson  map[string]EntityID     // normalized email -> person ID
	domainToOrg    map[string]EntityID     // normalized domain -> org ID
	merchantToOrg  map[string]EntityID     // normalized merchant -> org ID
	emailToAccount map[string]EntityID     // normalized email -> email account ID
	personToEmails map[EntityID][]EntityID // person ID -> email account IDs
}

// NewInMemoryRepository creates a new in-memory repository.
func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		entities:       make(map[EntityID]Entity),
		emailToPerson:  make(map[string]EntityID),
		domainToOrg:    make(map[string]EntityID),
		merchantToOrg:  make(map[string]EntityID),
		emailToAccount: make(map[string]EntityID),
		personToEmails: make(map[EntityID][]EntityID),
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

// Verify interface compliance at compile time.
var (
	_ Repository            = (*InMemoryRepository)(nil)
	_ UnificationRepository = (*InMemoryRepository)(nil)
)
