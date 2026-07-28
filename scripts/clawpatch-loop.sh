#!/usr/bin/env bash
#
# clawpatch-loop.sh — sequentially next -> fix -> revalidate -> commit until no findings remain.

set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

num=0
while :; do
  id=$(clawpatch next --json 2>/dev/null \
    | jq -r '(.finding // .[0] // .).id // empty')
  [[ -z "$id" || "$id" == "null" ]] && { echo "no open findings remaining"; break; }

  num=$((num + 1))

  echo ">>> fix $id"
  clawpatch fix --finding "$id"

  echo ">>> revalidate $id"
  clawpatch revalidate --finding "$id"

  git add .
  if git diff --cached --quiet; then
    echo ">>> nothing to commit"
  else
    git commit -m "fix: clawpatch ${num}"
    echo ">>> committed clawpatch ${num} ($id)"
  fi
done
