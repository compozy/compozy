#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
target_dir=$(mktemp -d "${TMPDIR:-/tmp}/compozy-desktop-release-build.XXXXXX")

cleanup() {
  local status=$?
  trap - EXIT INT TERM
  rm -rf "${target_dir}"
  exit "${status}"
}
trap cleanup EXIT INT TERM

expect_rejected() {
  local expected=$1
  shift
  local output
  if output=$("$@" 2>&1); then
    echo "desktop release build contract: command unexpectedly succeeded: $*" >&2
    exit 1
  fi
  if [[ "${output}" != *"${expected}"* ]]; then
    printf 'desktop release build contract: expected %q, got:\n' "${expected}" >&2
    echo "${output}" >&2
    exit 1
  fi
}

cargo_check=(
  cargo check --locked --release
  --manifest-path "${repo_root}/desktop/src-tauri/Cargo.toml"
)
common_env=(
  env
  -u COMPOZY_RELEASE_CHANNEL
  -u TAURI_CONFIG
  -u TAURI_SIGNING_PRIVATE_KEY
  -u TAURI_SIGNING_PRIVATE_KEY_PASSWORD
  "CARGO_TARGET_DIR=${target_dir}"
)
complete_config='{"version":"0.4.0-beta.2","plugins":{"updater":{"endpoints":["https://releases.compozy.com/desktop/beta/latest.json"],"pubkey":"fixture-public-key"}}}'

expect_rejected "COMPOZY_RELEASE_CHANNEL is required" "${common_env[@]}" "${cargo_check[@]}"
expect_rejected "TAURI_CONFIG is required" "${common_env[@]}" \
  "COMPOZY_RELEASE_CHANNEL=beta" "${cargo_check[@]}"
expect_rejected "TAURI_SIGNING_PRIVATE_KEY is required" "${common_env[@]}" \
  "COMPOZY_RELEASE_CHANNEL=beta" "TAURI_CONFIG=${complete_config}" "${cargo_check[@]}"
expect_rejected "TAURI_SIGNING_PRIVATE_KEY_PASSWORD is required" "${common_env[@]}" \
  "COMPOZY_RELEASE_CHANNEL=beta" "TAURI_CONFIG=${complete_config}" \
  "TAURI_SIGNING_PRIVATE_KEY=fixture-private-key" "${cargo_check[@]}"

echo "desktop release build contract: PASS"
