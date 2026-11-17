#!/usr/bin/env bash
# Daily, artifact-only census. No job log downloads or whole-job reruns.
set -euo pipefail
root=$(mktemp -d)
cutoff=$(date -u -d '7 days ago' +%Y-%m-%dT%H:%M:%SZ)
mkdir -p "$root/receipts"
# Name filtering keeps API use proportional to recovered-flake runs, not all
# CI jobs/artifacts. Artifact metadata contains the producing repository IDs.
gh api --paginate "repos/${GITHUB_REPOSITORY}/actions/artifacts?name=flake-census&per_page=100" \
  --jq ".artifacts[] | select(.expired == false and .created_at >= \"$cutoff\" and .workflow_run.head_repository_id == .workflow_run.repository_id) | .id" \
  > "$root/artifacts"
while read -r id; do
  [[ "$id" =~ ^[0-9]+$ ]]
  mkdir -p "$root/receipts/$id"
  gh api "repos/${GITHUB_REPOSITORY}/actions/artifacts/${id}/zip" > "$root/$id.zip"
  # Extract only the fixed JSON file, never executable content or source paths.
  unzip -p "$root/$id.zip" flakes.json > "$root/receipts/$id/flakes.json"
done < "$root/artifacts"
go run github.com/magefile/mage@v1.17.2 testFlakeCensus "$root/receipts" "$root/body.md"
title='CI flake ledger: tests recovered on retry'
issue=$(gh issue list --state all --search '"CI flake ledger" in:title' --limit 100 \
  --json number,title --jq ".[] | select(.title == \"$title\") | .number" | head -n 1)
if [[ -z "$issue" ]]; then
  if [[ ! -s "$root/artifacts" ]]; then
    echo '::notice::No recovered flakes in the seven-day census; no issue needed.'
    exit 0
  fi
  gh issue create --title "$title" --type Bug --body-file "$root/body.md"
else
  # Own only the dedicated census issue; never overwrite the historical umbrella.
  gh issue view "$issue" --json body --jq .body > "$root/existing.md"
  grep -q '^<!-- compozy-flake-census -->' "$root/existing.md"
  if ! cmp -s "$root/existing.md" "$root/body.md"; then
    gh issue edit "$issue" --body-file "$root/body.md"
  fi
fi
