#!/bin/bash
# Phase 55: Observer Consent Activation UI Guardrails
# Ensures all Phase 55 invariants are enforced.
#
# CRITICAL: Consent controls OBSERVATION, not action capability.
# CRITICAL: Period key is server-derived ONLY (clients cannot provide it).
# CRITICAL: Forbidden client fields rejected (period, periodKey, email, etc.).
# CRITICAL: Hash-only storage - no raw identifiers.
# CRITICAL: Bounded retention: max 200 records OR 30 days.
# CRITICAL: POST-only mutations.
# CRITICAL: No time.Now() in pkg/ or internal/.
# CRITICAL: No goroutines in pkg/ or internal/.
#
# Reference: docs/ADR/ADR-0092-phase55-observer-consent-activation-ui.md

set -e

PASS_COUNT=0
FAIL_COUNT=0

pass() {
    echo "  ✓ $1"
    PASS_COUNT=$((PASS_COUNT + 1))
}

fail() {
    echo "  ✗ $1"
    FAIL_COUNT=$((FAIL_COUNT + 1))
}

check() {
    local desc="$1"
    shift
    if "$@" > /dev/null 2>&1; then
        pass "$desc"
    else
        fail "$desc"
    fi
}

check_not() {
    local desc="$1"
    shift
    if ! "$@" > /dev/null 2>&1; then
        pass "$desc"
    else
        fail "$desc"
    fi
}

echo "=============================================="
echo "Phase 55: Observer Consent Activation UI Guardrails"
echo "=============================================="
echo ""

# ============================================================================
# Section 1: Required Files Exist
# ============================================================================
echo "Section 1: Required Files Exist"

check "ADR-0092 exists" test -f docs/ADR/ADR-0092-phase55-observer-consent-activation-ui.md
check "Domain types exist" test -f pkg/domain/observerconsent/types.go
check "Engine exists" test -f internal/observerconsent/engine.go
check "Consent store exists" test -f internal/persist/observer_consent_store.go
check "Consent ack store exists" test -f internal/persist/observer_consent_ack_store.go
check "Demo tests exist" test -f internal/demo_phase55_observer_consent/demo_test.go

echo ""

# ============================================================================
# Section 2: Domain Enums - ObserverKind
# ============================================================================
echo "Section 2: Domain Enums - ObserverKind"

check "ObserverKind type defined" grep -q "type ObserverKind string" pkg/domain/observerconsent/types.go
check "KindReceipt defined" grep -q 'KindReceipt.*=.*"receipt"' pkg/domain/observerconsent/types.go
check "KindCalendar defined" grep -q 'KindCalendar.*=.*"calendar"' pkg/domain/observerconsent/types.go
check "KindCommerce defined" grep -q 'KindCommerce.*=.*"commerce"' pkg/domain/observerconsent/types.go
check "KindFinanceCommerce defined" grep -q 'KindFinanceCommerce.*=.*"finance_commerce"' pkg/domain/observerconsent/types.go
check "KindNotification defined" grep -q 'KindNotification.*=.*"notification"' pkg/domain/observerconsent/types.go
check "KindUnknown defined" grep -q 'KindUnknown.*=.*"unknown"' pkg/domain/observerconsent/types.go
check "ObserverKind.CanonicalString exists" grep -q "func (k ObserverKind) CanonicalString()" pkg/domain/observerconsent/types.go
check "ObserverKind.Validate exists" grep -q "func (k ObserverKind) Validate()" pkg/domain/observerconsent/types.go
check "KindFromCapability exists" grep -q "func KindFromCapability" pkg/domain/observerconsent/types.go

echo ""

# ============================================================================
# Section 3: Domain Enums - ConsentAction
# ============================================================================
echo "Section 3: Domain Enums - ConsentAction"

