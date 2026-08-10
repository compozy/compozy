#!/usr/bin/env bash
set -euo pipefail

desktop_dir="${1:?normalized desktop artifact directory is required}"
runtime_dir="${2:?runtime artifact directory is required}"
feed_dir="${3:?feed directory is required}"
channel="${4:?channel is required}"
release_version="${5:?release version is required}"
bucket="${6:?R2 bucket is required}"
endpoint="${7:?R2 endpoint is required}"

if [[ "${channel}" != "beta" ]]; then
  echo "desktop release: only beta may publish; stable remains reserved" >&2
  exit 1
fi
for name in AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY TAURI_SIGNING_PUBLIC_KEY; do
  if [[ -z "${!name:-}" ]]; then
    echo "desktop release: ${name} is required before publication" >&2
    exit 1
  fi
done

scripts/assert-desktop-artifacts.sh "${desktop_dir}" "${release_version}"
go run ./cmd/compozy-desktop-release assert-comparator \
  --source desktop/src-tauri/src/update/app_update.rs

for artifact in \
  "CompozyOS_${release_version}_universal.app.tar.gz" \
  "CompozyOS_${release_version}_amd64.AppImage" \
  "CompozyOS_${release_version}_x64-setup.exe"; do
  scripts/verify-desktop-signature.sh \
    "${desktop_dir}/${artifact}" \
    "${desktop_dir}/${artifact}.sig" \
    "${TAURI_SIGNING_PUBLIC_KEY}"
done
scripts/verify-desktop-signature.sh \
  "${feed_dir}/runtime.json" \
  "${feed_dir}/runtime.json.minisig" \
  "${TAURI_SIGNING_PUBLIC_KEY}"

live_version=""
live_feed="$(mktemp)"
verify_dir="$(mktemp -d)"
trap 'rm -f "${live_feed}"; rm -rf "${verify_dir}"' EXIT
scripts/assert-runtime-manifest-payloads.sh \
  "${feed_dir}/runtime.json" \
  "${runtime_dir}" \
  "${release_version}"
jq -er '.platforms | to_entries[] | [.value.url, (.value.size | tostring), .value.sha256] | @tsv' \
  "${feed_dir}/runtime.json" >"${verify_dir}/runtime-inventory.tsv"
live_status="$(curl --silent --show-error --output "${live_feed}" --write-out '%{http_code}' \
  "https://releases.compozy.com/desktop/${channel}/latest.json")"
case "${live_status}" in
  200)
    live_version="$(jq -er '.version' "${live_feed}")"
    ;;
  404)
    echo "desktop release: no live beta feed exists; treating this as the first desktop publication"
    ;;
  *)
    echo "desktop release: live feed returned HTTP ${live_status}" >&2
    exit 1
    ;;
esac
scripts/assert-desktop-version-strictly-greater.sh "${release_version}" "${live_version}"

immutable_cache='public,max-age=31536000,immutable'
for artifact_path in "${desktop_dir}"/*; do
  artifact_name="$(basename "${artifact_path}")"
  aws s3 cp "${artifact_path}" \
    "s3://${bucket}/desktop/v/${release_version}/${artifact_name}" \
    --endpoint-url "${endpoint}" \
    --cache-control "${immutable_cache}"
done
while IFS=$'\t' read -r runtime_url _runtime_size _runtime_sha256; do
  runtime_name="${runtime_url##*/}"
  runtime_path="${runtime_dir}/${runtime_name}"
  aws s3 cp "${runtime_path}" \
    "s3://${bucket}/desktop/v/runtime/${release_version}/${runtime_name}" \
    --endpoint-url "${endpoint}" \
    --cache-control "${immutable_cache}"
done <"${verify_dir}/runtime-inventory.tsv"

# Manifests are the commit point. Each command is one atomic PutObject and runs
# only after every immutable payload and signature is present.
aws s3 cp "${feed_dir}/runtime.json" \
  "s3://${bucket}/desktop/${channel}/runtime.json" \
  --endpoint-url "${endpoint}" \
  --cache-control no-cache \
  --content-type application/json
aws s3 cp "${feed_dir}/runtime.json.minisig" \
  "s3://${bucket}/desktop/${channel}/runtime.json.minisig" \
  --endpoint-url "${endpoint}" \
  --cache-control no-cache \
  --content-type text/plain
aws s3 cp "${feed_dir}/latest.json" \
  "s3://${bucket}/desktop/${channel}/latest.json" \
  --endpoint-url "${endpoint}" \
  --cache-control no-cache \
  --content-type application/json

curl --fail --silent --show-error \
  "https://releases.compozy.com/desktop/${channel}/latest.json" \
  --output "${verify_dir}/latest.json"
curl --fail --silent --show-error \
  "https://releases.compozy.com/desktop/${channel}/runtime.json" \
  --output "${verify_dir}/runtime.json"
curl --fail --silent --show-error \
  "https://releases.compozy.com/desktop/${channel}/runtime.json.minisig" \
  --output "${verify_dir}/runtime.json.minisig"

published_version="$(jq -er '.version' "${verify_dir}/latest.json")"
if [[ "${published_version}" != "${release_version}" ]]; then
  echo "desktop release: published feed version ${published_version} does not match ${release_version}" >&2
  exit 1
fi
cmp "${feed_dir}/latest.json" "${verify_dir}/latest.json"
cmp "${feed_dir}/runtime.json" "${verify_dir}/runtime.json"
scripts/verify-desktop-signature.sh \
  "${verify_dir}/runtime.json" \
  "${verify_dir}/runtime.json.minisig" \
  "${TAURI_SIGNING_PUBLIC_KEY}"

runtime_verify_dir="${verify_dir}/runtime-payloads"
mkdir -p "${runtime_verify_dir}"
while IFS=$'\t' read -r runtime_url _runtime_size _runtime_sha256; do
  runtime_name="${runtime_url##*/}"
  curl --fail --silent --show-error \
    "${runtime_url}" \
    --output "${runtime_verify_dir}/${runtime_name}"
done <"${verify_dir}/runtime-inventory.tsv"
scripts/assert-runtime-manifest-payloads.sh \
  "${verify_dir}/runtime.json" \
  "${runtime_verify_dir}" \
  "${release_version}"

for platform in darwin-aarch64 darwin-x86_64 linux-x86_64 windows-x86_64; do
  payload_url="$(jq -er --arg platform "${platform}" '.platforms[$platform].url' "${verify_dir}/latest.json")"
  jq -er --arg platform "${platform}" '.platforms[$platform].signature' \
    "${verify_dir}/latest.json" >"${verify_dir}/${platform}.sig"
  curl --fail --silent --show-error "${payload_url}" --output "${verify_dir}/${platform}.payload"
  scripts/verify-desktop-signature.sh \
    "${verify_dir}/${platform}.payload" \
    "${verify_dir}/${platform}.sig" \
    "${TAURI_SIGNING_PUBLIC_KEY}"
done

echo "desktop release publication: PASS (${release_version})"
