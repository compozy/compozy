#!/usr/bin/env bash
set -euo pipefail

output="${1:?output path for the private key is required}"

if [[ -n "${CI:-}" ]]; then
  echo "desktop release: update signing keys must never be generated in CI" >&2
  exit 1
fi
if [[ -z "${TAURI_SIGNING_PRIVATE_KEY_PASSWORD:-}" ]]; then
  echo "desktop release: TAURI_SIGNING_PRIVATE_KEY_PASSWORD must be set for key generation" >&2
  exit 1
fi

bunx @tauri-apps/cli@2.11.4 signer generate \
  --password "${TAURI_SIGNING_PRIVATE_KEY_PASSWORD}" \
  --write-keys "${output}"

