#!/bin/bash
set -euo pipefail

# Script: create-git-tag.sh
# Purpose: Create and push a git tag for release
# Usage: ./scripts/release/create-git-tag.sh <version>

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

# Check if tag already exists (idempotency)
if git rev-parse "$VERSION" >/dev/null 2>&1; then
    echo "WARNING: Tag $VERSION already exists locally, skipping creation" >&2
    exit 0
fi

if git ls-remote --tags origin "refs/tags/$VERSION" | grep -q "$VERSION"; then
    echo "WARNING: Tag $VERSION already exists on remote, skipping creation" >&2
    exit 0
fi

# Configure git for the operation
git config user.name "github-actions[bot]"
git config user.email "github-actions[bot]@users.noreply.github.com"

# Create the annotated tag
echo "Creating tag $VERSION..."
if ! git tag -a "$VERSION" \
    -m "Release $VERSION" \
    -m "Automated release via GitHub Actions"; then
    echo "ERROR: Failed to create tag $VERSION" >&2
    exit 1
fi

# Push the tag to origin
echo "Pushing tag $VERSION to origin..."
if ! git push origin "$VERSION"; then
    echo "ERROR: Failed to push tag $VERSION" >&2
    # Try to clean up the local tag
    git tag -d "$VERSION" 2>/dev/null || true
    exit 1
fi

echo "✅ Successfully created and pushed tag $VERSION"
