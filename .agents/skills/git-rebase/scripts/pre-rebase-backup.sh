#!/usr/bin/env bash
set -euo pipefail

# Invariant: preserve HEAD with a backup ref, without dirtying the worktree.
# Refuse dirty or detached input; never stash, stage, reset, or publish.
backup_current_branch=$(git symbolic-ref --quiet --short HEAD) || {
    echo "Cannot back up a detached HEAD with this helper; inspect the active rebase first." >&2
    exit 1
}
if [ -n "$(git status --porcelain)" ]; then
    echo "Worktree has uncommitted changes. Preserve them or use an isolated checkout; no backup created." >&2
    exit 1
fi
backup_current_sha=$(git rev-parse HEAD)
backup_timestamp=$(date +%Y%m%d_%H%M%S)
backup_ref="backup-rebase-${backup_current_branch}-${backup_timestamp}"
git branch "$backup_ref" "$backup_current_sha"
backup_info_path=$(git rev-parse --git-path rebase-backup-info)
{
    printf 'Backup ref: %s\n' "$backup_ref"
    printf 'Original branch: %s\n' "$backup_current_branch"
    printf 'Original HEAD: %s\n' "$backup_current_sha"
} > "$backup_info_path"
printf 'Backup created: %s (%s)\nMetadata: %s\n' "$backup_ref" "$backup_current_sha" "$backup_info_path"
