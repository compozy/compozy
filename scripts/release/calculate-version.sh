#!/bin/bash
set -euo pipefail

# Script: calculate-version.sh
# Purpose: Calculate next version using git-cliff based on conventional commits
# Usage: ./scripts/release/calculate-version.sh [initial_version]

INITIAL_VERSION="${1:-0.0.4}"

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

echo "Next version: $NEXT_VERSION"
echo "version=$NEXT_VERSION"
echo "version_number=${NEXT_VERSION#v}"
