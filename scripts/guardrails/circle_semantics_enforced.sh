#!/bin/bash
# Phase 45: Circle Semantics & Necessity Declaration Guardrails
# Ensures meaning-only semantics layer does NOT wire into decision/delivery/execution.

set -e

PASS=0
FAIL=0

pass() {
    echo -e "  \033[0;32m✓\033[0m $1"
    PASS=$((PASS + 1))
}

fail() {
    echo -e "  \033[0;31m✗\033[0m $1"
    FAIL=$((FAIL + 1))
}

# ============================================================================
echo "--- Section 1: File Existence ---"
# ============================================================================

[ -f "pkg/domain/circlesemantics/types.go" ] && pass "Domain types file exists" || fail "Domain types file missing"
[ -f "internal/circlesemantics/engine.go" ] && pass "Engine file exists" || fail "Engine file missing"
[ -f "internal/persist/circle_semantics_store.go" ] && pass "Store file exists" || fail "Store file missing"
[ -f "docs/ADR/ADR-0083-phase45-circle-semantics-necessity.md" ] && pass "ADR file exists" || fail "ADR file missing"
[ -f "internal/demo_phase45_circle_semantics/demo_test.go" ] && pass "Demo test file exists" || fail "Demo test file missing"

# ============================================================================
echo "--- Section 2: Package Headers ---"
# ============================================================================

grep -q "package circlesemantics" pkg/domain/circlesemantics/types.go && pass "Domain package header present" || fail "Domain package header missing"
grep -q "package circlesemantics" internal/circlesemantics/engine.go && pass "Engine package header present" || fail "Engine package header missing"
grep -q "package persist" internal/persist/circle_semantics_store.go && pass "Store package header present" || fail "Store package header missing"

# ============================================================================
echo "--- Section 3: No Goroutines ---"
# ============================================================================

! grep -r "go func" pkg/domain/circlesemantics/ 2>/dev/null && pass "No 'go func' in domain package" || fail "Found 'go func' in domain package"
! grep -r "go func" internal/circlesemantics/ 2>/dev/null && pass "No 'go func' in engine package" || fail "Found 'go func' in engine package"
! grep -rE "^\s+go [a-zA-Z]" pkg/domain/circlesemantics/ 2>/dev/null && pass "No goroutine spawn in domain" || fail "Found goroutine spawn in domain"
! grep -rE "^\s+go [a-zA-Z]" internal/circlesemantics/ 2>/dev/null && pass "No goroutine spawn in engine" || fail "Found goroutine spawn in engine"

# ============================================================================
echo "--- Section 4: Clock Injection ---"
# ============================================================================

! grep -r "time\.Now()" pkg/domain/circlesemantics/ 2>/dev/null && pass "No time.Now() in domain" || fail "Found time.Now() in domain"
! grep -r "time\.Now()" internal/circlesemantics/ 2>/dev/null && pass "No time.Now() in engine" || fail "Found time.Now() in engine"

# ============================================================================
echo "--- Section 5: Forbidden Patterns ---"
# ============================================================================

! grep -rE "@[a-zA-Z0-9]" pkg/domain/circlesemantics/ 2>/dev/null && pass "No @ pattern in domain" || fail "Found @ pattern in domain"
! grep -rE "http://" pkg/domain/circlesemantics/ 2>/dev/null && pass "No http:// in domain" || fail "Found http:// in domain"
! grep -rE "https://" pkg/domain/circlesemantics/ 2>/dev/null && pass "No https:// in domain" || fail "Found https:// in domain"
! grep -ri "gmail" pkg/domain/circlesemantics/ 2>/dev/null && pass "No gmail in domain" || fail "Found gmail in domain"
! grep -ri "truelayer" pkg/domain/circlesemantics/ 2>/dev/null && pass "No truelayer in domain" || fail "Found truelayer in domain"
! grep -ri "merchant" pkg/domain/circlesemantics/ 2>/dev/null && pass "No merchant in domain" || fail "Found merchant in domain"
! grep -ri "amount" pkg/domain/circlesemantics/ 2>/dev/null && pass "No amount in domain" || fail "Found amount in domain"
! grep -ri "vendor" pkg/domain/circlesemantics/ 2>/dev/null && pass "No vendor in domain" || fail "Found vendor in domain"
! grep -rE "\\$|£|€|¥" pkg/domain/circlesemantics/ 2>/dev/null && pass "No currency symbols in domain" || fail "Found currency symbols in domain"

