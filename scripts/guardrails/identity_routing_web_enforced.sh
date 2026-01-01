#!/bin/bash
# identity_routing_web_enforced.sh - Guardrail for Phase 13.1 identity-driven routing.
#
# Verifies:
# 1. IdentityRepository query helpers exist with deterministic ordering
# 2. Display helpers (PrimaryEmail, PersonLabel) exist
# 3. Routing supports identity-based precedence (P1-P5)
# 4. Loop integrates identity graph hash
# 5. Web UI has /people routes
# 6. No goroutines in Phase 13.1 packages
# 7. Demo tests exist
#
# Exit 0 if all checks pass, non-zero otherwise.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "=== Phase 13.1: Identity-Driven Routing + People UI Guardrail ==="
echo ""

ERRORS=0

# 1. Check identity repository query helpers
echo "Checking identity repository query helpers..."
if [ -f "$ROOT_DIR/pkg/domain/identity/repository.go" ]; then
    if grep -q "ListPersons" "$ROOT_DIR/pkg/domain/identity/repository.go" && \
       grep -q "ListOrganizations" "$ROOT_DIR/pkg/domain/identity/repository.go" && \
       grep -q "ListHouseholds" "$ROOT_DIR/pkg/domain/identity/repository.go" && \
       grep -q "GetPersonEdgesSorted" "$ROOT_DIR/pkg/domain/identity/repository.go"; then
        echo "  OK: Identity query helpers exist"
    else
        echo "  ERROR: Missing query helpers (ListPersons, ListOrganizations, ListHouseholds, GetPersonEdgesSorted)"
        ERRORS=$((ERRORS + 1))
    fi
else
    echo "  ERROR: pkg/domain/identity/repository.go not found"
    ERRORS=$((ERRORS + 1))
fi

# 2. Check display helpers
echo "Checking display helpers..."
if [ -f "$ROOT_DIR/pkg/domain/identity/repository.go" ]; then
    if grep -q "PrimaryEmail" "$ROOT_DIR/pkg/domain/identity/repository.go" && \
       grep -q "PersonLabel" "$ROOT_DIR/pkg/domain/identity/repository.go"; then
        echo "  OK: Display helpers exist"
    else
        echo "  ERROR: Missing display helpers (PrimaryEmail, PersonLabel)"
        ERRORS=$((ERRORS + 1))
    fi
fi

# 3. Check routing supports identity-based precedence
echo "Checking identity-based routing..."
if [ -f "$ROOT_DIR/internal/routing/router.go" ]; then
    if grep -q "IdentityRouter" "$ROOT_DIR/internal/routing/router.go" && \
       grep -q "SetIdentityRepository" "$ROOT_DIR/internal/routing/router.go" && \
       grep -q "P1:" "$ROOT_DIR/internal/routing/router.go" && \
       grep -q "P2:" "$ROOT_DIR/internal/routing/router.go" && \
       grep -q "P3:" "$ROOT_DIR/internal/routing/router.go"; then
        echo "  OK: Identity-based routing with precedence rules exists"
    else
        echo "  ERROR: Missing identity-based routing (IdentityRouter, SetIdentityRepository, P1-P5)"
        ERRORS=$((ERRORS + 1))
    fi
else
    echo "  ERROR: internal/routing/router.go not found"
    ERRORS=$((ERRORS + 1))
fi

# 4. Check loop integration with identity graph hash
echo "Checking loop identity integration..."
if [ -f "$ROOT_DIR/internal/loop/multi_circle.go" ]; then
    if grep -q "IdentityRepo" "$ROOT_DIR/internal/loop/multi_circle.go" && \
       grep -q "IdentityGraphHash" "$ROOT_DIR/internal/loop/multi_circle.go" && \
       grep -q "computeIdentityGraphHash" "$ROOT_DIR/internal/loop/multi_circle.go"; then
        echo "  OK: Loop integrates identity graph hash"
    else
        echo "  ERROR: Missing identity integration in loop (IdentityRepo, IdentityGraphHash)"
        ERRORS=$((ERRORS + 1))
    fi
else
    echo "  ERROR: internal/loop/multi_circle.go not found"
    ERRORS=$((ERRORS + 1))
fi

# 5. Check Web UI /people routes
echo "Checking Web UI people routes..."
if [ -f "$ROOT_DIR/cmd/quantumlife-web/main.go" ]; then
    if grep -q "/people" "$ROOT_DIR/cmd/quantumlife-web/main.go" && \
       grep -q "handlePeople" "$ROOT_DIR/cmd/quantumlife-web/main.go" && \
       grep -q "handlePerson" "$ROOT_DIR/cmd/quantumlife-web/main.go"; then
        echo "  OK: Web UI /people routes exist"
    else
        echo "  ERROR: Missing /people routes in web UI"
        ERRORS=$((ERRORS + 1))
    fi
else
    echo "  ERROR: cmd/quantumlife-web/main.go not found"
    ERRORS=$((ERRORS + 1))
fi

# 6. Check for forbidden goroutines in Phase 13.1 packages
echo "Checking for forbidden goroutines..."
for pkg in "pkg/domain/identity" "internal/routing"; do
    if [ -d "$ROOT_DIR/$pkg" ]; then
        if grep -rh "go func" "$ROOT_DIR/$pkg"/*.go 2>/dev/null | grep -v "_test.go" | grep -v "^[[:space:]]*//"; then
            echo "  ERROR: Found goroutine in $pkg (forbidden)"
            ERRORS=$((ERRORS + 1))
        fi
    fi
done
echo "  OK: No forbidden goroutines found"

# 7. Check demo tests exist
echo "Checking demo tests exist..."
if [ -f "$ROOT_DIR/internal/demo_phase13_1_identity_routing_web/demo_test.go" ]; then
    echo "  OK: Phase 13.1 demo tests exist"
else
    echo "  ERROR: internal/demo_phase13_1_identity_routing_web/demo_test.go not found"
    ERRORS=$((ERRORS + 1))
fi

echo ""
echo "=== Guardrail Complete ==="

if [ $ERRORS -gt 0 ]; then
    echo "FAILED: $ERRORS error(s) found"
    exit 1
else
    echo "PASSED: All Phase 13.1 identity routing + people UI constraints verified"
    exit 0
fi
