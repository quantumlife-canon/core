// Package identity provides deterministic identity generation and entity unification
// for the QuantumLife identity graph.
//
// The identity graph enables:
// - Deterministic ID generation from canonical strings
// - Multi-account unification (same person across 20 email accounts)
// - Entity reference tracking across all ingested data
//
// Reference: docs/IDENTITY_GRAPH_V1.md
package identity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// EntityType identifies the kind of entity in the identity graph.
type EntityType string

const (
	EntityTypePerson       EntityType = "person"
	EntityTypeOrganization EntityType = "organization"
	EntityTypeEmailAccount EntityType = "email_account"
	EntityTypeCalAccount   EntityType = "calendar_account"
	EntityTypeFinAccount   EntityType = "finance_account"
	EntityTypeDevice       EntityType = "device"
	EntityTypeCircle       EntityType = "circle"
	EntityTypeIntersection EntityType = "intersection"
	EntityTypeVendor       EntityType = "vendor"
	EntityTypePayee        EntityType = "payee"
	EntityTypeHousehold    EntityType = "household"
	EntityTypePhoneNumber  EntityType = "phone_number"
)

// EdgeType identifies a relationship between entities in the identity graph.
type EdgeType string

const (
	// Ownership edges
	EdgeTypeOwnsEmail    EdgeType = "owns_email"    // Person -> EmailAccount
	EdgeTypeOwnsPhone    EdgeType = "owns_phone"    // Person -> PhoneNumber
	EdgeTypeOwnsCalendar EdgeType = "owns_calendar" // Person -> CalendarAccount
	EdgeTypeOwnsDevice   EdgeType = "owns_device"   // Person -> Device

	// Organizational edges
	EdgeTypeMemberOfOrg EdgeType = "member_of_org" // Person -> Organization
	EdgeTypeWorksAt     EdgeType = "works_at"      // Person -> Organization

	// Family edges
	EdgeTypeSpouseOf   EdgeType = "spouse_of"    // Person -> Person
	EdgeTypeParentOf   EdgeType = "parent_of"    // Person -> Person
	EdgeTypeChildOf    EdgeType = "child_of"     // Person -> Person
	EdgeTypeMemberOfHH EdgeType = "member_of_hh" // Person -> Household

	// Vendor edges
	EdgeTypeVendorOf EdgeType = "vendor_of" // Organization -> Person (org sells to person)

	// Alias edges (for unification)
	EdgeTypeAliasOf EdgeType = "alias_of" // Entity -> Entity (same real-world entity)
)

// Confidence represents the certainty of an identity link.
type Confidence string

const (
	ConfidenceHigh   Confidence = "high"   // Explicit configuration or exact match
	ConfidenceMedium Confidence = "medium" // Strong heuristic match
	ConfidenceLow    Confidence = "low"    // Weak heuristic match
)

// Edge represents a directional relationship between two entities.
type Edge struct {
	id           EntityID
	canonicalStr string
	createdAt    time.Time

	// Edge details
	EdgeType   EdgeType
	FromID     EntityID
	ToID       EntityID
	Confidence Confidence
	Provenance string // Where this relationship was discovered
}

func (e *Edge) ID() EntityID            { return e.id }
func (e *Edge) Type() EntityType        { return EntityType("edge") }
func (e *Edge) CanonicalString() string { return e.canonicalStr }
func (e *Edge) CreatedAt() time.Time    { return e.createdAt }

// NewEdge creates a new edge between entities.
func NewEdge(edgeType EdgeType, fromID, toID EntityID, confidence Confidence, provenance string, createdAt time.Time) *Edge {
	canonicalStr := fmt.Sprintf("edge|%s|%s|%s", edgeType, fromID, toID)
	return &Edge{
		id:           generateID(EntityType("edge"), canonicalStr),
		canonicalStr: canonicalStr,
		createdAt:    createdAt,
		EdgeType:     edgeType,
		FromID:       fromID,
		ToID:         toID,
		Confidence:   confidence,
		Provenance:   provenance,
	}
}

// EntityID is a deterministic identifier for an entity.
// Format: {type}_{hash_prefix}
// Example: person_a1b2c3d4, email_account_e5f6g7h8
type EntityID string

// Type extracts the entity type from the ID.
func (id EntityID) Type() EntityType {
	s := string(id)
	idx := strings.LastIndex(s, "_")
	if idx == -1 {
		return ""
	}
	return EntityType(s[:idx])
}