# ============================================================================
echo "--- Section 6: Domain Enums ---"
# ============================================================================

grep -q "CircleSemanticKind" pkg/domain/circlesemantics/types.go && pass "CircleSemanticKind type defined" || fail "CircleSemanticKind type missing"
grep -q "UrgencyModel" pkg/domain/circlesemantics/types.go && pass "UrgencyModel type defined" || fail "UrgencyModel type missing"
grep -q "NecessityLevel" pkg/domain/circlesemantics/types.go && pass "NecessityLevel type defined" || fail "NecessityLevel type missing"
grep -q "SemanticsProvenance" pkg/domain/circlesemantics/types.go && pass "SemanticsProvenance type defined" || fail "SemanticsProvenance type missing"
grep -q "SemanticsEffect" pkg/domain/circlesemantics/types.go && pass "SemanticsEffect type defined" || fail "SemanticsEffect type missing"

# ============================================================================
echo "--- Section 7: Semantic Kind Values ---"
# ============================================================================

grep -q "semantic_human" pkg/domain/circlesemantics/types.go && pass "semantic_human value defined" || fail "semantic_human value missing"
grep -q "semantic_institution" pkg/domain/circlesemantics/types.go && pass "semantic_institution value defined" || fail "semantic_institution value missing"
grep -q "semantic_service_essential" pkg/domain/circlesemantics/types.go && pass "semantic_service_essential value defined" || fail "semantic_service_essential value missing"
grep -q "semantic_service_transactional" pkg/domain/circlesemantics/types.go && pass "semantic_service_transactional value defined" || fail "semantic_service_transactional value missing"
grep -q "semantic_service_optional" pkg/domain/circlesemantics/types.go && pass "semantic_service_optional value defined" || fail "semantic_service_optional value missing"
grep -q "semantic_unknown" pkg/domain/circlesemantics/types.go && pass "semantic_unknown value defined" || fail "semantic_unknown value missing"

# ============================================================================
echo "--- Section 8: Urgency Model Values ---"
# ============================================================================

grep -q "urgency_never_interrupt" pkg/domain/circlesemantics/types.go && pass "urgency_never_interrupt value defined" || fail "urgency_never_interrupt value missing"
grep -q "urgency_hard_deadline" pkg/domain/circlesemantics/types.go && pass "urgency_hard_deadline value defined" || fail "urgency_hard_deadline value missing"
grep -q "urgency_human_waiting" pkg/domain/circlesemantics/types.go && pass "urgency_human_waiting value defined" || fail "urgency_human_waiting value missing"
grep -q "urgency_time_window" pkg/domain/circlesemantics/types.go && pass "urgency_time_window value defined" || fail "urgency_time_window value missing"
grep -q "urgency_soft_reminder" pkg/domain/circlesemantics/types.go && pass "urgency_soft_reminder value defined" || fail "urgency_soft_reminder value missing"
grep -q "urgency_unknown" pkg/domain/circlesemantics/types.go && pass "urgency_unknown value defined" || fail "urgency_unknown value missing"

# ============================================================================
echo "--- Section 9: Necessity Level Values ---"
# ============================================================================

grep -q "necessity_low" pkg/domain/circlesemantics/types.go && pass "necessity_low value defined" || fail "necessity_low value missing"
grep -q "necessity_medium" pkg/domain/circlesemantics/types.go && pass "necessity_medium value defined" || fail "necessity_medium value missing"
grep -q "necessity_high" pkg/domain/circlesemantics/types.go && pass "necessity_high value defined" || fail "necessity_high value missing"
grep -q "necessity_unknown" pkg/domain/circlesemantics/types.go && pass "necessity_unknown value defined" || fail "necessity_unknown value missing"

# ============================================================================
echo "--- Section 10: Provenance Values ---"
# ============================================================================

grep -q "provenance_user_declared" pkg/domain/circlesemantics/types.go && pass "provenance_user_declared value defined" || fail "provenance_user_declared value missing"
grep -q "provenance_derived_rules" pkg/domain/circlesemantics/types.go && pass "provenance_derived_rules value defined" || fail "provenance_derived_rules value missing"
grep -q "provenance_imported_connector" pkg/domain/circlesemantics/types.go && pass "provenance_imported_connector value defined" || fail "provenance_imported_connector value missing"

