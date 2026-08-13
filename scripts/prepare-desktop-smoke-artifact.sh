#!/usr/bin/env bash
set -euo pipefail

platform=${1:?usage: prepare-desktop-smoke-artifact.sh <macos|linux> <artifact>}
artifact=${2:?desktop artifact is required}

[[ -f "${artifact}" ]] || {
  echo "desktop release smoke: artifact does not exist: ${artifact}" >&2
  exit 1
}

case "${platform}" in
  macos) ;;
  linux) chmod u+x "${artifact}" ;;
  *)
    echo "desktop release smoke: unsupported platform ${platform}" >&2
    exit 2
    ;;
esac
