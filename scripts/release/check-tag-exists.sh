#!/bin/bash
set -euo pipefail

# Script: check-tag-exists.sh
# Purpose: Check if a git tag already exists (for idempotency)
# Usage: ./scripts/release/check-tag-exists.sh <version>

# Validate arguments
if [[ $# -lt 1 ]]; then
    echo "ERROR: Version argument is required" >&2
    echo "Usage: $0 <version>" >&2
    exit 1
fi

VERSION="${1}"

# Function to validate version format
validate_version() {
    local version="${1}"
    if [[ ! "$version" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.-]+)?(\+[a-zA-Z0-9.-]+)?$ ]]; then
        return 1
    fi
    return 0
}

# Validate version format
if ! validate_version "$VERSION"; then
    echo "ERROR: Invalid version format: $VERSION" >&2
    echo "Expected format: vX.Y.Z or X.Y.Z" >&2
    exit 1
fi

# Check if tag exists locally or remotely
if git rev-parse "$VERSION" >/dev/null 2>&1; then
    echo "Tag $VERSION already exists locally" >&2
    TAG_EXISTS=true
elif git ls-remote --tags origin "refs/tags/$VERSION" | grep -q "$VERSION"; then
    echo "Tag $VERSION already exists on remote" >&2
    TAG_EXISTS=true
else
    echo "Tag $VERSION does not exist" >&2
    TAG_EXISTS=false
fi

# GitHub Actions output (write directly if available)
if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
    echo "tag_exists=$TAG_EXISTS" >> "$GITHUB_OUTPUT"
else
    echo "tag_exists=$TAG_EXISTS"
fi

# Exit with appropriate code
if [[ "$TAG_EXISTS" == "true" ]]; then
    echo "❌ Tag $VERSION already exists!"
    exit 0  # Not an error, just idempotency check
else
    echo "✅ Tag does not exist, proceeding with release"
    exit 0
fi