# ============================================================================
echo "--- Section 11: Effect Value (MUST be no_power only) ---"
# ============================================================================

grep -q "effect_no_power" pkg/domain/circlesemantics/types.go && pass "effect_no_power value defined" || fail "effect_no_power value missing"
! grep -q "effect_has_power" pkg/domain/circlesemantics/types.go && pass "No effect_has_power (forbidden)" || fail "Found forbidden effect_has_power"
! grep -q "effect_can_interrupt" pkg/domain/circlesemantics/types.go && pass "No effect_can_interrupt (forbidden)" || fail "Found forbidden effect_can_interrupt"
! grep -q "effect_can_deliver" pkg/domain/circlesemantics/types.go && pass "No effect_can_deliver (forbidden)" || fail "Found forbidden effect_can_deliver"
! grep -q "effect_can_execute" pkg/domain/circlesemantics/types.go && pass "No effect_can_execute (forbidden)" || fail "Found forbidden effect_can_execute"

# ============================================================================
echo "--- Section 12: Core Structs ---"
# ============================================================================

grep -q "CircleSemanticsKey" pkg/domain/circlesemantics/types.go && pass "CircleSemanticsKey struct defined" || fail "CircleSemanticsKey struct missing"
grep -q "CircleSemantics" pkg/domain/circlesemantics/types.go && pass "CircleSemantics struct defined" || fail "CircleSemantics struct missing"
grep -q "SemanticsRecord" pkg/domain/circlesemantics/types.go && pass "SemanticsRecord struct defined" || fail "SemanticsRecord struct missing"
grep -q "SemanticsChange" pkg/domain/circlesemantics/types.go && pass "SemanticsChange struct defined" || fail "SemanticsChange struct missing"
grep -q "SemanticsSettingsPage" pkg/domain/circlesemantics/types.go && pass "SemanticsSettingsPage struct defined" || fail "SemanticsSettingsPage struct missing"
grep -q "SemanticsProofPage" pkg/domain/circlesemantics/types.go && pass "SemanticsProofPage struct defined" || fail "SemanticsProofPage struct missing"
grep -q "SemanticsCue" pkg/domain/circlesemantics/types.go && pass "SemanticsCue struct defined" || fail "SemanticsCue struct missing"
grep -q "SemanticsProofAck" pkg/domain/circlesemantics/types.go && pass "SemanticsProofAck struct defined" || fail "SemanticsProofAck struct missing"

# ============================================================================
echo "--- Section 13: Validate Methods ---"
# ============================================================================

grep -q "func (k CircleSemanticKind) Validate()" pkg/domain/circlesemantics/types.go && pass "CircleSemanticKind.Validate() exists" || fail "CircleSemanticKind.Validate() missing"
grep -q "func (u UrgencyModel) Validate()" pkg/domain/circlesemantics/types.go && pass "UrgencyModel.Validate() exists" || fail "UrgencyModel.Validate() missing"
grep -q "func (n NecessityLevel) Validate()" pkg/domain/circlesemantics/types.go && pass "NecessityLevel.Validate() exists" || fail "NecessityLevel.Validate() missing"
grep -q "func (p SemanticsProvenance) Validate()" pkg/domain/circlesemantics/types.go && pass "SemanticsProvenance.Validate() exists" || fail "SemanticsProvenance.Validate() missing"
grep -q "func (e SemanticsEffect) Validate()" pkg/domain/circlesemantics/types.go && pass "SemanticsEffect.Validate() exists" || fail "SemanticsEffect.Validate() missing"

# ============================================================================
echo "--- Section 14: CanonicalString Methods ---"
# ============================================================================

grep -q "func (k CircleSemanticKind) CanonicalString()" pkg/domain/circlesemantics/types.go && pass "CircleSemanticKind.CanonicalString() exists" || fail "CircleSemanticKind.CanonicalString() missing"
grep -q "func (u UrgencyModel) CanonicalString()" pkg/domain/circlesemantics/types.go && pass "UrgencyModel.CanonicalString() exists" || fail "UrgencyModel.CanonicalString() missing"
grep -q "func (n NecessityLevel) CanonicalString()" pkg/domain/circlesemantics/types.go && pass "NecessityLevel.CanonicalString() exists" || fail "NecessityLevel.CanonicalString() missing"
grep -q "CanonicalStringV1" pkg/domain/circlesemantics/types.go && pass "CanonicalStringV1 method exists" || fail "CanonicalStringV1 method missing"