// Hash extracts the hash portion from the ID.
func (id EntityID) Hash() string {
	s := string(id)
	idx := strings.LastIndex(s, "_")
	if idx == -1 {
		return s
	}
	return s[idx+1:]
}

// Entity is the base interface for all identity graph entities.
type Entity interface {
	// ID returns the deterministic entity ID.
	ID() EntityID

	// Type returns the entity type.
	Type() EntityType

	// CanonicalString returns the string used for ID generation.
	// This MUST be deterministic and stable.
	CanonicalString() string

	// CreatedAt returns when the entity was first observed.
	CreatedAt() time.Time
}

// Person represents a human in the identity graph.
// A person may have multiple email accounts, devices, etc.
type Person struct {
	id           EntityID
	canonicalStr string
	createdAt    time.Time

	// Primary identifier (email, phone, or name)
	PrimaryEmail string
	DisplayName  string
	PhoneNumber  string

	// Linked entities
	EmailAccounts []EntityID
	Devices       []EntityID

	// Aliases for unification
	Aliases []string

	// Metadata
	Source string // Where this person was first observed
}

func (p *Person) ID() EntityID            { return p.id }
func (p *Person) Type() EntityType        { return EntityTypePerson }
func (p *Person) CanonicalString() string { return p.canonicalStr }
func (p *Person) CreatedAt() time.Time    { return p.createdAt }

// EmailAccount represents an email address as an entity.
type EmailAccount struct {
	id           EntityID
	canonicalStr string
	createdAt    time.Time

	// Email details
	Address     string
	DisplayName string
	Provider    string // gmail, outlook, etc.

	// Owner (if known)
	OwnerID EntityID

	// Account type
	IsPersonal bool
	IsWork     bool
}

func (e *EmailAccount) ID() EntityID            { return e.id }
func (e *EmailAccount) Type() EntityType        { return EntityTypeEmailAccount }
func (e *EmailAccount) CanonicalString() string { return e.canonicalStr }
func (e *EmailAccount) CreatedAt() time.Time    { return e.createdAt }

// CalendarAccount represents a calendar account.
type CalendarAccount struct {
	id           EntityID
	canonicalStr string
	createdAt    time.Time

	// Calendar details
	AccountID   string
	Provider    string // google, outlook, apple
	DisplayName string

	// Owner
	OwnerID EntityID
}

func (c *CalendarAccount) ID() EntityID            { return c.id }
func (c *CalendarAccount) Type() EntityType        { return EntityTypeCalAccount }
func (c *CalendarAccount) CanonicalString() string { return c.canonicalStr }
func (c *CalendarAccount) CreatedAt() time.Time    { return c.createdAt }

// FinanceAccount represents a bank account or financial account.
type FinanceAccount struct {
	id           EntityID
	canonicalStr string
	createdAt    time.Time

	// Account details
	AccountID    string
	Provider     string // plaid, truelayer, etc.
	Institution  string // Bank name
	AccountType  string // checking, savings, credit
	MaskedNumber string // ****1234
	Currency     string

	// Owner
	OwnerID EntityID

	// Shared ownership (for joint accounts)
	SharedWith []EntityID
}

func (f *FinanceAccount) ID() EntityID            { return f.id }
func (f *FinanceAccount) Type() EntityType        { return EntityTypeFinAccount }
func (f *FinanceAccount) CanonicalString() string { return f.canonicalStr }
func (f *FinanceAccount) CreatedAt() time.Time    { return f.createdAt }

// Organization represents a company, institution, or merchant.
type Organization struct {
	id           EntityID
	canonicalStr string
	createdAt    time.Time

	// Organization details
	Name           string
	Domain         string
	Category       string
	NormalizedName string

	// Aliases for merchant unification
	Aliases []string
}

func (o *Organization) ID() EntityID            { return o.id }
func (o *Organization) Type() EntityType        { return EntityTypeOrganization }
func (o *Organization) CanonicalString() string { return o.canonicalStr }
func (o *Organization) CreatedAt() time.Time    { return o.createdAt }

// Device represents a user's device.
type Device struct {
	id           EntityID
	canonicalStr string
	createdAt    time.Time

	// Device details
	DeviceID   string
	DeviceType string // phone, watch, tablet
	Platform   string // ios, android
	Model      string

	// Owner
	OwnerID EntityID
}

