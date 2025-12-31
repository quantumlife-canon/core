# QuantumLife Identity Graph v1

## Document Status

| Field | Value |
|-------|-------|
| Version | 1.0 |
| Status | Draft |
| Author | QuantumLife Core Team |
| Date | 2025-01-01 |
| Related | ARCHITECTURE_LIFE_OS_V1.md, SATISH_CIRCLES_TAXONOMY_V1.md |

---

## 1. Purpose

The Identity Graph defines how QuantumLife represents and unifies entities across multiple data sources. When the same person, organization, or concept appears in different systems (email, calendar, bank, messaging), the Identity Graph ensures they are recognized as the same entity.

**Goal**: A single, unified view of "who" and "what" across all of Satish's digital life.

---

## 2. Core Entities

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        IDENTITY GRAPH SCHEMA                                 │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                           PERSON                                     │   │
│   │   Represents a human being across all contexts                      │   │
│   ├─────────────────────────────────────────────────────────────────────┤   │
│   │   person_id: string (deterministic, stable)                         │   │
│   │   canonical_name: string                                            │   │
│   │   relationship: enum (self, spouse, child, parent, friend,          │   │
│   │                       colleague, acquaintance, unknown)             │   │
│   │   circles: []CircleRef                                              │   │
│   │   identifiers: []PersonIdentifier                                   │   │
│   │   created_at: timestamp                                             │   │
│   │   last_seen_at: timestamp                                           │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                        ORGANIZATION                                  │   │
│   │   Represents a company, institution, or group                       │   │
│   ├─────────────────────────────────────────────────────────────────────┤   │
│   │   org_id: string (deterministic, stable)                            │   │
│   │   canonical_name: string                                            │   │
│   │   org_type: enum (employer, school, bank, utility, merchant,        │   │
│   │                   government, healthcare, other)                    │   │
│   │   circles: []CircleRef                                              │   │
│   │   identifiers: []OrgIdentifier                                      │   │
│   │   created_at: timestamp                                             │   │
│   │   last_seen_at: timestamp                                           │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                           ACCOUNT                                    │   │
│   │   Represents a financial account                                    │   │
│   ├─────────────────────────────────────────────────────────────────────┤   │
│   │   account_id: string (deterministic, stable)                        │   │
│   │   canonical_name: string                                            │   │
│   │   account_type: enum (current, savings, credit, loan, investment)   │   │
│   │   institution: OrgRef                                               │   │
│   │   owners: []PersonRef (for joint accounts)                          │   │
│   │   identifiers: []AccountIdentifier                                  │   │
│   │   created_at: timestamp                                             │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                            PAYEE                                     │   │
│   │   Represents a payment recipient (v9.10 payee registry)             │   │
│   ├─────────────────────────────────────────────────────────────────────┤   │
│   │   payee_id: string (matches v9.10 PayeeID)                          │   │
│   │   canonical_name: string                                            │   │
│   │   payee_type: enum (utility, merchant, individual, organization)    │   │
│   │   linked_entity: PersonRef | OrgRef                                 │   │
│   │   payment_details: PaymentDetails (redacted)                        │   │
│   │   created_at: timestamp                                             │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                           DEVICE                                     │   │
│   │   Represents a user device                                          │   │
│   ├─────────────────────────────────────────────────────────────────────┤   │
│   │   device_id: string                                                 │   │
│   │   device_type: enum (iphone, ipad, watch, web)                      │   │
│   │   owner: PersonRef                                                  │   │
│   │   push_token: string (encrypted)                                    │   │
│   │   last_active: timestamp                                            │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 3. Identifier Types