# ============================================================================
echo "--- Section 15: Pipe-Delimited Canonical Strings ---"
# ============================================================================

grep -q '"|"' pkg/domain/circlesemantics/types.go && pass "Pipe delimiter used in domain" || fail "Pipe delimiter missing in domain"

# ============================================================================
echo "--- Section 16: Hash Functions ---"
# ============================================================================

grep -q "HashString" pkg/domain/circlesemantics/types.go && pass "HashString function exists" || fail "HashString function missing"
grep -q "ComputeSemanticsHash" pkg/domain/circlesemantics/types.go && pass "ComputeSemanticsHash function exists" || fail "ComputeSemanticsHash function missing"
grep -q "ComputeStatusHash" pkg/domain/circlesemantics/types.go && pass "ComputeStatusHash function exists" || fail "ComputeStatusHash function missing"
grep -q "crypto/sha256" pkg/domain/circlesemantics/types.go && pass "crypto/sha256 imported in domain" || fail "crypto/sha256 missing in domain"

# ============================================================================
echo "--- Section 17: Engine Existence ---"
# ============================================================================

grep -q "type Engine struct" internal/circlesemantics/engine.go && pass "Engine struct defined" || fail "Engine struct missing"
grep -q "func NewEngine" internal/circlesemantics/engine.go && pass "NewEngine function exists" || fail "NewEngine function missing"

# ============================================================================
echo "--- Section 18: Engine Methods ---"
# ============================================================================

grep -q "DeriveDefaultSemantics" internal/circlesemantics/engine.go && pass "DeriveDefaultSemantics method exists" || fail "DeriveDefaultSemantics method missing"
grep -q "BuildSettingsPage" internal/circlesemantics/engine.go && pass "BuildSettingsPage method exists" || fail "BuildSettingsPage method missing"
grep -q "ApplyUserDeclaration" internal/circlesemantics/engine.go && pass "ApplyUserDeclaration method exists" || fail "ApplyUserDeclaration method missing"
grep -q "BuildProofPage" internal/circlesemantics/engine.go && pass "BuildProofPage method exists" || fail "BuildProofPage method missing"
grep -q "ComputeCue" internal/circlesemantics/engine.go && pass "ComputeCue method exists" || fail "ComputeCue method missing"

# ============================================================================
echo "--- Section 19: Engine Clock Injection ---"
# ============================================================================

grep -q "type Clock interface" internal/circlesemantics/engine.go && pass "Clock interface defined" || fail "Clock interface missing"
grep -q "clock Clock" internal/circlesemantics/engine.go && pass "clock field in Engine" || fail "clock field missing in Engine"

# ============================================================================
echo "--- Section 20: Engine Default Derivation Rules ---"
# ============================================================================

grep -q "CircleTypeHuman" internal/circlesemantics/engine.go && pass "CircleTypeHuman constant exists" || fail "CircleTypeHuman constant missing"
grep -q "CircleTypeInstitution" internal/circlesemantics/engine.go && pass "CircleTypeInstitution constant exists" || fail "CircleTypeInstitution constant missing"
grep -q "CircleTypeCommerce" internal/circlesemantics/engine.go && pass "CircleTypeCommerce constant exists" || fail "CircleTypeCommerce constant missing"
grep -q "CircleTypeUnknown" internal/circlesemantics/engine.go && pass "CircleTypeUnknown constant exists" || fail "CircleTypeUnknown constant missing"

# ============================================================================
echo "--- Section 21: Engine Effect Enforcement ---"
# ============================================================================

grep -q "EffectNoPower" internal/circlesemantics/engine.go && pass "EffectNoPower used in engine" || fail "EffectNoPower not used in engine"
grep -q "Effect.*=.*EffectNoPower" internal/circlesemantics/engine.go && pass "Effect always set to EffectNoPower" || fail "Effect not always EffectNoPower"

# ============================================================================
echo "--- Section 22: Store Existence ---"
# ============================================================================

