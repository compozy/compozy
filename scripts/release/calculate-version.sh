#!/bin/bash
set -euo pipefail

# Script: calculate-version.sh
# Purpose: Calculate next version using git-cliff based on conventional commits
# Usage: ./scripts/release/calculate-version.sh [initial_version]

# Function to validate version format
validate_version() {
    local version="${1}"
    if [[ ! "$version" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.-]+)?(\+[a-zA-Z0-9.-]+)?$ ]]; then
        return 1
    fi
    return 0
}

INITIAL_VERSION="${1:-0.0.4}"

# Validate initial version if provided
if [[ -n "${1:-}" ]] && ! validate_version "$INITIAL_VERSION"; then
    echo "ERROR: Invalid initial version format: $INITIAL_VERSION" >&2
    echo "Expected format: X.Y.Z or vX.Y.Z" >&2
    exit 1
fi

echo "Calculating next version (initial: $INITIAL_VERSION)..."

# Check if git-cliff is available
if ! command -v git-cliff &> /dev/null; then
    echo "WARNING: git-cliff not found, using initial version" >&2
    NEXT_VERSION="$INITIAL_VERSION"
else
    # Use git-cliff to calculate the next version based on conventional commits
    NEXT_VERSION=$(git-cliff --bump --unreleased --strip all 2>/dev/null || echo "")
    
    if [[ -z "$NEXT_VERSION" ]]; then
        echo "WARNING: git-cliff could not determine version, using initial version" >&2
        NEXT_VERSION="$INITIAL_VERSION"
    fi
fi

# Normalize version format - always use 'v' prefix
NEXT_VERSION="v${NEXT_VERSION#v}"

# Final validation
if ! validate_version "$NEXT_VERSION"; then
    echo "ERROR: Calculated version is invalid: $NEXT_VERSION" >&2
    exit 1
fi

echo "Next version: $NEXT_VERSION"

# GitHub Actions output (write directly if available)
if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
    echo "version=$NEXT_VERSION" >> "$GITHUB_OUTPUT"
    echo "version_number=${NEXT_VERSION#v}" >> "$GITHUB_OUTPUT"
else
    echo "version=$NEXT_VERSION"
    echo "version_number=${NEXT_VERSION#v}"
fi