check "ConsentAction type defined" grep -q "type ConsentAction string" pkg/domain/observerconsent/types.go
check "ActionEnable defined" grep -q 'ActionEnable.*=.*"enable"' pkg/domain/observerconsent/types.go
check "ActionDisable defined" grep -q 'ActionDisable.*=.*"disable"' pkg/domain/observerconsent/types.go
check "ConsentAction.CanonicalString exists" grep -q "func (a ConsentAction) CanonicalString()" pkg/domain/observerconsent/types.go
check "ConsentAction.Validate exists" grep -q "func (a ConsentAction) Validate()" pkg/domain/observerconsent/types.go

echo ""

# ============================================================================
# Section 4: Domain Enums - ConsentResult
# ============================================================================
echo "Section 4: Domain Enums - ConsentResult"

check "ConsentResult type defined" grep -q "type ConsentResult string" pkg/domain/observerconsent/types.go
check "ResultApplied defined" grep -q 'ResultApplied.*=.*"applied"' pkg/domain/observerconsent/types.go
check "ResultNoChange defined" grep -q 'ResultNoChange.*=.*"no_change"' pkg/domain/observerconsent/types.go
check "ResultRejected defined" grep -q 'ResultRejected.*=.*"rejected"' pkg/domain/observerconsent/types.go
check "ConsentResult.CanonicalString exists" grep -q "func (r ConsentResult) CanonicalString()" pkg/domain/observerconsent/types.go
check "ConsentResult.Validate exists" grep -q "func (r ConsentResult) Validate()" pkg/domain/observerconsent/types.go

echo ""

# ============================================================================
# Section 5: Domain Enums - RejectReason
# ============================================================================
echo "Section 5: Domain Enums - RejectReason"

check "RejectReason type defined" grep -q "type RejectReason string" pkg/domain/observerconsent/types.go
check "RejectNone defined" grep -q 'RejectNone.*RejectReason.*=.*""' pkg/domain/observerconsent/types.go
check "RejectNotAllowlisted defined" grep -q 'RejectNotAllowlisted.*=.*"reject_not_allowlisted"' pkg/domain/observerconsent/types.go
check "RejectInvalid defined" grep -q 'RejectInvalid.*=.*"reject_invalid"' pkg/domain/observerconsent/types.go
check "RejectMissingCircle defined" grep -q 'RejectMissingCircle.*=.*"reject_missing_circle"' pkg/domain/observerconsent/types.go
check "RejectPeriodInvalid defined" grep -q 'RejectPeriodInvalid.*=.*"reject_period_invalid"' pkg/domain/observerconsent/types.go
check "RejectReason.CanonicalString exists" grep -q "func (r RejectReason) CanonicalString()" pkg/domain/observerconsent/types.go
check "RejectReason.Validate exists" grep -q "func (r RejectReason) Validate()" pkg/domain/observerconsent/types.go

echo ""

# ============================================================================
# Section 6: Domain Enums - ConsentAckKind
# ============================================================================
echo "Section 6: Domain Enums - ConsentAckKind"

check "ConsentAckKind type defined" grep -q "type ConsentAckKind string" pkg/domain/observerconsent/types.go
check "AckDismissed defined" grep -q 'AckDismissed.*=.*"ack_dismissed"' pkg/domain/observerconsent/types.go
check "ConsentAckKind.CanonicalString exists" grep -q "func (k ConsentAckKind) CanonicalString()" pkg/domain/observerconsent/types.go
check "ConsentAckKind.Validate exists" grep -q "func (k ConsentAckKind) Validate()" pkg/domain/observerconsent/types.go

echo ""

# ============================================================================
# Section 7: Domain Structs
# ============================================================================
echo "Section 7: Domain Structs"

