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
# Windows is paused until Trusted Signing is restored.
case "${platform}" in
  linux)
    ;;
  *)
    echo "usage: assert-desktop-signing-material.sh <linux>" >&2
    exit 2
    ;;
esac

echo "desktop release signing preflight: PASS (${platform})"

