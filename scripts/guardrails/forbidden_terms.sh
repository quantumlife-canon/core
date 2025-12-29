#!/usr/bin/env bash
#
# forbidden_terms.sh
#
# Scans Go code for terms that violate QuantumLife Canon.
# These terms represent forbidden concepts that must not appear in core code.
#
# Reference: docs/QUANTUMLIFE_CANON_V1.md §Ontology (Forbidden at Core)
#
# Exit codes:
#   0 - No forbidden terms found
#   1 - Forbidden terms found or error
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
ALLOWLIST="${SCRIPT_DIR}/allowlist_terms.txt"

# Forbidden terms (case-insensitive patterns)
# These map directly to Canon §Forbidden at Core
FORBIDDEN_TERMS=(
    '\buser\b'
    '\busers\b'
    '\baccount\b'
    '\baccounts\b'
    '\bworkspace\b'
    '\bworkspaces\b'
    '\brole\b'
    '\broles\b'
    'global[[:space:]]*state'
    'global[[:space:]]*namespace'
    'admin[[:space:]]*override'
    '\bsuperuser\b'
)

# Directories to scan
SCAN_DIRS=(
    "${REPO_ROOT}/internal"
    "${REPO_ROOT}/pkg"
    "${REPO_ROOT}/cmd"
)

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

echo "=========================================="
echo "Forbidden Terms Check"
echo "=========================================="
echo "Reference: docs/QUANTUMLIFE_CANON_V1.md §Forbidden at Core"
echo ""

# Load allowlist if exists
ALLOWLIST_PATTERNS=()
if [[ -f "${ALLOWLIST}" ]]; then
    while IFS= read -r line || [[ -n "$line" ]]; do
        # Skip empty lines and comments
        [[ -z "$line" || "$line" =~ ^# ]] && continue
        ALLOWLIST_PATTERNS+=("$line")
    done < "${ALLOWLIST}"
fi

violations_found=0
declare -A violation_map

for dir in "${SCAN_DIRS[@]}"; do
    if [[ ! -d "$dir" ]]; then
        continue
    fi

    # Find all .go files
    while IFS= read -r -d '' file; do
        relative_path="${file#${REPO_ROOT}/}"

        for term in "${FORBIDDEN_TERMS[@]}"; do
            # Case-insensitive grep
            if grep -inE "$term" "$file" 2>/dev/null | grep -v "^[[:space:]]*//"; then
                # Check if in allowlist
                is_allowed=false
                for allow_pattern in "${ALLOWLIST_PATTERNS[@]}"; do
                    if [[ "$relative_path" == $allow_pattern ]]; then
                        is_allowed=true
                        break
                    fi
                done

                if [[ "$is_allowed" == "false" ]]; then
                    # Extract matching lines with line numbers
                    matches=$(grep -inE "$term" "$file" 2>/dev/null | grep -v "^[[:space:]]*//" || true)
                    if [[ -n "$matches" ]]; then
                        if [[ -z "${violation_map[$relative_path]:-}" ]]; then
                            violation_map[$relative_path]=""
                        fi
                        violation_map[$relative_path]+="  Pattern: $term"$'\n'"$matches"$'\n'
                        violations_found=1
                    fi
                fi
            fi
        done
    done < <(find "$dir" -name "*.go" -type f -print0)
done

if [[ $violations_found -eq 1 ]]; then
    echo -e "${RED}FAILED: Forbidden terms found in code${NC}"
    echo ""
    echo "The following files contain terms forbidden by Canon:"
    echo ""
    for file in "${!violation_map[@]}"; do
        echo -e "${YELLOW}$file${NC}"
        echo "${violation_map[$file]}"
    done
    echo ""
    echo "These terms are forbidden because they represent concepts"
    echo "that violate QuantumLife's sovereignty model:"
    echo "  - 'user/account' → Use 'circle' instead"
    echo "  - 'workspace' → Use 'intersection' instead"
    echo "  - 'role' → Authority is explicit and scoped"
    echo "  - 'global state/namespace' → All state is circle/intersection-owned"
    echo "  - 'admin override/superuser' → No backdoors allowed"
    echo ""
    echo "To add an exception, add the file path to:"
    echo "  ${ALLOWLIST}"
    echo ""
    exit 1
else
    echo -e "${GREEN}PASSED: No forbidden terms found${NC}"
    exit 0
fi