check "ObserverConsentRequest struct exists" grep -q "type ObserverConsentRequest struct" pkg/domain/observerconsent/types.go
check "ObserverConsentReceipt struct exists" grep -q "type ObserverConsentReceipt struct" pkg/domain/observerconsent/types.go
check "ObserverConsentReceipt.CanonicalStringV1 exists" grep -q "func (r ObserverConsentReceipt) CanonicalStringV1()" pkg/domain/observerconsent/types.go
check "ObserverConsentReceipt.DedupKey exists" grep -q "func (r ObserverConsentReceipt) DedupKey()" pkg/domain/observerconsent/types.go
check "ObserverConsentAck struct exists" grep -q "type ObserverConsentAck struct" pkg/domain/observerconsent/types.go
check "ObserverConsentAck.CanonicalStringV1 exists" grep -q "func (a ObserverConsentAck) CanonicalStringV1()" pkg/domain/observerconsent/types.go
check "ObserverConsentAck.DedupKey exists" grep -q "func (a ObserverConsentAck) DedupKey()" pkg/domain/observerconsent/types.go
check "ObserverSettingsPage struct exists" grep -q "type ObserverSettingsPage struct" pkg/domain/observerconsent/types.go
check "ObserverProofPage struct exists" grep -q "type ObserverProofPage struct" pkg/domain/observerconsent/types.go
check "ObserverCapabilityStatus struct exists" grep -q "type ObserverCapabilityStatus struct" pkg/domain/observerconsent/types.go

echo ""

# ============================================================================
# Section 8: Domain Hash Helpers
# ============================================================================
echo "Section 8: Domain Hash Helpers"

check "HashCircleID exists" grep -q "func HashCircleID" pkg/domain/observerconsent/types.go
check "ComputeReceiptHash exists" grep -q "func.*ComputeReceiptHash" pkg/domain/observerconsent/types.go
check "Uses sha256" grep -q "sha256" pkg/domain/observerconsent/types.go
check "Uses hex encoding" grep -q "hex.EncodeToString" pkg/domain/observerconsent/types.go
check "Domain imports crypto/sha256" grep -q '"crypto/sha256"' pkg/domain/observerconsent/types.go
check "Domain imports encoding/hex" grep -q '"encoding/hex"' pkg/domain/observerconsent/types.go

echo ""

# ============================================================================
# Section 9: Engine Methods
# ============================================================================
echo "Section 9: Engine Methods"

check "Engine struct exists" grep -q "type Engine struct" internal/observerconsent/engine.go
check "NewEngine exists" grep -q "func NewEngine" internal/observerconsent/engine.go
check "ApplyConsent exists" grep -q "func (e \*Engine) ApplyConsent" internal/observerconsent/engine.go
check "BuildSettingsPage exists" grep -q "func (e \*Engine) BuildSettingsPage" internal/observerconsent/engine.go
check "BuildProofPage exists" grep -q "func (e \*Engine) BuildProofPage" internal/observerconsent/engine.go
check "ValidateNoForbiddenFields exists" grep -q "func.*ValidateNoForbiddenFields" internal/observerconsent/engine.go
check "Engine has periodKeyFunc field" grep -q "periodKeyFunc.*func().*string" internal/observerconsent/engine.go
check "Engine imports coverageplan" grep -q 'quantumlife/pkg/domain/coverageplan' internal/observerconsent/engine.go
check "Engine uses periodKeyFunc" grep -q "periodKeyFunc()" internal/observerconsent/engine.go

echo ""

# ============================================================================
# Section 10: Forbidden Field Validation
# ============================================================================
echo "Section 10: Forbidden Field Validation"

check "ForbiddenClientFields function exists" grep -q "func ForbiddenClientFields" pkg/domain/observerconsent/types.go
check "Forbidden: period" grep -q '"period"' pkg/domain/observerconsent/types.go
check "Forbidden: periodKey" grep -q '"periodKey"' pkg/domain/observerconsent/types.go
check "Forbidden: email" grep -q '"email"' pkg/domain/observerconsent/types.go
check "Forbidden: token" grep -q '"token"' pkg/domain/observerconsent/types.go
check "Forbidden: device" grep -q '"device"' pkg/domain/observerconsent/types.go
check "ValidateNoForbiddenFields called in engine" grep -q "ValidateNoForbiddenFields" internal/observerconsent/engine.go

echo ""

# ============================================================================
# Section 11: Allowlist Enforcement
# ============================================================================
echo "Section 11: Allowlist Enforcement"