grep -q "CircleSemanticsStore" internal/persist/circle_semantics_store.go && pass "CircleSemanticsStore struct defined" || fail "CircleSemanticsStore struct missing"
grep -q "CircleSemanticsAckStore" internal/persist/circle_semantics_store.go && pass "CircleSemanticsAckStore struct defined" || fail "CircleSemanticsAckStore struct missing"

# ============================================================================
echo "--- Section 23: Store Methods ---"
# ============================================================================

grep -q "func.*Upsert" internal/persist/circle_semantics_store.go && pass "Store.Upsert method exists" || fail "Store.Upsert method missing"
grep -q "func.*GetLatest" internal/persist/circle_semantics_store.go && pass "Store.GetLatest method exists" || fail "Store.GetLatest method missing"
grep -q "func.*ListLatestAll" internal/persist/circle_semantics_store.go && pass "Store.ListLatestAll method exists" || fail "Store.ListLatestAll method missing"
grep -q "func.*ListByPeriod" internal/persist/circle_semantics_store.go && pass "Store.ListByPeriod method exists" || fail "Store.ListByPeriod method missing"
grep -q "func.*RecordProofAck" internal/persist/circle_semantics_store.go && pass "AckStore.RecordProofAck method exists" || fail "AckStore.RecordProofAck method missing"
grep -q "func.*IsProofDismissed" internal/persist/circle_semantics_store.go && pass "AckStore.IsProofDismissed method exists" || fail "AckStore.IsProofDismissed method missing"

# ============================================================================
echo "--- Section 24: Bounded Retention ---"
# ============================================================================

grep -q "CircleSemanticsMaxRecords.*=.*200" internal/persist/circle_semantics_store.go && pass "MaxRecords = 200" || fail "MaxRecords != 200"
grep -q "CircleSemanticsMaxRetentionDays.*=.*30" internal/persist/circle_semantics_store.go && pass "MaxRetentionDays = 30" || fail "MaxRetentionDays != 30"

# ============================================================================
echo "--- Section 25: FIFO Eviction ---"
# ============================================================================

grep -q "evict" internal/persist/circle_semantics_store.go && pass "Store has eviction logic" || fail "Store missing eviction logic"

# ============================================================================
echo "--- Section 26: Storelog Record Types ---"
# ============================================================================

grep -q "RecordTypeCircleSemanticsRecord" pkg/domain/storelog/log.go && pass "RecordTypeCircleSemanticsRecord defined" || fail "RecordTypeCircleSemanticsRecord missing"
grep -q "RecordTypeCircleSemanticsProofAck" pkg/domain/storelog/log.go && pass "RecordTypeCircleSemanticsProofAck defined" || fail "RecordTypeCircleSemanticsProofAck missing"

# ============================================================================
echo "--- Section 27: Events ---"
# ============================================================================

grep -q "Phase45SemanticsSettingsViewed" pkg/events/events.go && pass "Phase45SemanticsSettingsViewed event defined" || fail "Phase45SemanticsSettingsViewed event missing"
grep -q "Phase45SemanticsSaved" pkg/events/events.go && pass "Phase45SemanticsSaved event defined" || fail "Phase45SemanticsSaved event missing"
grep -q "Phase45SemanticsProofRendered" pkg/events/events.go && pass "Phase45SemanticsProofRendered event defined" || fail "Phase45SemanticsProofRendered event missing"
grep -q "Phase45SemanticsProofDismissed" pkg/events/events.go && pass "Phase45SemanticsProofDismissed event defined" || fail "Phase45SemanticsProofDismissed event missing"
grep -q "Phase45SemanticsCueComputed" pkg/events/events.go && pass "Phase45SemanticsCueComputed event defined" || fail "Phase45SemanticsCueComputed event missing"

# ============================================================================
echo "--- Section 28: Web Routes ---"
# ============================================================================

grep -q "/settings/semantics" cmd/quantumlife-web/main.go && pass "/settings/semantics route exists" || fail "/settings/semantics route missing"
grep -q "/settings/semantics/save" cmd/quantumlife-web/main.go && pass "/settings/semantics/save route exists" || fail "/settings/semantics/save route missing"
grep -q "/proof/semantics" cmd/quantumlife-web/main.go && pass "/proof/semantics route exists" || fail "/proof/semantics route missing"
grep -q "/proof/semantics/dismiss" cmd/quantumlife-web/main.go && pass "/proof/semantics/dismiss route exists" || fail "/proof/semantics/dismiss route missing"

