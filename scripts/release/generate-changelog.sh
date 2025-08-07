#!/bin/bash
set -euo pipefail

# Script: generate-changelog.sh
# Purpose: Generate changelog using git-cliff
# Usage: ./scripts/release/generate-changelog.sh <version> [mode]

# Function to validate version format
validate_version() {
    local version="${1}"
    if [[ ! "$version" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.-]+)?(\+[a-zA-Z0-9.-]+)?$ ]]; then
        return 1
    fi
    return 0
}

# Validate arguments
if [[ $# -lt 1 ]]; then
    echo "ERROR: Version argument is required" >&2
    echo "Usage: $0 <version> [mode]" >&2
    exit 1
fi

VERSION="${1}"
MODE="${2:-update}" # update or release

# Validate version format
if ! validate_version "$VERSION"; then
    echo "ERROR: Invalid version format: $VERSION" >&2
    echo "Expected format: vX.Y.Z or X.Y.Z" >&2
    exit 1
fi

# Validate mode
if [[ "$MODE" != "update" && "$MODE" != "release" ]]; then
    echo "ERROR: Invalid mode: $MODE" >&2
    echo "Expected: update or release" >&2
    exit 1
fi

# Check for git-cliff dependency
if ! command -v git-cliff &> /dev/null; then
    echo "ERROR: git-cliff is required but not installed" >&2
    echo "Install: https://github.com/orhun/git-cliff#installation" >&2
    exit 1
fi

if [[ "$MODE" = "update" ]]; then
    # For PR creation/update
    # Use git-cliff's native prepend feature for better changelog management
    if [[ -f CHANGELOG.md ]]; then
        # Prepend unreleased changes to existing changelog
        git-cliff --unreleased --tag "$VERSION" --prepend CHANGELOG.md
    else
        # Create new changelog
        git-cliff --unreleased --tag "$VERSION" -o CHANGELOG.md
    fi
    
    # Generate a PR-specific changelog for the PR body (just the unreleased portion)
    git-cliff --unreleased --strip all -o CHANGELOG_PR.md
    
elif [[ "$MODE" = "release" ]]; then
    # For final release
    # Use git-cliff's native update feature to regenerate the full changelog
    git-cliff --tag "$VERSION" -o CHANGELOG.md
    
    # Extract just this release's notes for GoReleaser using --current
    git-cliff --current --strip all -o RELEASE_NOTES.md
fi

echo "✅ Changelog generation complete for $VERSION (mode: $MODE)"
