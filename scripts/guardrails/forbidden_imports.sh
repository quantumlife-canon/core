#!/usr/bin/env bash
#
# forbidden_imports.sh
#
# Enforces import boundaries between packages to maintain architectural integrity.
# These rules enforce the Control Plane vs Data Plane separation from Technical Split.
#
# Reference: docs/TECHNICAL_SPLIT_V1.md §4 Control Plane vs Data Plane
#
# Rules enforced:
# 1. internal/execution MUST NOT import internal/negotiation or internal/authority
#    (Data plane must not have decision-making dependencies)
# 2. internal/execution MUST NOT import any LLM/SLM packages
# 3. internal/* packages MUST NOT import other internal/* packages
#    (Each layer is isolated; cross-cutting via interfaces only)
#    EXCEPTION: impl_inmem subdirectories and demo package (wiring layers)
# 4. internal/* MAY import pkg/*
#
# Exit codes:
#   0 - No forbidden imports found
#   1 - Forbidden imports found or error
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

echo "=========================================="
echo "Forbidden Imports Check"
echo "=========================================="
echo "Reference: docs/TECHNICAL_SPLIT_V1.md §4 Control Plane vs Data Plane"
echo ""

violations_found=0

# Function to extract imports from a Go file
get_imports() {
    local file="$1"
    # Extract import block and individual imports
    awk '/^import[[:space:]]*\(/{p=1; next} p && /^\)/{p=0} p{print} /^import[[:space:]]+"/{gsub(/^import[[:space:]]+/, ""); print}' "$file" | \
        grep -oE '"[^"]+"' | tr -d '"' || true
}

# Function to get package path from directory
get_package_path() {
    local dir="$1"
    local relative="${dir#${REPO_ROOT}/}"
    echo "quantumlife/${relative}"
}

echo "Checking Rule 1: internal/execution must not import decision packages..."
echo ""

EXECUTION_DIR="${REPO_ROOT}/internal/execution"
if [[ -d "$EXECUTION_DIR" ]]; then
    while IFS= read -r -d '' file; do
        relative_path="${file#${REPO_ROOT}/}"
        imports=$(get_imports "$file")

        # Check for forbidden negotiation import
        if echo "$imports" | grep -q "quantumlife/internal/negotiation"; then
            echo -e "${RED}VIOLATION: ${relative_path}${NC}"
            echo "  Imports: quantumlife/internal/negotiation"
            echo "  Reason: Data plane (execution) must not import control plane (negotiation)"
            violations_found=1
        fi

        # Check for forbidden authority import
        if echo "$imports" | grep -q "quantumlife/internal/authority"; then
            echo -e "${RED}VIOLATION: ${relative_path}${NC}"
            echo "  Imports: quantumlife/internal/authority"
            echo "  Reason: Data plane (execution) must not import control plane (authority)"
            violations_found=1
        fi

        # Check for LLM/SLM packages (future-proofing)
        for pattern in "llm" "slm" "openai" "anthropic" "langchain"; do
            if echo "$imports" | grep -qi "$pattern"; then
                matched=$(echo "$imports" | grep -i "$pattern")
                echo -e "${RED}VIOLATION: ${relative_path}${NC}"
                echo "  Imports: $matched"
                echo "  Reason: Data plane (execution) must not use LLM/SLM packages"
                violations_found=1
            fi
        done
    done < <(find "$EXECUTION_DIR" -name "*.go" -type f -print0)
fi

echo ""
echo "Checking Rule 2: internal/* must not import other internal/* packages..."
echo "(Exceptions: impl_inmem, demo, loop, and persist are wiring layers)"
echo ""