### 3.1 Person Identifiers

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                      PERSON IDENTIFIER TYPES                                 │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   Type: EMAIL                                                                │
│   ─────────────                                                              │
│   - value: "satish@example.com"                                             │
│   - source: "gmail_inbox_personal"                                          │
│   - confidence: 1.0 (explicit)                                              │
│   - verified: true                                                          │
│                                                                              │
│   Type: PHONE                                                                │
│   ─────────────                                                              │
│   - value: "+447700900123" (E.164 format)                                   │
│   - source: "contacts_sync"                                                 │
│   - confidence: 0.9                                                         │
│   - verified: false                                                         │
│                                                                              │
│   Type: SOCIAL_HANDLE                                                        │
│   ────────────────────                                                       │
│   - platform: "whatsapp" | "linkedin" | "twitter" | "github"                │
│   - value: "@satish_founder"                                                │
│   - source: "profile_scrape"                                                │
│   - confidence: 0.8                                                         │
│                                                                              │
│   Type: CALENDAR_ORGANIZER                                                   │
│   ────────────────────────                                                   │
│   - value: "satish@company.com"                                             │
│   - source: "google_calendar"                                               │
│   - confidence: 0.95                                                        │
│                                                                              │
│   Type: BANK_ACCOUNT_HOLDER                                                  │
│   ────────────────────────────                                               │
│   - value: "Mr Satish Kumar" (as appears on account)                        │
│   - source: "plaid_account"                                                 │
│   - confidence: 1.0                                                         │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 3.2 Organization Identifiers

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    ORGANIZATION IDENTIFIER TYPES                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   Type: DOMAIN                                                               │
│   ────────────                                                               │
│   - value: "company.com"                                                    │
│   - source: "email_sender"                                                  │
│   - confidence: 0.95                                                        │
│                                                                              │
│   Type: COMPANY_NUMBER                                                       │
│   ─────────────────────                                                      │
│   - value: "12345678" (Companies House UK)                                  │
│   - source: "manual_entry"                                                  │
│   - confidence: 1.0                                                         │
│                                                                              │
│   Type: SORT_CODE_PREFIX                                                     │
│   ───────────────────────                                                    │
│   - value: "20-00" (Barclays)                                               │
│   - source: "plaid_transaction"                                             │
│   - confidence: 0.9                                                         │
│                                                                              │
│   Type: MERCHANT_ID                                                          │
│   ─────────────────                                                          │
│   - value: "AMAZON UK RETAIL"                                               │
│   - source: "plaid_transaction"                                             │
│   - confidence: 0.85                                                        │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 4. Deterministic ID Generation

### 4.1 Person ID

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    PERSON ID GENERATION                                      │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   Algorithm: Canonical hash of primary identifier                           │
│                                                                              │
│   Step 1: Select primary identifier                                         │
│   - Priority: email > phone > social_handle                                 │
│   - Use lowest (lexicographically) if multiple at same priority             │
│                                                                              │
│   Step 2: Normalize identifier                                              │
│   - Email: lowercase, trim whitespace                                       │
│   - Phone: E.164 format                                                     │
│   - Social: lowercase, remove @ prefix                                      │
│                                                                              │
│   Step 3: Generate ID                                                       │
│   - person_id = "person_" + sha256(normalized_identifier)[:16]              │
│                                                                              │
│   Example:                                                                   │
│   - Input: "Satish@Example.COM"                                             │
│   - Normalized: "satish@example.com"                                        │
│   - Hash: sha256("satish@example.com") = "a1b2c3d4..."                      │
│   - person_id: "person_a1b2c3d4e5f6g7h8"                                    │
│                                                                              │
│   Stability: ID remains stable as long as primary identifier unchanged      │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 4.2 Organization ID

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    ORGANIZATION ID GENERATION                                │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   Algorithm: Canonical hash of primary identifier                           │
│                                                                              │
│   Step 1: Select primary identifier                                         │
│   - Priority: company_number > domain > merchant_id                         │
│                                                                              │
│   Step 2: Normalize identifier                                              │
│   - Company number: remove spaces, uppercase                                │
│   - Domain: lowercase, remove www prefix                                    │
│   - Merchant ID: uppercase, remove spaces                                   │
│                                                                              │
│   Step 3: Generate ID                                                       │
│   - org_id = "org_" + sha256(normalized_identifier)[:16]                    │
│                                                                              │
│   Example:                                                                   │
│   - Input domain: "www.Barclays.co.uk"                                      │
│   - Normalized: "barclays.co.uk"                                            │
│   - org_id: "org_b4rc14y5..."                                               │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 4.3 Account ID

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    ACCOUNT ID GENERATION                                     │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   Algorithm: Hash of institution + account number (last 4 only for privacy) │
│                                                                              │
│   Step 1: Get institution identifier                                        │
│   - Use org_id of institution                                               │
│                                                                              │
│   Step 2: Get account suffix                                                │
│   - Last 4 digits of account number                                         │
│   - Or Plaid account_id if available                                        │
│                                                                              │
│   Step 3: Generate ID                                                       │
│   - account_id = "acct_" + sha256(org_id + "|" + suffix)[:16]               │
│                                                                              │
│   Example:                                                                   │
│   - Institution: org_barclays123                                            │
│   - Account suffix: "7890"                                                  │
│   - account_id: "acct_bc1a2b3c..."                                          │
│                                                                              │
│   Privacy: Full account numbers never stored in plain text                  │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 5. Multi-Account Unification

### 5.1 The Problem

Satish appears differently across systems:
- Gmail personal: `satish.kumar@gmail.com`
- Gmail work: `satish@company.com`
- WhatsApp: `+447700900123`
- Bank: `Mr S Kumar`
- Calendar: `Satish Kumar`

