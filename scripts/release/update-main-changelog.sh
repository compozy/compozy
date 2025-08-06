#!/bin/bash
set -euo pipefail

# Script: update-main-changelog.sh
# Purpose: Update CHANGELOG.md in main branch after release
# Usage: ./scripts/release/update-main-changelog.sh <version>

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
    echo "Usage: $0 <version>" >&2
    exit 1
fi

VERSION="${1}"

# Validate version format
if ! validate_version "$VERSION"; then
    echo "ERROR: Invalid version format: $VERSION" >&2
    echo "Expected format: vX.Y.Z or X.Y.Z" >&2
    exit 1
fi

echo "=== Updating CHANGELOG in main branch for $VERSION ==="

# Store the current CHANGELOG content (from release branch)
if [[ ! -f "CHANGELOG.md" ]]; then
    echo "ERROR: CHANGELOG.md not found in current directory" >&2
    exit 1
fi

# Create a temporary file for atomic operations
TEMP_CHANGELOG=$(mktemp "CHANGELOG.XXXXXX.md")
trap "rm -f '$TEMP_CHANGELOG'" EXIT

# Save the updated changelog content
cp CHANGELOG.md "$TEMP_CHANGELOG"

# Switch to main branch
echo "Switching to main branch..."
if ! git checkout main; then
    echo "ERROR: Failed to switch to main branch" >&2
    exit 1
fi

# Compare the changelogs to see if there are actual differences
if ! diff -q "$TEMP_CHANGELOG" CHANGELOG.md > /dev/null 2>&1; then
    echo "Changes detected in CHANGELOG.md"
    
    # Replace the old changelog with the updated one (atomic operation)
    mv "$TEMP_CHANGELOG" CHANGELOG.md
    
    # Configure git for commit
    git config user.name "github-actions[bot]"
    git config user.email "github-actions[bot]@users.noreply.github.com"
    
    # Commit the changes
    git add CHANGELOG.md
    if ! git commit -m "docs: update CHANGELOG.md for $VERSION [skip ci]"; then
        echo "ERROR: Failed to commit CHANGELOG.md changes" >&2
        # Try to restore original state
        git checkout -- CHANGELOG.md
        exit 1
    fi
    
    # Push the changes
    if ! git push origin main; then
        echo "ERROR: Failed to push CHANGELOG.md to main branch" >&2
        # Reset the commit
        git reset --hard HEAD~1
        exit 1
    fi
    
    echo "✅ CHANGELOG.md updated in main branch"
else
    echo "No changes detected in CHANGELOG.md"
fi

echo "=== CHANGELOG update complete ==="