func (d *Device) ID() EntityID            { return d.id }
func (d *Device) Type() EntityType        { return EntityTypeDevice }
func (d *Device) CanonicalString() string { return d.canonicalStr }
func (d *Device) CreatedAt() time.Time    { return d.createdAt }

// Circle represents a user's life domain.
type Circle struct {
	id           EntityID
	canonicalStr string
	createdAt    time.Time

	// Circle details
	Name        string
	ParentID    EntityID // For sub-circles
	OwnerID     EntityID
	Description string
}

func (c *Circle) ID() EntityID            { return c.id }
func (c *Circle) Type() EntityType        { return EntityTypeCircle }
func (c *Circle) CanonicalString() string { return c.canonicalStr }
func (c *Circle) CreatedAt() time.Time    { return c.createdAt }

// Intersection represents a shared domain between circles.
type Intersection struct {
	id           EntityID
	canonicalStr string
	createdAt    time.Time

	// Intersection details
	Name       string
	CircleIDs  []EntityID
	PolicyHash string
}

func (i *Intersection) ID() EntityID            { return i.id }
func (i *Intersection) Type() EntityType        { return EntityTypeIntersection }
func (i *Intersection) CanonicalString() string { return i.canonicalStr }
func (i *Intersection) CreatedAt() time.Time    { return i.createdAt }

// Payee represents a registered payment recipient.
type Payee struct {
	id           EntityID
	canonicalStr string
	createdAt    time.Time

	// Payee details
	Name           string
	NormalizedName string
	AccountDetails PayeeAccountDetails
	OrganizationID EntityID // Link to org if known
}

func (p *Payee) ID() EntityID            { return p.id }
func (p *Payee) Type() EntityType        { return EntityTypePayee }
func (p *Payee) CanonicalString() string { return p.canonicalStr }
func (p *Payee) CreatedAt() time.Time    { return p.createdAt }

// PayeeAccountDetails holds payment destination info.
type PayeeAccountDetails struct {
	SortCode      string
	AccountNumber string
	IBAN          string
	BIC           string
}

// Household represents a family unit or shared living arrangement.
type Household struct {
	id           EntityID
	canonicalStr string
	createdAt    time.Time

	// Household details
	Name    string     // e.g., "Rajan Household"
	Members []EntityID // Person IDs
	Address string     // Optional
}

func (h *Household) ID() EntityID            { return h.id }
func (h *Household) Type() EntityType        { return EntityTypeHousehold }
func (h *Household) CanonicalString() string { return h.canonicalStr }
func (h *Household) CreatedAt() time.Time    { return h.createdAt }

// PhoneNumber represents a phone number as a first-class entity.
// This allows tracking ownership and linking across providers.
type PhoneNumber struct {
	id           EntityID
	canonicalStr string
	createdAt    time.Time

	// Phone details
	Number     string   // Normalized E.164 format
	RawNumbers []string // Original formats seen
	Provider   string   // Carrier if known
	IsVerified bool

	// Owner (if known)
	OwnerID EntityID
}

func (p *PhoneNumber) ID() EntityID            { return p.id }
func (p *PhoneNumber) Type() EntityType        { return EntityTypePhoneNumber }
func (p *PhoneNumber) CanonicalString() string { return p.canonicalStr }
func (p *PhoneNumber) CreatedAt() time.Time    { return p.createdAt }

// EntityRef is a lightweight reference to an entity.
// Used in events to link to identity graph without embedding full entity.
type EntityRef struct {
	ID   EntityID   `json:"id"`
	Type EntityType `json:"type"`
	Name string     `json:"name,omitempty"` // Display hint
}

// NewEntityRef creates a reference from an entity.
func NewEntityRef(e Entity, displayName string) EntityRef {
	return EntityRef{
		ID:   e.ID(),
		Type: e.Type(),
		Name: displayName,
	}
}

// generateID creates a deterministic ID from type and canonical string.
func generateID(entityType EntityType, canonicalStr string) EntityID {
	// Hash the canonical string
	hash := sha256.Sum256([]byte(canonicalStr))
	hashHex := hex.EncodeToString(hash[:])

	// Use first 16 chars of hash for readability
	return EntityID(fmt.Sprintf("%s_%s", entityType, hashHex[:16]))
}
