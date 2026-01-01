#!/bin/bash
# persistence_replay_enforced.sh - Guardrail for Phase 12 persistence and replay constraints.
#
# Verifies:
# 1. Append-only log uses stdlib only (no external libs)
# 2. Log records include hash verification
# 3. Persistent stores implement replay
# 4. Run snapshots compute deterministic hashes
# 5. No goroutines in Phase 12 packages
# 6. No time.Now() in Phase 12 packages
# 7. Phase 12 events/types are defined
#
# Exit 0 if all checks pass, non-zero otherwise.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "=== Phase 12: Persistence & Replay Guardrail ==="
echo ""

ERRORS=0

# 1. Check storelog uses stdlib only
echo "Checking storelog uses stdlib only..."
if [ -d "$ROOT_DIR/pkg/domain/storelog" ]; then
    if grep -rq 'github.com/' "$ROOT_DIR/pkg/domain/storelog"/*.go 2>/dev/null; then
        echo "  ERROR: storelog must use stdlib only (no github.com imports)"
        ERRORS=$((ERRORS + 1))
    else
        echo "  OK: storelog uses stdlib only"
    fi
else
    echo "  ERROR: pkg/domain/storelog not found"
    ERRORS=$((ERRORS + 1))
fi

# 2. Check log records include hash verification
echo "Checking log records include hash verification..."
if [ -f "$ROOT_DIR/pkg/domain/storelog/log.go" ]; then
    if grep -q "ComputeHash" "$ROOT_DIR/pkg/domain/storelog/log.go" && \
       grep -q "sha256" "$ROOT_DIR/pkg/domain/storelog/log.go"; then
        echo "  OK: Log records include hash verification"
    else
        echo "  ERROR: Log records must include hash verification (ComputeHash + sha256)"
        ERRORS=$((ERRORS + 1))
    fi
else
    echo "  ERROR: pkg/domain/storelog/log.go not found"
    ERRORS=$((ERRORS + 1))
fi

# 3. Check persistent stores implement replay
echo "Checking persistent stores implement replay..."
if [ -d "$ROOT_DIR/internal/persist" ]; then
    if grep -rq "replay" "$ROOT_DIR/internal/persist"/*.go 2>/dev/null; then
        echo "  OK: Persistent stores implement replay"
    else
        echo "  ERROR: Persistent stores must implement replay()"
        ERRORS=$((ERRORS + 1))
    fi
else
    echo "  ERROR: internal/persist not found"
    ERRORS=$((ERRORS + 1))
fi

# 4. Check run snapshots compute deterministic hashes
echo "Checking run snapshots compute deterministic hashes..."
if [ -f "$ROOT_DIR/pkg/domain/runlog/snapshot.go" ]; then
    if grep -q "ComputeResultHash" "$ROOT_DIR/pkg/domain/runlog/snapshot.go" && \
       grep -q "FinalizeSnapshot" "$ROOT_DIR/pkg/domain/runlog/snapshot.go"; then
        echo "  OK: Run snapshots compute deterministic hashes"
    else
        echo "  ERROR: Run snapshots must compute deterministic hashes"
        ERRORS=$((ERRORS + 1))
    fi
else
    echo "  ERROR: pkg/domain/runlog/snapshot.go not found"
    ERRORS=$((ERRORS + 1))
fi

# 5. No goroutines in Phase 12 packages
echo "Checking for forbidden goroutines..."
for pkg in "pkg/domain/storelog" "pkg/domain/runlog" "internal/persist"; do
    if [ -d "$ROOT_DIR/$pkg" ]; then
        if grep -r "go func" "$ROOT_DIR/$pkg"/*.go 2>/dev/null | grep -v "_test.go"; then
            echo "  ERROR: Found goroutine in $pkg (forbidden)"
            ERRORS=$((ERRORS + 1))
        fi
    fi
done
echo "  OK: No forbidden goroutines found"

# 6. No time.Now() in Phase 12 packages (excluding comments)
echo "Checking for forbidden time.Now()..."
for pkg in "pkg/domain/storelog" "pkg/domain/runlog" "internal/persist"; do
    if [ -d "$ROOT_DIR/$pkg" ]; then
        # Only match actual code usage, not comments (lines starting with //)
        if grep -rh "time.Now()" "$ROOT_DIR/$pkg"/*.go 2>/dev/null | grep -v "_test.go" | grep -v "^[[:space:]]*//" | grep -v "No time.Now"; then
            echo "  ERROR: Found time.Now() in $pkg (forbidden - use injected clock)"
            ERRORS=$((ERRORS + 1))
        fi
    fi
done
echo "  OK: No forbidden time.Now() found"

# 7. Check RunStore interface exists
echo "Checking RunStore interface exists..."
if [ -d "$ROOT_DIR/pkg/domain/runlog" ]; then
    if grep -rq "type RunStore interface" "$ROOT_DIR/pkg/domain/runlog"/*.go 2>/dev/null; then
        echo "  OK: RunStore interface exists"
    else
        echo "  ERROR: RunStore interface not found"
        ERRORS=$((ERRORS + 1))
    fi
else
    echo "  ERROR: pkg/domain/runlog not found"
    ERRORS=$((ERRORS + 1))
fi

# 8. Check AppendOnlyLog interface exists
echo "Checking AppendOnlyLog interface exists..."
if [ -f "$ROOT_DIR/pkg/domain/storelog/log.go" ]; then
    if grep -q "type AppendOnlyLog interface" "$ROOT_DIR/pkg/domain/storelog/log.go"; then
        echo "  OK: AppendOnlyLog interface exists"
    else
        echo "  ERROR: AppendOnlyLog interface not found"
        ERRORS=$((ERRORS + 1))
    fi
else
    echo "  ERROR: pkg/domain/storelog/log.go not found"
    ERRORS=$((ERRORS + 1))
fi

# 9. Check demo tests exist
echo "Checking demo tests exist..."
if [ -f "$ROOT_DIR/internal/demo_phase12_persistence_replay/demo_test.go" ]; then
    echo "  OK: Phase 12 demo tests exist"
else
    echo "  ERROR: internal/demo_phase12_persistence_replay/demo_test.go not found"
    ERRORS=$((ERRORS + 1))
fi

# 10. Check replay verification exists
echo "Checking replay verification exists..."
if [ -f "$ROOT_DIR/pkg/domain/runlog/snapshot.go" ]; then
    if grep -q "VerifyReplay" "$ROOT_DIR/pkg/domain/runlog/snapshot.go"; then
        echo "  OK: Replay verification exists"
    else
        echo "  ERROR: VerifyReplay function not found"
        ERRORS=$((ERRORS + 1))
    fi
else
    echo "  ERROR: pkg/domain/runlog/snapshot.go not found"
    ERRORS=$((ERRORS + 1))
fi

echo ""
echo "=== Guardrail Complete ==="

if [ $ERRORS -gt 0 ]; then
    echo "FAILED: $ERRORS error(s) found"
    exit 1
else
    echo "PASSED: All Phase 12 persistence & replay constraints verified"
    exit 0
fi
