#!/bin/bash
set -euo pipefail

VERSION="${1}"
BRANCH_PREFIX="${2:-release/}"
GITHUB_REPOSITORY="${3}"

BRANCH_NAME="${BRANCH_PREFIX}${VERSION}"

# Use gh cli for branch operations when possible
if gh api repos/"$GITHUB_REPOSITORY"/branches/"$BRANCH_NAME" --silent 2>/dev/null; then
    echo "Branch $BRANCH_NAME already exists, will update it"
    git checkout -B "$BRANCH_NAME" origin/"$BRANCH_NAME"
    git reset --hard HEAD
else
    echo "Creating new branch $BRANCH_NAME"
    git checkout -b "$BRANCH_NAME"
fi

echo "branch_name=$BRANCH_NAME"
