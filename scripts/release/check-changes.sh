#!/bin/bash
set -euo pipefail

# Script: check-changes.sh
# Purpose: Check if there are changes that warrant a new release
# Usage: ./scripts/release/check-changes.sh

# Get the latest tag
LATEST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "")

# GitHub Actions output (write directly if available)
if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
    echo "latest_tag=$LATEST_TAG" >> "$GITHUB_OUTPUT"
else
    echo "latest_tag=$LATEST_TAG"
fi

if [[ -z "$LATEST_TAG" ]]; then
    echo "No previous tags found, will create initial release"
    if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
        echo "has_changes=true" >> "$GITHUB_OUTPUT"
    else
        echo "has_changes=true"
    fi
    exit 0
fi

# Check if there are any commits since last tag
COMMITS_SINCE=$(git rev-list --count "$LATEST_TAG"..HEAD)

if [[ "$COMMITS_SINCE" -eq 0 ]]; then
    echo "No commits since last tag"
    if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
        echo "has_changes=false" >> "$GITHUB_OUTPUT"
    else
        echo "has_changes=false"
    fi
    exit 0
fi

echo "Found $COMMITS_SINCE commits since $LATEST_TAG"

# Use git-cliff to check if any commits would trigger a version bump
# git-cliff analyzes commits and determines if they contain feat, fix, or breaking changes
if command -v git-cliff &> /dev/null; then
    echo "Checking if commits warrant a version bump using git-cliff..."
    
    # Calculate what the next version would be
    CURRENT_VERSION="${LATEST_TAG#v}"
    NEXT_VERSION=$(git-cliff --unreleased --bump --strip all 2>/dev/null | head -1 || echo "")
    
    # If git-cliff couldn't determine a version, try alternative method
    if [[ -z "$NEXT_VERSION" ]]; then
        # Check for conventional commits that trigger bumps (feat, fix, breaking)
        BUMP_COMMITS=$(git log "$LATEST_TAG"..HEAD --oneline | grep -E "^[a-f0-9]+ (feat|fix|perf|refactor|revert|build|ci|docs|style|test|chore)(\(.+\))?!?:" || true)
        BREAKING_COMMITS=$(git log "$LATEST_TAG"..HEAD --oneline | grep -E "^[a-f0-9]+ .+!:" || true)
        VERSION_COMMITS=$(git log "$LATEST_TAG"..HEAD --oneline | grep -E "^[a-f0-9]+ (feat|fix|perf)(\(.+\))?:" || true)
        
        if [[ -n "$BREAKING_COMMITS" ]] || [[ -n "$VERSION_COMMITS" ]]; then
            echo "Found commits that warrant a version bump"
            if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
                echo "has_changes=true" >> "$GITHUB_OUTPUT"
            else
                echo "has_changes=true"
            fi
        else
            echo "Commits found but they don't warrant a version bump (only chores, docs, etc.)"
            if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
                echo "has_changes=false" >> "$GITHUB_OUTPUT"
            else
                echo "has_changes=false"
            fi
        fi
    else
        # Compare versions to see if bump is needed
        if [[ "$NEXT_VERSION" != "$CURRENT_VERSION" ]]; then
            echo "Version would bump from $CURRENT_VERSION to $NEXT_VERSION"
            if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
                echo "has_changes=true" >> "$GITHUB_OUTPUT"
            else
                echo "has_changes=true"
            fi
        else
            echo "No version bump needed (current: $CURRENT_VERSION, calculated: $NEXT_VERSION)"
            if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
                echo "has_changes=false" >> "$GITHUB_OUTPUT"
            else
                echo "has_changes=false"
            fi
        fi
    fi
else
    # Fallback: check for conventional commits manually
    echo "git-cliff not found, checking for conventional commits manually..."
    
    # Look for commits that would trigger a version bump
    VERSION_COMMITS=$(git log "$LATEST_TAG"..HEAD --oneline | grep -E "^[a-f0-9]+ (feat|fix|perf)(\(.+\))?:" || true)
    BREAKING_COMMITS=$(git log "$LATEST_TAG"..HEAD --oneline | grep -E "^[a-f0-9]+ .+!:" || true)
    
    if [ -n "$BREAKING_COMMITS" ] || [ -n "$VERSION_COMMITS" ]; then
        echo "Found commits that warrant a version bump"
        if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
            echo "has_changes=true" >> "$GITHUB_OUTPUT"
        else
            echo "has_changes=true"
        fi
    else
        echo "Only non-bumping commits found (chores, docs, etc.)"
        if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
            echo "has_changes=false" >> "$GITHUB_OUTPUT"
        else
            echo "has_changes=false"
        fi
    fi
fi
