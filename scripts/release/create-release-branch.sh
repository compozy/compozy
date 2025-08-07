#!/bin/bash
set -euo pipefail

# Script: create-release-branch.sh
# Purpose: Create or update a release branch
# Usage: ./scripts/release/create-release-branch.sh <version> [branch_prefix] <github_repository>

# Validate arguments
if [[ $# -lt 3 ]]; then
    echo "ERROR: Required arguments missing" >&2
    echo "Usage: $0 <version> [branch_prefix] <github_repository>" >&2
    exit 1
fi

VERSION="${1}"
BRANCH_PREFIX="${2:-release/}"
GITHUB_REPOSITORY="${3}"

# Function to validate version format
validate_version() {
    local version="${1}"
    if [[ ! "$version" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.-]+)?(\+[a-zA-Z0-9.-]+)?$ ]]; then
        return 1
    fi
    return 0
}

# Validate inputs
if ! validate_version "$VERSION"; then
    echo "ERROR: Invalid version format: $VERSION" >&2
    echo "Expected format: vX.Y.Z or X.Y.Z" >&2
    exit 1
fi

if [[ ! "$GITHUB_REPOSITORY" =~ ^[a-zA-Z0-9._-]+/[a-zA-Z0-9._-]+$ ]]; then
    echo "ERROR: Invalid GitHub repository format: $GITHUB_REPOSITORY" >&2
    echo "Expected format: owner/repo" >&2
    exit 1
fi

# Sanitize branch name (remove special characters that might cause issues)
BRANCH_NAME="${BRANCH_PREFIX}${VERSION}"
BRANCH_NAME="${BRANCH_NAME//[^a-zA-Z0-9._\/-]/}"

echo "Working with branch: $BRANCH_NAME"

# Check if branch exists remotely using gh CLI if available
BRANCH_EXISTS=false
if command -v gh &> /dev/null; then
    if gh api "repos/$GITHUB_REPOSITORY/branches/${BRANCH_NAME//\//%2F}" --silent 2>/dev/null; then
        BRANCH_EXISTS=true
    fi
else
    # Fallback to git ls-remote
    if git ls-remote --heads origin "refs/heads/$BRANCH_NAME" | grep -q "$BRANCH_NAME"; then
        BRANCH_EXISTS=true
    fi
fi

if [[ "$BRANCH_EXISTS" == "true" ]]; then
    echo "Branch $BRANCH_NAME already exists, updating it"
    # Fetch the latest state of the branch
    git fetch origin "$BRANCH_NAME:$BRANCH_NAME" 2>/dev/null || true
    # Checkout and reset to match remote
    git checkout -B "$BRANCH_NAME" "origin/$BRANCH_NAME" 2>/dev/null || git checkout -b "$BRANCH_NAME"
    git reset --hard HEAD
else
    echo "Creating new branch $BRANCH_NAME"
    git checkout -b "$BRANCH_NAME"
fi

# GitHub Actions output (write directly if available)
if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
    echo "branch_name=$BRANCH_NAME" >> "$GITHUB_OUTPUT"
else
    echo "branch_name=$BRANCH_NAME"
fi