check "AllowlistedCapabilities function exists" grep -q "func AllowlistedCapabilities" pkg/domain/observerconsent/types.go
check "IsAllowlisted function exists" grep -q "func IsAllowlisted" pkg/domain/observerconsent/types.go
check "CapReceiptObserver allowlisted" grep -q "CapReceiptObserver" pkg/domain/observerconsent/types.go
check "CapCommerceObserver allowlisted" grep -q "CapCommerceObserver" pkg/domain/observerconsent/types.go
check "CapFinanceCommerceObserver allowlisted" grep -q "CapFinanceCommerceObserver" pkg/domain/observerconsent/types.go
check "CapNotificationMetadata allowlisted" grep -q "CapNotificationMetadata" pkg/domain/observerconsent/types.go
check "Allowlist check in ApplyConsent" grep -q "IsAllowlisted" internal/observerconsent/engine.go
check "RejectNotAllowlisted used in engine" grep -q "RejectNotAllowlisted" internal/observerconsent/engine.go

echo ""

# ============================================================================
# Section 12: Persistence Store - ObserverConsentStore
# ============================================================================
echo "Section 12: Persistence Store - ObserverConsentStore"

check "ObserverConsentStore struct exists" grep -q "type ObserverConsentStore struct" internal/persist/observer_consent_store.go
check "NewObserverConsentStore exists" grep -q "func NewObserverConsentStore" internal/persist/observer_consent_store.go
check "AppendReceipt exists" grep -q "func (s \*ObserverConsentStore) AppendReceipt" internal/persist/observer_consent_store.go
check "ListByCircle exists" grep -q "func (s \*ObserverConsentStore) ListByCircle" internal/persist/observer_consent_store.go
check "ListByCircleAndPeriod exists" grep -q "func (s \*ObserverConsentStore) ListByCircleAndPeriod" internal/persist/observer_consent_store.go
check "ListAll exists" grep -q "func (s \*ObserverConsentStore) ListAll" internal/persist/observer_consent_store.go
check "IsDuplicate exists in consent store" grep -q "func (s \*ObserverConsentStore) IsDuplicate" internal/persist/observer_consent_store.go
check "Store has clock field" grep -q "clock.*func().*time.Time" internal/persist/observer_consent_store.go
check "Store has dedupIndex" grep -q "dedupIndex.*map" internal/persist/observer_consent_store.go
check "Store has mu sync.RWMutex" grep -q "mu.*sync.RWMutex" internal/persist/observer_consent_store.go

echo ""

# ============================================================================
# Section 13: Persistence Store - ObserverConsentAckStore
# ============================================================================
echo "Section 13: Persistence Store - ObserverConsentAckStore"

check "ObserverConsentAckStore struct exists" grep -q "type ObserverConsentAckStore struct" internal/persist/observer_consent_ack_store.go
check "NewObserverConsentAckStore exists" grep -q "func NewObserverConsentAckStore" internal/persist/observer_consent_ack_store.go
check "AppendAck exists" grep -q "func (s \*ObserverConsentAckStore) AppendAck" internal/persist/observer_consent_ack_store.go
check "IsProofDismissed exists" grep -q "func (s \*ObserverConsentAckStore) IsProofDismissed" internal/persist/observer_consent_ack_store.go
check "IsDuplicate exists in ack store" grep -q "func (s \*ObserverConsentAckStore) IsDuplicate" internal/persist/observer_consent_ack_store.go
check "ListAll exists in ack store" grep -q "func (s \*ObserverConsentAckStore) ListAll" internal/persist/observer_consent_ack_store.go

echo ""

# ============================================================================
# Section 14: Bounded Retention
# ============================================================================
echo "Section 14: Bounded Retention"

