#!/bin/bash
set -euo pipefail

VERSION="${1}"
MODE="${2:-update}" # update or release

if [ "$MODE" = "update" ]; then
    # For PR creation/update
    # Use git-cliff's native prepend feature for better changelog management
    if [ -f CHANGELOG.md ]; then
        # Prepend unreleased changes to existing changelog
        git-cliff --unreleased --tag "$VERSION" --prepend CHANGELOG.md
    else
        # Create new changelog
        git-cliff --unreleased --tag "$VERSION" -o CHANGELOG.md
    fi
    
    # Generate a PR-specific changelog for the PR body (just the unreleased portion)
    git-cliff --unreleased --strip all -o CHANGELOG_PR.md
    
elif [ "$MODE" = "release" ]; then
    # For final release
    # Use git-cliff's native update feature to regenerate the full changelog
    git-cliff --tag "$VERSION" -o CHANGELOG.md
    
    # Extract just this release's notes for GoReleaser using --current
    git-cliff --current --strip all -o RELEASE_NOTES.md
fi