# ============================================================================
echo "--- Section 29: Handler POST Enforcement ---"
# ============================================================================

grep -A5 "handleCircleSemanticsSave" cmd/quantumlife-web/main.go | grep -q "MethodPost" && pass "handleCircleSemanticsSave enforces POST" || fail "handleCircleSemanticsSave does not enforce POST"
grep -A5 "handleCircleSemanticsProofDismiss" cmd/quantumlife-web/main.go | grep -q "MethodPost" && pass "handleCircleSemanticsProofDismiss enforces POST" || fail "handleCircleSemanticsProofDismiss does not enforce POST"

# ============================================================================
echo "--- Section 30: Templates ---"
# ============================================================================

grep -q "circle-semantics-settings" cmd/quantumlife-web/main.go && pass "circle-semantics-settings template defined" || fail "circle-semantics-settings template missing"
grep -q "circle-semantics-proof" cmd/quantumlife-web/main.go && pass "circle-semantics-proof template defined" || fail "circle-semantics-proof template missing"

# ============================================================================
echo "--- Section 31: No Forbidden Raw Identifiers ---"
# ============================================================================

! grep -rE "Name\s+string" pkg/domain/circlesemantics/types.go 2>/dev/null && pass "No Name field in domain structs" || fail "Found Name field in domain"
! grep -rE "Email\s+string" pkg/domain/circlesemantics/types.go 2>/dev/null && pass "No Email field in domain structs" || fail "Found Email field in domain"
! grep -rE "Merchant\s+string" pkg/domain/circlesemantics/types.go 2>/dev/null && pass "No Merchant field in domain structs" || fail "Found Merchant field in domain"
! grep -rE "Amount\s+" pkg/domain/circlesemantics/types.go 2>/dev/null && pass "No Amount field in domain structs" || fail "Found Amount field in domain"
! grep -rE "Subject\s+string" pkg/domain/circlesemantics/types.go 2>/dev/null && pass "No Subject field in domain structs" || fail "Found Subject field in domain"
! grep -rE "Sender\s+string" pkg/domain/circlesemantics/types.go 2>/dev/null && pass "No Sender field in domain structs" || fail "Found Sender field in domain"

# ============================================================================
echo "--- Section 32: No Forbidden Imports in Domain ---"
# ============================================================================

! grep -q "pressuredecision" pkg/domain/circlesemantics/types.go && pass "Domain does not import pressuredecision" || fail "Domain imports pressuredecision"
! grep -q "interruptpolicy" pkg/domain/circlesemantics/types.go && pass "Domain does not import interruptpolicy" || fail "Domain imports interruptpolicy"
! grep -q "interruptpreview" pkg/domain/circlesemantics/types.go && pass "Domain does not import interruptpreview" || fail "Domain imports interruptpreview"
! grep -q "pushtransport" pkg/domain/circlesemantics/types.go && pass "Domain does not import pushtransport" || fail "Domain imports pushtransport"
! grep -q "interruptdelivery" pkg/domain/circlesemantics/types.go && pass "Domain does not import interruptdelivery" || fail "Domain imports interruptdelivery"

# ============================================================================
echo "--- Section 33: No Forbidden Imports in Engine ---"
# ============================================================================

! grep -q "pressuredecision" internal/circlesemantics/engine.go && pass "Engine does not import pressuredecision" || fail "Engine imports pressuredecision"
! grep -q "interruptpolicy" internal/circlesemantics/engine.go && pass "Engine does not import interruptpolicy" || fail "Engine imports interruptpolicy"
! grep -q "interruptpreview" internal/circlesemantics/engine.go && pass "Engine does not import interruptpreview" || fail "Engine imports interruptpreview"
! grep -q "pushtransport" internal/circlesemantics/engine.go && pass "Engine does not import pushtransport" || fail "Engine imports pushtransport"
! grep -q "interruptdelivery" internal/circlesemantics/engine.go && pass "Engine does not import interruptdelivery" || fail "Engine imports interruptdelivery"
! grep -q "enforcementclamp" internal/circlesemantics/engine.go && pass "Engine does not import enforcementclamp" || fail "Engine imports enforcementclamp"

# ============================================================================
echo "--- Section 34: Decision Packages Do Not Import Semantics ---"
# ============================================================================

