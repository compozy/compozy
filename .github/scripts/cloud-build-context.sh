#!/usr/bin/env bash
# Fails when cmd/compozy-cloud imports a repository package that the Dockerfile
# build stage does not copy. PR CI never builds the cloud image, so a missing
# COPY otherwise surfaces only when the staging rollout builds it.
set -euo pipefail

module=$(awk '/^module /{print $2; exit}' go.mod)
# Synchronous assignments so a failing awk or go list stops the script.
copies=$(awk '
  /^FROM .* AS build/ { build = 1; next }
  /^FROM / { build = 0 }
  build && $1 == "COPY" && $2 !~ /^--/ { for (i = 2; i < NF; i++) print $i }
' Dockerfile)
deps=$(go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./cmd/compozy-cloud)
packages=$(grep "^$module/" <<< "$deps" || true)
if [[ -z "$copies" || -z "$packages" ]]; then
  echo "Could not read the Dockerfile COPY sources or the cmd/compozy-cloud packages." >&2
  exit 1
fi
mapfile -t sources <<< "$copies"

missing=0
while IFS= read -r package; do
  dir=${package#"$module"/}
  covered=false
  for source in "${sources[@]}"; do
    # A package is covered by a copy of its directory, an ancestor, or files inside it.
    if [[ "$dir" == "$source" || "$dir" == "$source"/* || "$source" == "$dir"/* ]]; then
      covered=true
      break
    fi
  done
  if [[ "$covered" != true ]]; then
    echo "Dockerfile does not copy $dir, which cmd/compozy-cloud imports." >&2
    missing=1
  fi
done <<< "$packages"

exit "$missing"
