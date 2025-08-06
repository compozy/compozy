#!/bin/bash
set -euo pipefail

VERSION="${1}"

git config user.name "github-actions[bot]"
git config user.email "github-actions[bot]@users.noreply.github.com"

# Create and push the tag
git tag -a "$VERSION" \
    -m "Release $VERSION" \
    -m "Automated release via GitHub Actions"

git push origin "$VERSION"
