package identity

import (
	"strings"
	"time"
)

// Generator creates entities with deterministic IDs.
// All IDs are derived from canonical strings using SHA256.
type Generator struct{}

// NewGenerator creates a new identity generator.
func NewGenerator() *Generator {
	return &Generator{}
}

// PersonFromEmail creates a Person entity from an email address.
// The canonical string is the normalized email address.
func (g *Generator) PersonFromEmail(email string, createdAt time.Time) *Person {
	normalizedEmail := normalizeEmail(email)
	canonicalStr := "person:email:" + normalizedEmail

	return &Person{
		id:           generateID(EntityTypePerson, canonicalStr),
		canonicalStr: canonicalStr,
		createdAt:    createdAt,
		PrimaryEmail: normalizedEmail,
		Aliases:      []string{normalizedEmail},
		Source:       "email",
	}
}

// PersonFromPhone creates a Person entity from a phone number.
func (g *Generator) PersonFromPhone(phone string, createdAt time.Time) *Person {
	normalizedPhone := normalizePhone(phone)
	canonicalStr := "person:phone:" + normalizedPhone

	return &Person{
		id:           generateID(EntityTypePerson, canonicalStr),
		canonicalStr: canonicalStr,
		createdAt:    createdAt,
		PhoneNumber:  normalizedPhone,
		Aliases:      []string{normalizedPhone},
		Source:       "phone",
	}
}

// EmailAccountFromAddress creates an EmailAccount entity.
func (g *Generator) EmailAccountFromAddress(address string, provider string, createdAt time.Time) *EmailAccount {
	normalizedAddress := normalizeEmail(address)
	canonicalStr := "email_account:" + normalizedAddress

	return &EmailAccount{
		id:           generateID(EntityTypeEmailAccount, canonicalStr),
		canonicalStr: canonicalStr,
		createdAt:    createdAt,
		Address:      normalizedAddress,
		Provider:     provider,
		IsPersonal:   !isWorkEmail(normalizedAddress),
		IsWork:       isWorkEmail(normalizedAddress),
	}
}

// CalendarAccountFromID creates a CalendarAccount entity.
func (g *Generator) CalendarAccountFromID(accountID string, provider string, createdAt time.Time) *CalendarAccount {
	canonicalStr := "calendar_account:" + provider + ":" + accountID

	return &CalendarAccount{
		id:           generateID(EntityTypeCalAccount, canonicalStr),
		canonicalStr: canonicalStr,
		createdAt:    createdAt,
		AccountID:    accountID,
		Provider:     provider,
	}
}

// FinanceAccountFromDetails creates a FinanceAccount entity.
func (g *Generator) FinanceAccountFromDetails(
	provider string,
	institution string,
	accountType string,
	maskedNumber string,
	currency string,
	createdAt time.Time,
) *FinanceAccount {
	// Canonical string includes provider + institution + masked number for uniqueness
	canonicalStr := "finance_account:" + provider + ":" + institution + ":" + maskedNumber

	return &FinanceAccount{
		id:           generateID(EntityTypeFinAccount, canonicalStr),
		canonicalStr: canonicalStr,
		createdAt:    createdAt,
		Provider:     provider,
		Institution:  institution,
		AccountType:  accountType,
		MaskedNumber: maskedNumber,
		Currency:     currency,
	}
}

// OrganizationFromDomain creates an Organization entity from a domain.
func (g *Generator) OrganizationFromDomain(domain string, createdAt time.Time) *Organization {
	normalizedDomain := normalizeDomain(domain)
	canonicalStr := "organization:domain:" + normalizedDomain

	return &Organization{
		id:             generateID(EntityTypeOrganization, canonicalStr),
		canonicalStr:   canonicalStr,
		createdAt:      createdAt,
		Domain:         normalizedDomain,
		NormalizedName: domainToName(normalizedDomain),
	}
}

// OrganizationFromMerchant creates an Organization from a merchant name.
func (g *Generator) OrganizationFromMerchant(merchantName string, createdAt time.Time) *Organization {
	normalizedName := normalizeMerchant(merchantName)
	canonicalStr := "organization:merchant:" + normalizedName

	return &Organization{
		id:             generateID(EntityTypeOrganization, canonicalStr),
		canonicalStr:   canonicalStr,
		createdAt:      createdAt,
		Name:           merchantName,
		NormalizedName: normalizedName,
		Aliases:        []string{merchantName},
	}
}

// DeviceFromID creates a Device entity.
func (g *Generator) DeviceFromID(deviceID string, deviceType string, platform string, createdAt time.Time) *Device {
	canonicalStr := "device:" + platform + ":" + deviceID

	return &Device{
		id:           generateID(EntityTypeDevice, canonicalStr),
		canonicalStr: canonicalStr,
		createdAt:    createdAt,
		DeviceID:     deviceID,
		DeviceType:   deviceType,
		Platform:     platform,
	}
}

// CircleFromName creates a Circle entity.
func (g *Generator) CircleFromName(ownerID EntityID, name string, createdAt time.Time) *Circle {
	canonicalStr := "circle:" + string(ownerID) + ":" + strings.ToLower(name)

	return &Circle{
		id:           generateID(EntityTypeCircle, canonicalStr),
		canonicalStr: canonicalStr,
		createdAt:    createdAt,
		Name:         name,
		OwnerID:      ownerID,
	}
}

