#!/usr/bin/env bash
set -euo pipefail

platform="${1:-}"

require_value() {
  local name="$1"
  if [[ -z "${!name:-}" ]]; then
    echo "desktop release: required signing value ${name} is missing" >&2
    exit 1
  fi
}

for name in TAURI_SIGNING_PRIVATE_KEY TAURI_SIGNING_PRIVATE_KEY_PASSWORD TAURI_SIGNING_PUBLIC_KEY; do
  require_value "${name}"
done

case "${platform}" in
  macos)
    for name in APPLE_CERTIFICATE APPLE_CERTIFICATE_PASSWORD APPLE_SIGNING_IDENTITY APPLE_API_ISSUER APPLE_API_KEY APPLE_API_KEY_PATH; do
      require_value "${name}"
    done
    if [[ ! -s "${APPLE_API_KEY_PATH}" ]]; then
      echo "desktop release: APPLE_API_KEY_PATH must name the non-empty ASC API .p8 file" >&2
      exit 1
    fi
    ;;
  linux)
    ;;
  windows)
    for name in AZURE_TENANT_ID AZURE_CLIENT_ID AZURE_CLIENT_SECRET AZURE_ARTIFACT_SIGNING_ACCOUNT AZURE_ARTIFACT_SIGNING_CERTIFICATE_PROFILE AZURE_ARTIFACT_SIGNING_ENDPOINT; do
      require_value "${name}"
    done
    ;;
  *)
    echo "usage: assert-desktop-signing-material.sh <macos|linux|windows>" >&2
    exit 2
    ;;
esac

echo "desktop release signing preflight: PASS (${platform})"

