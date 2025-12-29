#!/usr/bin/env bash
#
# dependency_policy.sh
#
# Enforces dependency policy for QuantumLife.
# Currently: standard library only. New dependencies require ADR + approval.
#
# Reference: docs/TECHNOLOGY_SELECTION_V1.md §11 Non-Choices
#
# Exit codes:
#   0 - No policy violations
#   1 - Unauthorized dependencies found
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
GO_MOD="${REPO_ROOT}/go.mod"

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

echo "=========================================="
echo "Dependency Policy Check"
echo "=========================================="
echo "Policy: Standard library only (v1)"
echo "New dependencies require ADR + explicit approval"
echo ""

if [[ ! -f "$GO_MOD" ]]; then
    echo -e "${RED}ERROR: go.mod not found at ${GO_MOD}${NC}"
    exit 1
fi

# Check for require blocks or single require statements
# Allowed lines:
#   - module quantumlife
#   - go X.Y
#   - empty lines
#   - comments
#   - toolchain (Go 1.21+)

violations_found=0
in_require_block=false

while IFS= read -r line; do
    # Skip empty lines
    [[ -z "$line" ]] && continue

    # Skip comments
    [[ "$line" =~ ^[[:space:]]*// ]] && continue

    # Check for require block start
    if [[ "$line" =~ ^require[[:space:]]*\( ]]; then
        in_require_block=true
        continue
    fi

    # Check for require block end
    if [[ "$in_require_block" == "true" && "$line" =~ ^\) ]]; then
        in_require_block=false
        continue
    fi

    # If inside require block, any line is a dependency
    if [[ "$in_require_block" == "true" ]]; then
        # Skip empty lines and comments inside block
        trimmed=$(echo "$line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
        [[ -z "$trimmed" ]] && continue
        [[ "$trimmed" =~ ^// ]] && continue

        echo -e "${RED}VIOLATION: Unauthorized dependency${NC}"
        echo "  $trimmed"
        violations_found=1
        continue
    fi

    # Check for single-line require
    if [[ "$line" =~ ^require[[:space:]] ]]; then
        dep=$(echo "$line" | sed 's/^require[[:space:]]*//')
        echo -e "${RED}VIOLATION: Unauthorized dependency${NC}"
        echo "  $dep"
        violations_found=1
        continue
    fi

    # Allowed directives
    if [[ "$line" =~ ^module[[:space:]] ]]; then
        continue
    fi
    if [[ "$line" =~ ^go[[:space:]] ]]; then
        continue
    fi
    if [[ "$line" =~ ^toolchain[[:space:]] ]]; then
        continue
    fi

done < "$GO_MOD"

echo ""

if [[ $violations_found -eq 1 ]]; then
    echo -e "${RED}FAILED: Unauthorized dependencies found${NC}"
    echo ""
    echo "QuantumLife v1 uses standard library only."
    echo ""
    echo "To add a new dependency:"
    echo "  1. Create an ADR in docs/ADR/ explaining:"
    echo "     - Why the dependency is needed"
    echo "     - Alternatives considered"
    echo "     - Security/license review"
    echo "  2. Get explicit approval"
    echo "  3. Add to approved dependencies list"
    echo ""
    echo "See: docs/TECHNOLOGY_SELECTION_V1.md §11 Non-Choices"
    exit 1
else
    echo -e "${GREEN}PASSED: No unauthorized dependencies${NC}"
    echo "  Module: $(grep '^module' "$GO_MOD" | head -1)"
    echo "  Go version: $(grep '^go ' "$GO_MOD" | head -1)"
    exit 0
fi