check "ObserverConsentMaxRecords constant" grep -q "ObserverConsentMaxRecords" internal/persist/observer_consent_store.go
check "Max records is 200" grep -q "ObserverConsentMaxRecords.*=.*200" internal/persist/observer_consent_store.go
check "ObserverConsentMaxRetentionDays constant" grep -q "ObserverConsentMaxRetentionDays" internal/persist/observer_consent_store.go
check "Max days is 30" grep -q "ObserverConsentMaxRetentionDays.*=.*30" internal/persist/observer_consent_store.go
check "evictOldRecordsLocked in consent store" grep -q "evictOldRecordsLocked" internal/persist/observer_consent_store.go
check "ObserverConsentAckMaxRecords constant" grep -q "ObserverConsentAckMaxRecords" internal/persist/observer_consent_ack_store.go
check "Ack max records is 200" grep -q "ObserverConsentAckMaxRecords.*=.*200" internal/persist/observer_consent_ack_store.go
check "evictOldAcksLocked in ack store" grep -q "evictOldAcksLocked" internal/persist/observer_consent_ack_store.go

echo ""

# ============================================================================
# Section 15: Events
# ============================================================================
echo "Section 15: Events"

check "Phase55ObserverConsentPageRendered event" grep -q 'Phase55ObserverConsentPageRendered.*=.*"phase55.observer_consent.page.rendered"' pkg/events/events.go
check "Phase55ObserverConsentRequested event" grep -q 'Phase55ObserverConsentRequested.*=.*"phase55.observer_consent.requested"' pkg/events/events.go
check "Phase55ObserverConsentApplied event" grep -q 'Phase55ObserverConsentApplied.*=.*"phase55.observer_consent.applied"' pkg/events/events.go
check "Phase55ObserverConsentRejected event" grep -q 'Phase55ObserverConsentRejected.*=.*"phase55.observer_consent.rejected"' pkg/events/events.go
check "Phase55ObserverConsentPersisted event" grep -q 'Phase55ObserverConsentPersisted.*=.*"phase55.observer_consent.persisted"' pkg/events/events.go
check "Phase55ObserverConsentProofRendered event" grep -q 'Phase55ObserverConsentProofRendered.*=.*"phase55.observer_consent.proof.rendered"' pkg/events/events.go
check "Phase55ObserverConsentAckDismissed event" grep -q 'Phase55ObserverConsentAckDismissed.*=.*"phase55.observer_consent.ack.dismissed"' pkg/events/events.go

echo ""

# ============================================================================
# Section 16: Storelog Record Types
# ============================================================================
echo "Section 16: Storelog Record Types"

check "RecordTypeObserverConsentReceipt exists" grep -q 'RecordTypeObserverConsentReceipt.*=.*"OBSERVER_CONSENT_RECEIPT"' pkg/domain/storelog/log.go
check "RecordTypeObserverConsentAck exists" grep -q 'RecordTypeObserverConsentAck.*=.*"OBSERVER_CONSENT_ACK"' pkg/domain/storelog/log.go

echo ""

# ============================================================================
# Section 17: Web Routes
# ============================================================================
echo "Section 17: Web Routes"

check "/settings/observers route" grep -q '"/settings/observers"' cmd/quantumlife-web/main.go
check "/settings/observers/enable route" grep -q '"/settings/observers/enable"' cmd/quantumlife-web/main.go
check "/settings/observers/disable route" grep -q '"/settings/observers/disable"' cmd/quantumlife-web/main.go
check "/proof/observers route" grep -q '"/proof/observers"' cmd/quantumlife-web/main.go
check "/proof/observers/dismiss route" grep -q '"/proof/observers/dismiss"' cmd/quantumlife-web/main.go

echo ""

# ============================================================================
# Section 18: POST-only Mutations
# ============================================================================
echo "Section 18: POST-only Mutations"

check "Consent change checks http.MethodPost" grep -q "r.Method != http.MethodPost" cmd/quantumlife-web/main.go
check "Enable handler delegates to consent change" grep -q "handleObserverConsentChange.*ActionEnable" cmd/quantumlife-web/main.go
check "Disable handler delegates to consent change" grep -q "handleObserverConsentChange.*ActionDisable" cmd/quantumlife-web/main.go

echo ""

# ============================================================================
# Section 19: No time.Now() Calls in pkg/ or internal/
# ============================================================================
echo "Section 19: No time.Now() Calls in pkg/ or internal/"

