#!/bin/bash
# identity_graph_enforced.sh - Guardrail for Phase 13 identity graph constraints.
#
# Verifies:
# 1. Identity types package exists with required types
# 2. Edge types are defined
# 3. Identity resolution engine exists
# 4. Identity persistence with replay
# 5. No goroutines in Phase 13 packages
# 6. No time.Now() in Phase 13 packages
# 7. Canonical strings use pipe-delimited format (NOT JSON)
# 8. Demo tests exist
#
# Exit 0 if all checks pass, non-zero otherwise.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "=== Phase 13: Identity Graph Guardrail ==="
echo ""

ERRORS=0

# 1. Check identity types package exists
echo "Checking identity types package..."
if [ -f "$ROOT_DIR/pkg/domain/identity/types.go" ]; then
    # Check for required entity types
    if grep -q "EntityTypePerson" "$ROOT_DIR/pkg/domain/identity/types.go" && \
       grep -q "EntityTypeOrganization" "$ROOT_DIR/pkg/domain/identity/types.go" && \
       grep -q "EntityTypeHousehold" "$ROOT_DIR/pkg/domain/identity/types.go" && \
       grep -q "EntityTypePhoneNumber" "$ROOT_DIR/pkg/domain/identity/types.go"; then
        echo "  OK: Identity entity types defined"
    else
        echo "  ERROR: Missing required entity types (Person, Organization, Household, PhoneNumber)"
        ERRORS=$((ERRORS + 1))
    fi
else
    echo "  ERROR: pkg/domain/identity/types.go not found"
    ERRORS=$((ERRORS + 1))
fi

# 2. Check edge types are defined
echo "Checking edge types..."
if [ -f "$ROOT_DIR/pkg/domain/identity/types.go" ]; then
    if grep -q "EdgeType" "$ROOT_DIR/pkg/domain/identity/types.go" && \
       grep -q "EdgeTypeOwnsEmail" "$ROOT_DIR/pkg/domain/identity/types.go" && \
       grep -q "EdgeTypeSpouseOf" "$ROOT_DIR/pkg/domain/identity/types.go" && \
       grep -q "EdgeTypeWorksAt" "$ROOT_DIR/pkg/domain/identity/types.go"; then
        echo "  OK: Edge types defined"
    else
        echo "  ERROR: Missing required edge types (EdgeType, EdgeTypeOwnsEmail, EdgeTypeSpouseOf, EdgeTypeWorksAt)"
        ERRORS=$((ERRORS + 1))
    fi
fi

# 3. Check identity resolution engine exists
echo "Checking identity resolution engine..."
if [ -f "$ROOT_DIR/internal/identityresolve/resolver.go" ]; then
    if grep -q "type Resolver struct" "$ROOT_DIR/internal/identityresolve/resolver.go" && \
       grep -q "ProcessEvent" "$ROOT_DIR/internal/identityresolve/resolver.go"; then
        echo "  OK: Identity resolution engine exists"
    else
        echo "  ERROR: Resolver struct or ProcessEvent method not found"
        ERRORS=$((ERRORS + 1))
    fi
else
    echo "  ERROR: internal/identityresolve/resolver.go not found"
    ERRORS=$((ERRORS + 1))
fi

# 4. Check identity persistence with replay
echo "Checking identity persistence..."
if [ -f "$ROOT_DIR/internal/persist/identity_store.go" ]; then
    if grep -q "replay" "$ROOT_DIR/internal/persist/identity_store.go" && \
       grep -q "StoreEntity" "$ROOT_DIR/internal/persist/identity_store.go" && \
       grep -q "StoreEdge" "$ROOT_DIR/internal/persist/identity_store.go"; then
        echo "  OK: Identity persistence with replay exists"
    else
        echo "  ERROR: Identity store missing replay or store methods"
        ERRORS=$((ERRORS + 1))
    fi
else
    echo "  ERROR: internal/persist/identity_store.go not found"
    ERRORS=$((ERRORS + 1))
fi