These must all resolve to the same `person_id`.

### 5.2 Unification Algorithm

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    IDENTITY UNIFICATION FLOW                                 │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   When new identifier observed:                                             │
│                                                                              │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │   STEP 1: Check exact match                                          │   │
│   │                                                                      │   │
│   │   SELECT person_id FROM person_identifiers                          │   │
│   │   WHERE identifier_type = :type                                     │   │
│   │   AND identifier_value = :normalized_value                          │   │
│   │                                                                      │   │
│   │   IF found: RETURN existing person_id                               │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                            │                                                 │
│                            ▼                                                 │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │   STEP 2: Check linked identifiers                                   │   │
│   │                                                                      │   │
│   │   IF identifier came with context (e.g., email signature with phone):│   │
│   │       Look up other identifier                                      │   │
│   │       IF found: Link new identifier to existing person              │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                            │                                                 │
│                            ▼                                                 │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │   STEP 3: Check name similarity (for known contexts)                 │   │
│   │                                                                      │   │
│   │   IF identifier is from trusted source (bank, calendar):            │   │
│   │       Extract name from identifier context                          │   │
│   │       Compare to existing persons with fuzzy match                  │   │
│   │       IF high confidence match (>0.9): Suggest merge (manual)       │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                            │                                                 │
│                            ▼                                                 │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │   STEP 4: Create new person                                          │   │
│   │                                                                      │   │
│   │   IF no match found:                                                │   │
│   │       Generate new person_id                                        │   │
│   │       Create person with this identifier as primary                 │   │
│   │       Set relationship = "unknown" until classified                 │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 5.3 Manual Merge

When automatic unification fails or is ambiguous:

```
User sees: "These might be the same person. Merge?"

Person A: satish.kumar@gmail.com
  - Seen in: Email (47 messages)
  - Circle: Personal

Person B: satish@company.com
  - Seen in: Calendar (23 events)
  - Circle: Work

[Merge] [Keep Separate] [Decide Later]
```

If merged:
- All identifiers linked to single person_id
- Audit event: `identity.merged`
- Historical items re-associated

---

## 6. Relationship Classification

### 6.1 Relationship Types

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    RELATIONSHIP HIERARCHY                                    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   SELF                                                                       │
│   - The user (Satish)                                                       │
│   - Only one person has this relationship                                   │
│                                                                              │
│   FAMILY_IMMEDIATE                                                           │
│   - spouse: Wife                                                            │
│   - child: Son, Daughter                                                    │
│   - parent: Mother, Father                                                  │
│   - sibling: Brother, Sister                                                │
│                                                                              │
│   FAMILY_EXTENDED                                                            │
│   - in_law: Mother-in-law, Father-in-law, etc.                              │
│   - cousin, aunt, uncle, grandparent                                        │
│                                                                              │
│   PROFESSIONAL                                                               │
│   - manager: Direct manager                                                 │
│   - report: Direct report                                                   │
│   - colleague: Same organization                                            │
│   - client: External client                                                 │
│   - vendor: External vendor                                                 │
│                                                                              │
│   SOCIAL                                                                     │
│   - close_friend: High interaction, personal                                │
│   - friend: Regular interaction                                             │
│   - acquaintance: Occasional interaction                                    │
│                                                                              │
│   SERVICE                                                                    │
│   - service_provider: Cleaner, plumber, etc.                                │
│   - healthcare: Doctor, dentist                                             │
│   - education: Teacher, tutor                                               │
│                                                                              │
│   UNKNOWN                                                                    │
│   - Default until classified                                                │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 6.2 Automatic Classification

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    RELATIONSHIP INFERENCE                                    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   Signal: Email domain                                                       │
│   - Same domain as work email → colleague                                   │
│   - @school.edu → education                                                 │
│   - @nhs.uk → healthcare                                                    │
│                                                                              │
│   Signal: Calendar co-occurrence                                            │
│   - Frequent 1:1 meetings → colleague or manager                            │
│   - Family calendar events → family                                         │
│   - Weekend social events → friend                                          │
│                                                                              │
│   Signal: Message frequency and timing                                      │
│   - High frequency, personal hours → close relationship                     │
│   - Low frequency, business hours → professional                            │
│                                                                              │
│   Signal: Contact labels                                                     │
│   - Contact saved as "Mum" → parent                                         │
│   - Contact in "Work" group → colleague                                     │
│                                                                              │
│   Signal: Transaction patterns                                               │
│   - Regular payments to individual → service_provider                       │
│   - Shared account → spouse                                                 │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 7. Circle Assignment