# Check for actual usage patterns like := time.Now() or = time.Now()
check_not "No time.Now() assignment in domain types" grep '=.*time\.Now()' pkg/domain/observerconsent/types.go
check_not "No time.Now() assignment in engine" grep '=.*time\.Now()' internal/observerconsent/engine.go
check_not "No time.Now() assignment in consent store" grep '=.*time\.Now()' internal/persist/observer_consent_store.go
check_not "No time.Now() assignment in ack store" grep '=.*time\.Now()' internal/persist/observer_consent_ack_store.go

echo ""

# ============================================================================
# Section 20: No Goroutines in pkg/ or internal/
# ============================================================================
echo "Section 20: No Goroutines in pkg/ or internal/"

check_not "No goroutines in domain types" grep -r "go func" pkg/domain/observerconsent/
check_not "No goroutines in engine" grep -r "go func" internal/observerconsent/
check_not "No goroutines in consent store" grep "go func" internal/persist/observer_consent_store.go
check_not "No goroutines in ack store" grep "go func" internal/persist/observer_consent_ack_store.go

echo ""

# ============================================================================
# Section 21: No Forbidden Imports in Domain
# ============================================================================
echo "Section 21: No Forbidden Imports in Domain"

check_not "No pressuredecision import" grep 'quantumlife.*pressuredecision' pkg/domain/observerconsent/types.go
check_not "No interruptpolicy import" grep 'quantumlife.*interruptpolicy' pkg/domain/observerconsent/types.go
check_not "No delivery import" grep 'quantumlife.*delivery' pkg/domain/observerconsent/types.go
check_not "No execution import" grep 'quantumlife.*execution' pkg/domain/observerconsent/types.go

echo ""

# ============================================================================
# Section 22: No Forbidden Tokens in Files
# ============================================================================
echo "Section 22: No Forbidden Tokens in Files"

check_not "No @ symbol in domain" grep "@" pkg/domain/observerconsent/types.go
check_not "No http:// in domain" grep "http://" pkg/domain/observerconsent/types.go
check_not "No https:// in domain" grep "https://" pkg/domain/observerconsent/types.go
check_not "No @ symbol in engine" grep "@" internal/observerconsent/engine.go
check_not "No http:// in engine" grep "http://" internal/observerconsent/engine.go
check_not "No https:// in engine" grep "https://" internal/observerconsent/engine.go
check_not "No @ symbol in consent store" grep "@" internal/persist/observer_consent_store.go
check_not "No http:// in consent store" grep "http://" internal/persist/observer_consent_store.go
check_not "No https:// in consent store" grep "https://" internal/persist/observer_consent_store.go
check_not "No @ symbol in ack store" grep "@" internal/persist/observer_consent_ack_store.go
check_not "No http:// in ack store" grep "http://" internal/persist/observer_consent_ack_store.go
check_not "No https:// in ack store" grep "https://" internal/persist/observer_consent_ack_store.go

echo ""

# ============================================================================
# Section 23: Pipe-delimited Canonical Strings
# ============================================================================
echo "Section 23: Pipe-delimited Canonical Strings"

check "Sprintf uses pipe delimiter" grep -q '%s|%s|%s' pkg/domain/observerconsent/types.go
check "DedupKey function exists" grep -q "func (r ObserverConsentReceipt) DedupKey()" pkg/domain/observerconsent/types.go
check "Ack DedupKey function exists" grep -q "func (a ObserverConsentAck) DedupKey()" pkg/domain/observerconsent/types.go

echo ""

# ============================================================================
# Section 24: Hash-Only Storage
# ============================================================================
echo "Section 24: Hash-Only Storage"

check "CircleIDHash field in Receipt" grep -q "CircleIDHash.*string" pkg/domain/observerconsent/types.go
check "ReceiptHash field in Receipt" grep -q "ReceiptHash.*string" pkg/domain/observerconsent/types.go
check "CircleIDHash field in Ack" grep -q "CircleIDHash" pkg/domain/observerconsent/types.go
check "HashCircleID helper exists" grep -q "func HashCircleID" pkg/domain/observerconsent/types.go

echo ""

