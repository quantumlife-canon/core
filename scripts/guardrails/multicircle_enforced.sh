#!/bin/bash
# multicircle_enforced.sh - Guardrail for Phase 11 multi-circle constraints.
#
# Verifies:
# 1. Config loader uses stdlib only (no YAML/JSON libs)
# 2. Multi-runner processes circles in deterministic order
# 3. Routing is deterministic (no randomness)
# 4. Events tagged with CircleID before storage
# 5. No goroutines in multi-circle packages
# 6. Phase 11 events are defined
#
# Exit 0 if all checks pass, non-zero otherwise.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "=== Phase 11: Multi-Circle Guardrail ==="
echo ""

ERRORS=0

# 1. Check config loader uses stdlib only
echo "Checking config loader uses stdlib only..."
if [ -f "$ROOT_DIR/internal/config/loader.go" ]; then
    if grep -q '"gopkg.in/yaml' "$ROOT_DIR/internal/config/loader.go" || \
       grep -q '"github.com/.*yaml' "$ROOT_DIR/internal/config/loader.go" || \
       grep -q '"encoding/json"' "$ROOT_DIR/internal/config/loader.go"; then
        echo "  ERROR: Config loader must use stdlib only (no YAML/JSON libs)"
        ERRORS=$((ERRORS + 1))
    else
        echo "  OK: Config loader uses stdlib only"
    fi
else
    echo "  ERROR: internal/config/loader.go not found"
    ERRORS=$((ERRORS + 1))
fi

# 2. Check multi-runner processes circles in sorted order
echo "Checking multi-runner deterministic order..."
if [ -f "$ROOT_DIR/internal/ingestion/multi_runner.go" ]; then
    if grep -q "sort.Slice\|sort.Sort\|sort.Strings" "$ROOT_DIR/internal/ingestion/multi_runner.go"; then
        echo "  OK: Multi-runner uses sorted order for circles"
    else
        echo "  ERROR: Multi-runner must process circles in sorted order"
        ERRORS=$((ERRORS + 1))
    fi
else
    echo "  ERROR: internal/ingestion/multi_runner.go not found"
    ERRORS=$((ERRORS + 1))
fi

# 3. Check router is deterministic (no rand usage)
echo "Checking router determinism..."
if [ -f "$ROOT_DIR/internal/routing/router.go" ]; then
    if grep -q '"math/rand"' "$ROOT_DIR/internal/routing/router.go"; then
        echo "  ERROR: Router must not use random selection"
        ERRORS=$((ERRORS + 1))
    else
        echo "  OK: Router does not use randomness"
    fi
else
    echo "  ERROR: internal/routing/router.go not found"
    ERRORS=$((ERRORS + 1))
fi

# 4. Check events can be tagged with CircleID
echo "Checking events can be tagged with CircleID..."
if grep -q "SetCircleID" "$ROOT_DIR/pkg/domain/events/canonical.go"; then
    echo "  OK: Events support CircleID tagging"
else
    echo "  ERROR: Events must support SetCircleID"
    ERRORS=$((ERRORS + 1))
fi

# 5. No goroutines in multi-circle packages
echo "Checking for forbidden goroutines..."
for pkg in "internal/config" "internal/routing" "internal/ingestion"; do
    if [ -d "$ROOT_DIR/$pkg" ]; then
        if grep -r "go func" "$ROOT_DIR/$pkg"/*.go 2>/dev/null | grep -v "_test.go"; then
            echo "  ERROR: Found goroutine in $pkg (forbidden)"
            ERRORS=$((ERRORS + 1))
        fi
    fi
done
echo "  OK: No forbidden goroutines found"

# 6. Check multi-circle runner validates config hash
echo "Checking multi-circle runner uses config hash..."
if [ -f "$ROOT_DIR/internal/loop/multi_circle.go" ]; then
    if grep -q "Config.Hash()\|computeResultHash\|computeRunID" "$ROOT_DIR/internal/loop/multi_circle.go"; then
        echo "  OK: Multi-circle runner uses deterministic hashing"
    else
        echo "  ERROR: Multi-circle runner must use deterministic hashing"
        ERRORS=$((ERRORS + 1))
    fi
else
    echo "  ERROR: internal/loop/multi_circle.go not found"
    ERRORS=$((ERRORS + 1))
fi

# 7. Check Phase 11 events are defined
echo "Checking Phase 11 events are defined..."
if grep -q "Phase11MultiCircleRunStarted" "$ROOT_DIR/pkg/events/events.go" && \
   grep -q "Phase11MultiCircleRunCompleted" "$ROOT_DIR/pkg/events/events.go" && \
   grep -q "Phase11ConfigLoaded" "$ROOT_DIR/pkg/events/events.go"; then
    echo "  OK: Phase 11 events are defined"
else
    echo "  ERROR: Phase 11 events must be defined in pkg/events/events.go"
    ERRORS=$((ERRORS + 1))
fi

# 8. Check config types exist (in pkg/domain/config for proper layering)
echo "Checking config types exist..."
if [ -f "$ROOT_DIR/pkg/domain/config/types.go" ]; then
    if grep -q "MultiCircleConfig" "$ROOT_DIR/pkg/domain/config/types.go" && \
       grep -q "CircleConfig" "$ROOT_DIR/pkg/domain/config/types.go" && \
       grep -q "RoutingConfig" "$ROOT_DIR/pkg/domain/config/types.go"; then
        echo "  OK: Config types are defined in pkg/domain/config"
    else
        echo "  ERROR: Config types incomplete"
        ERRORS=$((ERRORS + 1))
    fi
else
    echo "  ERROR: pkg/domain/config/types.go not found"
    ERRORS=$((ERRORS + 1))
fi

# 9. Check router tests exist
echo "Checking router tests exist..."
if [ -f "$ROOT_DIR/internal/routing/router_test.go" ]; then
    if grep -q "TestRouter_Determinism" "$ROOT_DIR/internal/routing/router_test.go"; then
        echo "  OK: Router determinism tests exist"
    else
        echo "  WARNING: Router should have determinism tests"
    fi
else
    echo "  ERROR: internal/routing/router_test.go not found"
    ERRORS=$((ERRORS + 1))
fi

# 10. Check demo tests exist
echo "Checking demo tests exist..."
if [ -d "$ROOT_DIR/internal/demo_phase11_multicircle" ]; then
    if [ -f "$ROOT_DIR/internal/demo_phase11_multicircle/demo_test.go" ]; then
        echo "  OK: Phase 11 demo tests exist"
    else
        echo "  ERROR: Phase 11 demo_test.go not found"
        ERRORS=$((ERRORS + 1))
    fi
else
    echo "  ERROR: internal/demo_phase11_multicircle directory not found"
    ERRORS=$((ERRORS + 1))
fi

echo ""
echo "=== Guardrail Complete ==="

if [ $ERRORS -gt 0 ]; then
    echo "FAILED: $ERRORS error(s) found"
    exit 1
else
    echo "PASSED: All Phase 11 multi-circle constraints verified"
    exit 0
fi
