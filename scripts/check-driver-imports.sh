#!/usr/bin/env bash
set -euo pipefail

# Prevent driver package imports from leaking into domain layers.
# Allowed: engine/infra/** and engine/infra/server/**, tests may live alongside drivers.

violations=$(rg -n "\bgithub.com/compozy/compozy/engine/infra/(postgres|redis)\b" \
  --glob 'engine/**' \
  --glob '!engine/infra/**' \
  --glob '!engine/infra/server/**' \
  --glob '!**/*_test.go' || true)

if [[ -n "$violations" ]]; then
  echo "Driver import leakage detected in non-infra code:" >&2
  echo "$violations" >&2
  echo "Please refactor to use contracts/providers; see .cursor/rules/architecture.mdc" >&2
  exit 1
fi

exit 0
