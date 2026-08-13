#!/usr/bin/env bash
set -euo pipefail

channel="${1:?release channel is required}"
expected_version="${2:?expected live version is required}"
bucket="${3:?R2 bucket is required}"
endpoint="${4:?R2 endpoint is required}"
verify_attempts="${COMPOZY_FEED_VERIFY_ATTEMPTS:-30}"
verify_interval_seconds="${COMPOZY_FEED_VERIFY_INTERVAL_SECONDS:-2}"
recovery_id="${COMPOZY_FEED_REPAIR_ID:-${expected_version}}"

if [[ "${channel}" != "beta" ]]; then
  echo "desktop feed repair: only beta may be repaired; stable remains reserved" >&2
  exit 1
fi
if [[ ! "${expected_version}" =~ ^[0-9]+\.[0-9]+\.[0-9]+-beta\.[0-9]+$ ]]; then
  echo "desktop feed repair: expected version must be a numeric beta release" >&2
  exit 1
fi
if [[ ! "${recovery_id}" =~ ^[A-Za-z0-9][A-Za-z0-9._-]*$ ]]; then
  echo "desktop feed repair: recovery id contains unsafe characters" >&2
  exit 1
fi
for name in AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY TAURI_SIGNING_PRIVATE_KEY \
  TAURI_SIGNING_PRIVATE_KEY_PASSWORD TAURI_SIGNING_PUBLIC_KEY; do
  if [[ -z "${!name:-}" ]]; then
    echo "desktop feed repair: ${name} is required" >&2
    exit 1
  fi
done
if [[ ! "${verify_attempts}" =~ ^[1-9][0-9]*$ || ! "${verify_interval_seconds}" =~ ^[0-9]+$ ]]; then
  echo "desktop feed repair: verification attempts must be positive and the interval must be non-negative" >&2
  exit 1
fi
for command in aws bunx cmp curl go jq minisign sleep; do
  command -v "${command}" >/dev/null 2>&1 || {
    echo "desktop feed repair: ${command} is required" >&2
    exit 1
  }
done

work_dir="$(mktemp -d "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/compozy-feed-repair.XXXXXX")"
mutation_started=false
commit_complete=false
feed_url="https://releases.compozy.com/desktop/${channel}"
recovery_prefix="desktop/recovery/${recovery_id}"

restore_live_feed() {
  local failed=false
  for name in runtime.json runtime.json.minisig latest.json; do
    local content_type=application/json
    if [[ "${name}" == "runtime.json.minisig" ]]; then
      content_type=text/plain
    fi
    if ! aws s3 cp "${work_dir}/live-${name}" "s3://${bucket}/desktop/${channel}/${name}" \
      --endpoint-url "${endpoint}" \
      --cache-control no-cache \
      --content-type "${content_type}"; then
      failed=true
    fi
  done
  for name in runtime.json runtime.json.minisig latest.json; do
    if ! aws s3 cp "s3://${bucket}/desktop/${channel}/${name}" "${work_dir}/restored-${name}" \
      --endpoint-url "${endpoint}" \
      || ! cmp -s "${work_dir}/live-${name}" "${work_dir}/restored-${name}"; then
      failed=true
    fi
  done
  if [[ "${failed}" == "true" ]]; then
    echo "desktop feed repair: rollback failed; restore from s3://${bucket}/${recovery_prefix}" >&2
    return 1
  fi
  echo "desktop feed repair: restored the original live feed after publication failure" >&2
}

cleanup() {
  local status=$?
  trap - EXIT
  if [[ "${status}" != "0" && "${mutation_started}" == "true" && "${commit_complete}" != "true" ]]; then
    if ! restore_live_feed; then
      status=1
    fi
  fi
  if ! rm -rf -- "${work_dir}"; then
    echo "desktop feed repair: could not remove temporary directory ${work_dir}" >&2
    status=1
  fi
  exit "${status}"
}
trap cleanup EXIT

for name in latest.json runtime.json runtime.json.minisig; do
  curl --fail --silent --show-error "${feed_url}/${name}" --output "${work_dir}/live-${name}"
done

