#!/bin/bash
set -euo pipefail

# Script: update-main-changelog.sh
# Purpose: Update CHANGELOG.md in main branch after release
# Usage: ./scripts/release/update-main-changelog.sh <version>

VERSION="${1}"

if [[ -z "$VERSION" ]]; then
    echo "ERROR: Version argument is required" >&2
    echo "Usage: $0 <version>" >&2
    exit 1
fi

echo "=== Updating CHANGELOG in main branch for $VERSION ==="

# Store the current CHANGELOG content (from release branch)
if [[ ! -f "CHANGELOG.md" ]]; then
    echo "ERROR: CHANGELOG.md not found in current directory" >&2
    exit 1
fi

# Save the updated changelog content
cp CHANGELOG.md CHANGELOG.updated.md

# Switch to main branch
echo "Switching to main branch..."
git checkout main

# Compare the changelogs to see if there are actual differences
if ! diff -q CHANGELOG.updated.md CHANGELOG.md > /dev/null 2>&1; then
    echo "Changes detected in CHANGELOG.md"
    
    # Replace the old changelog with the updated one
    mv CHANGELOG.updated.md CHANGELOG.md
    
    # Configure git for commit
    git config user.name "github-actions[bot]"
    git config user.email "github-actions[bot]@users.noreply.github.com"
    
    # Commit and push the changes
    git add CHANGELOG.md
    git commit -m "docs: update CHANGELOG.md for $VERSION [skip ci]"
    git push origin main
    
    echo "✅ CHANGELOG.md updated in main branch"
else
    echo "No changes detected in CHANGELOG.md"
    rm -f CHANGELOG.updated.md
fi

echo "=== CHANGELOG update complete ==="
