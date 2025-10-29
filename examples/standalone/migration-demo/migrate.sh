#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"
PHASE1="$ROOT_DIR/phase1-standalone"
PHASE2="$ROOT_DIR/phase2-distributed"

# Verify source and target
if [[ ! -f "$PHASE1/compozy.yaml" ]]; then
  echo "Error: Source config not found at $PHASE1/compozy.yaml" >&2
  exit 1
fi

if [[ ! -d "$PHASE2" ]]; then
  echo "Error: Target directory not found at $PHASE2" >&2
  exit 1
fi

echo "Generating phase2 configuration from phase1…"
if ! cp -f "$PHASE1/compozy.yaml" "$PHASE2/compozy.yaml"; then
  echo "Error: Failed to copy configuration into $PHASE2" >&2
  exit 1
fi

echo "Done. Updated: $PHASE2/compozy.yaml"