for manifest in latest runtime; do
  version="$(jq -er '.version | strings | select(length > 0)' "${work_dir}/live-${manifest}.json")"
  if [[ "${version}" != "${expected_version}" ]]; then
    echo "desktop feed repair: live ${manifest}.json version ${version} does not match ${expected_version}" >&2
    exit 1
  fi
  go run ./cmd/compozy-desktop-release canonicalize-json \
    --input "${work_dir}/live-${manifest}.json" >"${work_dir}/${manifest}.json"
  jq -e --slurp '.[0] == .[1]' \
    "${work_dir}/live-${manifest}.json" "${work_dir}/${manifest}.json" >/dev/null
done

scripts/verify-desktop-signature.sh \
  "${work_dir}/live-runtime.json" \
  "${work_dir}/live-runtime.json.minisig" \
  "${TAURI_SIGNING_PUBLIC_KEY}"

if cmp -s "${work_dir}/live-latest.json" "${work_dir}/latest.json" \
  && cmp -s "${work_dir}/live-runtime.json" "${work_dir}/runtime.json"; then
  echo "desktop feed repair: PASS (${channel} ${expected_version} is already canonical)"
  exit 0
fi

scripts/sign-runtime-manifest.sh "${work_dir}/runtime.json"
scripts/verify-desktop-signature.sh \
  "${work_dir}/runtime.json" \
  "${work_dir}/runtime.json.minisig" \
  "${TAURI_SIGNING_PUBLIC_KEY}"

for name in latest.json runtime.json runtime.json.minisig; do
  aws s3 cp "${work_dir}/live-${name}" "s3://${bucket}/${recovery_prefix}/${name}" \
    --endpoint-url "${endpoint}" \
    --cache-control 'public,max-age=31536000,immutable'
done
echo "desktop feed repair: recovery copy s3://${bucket}/${recovery_prefix}"

for name in latest.json runtime.json runtime.json.minisig; do
  curl --fail --silent --show-error "${feed_url}/${name}" --output "${work_dir}/current-${name}"
  cmp "${work_dir}/live-${name}" "${work_dir}/current-${name}"
done

mutation_started=true
aws s3 cp "${work_dir}/runtime.json" "s3://${bucket}/desktop/${channel}/runtime.json" \
  --endpoint-url "${endpoint}" \
  --cache-control no-cache \
  --content-type application/json
aws s3 cp "${work_dir}/runtime.json.minisig" "s3://${bucket}/desktop/${channel}/runtime.json.minisig" \
  --endpoint-url "${endpoint}" \
  --cache-control no-cache \
  --content-type text/plain
aws s3 cp "${work_dir}/latest.json" "s3://${bucket}/desktop/${channel}/latest.json" \
  --endpoint-url "${endpoint}" \
  --cache-control no-cache \
  --content-type application/json

for name in latest.json runtime.json runtime.json.minisig; do
  aws s3 cp "s3://${bucket}/desktop/${channel}/${name}" "${work_dir}/stored-${name}" \
    --endpoint-url "${endpoint}"
  cmp "${work_dir}/${name}" "${work_dir}/stored-${name}"
done
commit_complete=true

download_published_exact() {
  local name="$1"
  local attempt
  for ((attempt = 1; attempt <= verify_attempts; attempt++)); do
    if curl --fail --silent --show-error \
      "${feed_url}/${name}?repair=${recovery_id}-${attempt}" \
      --output "${work_dir}/published-${name}" \
      && cmp -s "${work_dir}/${name}" "${work_dir}/published-${name}"; then
      return 0
    fi
    if ((attempt < verify_attempts)); then
      sleep "${verify_interval_seconds}"
    fi
  done
  echo "desktop feed repair: published ${name} did not converge after ${verify_attempts} attempts" >&2
  return 1
}

for name in latest.json runtime.json runtime.json.minisig; do
  download_published_exact "${name}"
done
scripts/verify-desktop-signature.sh \
  "${work_dir}/published-runtime.json" \
  "${work_dir}/published-runtime.json.minisig" \
  "${TAURI_SIGNING_PUBLIC_KEY}"

echo "desktop feed repair: PASS (${channel} ${expected_version})"
