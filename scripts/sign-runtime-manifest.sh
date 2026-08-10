#!/usr/bin/env bash
set -euo pipefail

manifest="${1:?runtime manifest path is required}"

if [[ ! -s "${manifest}" ]]; then
  echo "desktop release: runtime manifest must be a non-empty file" >&2
  exit 1
fi
if [[ -z "${TAURI_SIGNING_PRIVATE_KEY:-}" || -z "${TAURI_SIGNING_PRIVATE_KEY_PASSWORD:-}" ]]; then
  echo "desktop release: runtime manifest signing material is incomplete" >&2
  exit 1
fi

bunx @tauri-apps/cli@2.11.4 signer sign "${manifest}"
openssl base64 -d -A <"${manifest}.sig" >"${manifest}.minisig"
rm "${manifest}.sig"