# 5. Check identity record types in storelog
echo "Checking identity record types in storelog..."
if [ -f "$ROOT_DIR/pkg/domain/storelog/log.go" ]; then
    if grep -q "RecordTypeIdentityEntity" "$ROOT_DIR/pkg/domain/storelog/log.go" && \
       grep -q "RecordTypeIdentityEdge" "$ROOT_DIR/pkg/domain/storelog/log.go"; then
        echo "  OK: Identity record types defined in storelog"
    else
        echo "  ERROR: Missing identity record types in storelog"
        ERRORS=$((ERRORS + 1))
    fi
fi

# 6. No goroutines in Phase 13 packages
echo "Checking for forbidden goroutines..."
for pkg in "pkg/domain/identity" "internal/identityresolve" "internal/persist/identity_store.go"; do
    if [ -e "$ROOT_DIR/$pkg" ]; then
        if [ -d "$ROOT_DIR/$pkg" ]; then
            if grep -r "go func" "$ROOT_DIR/$pkg"/*.go 2>/dev/null | grep -v "_test.go"; then
                echo "  ERROR: Found goroutine in $pkg (forbidden)"
                ERRORS=$((ERRORS + 1))
            fi
        elif [ -f "$ROOT_DIR/$pkg" ]; then
            if grep "go func" "$ROOT_DIR/$pkg" 2>/dev/null | grep -v "_test.go"; then
                echo "  ERROR: Found goroutine in $pkg (forbidden)"
                ERRORS=$((ERRORS + 1))
            fi
        fi
    fi
done
echo "  OK: No forbidden goroutines found"

# 7. No time.Now() in Phase 13 packages
echo "Checking for forbidden time.Now()..."
for pkg in "pkg/domain/identity" "internal/identityresolve"; do
    if [ -d "$ROOT_DIR/$pkg" ]; then
        if grep -rh "time.Now()" "$ROOT_DIR/$pkg"/*.go 2>/dev/null | grep -v "_test.go" | grep -v "^[[:space:]]*//"; then
            echo "  ERROR: Found time.Now() in $pkg (forbidden - use injected clock)"
            ERRORS=$((ERRORS + 1))
        fi
    fi
done
echo "  OK: No forbidden time.Now() found"

# 8. Check canonical string format (NOT JSON)
echo "Checking canonical string format..."
if [ -f "$ROOT_DIR/internal/identityresolve/resolver.go" ]; then
    if grep -q "CanonicalString" "$ROOT_DIR/internal/identityresolve/resolver.go"; then
        # Ensure we're using pipe-delimited, not JSON
        if grep -q 'json.Marshal' "$ROOT_DIR/internal/identityresolve/resolver.go"; then
            echo "  ERROR: CanonicalString should use pipe-delimited format, NOT JSON"
            ERRORS=$((ERRORS + 1))
        else
            echo "  OK: Canonical strings use pipe-delimited format"
        fi
    fi
fi

# 9. Check demo tests exist
echo "Checking demo tests exist..."
if [ -f "$ROOT_DIR/internal/demo_phase13_identity_graph/demo_test.go" ]; then
    echo "  OK: Phase 13 demo tests exist"
else
    echo "  ERROR: internal/demo_phase13_identity_graph/demo_test.go not found"
    ERRORS=$((ERRORS + 1))
fi

# 10. Check Confidence enum exists
echo "Checking Confidence enum..."
if [ -f "$ROOT_DIR/pkg/domain/identity/types.go" ]; then
    if grep -q "type Confidence" "$ROOT_DIR/pkg/domain/identity/types.go" && \
       grep -q "ConfidenceHigh" "$ROOT_DIR/pkg/domain/identity/types.go" && \
       grep -q "ConfidenceMedium" "$ROOT_DIR/pkg/domain/identity/types.go"; then
        echo "  OK: Confidence enum defined"
    else
        echo "  ERROR: Confidence enum not properly defined"
        ERRORS=$((ERRORS + 1))
    fi
fi

echo ""
echo "=== Guardrail Complete ==="

if [ $ERRORS -gt 0 ]; then
    echo "FAILED: $ERRORS error(s) found"
    exit 1
else
    echo "PASSED: All Phase 13 identity graph constraints verified"
    exit 0
fi
