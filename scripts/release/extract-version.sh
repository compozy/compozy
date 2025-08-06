#!/bin/bash
set -euo pipefail

# Script: extract-version.sh
# Purpose: Extract semantic version from commit message
# Usage: ./scripts/release/extract-version.sh "<commit_message>"

# Validate arguments
if [[ $# -lt 1 ]]; then
    echo "ERROR: Commit message argument is required" >&2
    echo "Usage: $0 <commit_message>" >&2
    exit 1
fi

COMMIT_MESSAGE="${1}"

# Function to validate version format
validate_version() {
    local version="${1}"
    if [[ ! "$version" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.-]+)?(\+[a-zA-Z0-9.-]+)?$ ]]; then
        return 1
    fi
    return 0
}

# Extract version from the commit message using POSIX-compliant regex
# Supports formats: v1.2.3, 1.2.3, v1.2.3-alpha, v1.2.3-rc.1, etc.
VERSION=$(echo "$COMMIT_MESSAGE" | grep -oE 'v?[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.-]+)?(\+[a-zA-Z0-9.-]+)?' | head -1)

# Normalize to always have 'v' prefix
if [[ -n "$VERSION" ]] && [[ ! "$VERSION" =~ ^v ]]; then
    VERSION="v$VERSION"
fi

# Validate extracted version
if [[ -z "$VERSION" ]]; then
    echo "ERROR: Could not extract version from commit message" >&2
    echo "Commit message: $COMMIT_MESSAGE" >&2
    exit 1
fi

if ! validate_version "$VERSION"; then
    echo "ERROR: Invalid version format: $VERSION" >&2
    echo "Expected format: vX.Y.Z or vX.Y.Z-prerelease" >&2
    exit 1
fi

# Output for GitHub Actions (write directly if available)
if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
    echo "version=$VERSION" >> "$GITHUB_OUTPUT"
else
    echo "version=$VERSION"
fi

echo "📦 Preparing to release version: $VERSION" >&2
