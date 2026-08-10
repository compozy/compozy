#!/usr/bin/env bash
set -euo pipefail

artifact="${1:?artifact path is required}"
signature="${2:?signature path is required}"
public_key="${3:-${TAURI_SIGNING_PUBLIC_KEY:-}}"

if [[ -z "${public_key}" ]]; then
  echo "desktop release: updater public key is required" >&2
  exit 1
fi
if [[ ! -s "${artifact}" || ! -s "${signature}" ]]; then
  echo "desktop release: artifact and signature must be non-empty files" >&2
  exit 1
fi

verify_dir="$(mktemp -d)"
trap 'rm -rf "${verify_dir}"' EXIT
printf '%s' "${public_key}" | base64 --decode >"${verify_dir}/public.key"

case "${signature}" in
  *.minisig)
    cp "${signature}" "${verify_dir}/signature.minisig"
    ;;
  *)
    base64 --decode <"${signature}" >"${verify_dir}/signature.minisig"
    ;;
esac

minisign -Vm "${artifact}" -p "${verify_dir}/public.key" -x "${verify_dir}/signature.minisig"