### 7.1 Person → Circle Mapping

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    CIRCLE ASSIGNMENT RULES                                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   Relationship              Primary Circle     Secondary Circles            │
│   ────────────              ──────────────     ─────────────────            │
│   spouse                    family             finance (joint accounts)     │
│   child                     family             kids_school                  │
│   parent                    family             -                            │
│   manager                   work               -                            │
│   colleague                 work               -                            │
│   close_friend              social             -                            │
│   service_provider          home               finance (payments)           │
│   healthcare                health             -                            │
│   education                 kids_school        -                            │
│   unknown                   (inferred)         -                            │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 7.2 Organization → Circle Mapping

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    ORGANIZATION CIRCLE ASSIGNMENT                            │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   Org Type                  Primary Circle     Notes                        │
│   ────────                  ──────────────     ─────                        │
│   employer                  work               User's employer              │
│   school                    kids_school        Children's schools           │
│   bank                      finance            All financial institutions   │
│   utility                   finance            Bills and payments           │
│   merchant                  finance            Regular purchases            │
│   healthcare                health             NHS, private healthcare      │
│   government                finance            HMRC, council                │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 8. Privacy Considerations

### 8.1 Data Minimization

- Only identifiers necessary for unification are stored
- Full phone numbers hashed; only last 4 digits visible
- Full account numbers never stored in plain text
- Email addresses stored but can be redacted in exports

### 8.2 Consent Model

- User explicitly connects each data source
- User can disconnect source and delete associated identifiers
- User can request full identity graph export (GDPR)
- User can request deletion of specific persons/identifiers

### 8.3 Cross-User Isolation

- Identity graphs are strictly per-user
- No cross-user entity resolution
- Multi-user households: each user has separate graph
- Shared entities (spouse) exist independently in each graph

---

## 9. Audit Events

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    IDENTITY GRAPH AUDIT EVENTS                               │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   identity.person.created                                                    │
│   - New person entity created                                               │
│   - Includes: person_id, primary_identifier, source                         │
│                                                                              │
│   identity.identifier.added                                                  │
│   - New identifier linked to existing person                                │
│   - Includes: person_id, identifier_type, source, confidence                │
│                                                                              │
│   identity.merged                                                            │
│   - Two persons merged into one                                             │
│   - Includes: source_person_id, target_person_id, trigger (auto/manual)     │
│                                                                              │
│   identity.relationship.classified                                           │
│   - Relationship type assigned or changed                                   │
│   - Includes: person_id, old_relationship, new_relationship, confidence     │
│                                                                              │
│   identity.circle.assigned                                                   │
│   - Person assigned to circle                                               │
│   - Includes: person_id, circle_id, reason                                  │
│                                                                              │
│   identity.deleted                                                           │
│   - Person or identifier deleted (user request)                             │
│   - Includes: entity_type, entity_id, reason                                │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 10. Example: Satish's Identity Graph

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    SATISH'S CORE IDENTITY GRAPH                              │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   SELF: person_satish_abc123                                                │
│   ├── email: satish.kumar@gmail.com (personal)                              │
│   ├── email: satish@company.com (work)                                      │
│   ├── email: satish.uk@outlook.com (backup)                                 │
│   ├── phone: +447700900123                                                  │
│   ├── whatsapp: +447700900123                                               │
│   └── github: @satish-founder                                               │
│                                                                              │
│   SPOUSE: person_wife_def456                                                │
│   ├── email: wife@gmail.com                                                 │
│   ├── phone: +447700900124                                                  │
│   ├── relationship: spouse                                                  │
│   └── circles: [family, finance]                                            │
│                                                                              │
│   CHILD_1: person_son_ghi789                                                │
│   ├── email: son@school.edu                                                 │
│   ├── relationship: child                                                   │
│   └── circles: [family, kids_school]                                        │
│                                                                              │
│   CHILD_2: person_daughter_jkl012                                           │
│   ├── email: daughter@school.edu                                            │
│   ├── relationship: child                                                   │
│   └── circles: [family, kids_school]                                        │
│                                                                              │
│   MANAGER: person_manager_mno345                                            │
│   ├── email: manager@company.com                                            │
│   ├── relationship: manager                                                 │
│   └── circles: [work]                                                       │
│                                                                              │
│   CLEANER: person_cleaner_pqr678                                            │
│   ├── phone: +447700900999                                                  │
│   ├── relationship: service_provider                                        │
│   └── circles: [home, finance]                                              │
│       └── payee: payee_cleaner_xyz (v9.10 registry)                         │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 11. Document History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2025-01-01 | Core Team | Initial version |