INTERNAL_DIR="${REPO_ROOT}/internal"
if [[ -d "$INTERNAL_DIR" ]]; then
    # Get list of internal packages
    internal_packages=()
    while IFS= read -r -d '' pkg_dir; do
        pkg_name=$(basename "$pkg_dir")
        internal_packages+=("quantumlife/internal/${pkg_name}")
    done < <(find "$INTERNAL_DIR" -mindepth 1 -maxdepth 1 -type d -print0)

    # Check each Go file in internal/
    while IFS= read -r -d '' file; do
        relative_path="${file#${REPO_ROOT}/}"

        # Skip wiring layers (implementation packages, demo, loop orchestrator, persist, and conformance)
        # These are allowed to import across internal packages for wiring/testing
        if [[ "$relative_path" == *"/impl_inmem/"* ]] || \
           [[ "$relative_path" == *"/impl_"* ]] || \
           [[ "$relative_path" == "internal/demo/"* ]] || \
           [[ "$relative_path" == internal/demo_* ]] || \
           [[ "$relative_path" == "internal/loop/"* ]] || \
           [[ "$relative_path" == "internal/persist/"* ]] || \
           [[ "$relative_path" == *"/conformance/"* ]]; then
            continue
        fi

        imports=$(get_imports "$file")

        # Get the package name of this file
        file_dir=$(dirname "$file")
        file_pkg_name=$(basename "$file_dir")

        for forbidden_pkg in "${internal_packages[@]}"; do
            # Skip self-import check
            if [[ "$forbidden_pkg" == "quantumlife/internal/${file_pkg_name}" ]]; then
                continue
            fi

            # Skip impl_inmem imports (they're part of the same logical package)
            if echo "$imports" | grep -q "^${forbidden_pkg}/impl_inmem$"; then
                continue
            fi

            if echo "$imports" | grep -q "^${forbidden_pkg}$"; then
                echo -e "${RED}VIOLATION: ${relative_path}${NC}"
                echo "  Imports: ${forbidden_pkg}"
                echo "  Reason: internal/* packages must not import other internal/* packages"
                echo "          Use interfaces in pkg/* for cross-cutting concerns"
                violations_found=1
            fi
        done
    done < <(find "$INTERNAL_DIR" -name "*.go" -type f -print0)
fi

echo ""
echo "Checking Rule 3: Audit/Negotiation coupling prevention..."
echo ""

# Check that no package imports both audit and negotiation
# This prevents using audit logs as decision input
if [[ -d "$INTERNAL_DIR" ]]; then
    while IFS= read -r -d '' pkg_dir; do
        pkg_name=$(basename "$pkg_dir")

        # Skip audit and negotiation packages themselves
        if [[ "$pkg_name" == "audit" || "$pkg_name" == "negotiation" ]]; then
            continue
        fi

        # Skip demo packages (wiring layers that use audit for display only)
        if [[ "$pkg_name" == demo_* ]]; then
            continue
        fi

        has_audit=false
        has_negotiation=false

        while IFS= read -r -d '' file; do
            imports=$(get_imports "$file")

            if echo "$imports" | grep -q "quantumlife/internal/audit"; then
                has_audit=true
            fi
            if echo "$imports" | grep -q "quantumlife/internal/negotiation"; then
                has_negotiation=true
            fi
        done < <(find "$pkg_dir" -name "*.go" -type f -print0)

        if [[ "$has_audit" == "true" && "$has_negotiation" == "true" ]]; then
            echo -e "${RED}VIOLATION: internal/${pkg_name}${NC}"
            echo "  Imports both: quantumlife/internal/audit AND quantumlife/internal/negotiation"
            echo "  Reason: Audit logs must not be used as decision input"
            echo "          (Technical Split §3.6: Audit logs ≠ operational memory)"
            violations_found=1
        fi
    done < <(find "$INTERNAL_DIR" -mindepth 1 -maxdepth 1 -type d -print0)
fi

echo ""

if [[ $violations_found -eq 1 ]]; then
    echo -e "${RED}FAILED: Forbidden imports found${NC}"
    echo ""
    echo "These import rules enforce the Control Plane vs Data Plane separation:"
    echo "  - Control Plane (negotiation, authority): May use LLM/SLM, makes decisions"
    echo "  - Data Plane (execution): Deterministic only, no decision-making"
    echo ""
    echo "See: docs/TECHNICAL_SPLIT_V1.md §4 Control Plane vs Data Plane"
    exit 1
else
    echo -e "${GREEN}PASSED: No forbidden imports found${NC}"
    exit 0
fi