// SubCircle creates a sub-circle under a parent circle.
func (g *Generator) SubCircle(parentID EntityID, name string, createdAt time.Time) *Circle {
	canonicalStr := "circle:" + string(parentID) + ":" + strings.ToLower(name)

	return &Circle{
		id:           generateID(EntityTypeCircle, canonicalStr),
		canonicalStr: canonicalStr,
		createdAt:    createdAt,
		Name:         name,
		ParentID:     parentID,
	}
}

// IntersectionFromCircles creates an Intersection entity.
func (g *Generator) IntersectionFromCircles(name string, circleIDs []EntityID, createdAt time.Time) *Intersection {
	// Sort circle IDs for deterministic canonical string
	sortedIDs := make([]string, len(circleIDs))
	for i, id := range circleIDs {
		sortedIDs[i] = string(id)
	}
	// Simple sort (stable)
	for i := 0; i < len(sortedIDs)-1; i++ {
		for j := i + 1; j < len(sortedIDs); j++ {
			if sortedIDs[i] > sortedIDs[j] {
				sortedIDs[i], sortedIDs[j] = sortedIDs[j], sortedIDs[i]
			}
		}
	}

	canonicalStr := "intersection:" + strings.Join(sortedIDs, "+")

	return &Intersection{
		id:           generateID(EntityTypeIntersection, canonicalStr),
		canonicalStr: canonicalStr,
		createdAt:    createdAt,
		Name:         name,
		CircleIDs:    circleIDs,
	}
}

// PayeeFromDetails creates a Payee entity.
func (g *Generator) PayeeFromDetails(name string, sortCode string, accountNumber string, createdAt time.Time) *Payee {
	normalizedName := normalizeMerchant(name)
	canonicalStr := "payee:" + sortCode + ":" + accountNumber

	return &Payee{
		id:             generateID(EntityTypePayee, canonicalStr),
		canonicalStr:   canonicalStr,
		createdAt:      createdAt,
		Name:           name,
		NormalizedName: normalizedName,
		AccountDetails: PayeeAccountDetails{
			SortCode:      sortCode,
			AccountNumber: accountNumber,
		},
	}
}

// Normalization helpers

func normalizeEmail(email string) string {
	// Lowercase, trim whitespace
	email = strings.ToLower(strings.TrimSpace(email))

	// Handle Gmail dot-insensitivity and plus-addressing
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return email
	}

	local, domain := parts[0], parts[1]

	// Gmail specific: remove dots and plus-addressing
	if domain == "gmail.com" || domain == "googlemail.com" {
		// Remove dots
		local = strings.ReplaceAll(local, ".", "")
		// Remove plus-addressing
		if idx := strings.Index(local, "+"); idx != -1 {
			local = local[:idx]
		}
		domain = "gmail.com" // Normalize googlemail.com to gmail.com
	}

	return local + "@" + domain
}

func normalizePhone(phone string) string {
	// Remove all non-digit characters except leading +
	var result strings.Builder
	for i, r := range phone {
		if r == '+' && i == 0 {
			result.WriteRune(r)
		} else if r >= '0' && r <= '9' {
			result.WriteRune(r)
		}
	}
	return result.String()
}

func normalizeDomain(domain string) string {
	domain = strings.ToLower(strings.TrimSpace(domain))
	// Remove www. prefix
	domain = strings.TrimPrefix(domain, "www.")
	return domain
}

func domainToName(domain string) string {
	// Simple conversion: amazon.co.uk -> Amazon
	parts := strings.Split(domain, ".")
	if len(parts) > 0 {
		name := parts[0]
		if len(name) > 0 {
			return strings.ToUpper(name[:1]) + name[1:]
		}
	}
	return domain
}

func normalizeMerchant(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))

	// Remove common noise tokens
	noiseTokens := []string{
		"pos ", "card ", "contactless ", "visa ", "mastercard ",
		"debit ", "credit ", "purchase ", "payment to ", "payment from ",
		"direct debit ", "standing order ", "faster payment ",
	}
	for _, token := range noiseTokens {
		name = strings.ReplaceAll(name, token, "")
	}

	// Remove trailing numbers (store IDs)
	// Simple approach: trim trailing digits and spaces
	name = strings.TrimSpace(name)
	for len(name) > 0 {
		last := name[len(name)-1]
		if last >= '0' && last <= '9' || last == ' ' || last == '#' {
			name = name[:len(name)-1]
		} else {
			break
		}
	}

	return strings.TrimSpace(name)
}

func isWorkEmail(email string) bool {
	// Simple heuristic: personal email providers
	personalDomains := []string{
		"gmail.com", "googlemail.com", "yahoo.com", "yahoo.co.uk",
		"hotmail.com", "hotmail.co.uk", "outlook.com", "live.com",
		"icloud.com", "me.com", "aol.com", "protonmail.com",
	}

	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return false
	}
	domain := parts[1]

	for _, pd := range personalDomains {
		if domain == pd {
			return false
		}
	}
	return true
}
