#!/usr/bin/env bash
set -euo pipefail

manifest="${1:?runtime manifest path is required}"
payload_dir="${2:?runtime payload directory is required}"
release_version="${3:?release version is required}"

if [[ ! -s "${manifest}" || ! -d "${payload_dir}" ]]; then
  echo "desktop release: runtime manifest and payload directory must exist" >&2
  exit 1
fi

inventory="$(mktemp)"
declared_names="$(mktemp)"
actual_paths="$(mktemp)"
trap 'rm -f "${inventory}" "${declared_names}" "${actual_paths}"' EXIT

if ! jq -er '.platforms | to_entries[] | [.value.url, (.value.size | tostring), .value.sha256] | @tsv' \
  "${manifest}" >"${inventory}"; then
  echo "desktop release: runtime manifest payload inventory is invalid" >&2
  exit 1
fi
if [[ ! -s "${inventory}" ]]; then
  echo "desktop release: runtime manifest payload inventory is empty" >&2
  exit 1
fi

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

url_prefix="https://releases.compozy.com/desktop/v/runtime/${release_version}/"
while IFS=$'\t' read -r url expected_size expected_sha256; do
  if [[ "${url}" != "${url_prefix}"* ]]; then
    echo "desktop release: runtime payload URL is outside the immutable release prefix: ${url}" >&2
    exit 1
  fi
  name="${url#"${url_prefix}"}"
  if [[ -z "${name}" || "${name}" == */* || "${name}" == *\\* ]]; then
    echo "desktop release: runtime payload URL has an invalid filename: ${url}" >&2
    exit 1
  fi
  printf '%s\n' "${name}" >>"${declared_names}"
  path="${payload_dir}/${name}"
  if [[ ! -f "${path}" || -L "${path}" ]]; then
    echo "desktop release: declared runtime payload is missing or not a regular file: ${name}" >&2
    exit 1
  fi
  actual_size="$(wc -c <"${path}" | tr -d '[:space:]')"
  if [[ "${actual_size}" != "${expected_size}" ]]; then
    echo "desktop release: runtime payload ${name} size ${actual_size} does not match ${expected_size}" >&2
    exit 1
  fi
  actual_sha256="$(sha256_file "${path}")"
  if [[ "${actual_sha256}" != "${expected_sha256}" ]]; then
    echo "desktop release: runtime payload ${name} SHA-256 does not match the signed manifest" >&2
    exit 1
  fi
done <"${inventory}"

duplicate="$(sort "${declared_names}" | uniq -d | head -n 1)"
if [[ -n "${duplicate}" ]]; then
  echo "desktop release: runtime manifest declares duplicate payload ${duplicate}" >&2
  exit 1
fi
if ! find "${payload_dir}" -mindepth 1 -maxdepth 1 -print0 >"${actual_paths}"; then
  echo "desktop release: failed to enumerate runtime payloads" >&2
  exit 1
fi
while IFS= read -r -d '' path; do
  name="$(basename "${path}")"
  if ! grep -Fqx -- "${name}" "${declared_names}"; then
    echo "desktop release: unexpected runtime payload ${name}" >&2
    exit 1
  fi
done <"${actual_paths}"

echo "desktop release: runtime payload inventory matches the signed manifest"
