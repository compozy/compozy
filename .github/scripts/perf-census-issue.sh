#!/usr/bin/env bash
# Keep one tracking issue for rejected E2E-023 statistical samples.
set -euo pipefail
report=${1:?usage: perf-census-issue.sh judgment-file}
root=$(mktemp -d)
trap 'rm -rf "$root"' EXIT
title='E2E-023 performance census: median budget exceeded'
cat > "$root/body.md" <<EOF_BODY
<!-- compozy-perf-census -->
# E2E-023 performance census

The latest dedicated census failed. Its five complete, unretried samples must
keep both switch and peer medians at or below 50 ms. Missing samples or functional
failures also reject the census. This signal is informational on pull requests
and is outside the merge-gating aggregators and Release publication graph.

Run: ${GITHUB_SERVER_URL:-https://github.com}/${GITHUB_REPOSITORY}/actions/runs/${GITHUB_RUN_ID}
Head: ${COMPOZY_FLAKE_SHA:-${GITHUB_SHA}}
Workflow attempt: ${GITHUB_RUN_ATTEMPT:-1}

The run's e2e-perf-census artifact retains every raw sample. The original failed
run remains evidence even if a later census passes. Refs #1736 for investigation.

## Latest judgment

\`\`\`text
EOF_BODY
# The local judgment is produced by Mage, not test-provided titles or job logs.
tail -c 12000 "$report" >> "$root/body.md"
printf '\n```\n' >> "$root/body.md"
issue=$(gh issue list --repo "$GITHUB_REPOSITORY" --state all \
  --search '"E2E-023 performance census" in:title' --limit 100 \
  --json number,title --jq ".[] | select(.title == \"$title\") | .number" | head -n 1)
if [[ -z "$issue" ]]; then
  gh issue create --repo "$GITHUB_REPOSITORY" --title "$title" --type Bug --body-file "$root/body.md"
else
  gh issue view "$issue" --repo "$GITHUB_REPOSITORY" --json body,state > "$root/existing.json"
  jq -r .body "$root/existing.json" > "$root/existing.md"
  grep -q '^<!-- compozy-perf-census -->' "$root/existing.md"
  if [[ "$(jq -r .state "$root/existing.json")" == "CLOSED" ]]; then
    gh issue reopen "$issue" --repo "$GITHUB_REPOSITORY"
  fi
  if ! cmp -s "$root/existing.md" "$root/body.md"; then
    gh issue edit "$issue" --repo "$GITHUB_REPOSITORY" --body-file "$root/body.md"
  fi
fi