if [ -f "internal/pressuredecision/engine.go" ]; then
    ! grep -q "circlesemantics" internal/pressuredecision/engine.go && pass "pressuredecision does not import circlesemantics" || fail "pressuredecision imports circlesemantics"
else
    pass "pressuredecision engine not found (skip)"
fi

if [ -f "internal/interruptpolicy/engine.go" ]; then
    ! grep -q "circlesemantics" internal/interruptpolicy/engine.go && pass "interruptpolicy does not import circlesemantics" || fail "interruptpolicy imports circlesemantics"
else
    pass "interruptpolicy engine not found (skip)"
fi

if [ -f "internal/interruptpreview/engine.go" ]; then
    ! grep -q "circlesemantics" internal/interruptpreview/engine.go && pass "interruptpreview does not import circlesemantics" || fail "interruptpreview imports circlesemantics"
else
    pass "interruptpreview engine not found (skip)"
fi

# ============================================================================
echo "--- Section 35: NotesBucket Allowlist ---"
# ============================================================================

grep -q "AllowedNotesBuckets" pkg/domain/circlesemantics/types.go && pass "AllowedNotesBuckets map exists" || fail "AllowedNotesBuckets map missing"
grep -q "NotesBucketNone" pkg/domain/circlesemantics/types.go && pass "NotesBucketNone constant exists" || fail "NotesBucketNone constant missing"
grep -q "NotesBucketUserSet" pkg/domain/circlesemantics/types.go && pass "NotesBucketUserSet constant exists" || fail "NotesBucketUserSet constant missing"
grep -q "NotesBucketDerived" pkg/domain/circlesemantics/types.go && pass "NotesBucketDerived constant exists" || fail "NotesBucketDerived constant missing"

# ============================================================================
echo "--- Section 36: Change Kind Values ---"
# ============================================================================

grep -q "ChangeKindCreated" pkg/domain/circlesemantics/types.go && pass "ChangeKindCreated constant exists" || fail "ChangeKindCreated constant missing"
grep -q "ChangeKindUpdated" pkg/domain/circlesemantics/types.go && pass "ChangeKindUpdated constant exists" || fail "ChangeKindUpdated constant missing"
grep -q "ChangeKindCleared" pkg/domain/circlesemantics/types.go && pass "ChangeKindCleared constant exists" || fail "ChangeKindCleared constant missing"
grep -q "ChangeKindNoChange" pkg/domain/circlesemantics/types.go && pass "ChangeKindNoChange constant exists" || fail "ChangeKindNoChange constant missing"

# ============================================================================
echo "--- Section 37: Ack Kind Values ---"
# ============================================================================

grep -q "AckKindViewed" pkg/domain/circlesemantics/types.go && pass "AckKindViewed constant exists" || fail "AckKindViewed constant missing"
grep -q "AckKindDismissed" pkg/domain/circlesemantics/types.go && pass "AckKindDismissed constant exists" || fail "AckKindDismissed constant missing"

# ============================================================================
echo "--- Section 38: UI Model Fields ---"
# ============================================================================

grep -q "SemanticsSettingsItem" pkg/domain/circlesemantics/types.go && pass "SemanticsSettingsItem struct defined" || fail "SemanticsSettingsItem struct missing"
grep -q "SemanticsProofEntry" pkg/domain/circlesemantics/types.go && pass "SemanticsProofEntry struct defined" || fail "SemanticsProofEntry struct missing"

# ============================================================================
echo "--- Section 39: MaxDisplayEntries ---"
# ============================================================================

grep -q "MaxDisplayEntries" pkg/domain/circlesemantics/types.go && pass "MaxDisplayEntries constant exists" || fail "MaxDisplayEntries constant missing"

# ============================================================================
echo "--- Section 40: CircleCountBucket ---"
# ============================================================================

grep -q "CircleCountBucket" pkg/domain/circlesemantics/types.go && pass "CircleCountBucket function exists" || fail "CircleCountBucket function missing"
grep -q "nothing" pkg/domain/circlesemantics/types.go && pass "CircleCountBucket returns 'nothing'" || fail "CircleCountBucket 'nothing' missing"
grep -q "a_few" pkg/domain/circlesemantics/types.go && pass "CircleCountBucket returns 'a_few'" || fail "CircleCountBucket 'a_few' missing"
grep -q "several" pkg/domain/circlesemantics/types.go && pass "CircleCountBucket returns 'several'" || fail "CircleCountBucket 'several' missing"

