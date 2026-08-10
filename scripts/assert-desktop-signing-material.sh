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

# macOS asserts live inline in .github/workflows/release.yml so dispatch
# recovery of an older release ref keeps using the current Apple ID contract.
case "${platform}" in
  linux)
    ;;
  windows)
    for name in AZURE_TENANT_ID AZURE_CLIENT_ID AZURE_CLIENT_SECRET AZURE_ARTIFACT_SIGNING_ACCOUNT AZURE_ARTIFACT_SIGNING_CERTIFICATE_PROFILE AZURE_ARTIFACT_SIGNING_ENDPOINT; do
      require_value "${name}"
    done
    ;;
  *)
    echo "usage: assert-desktop-signing-material.sh <linux|windows>" >&2
    exit 2
    ;;
esac

echo "desktop release signing preflight: PASS (${platform})"

