#!/usr/bin/env bash
set -euo pipefail

manifest_url="${1:?runtime manifest URL is required}"
work_dir="$(mktemp -d "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/compozy-runtime-feed.XXXXXX")"
trap 'rm -rf -- "${work_dir}"' EXIT

curl --fail --silent --show-error "${manifest_url}" --output "${work_dir}/runtime.json"
go run ./cmd/compozy-desktop-release canonicalize-json \
  --input "${work_dir}/runtime.json" >"${work_dir}/canonical.json"
if ! cmp -s "${work_dir}/runtime.json" "${work_dir}/canonical.json"; then
  echo "desktop release: live runtime manifest is not canonical JSON" >&2
  exit 1
fi
jq -er '.version | strings | select(length > 0)' "${work_dir}/runtime.json"