# ============================================================================
echo "--- Section 41: SemanticsInputs Struct ---"
# ============================================================================

grep -q "SemanticsInputs" internal/circlesemantics/engine.go && pass "SemanticsInputs struct defined" || fail "SemanticsInputs struct missing"
grep -q "CircleIDHashes" internal/circlesemantics/engine.go && pass "CircleIDHashes field exists" || fail "CircleIDHashes field missing"
grep -q "CircleTypes" internal/circlesemantics/engine.go && pass "CircleTypes field exists" || fail "CircleTypes field missing"
grep -q "HasGmail" internal/circlesemantics/engine.go && pass "HasGmail field exists" || fail "HasGmail field missing"
grep -q "HasTrueLayer" internal/circlesemantics/engine.go && pass "HasTrueLayer field exists" || fail "HasTrueLayer field missing"

# ============================================================================
echo "--- Section 42: Store Clock Injection ---"
# ============================================================================

grep -q "clock.*func().*time.Time" internal/persist/circle_semantics_store.go && pass "Store has injected clock" || fail "Store missing injected clock"

# ============================================================================
echo "--- Section 43: Dedup Index ---"
# ============================================================================

grep -q "dedupIndex" internal/persist/circle_semantics_store.go && pass "Store has dedup index" || fail "Store missing dedup index"

# ============================================================================
echo "--- Section 44: Makefile Targets ---"
# ============================================================================

grep -q "demo-phase45" Makefile && pass "demo-phase45 target exists" || fail "demo-phase45 target missing"
grep -q "check-circle-semantics" Makefile && pass "check-circle-semantics target exists" || fail "check-circle-semantics target missing"

# ============================================================================
echo "--- Section 45: ParseSemanticsFromForm ---"
# ============================================================================

grep -q "ParseSemanticsFromForm" internal/circlesemantics/engine.go && pass "ParseSemanticsFromForm function exists" || fail "ParseSemanticsFromForm function missing"

# ============================================================================
echo "--- Section 46: All Lists Functions ---"
# ============================================================================

grep -q "AllCircleSemanticKinds" pkg/domain/circlesemantics/types.go && pass "AllCircleSemanticKinds function exists" || fail "AllCircleSemanticKinds function missing"
grep -q "AllUrgencyModels" pkg/domain/circlesemantics/types.go && pass "AllUrgencyModels function exists" || fail "AllUrgencyModels function missing"
grep -q "AllNecessityLevels" pkg/domain/circlesemantics/types.go && pass "AllNecessityLevels function exists" || fail "AllNecessityLevels function missing"

# ============================================================================
echo "--- Section 47: Record Validate Methods ---"
# ============================================================================

grep -q "func (s CircleSemantics) Validate()" pkg/domain/circlesemantics/types.go && pass "CircleSemantics.Validate() exists" || fail "CircleSemantics.Validate() missing"
grep -q "func (r SemanticsRecord) Validate()" pkg/domain/circlesemantics/types.go && pass "SemanticsRecord.Validate() exists" || fail "SemanticsRecord.Validate() missing"
grep -q "func (c SemanticsChange) Validate()" pkg/domain/circlesemantics/types.go && pass "SemanticsChange.Validate() exists" || fail "SemanticsChange.Validate() missing"
grep -q "func (a SemanticsProofAck) Validate()" pkg/domain/circlesemantics/types.go && pass "SemanticsProofAck.Validate() exists" || fail "SemanticsProofAck.Validate() missing"

# ============================================================================
echo "--- Section 48: Engine Store Interfaces ---"
# ============================================================================

grep -q "AckStore interface" internal/circlesemantics/engine.go && pass "AckStore interface defined" || fail "AckStore interface missing"
grep -q "RecordStore interface" internal/circlesemantics/engine.go && pass "RecordStore interface defined" || fail "RecordStore interface missing"

# ============================================================================
echo ""
echo "================================================"
echo "Summary: $PASS passed, $FAIL failed"

if [ $FAIL -eq 0 ]; then
    echo -e "\033[0;32mPASS: All guardrails passed\033[0m"
    exit 0
else
    echo -e "\033[0;31mFAIL: Some guardrails failed\033[0m"
    exit 1
fi