# ============================================================================
# Section 25: Period Key Server-Derived
# ============================================================================
echo "Section 25: Period Key Server-Derived"

check "Engine uses periodKeyFunc" grep -q "periodKeyFunc()" internal/observerconsent/engine.go
check "Period key forbidden in client fields" grep -q '"periodKey"' pkg/domain/observerconsent/types.go
check "Period forbidden in client fields" grep -q '"period"' pkg/domain/observerconsent/types.go

echo ""

# ============================================================================
# Section 26: Proof Page Types
# ============================================================================
echo "Section 26: Proof Page Types"

check "ObserverProofPage has Title" grep -q "Title.*string" pkg/domain/observerconsent/types.go
check "ObserverProofPage has Receipts" grep -q "Receipts.*\[\]ObserverConsentReceipt" pkg/domain/observerconsent/types.go
check "ObserverProofPage has StatusHash" grep -q "StatusHash.*string" pkg/domain/observerconsent/types.go
check "ObserverProofPage has MaxReceipts" grep -q "MaxReceipts.*int" pkg/domain/observerconsent/types.go

echo ""

# ============================================================================
# Section 27: ADR Content
# ============================================================================
echo "Section 27: ADR Content"

check "ADR has Status section" grep -q "## Status" docs/ADR/ADR-0092-phase55-observer-consent-activation-ui.md
check "ADR has Context section" grep -q "## Context" docs/ADR/ADR-0092-phase55-observer-consent-activation-ui.md
check "ADR has Decision section" grep -q "## Decision" docs/ADR/ADR-0092-phase55-observer-consent-activation-ui.md
check "ADR mentions observation" grep -qi "observation" docs/ADR/ADR-0092-phase55-observer-consent-activation-ui.md
check "ADR mentions consent" grep -qi "consent" docs/ADR/ADR-0092-phase55-observer-consent-activation-ui.md
check "ADR mentions POST-only" grep -qi "POST" docs/ADR/ADR-0092-phase55-observer-consent-activation-ui.md
check "ADR mentions bounded retention" grep -qi "bounded" docs/ADR/ADR-0092-phase55-observer-consent-activation-ui.md
check "ADR mentions hash" grep -qi "hash" docs/ADR/ADR-0092-phase55-observer-consent-activation-ui.md

echo ""

# ============================================================================
# Section 28: Template Checks
# ============================================================================
echo "Section 28: Template Checks"

check "observer_settings template exists" grep -q 'define "observer_settings"' cmd/quantumlife-web/main.go
check "observer_proof template exists" grep -q 'define "observer_proof"' cmd/quantumlife-web/main.go
check "Settings template uses ObserverSettingsPage" grep -q "ObserverSettingsPage" cmd/quantumlife-web/main.go
check "Proof template uses ObserverProofPage" grep -q "ObserverProofPage" cmd/quantumlife-web/main.go

echo ""

# ============================================================================
# Section 29: Coverage Plan Integration
# ============================================================================
echo "Section 29: Coverage Plan Integration"

check "Engine imports coverageplan" grep -q 'coverageplan' internal/observerconsent/engine.go
check "Domain imports coverageplan" grep -q 'coverageplan' pkg/domain/observerconsent/types.go
check "CoverageCapability type used" grep -q "CoverageCapability" pkg/domain/observerconsent/types.go

echo ""

# ============================================================================
# Section 30: Makefile Targets
# ============================================================================
echo "Section 30: Makefile Targets"

check "demo-phase55 target exists" grep -q "demo-phase55" Makefile
check "check-observer-consent target exists" grep -q "check-observer-consent" Makefile

echo ""

# ============================================================================
# Summary
# ============================================================================
echo "=============================================="
echo "Summary"
echo "=============================================="
echo ""
echo "Passed: $PASS_COUNT"
echo "Failed: $FAIL_COUNT"
echo ""

if [ $FAIL_COUNT -gt 0 ]; then
    echo "Some guardrails failed!"
    exit 1
else
    echo "All Phase 55 guardrails passed!"
    exit 0
fi
