#!/bin/bash

# Script to delete ALL Git tags (local and remote) and ALL GitHub releases for a repository.
# WARNING: This is DESTRUCTIVE and IRREVERSIBLE! It will delete all tags and releases without confirmation prompts.
# Requirements:
# - gh CLI installed and authenticated (run `gh auth login` first).
# - Git repository cloned locally.
# Usage: ./delete_tags_and_releases.sh [owner] [repo]
# If owner and repo not provided, attempts to infer from git remote origin.
# Example: ./delete_tags_and_releases.sh compozy compozy

set -euo pipefail

# Function to infer owner and repo from git remote
get_repo_info() {
    local remote_url=$(git config --get remote.origin.url)
    if [[ -z "$remote_url" ]]; then
        echo "Error: No remote.origin.url found. Run in a git repo or provide owner/repo."
        exit 1
    fi
    # Extract owner/repo from URL (supports SSH and HTTPS)
    if [[ $remote_url =~ ^git@github\.com:(.+)/(.+)\.git$ ]]; then
        OWNER="${BASH_REMATCH[1]}"
        REPO="${BASH_REMATCH[2]}"
    elif [[ $remote_url =~ ^https://github\.com/(.+)/(.+)\.git$ ]]; then
        OWNER="${BASH_REMATCH[1]}"
        REPO="${BASH_REMATCH[2]}"
    else
        echo "Error: Could not parse owner/repo from remote URL: $remote_url"
        exit 1
    fi
}

# Check if gh is installed
if ! command -v gh &> /dev/null; then
    echo "Error: gh CLI is required. Install from https://cli.github.com/"
    exit 1
fi

# Get owner and repo
if [ $# -eq 2 ]; then
    OWNER="$1"
    REPO="$2"
elif [ $# -eq 0 ]; then
    get_repo_info
else
    echo "Usage: $0 [owner] [repo]"
    exit 1
fi

echo "WARNING: This will delete ALL GitHub artifacts, tags, and releases for $OWNER/$REPO."
echo "Press Enter to continue or Ctrl+C to abort."
read -r

# Step 1: Delete all GitHub Actions artifacts
echo "Cleaning up GitHub Actions artifacts..."

# Delete workflow runs
echo "Fetching and deleting workflow runs..."
WORKFLOW_RUNS=$(gh run list --repo "$OWNER/$REPO" --json databaseId --jq '.[].databaseId')
if [ -n "$WORKFLOW_RUNS" ]; then
    for run_id in $WORKFLOW_RUNS; do
        echo "Deleting workflow run: $run_id"
        gh run delete "$run_id" --repo "$OWNER/$REPO" || echo "Failed to delete workflow run $run_id"
    done
else
    echo "No workflow runs found."
fi

# Delete caches
echo "Deleting GitHub Actions caches..."
gh cache delete --all --repo "$OWNER/$REPO" || echo "Failed to delete some caches"

# Step 2: Delete all GitHub releases
echo "Fetching and deleting releases..."
RELEASES=$(gh release list --repo "$OWNER/$REPO" --json tagName --jq '.[].tagName')
if [ -n "$RELEASES" ]; then
    for tag in $RELEASES; do
        echo "Deleting release for tag: $tag"
        gh release delete "$tag" --repo "$OWNER/$REPO" --yes || echo "Failed to delete release $tag"
    done
else
    echo "No releases found."
fi

# Step 3: Delete all remote tags
echo "Fetching and deleting remote tags..."
REMOTE_TAGS=$(git ls-remote --tags origin | awk '{print $2}' | sed 's/refs\/tags\///' | grep -v '{}')
if [ -n "$REMOTE_TAGS" ]; then
    for tag in $REMOTE_TAGS; do
        echo "Deleting remote tag: $tag"
        git push origin --delete "$tag" || echo "Failed to delete remote tag $tag"
    done
else
    echo "No remote tags found."
fi

# Step 4: Delete all local tags
echo "Deleting local tags..."
LOCAL_TAGS=$(git tag -l)
if [ -n "$LOCAL_TAGS" ]; then
    git tag -d $LOCAL_TAGS || echo "Failed to delete some local tags"
else
    echo "No local tags found."
fi

echo "All GitHub artifacts, tags, and releases deleted successfully!"
