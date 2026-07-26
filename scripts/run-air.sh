#!/usr/bin/env bash

set -euo pipefail

if (($# < 1)); then
  echo "usage: scripts/run-air.sh <air-version> [air-args...]" >&2
  exit 2
fi

air_version=$1
shift

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cache_root=${AGH_AIR_CACHE_DIR:-$repo_root/.tmp/tools/air}
install_dir="$cache_root/$air_version"
air_binary="$install_dir/air"

if [[ -z ${AGH_AIR_DEV_RUN_ID:-} ]]; then
  AGH_AIR_DEV_RUN_ID="run-air-$$-$(date +%s)"
  export AGH_AIR_DEV_RUN_ID
fi

if [[ ! -x "$air_binary" ]]; then
  mkdir -p "$install_dir"
  echo "dev: installing Air $air_version"
  GOBIN="$install_dir" go install "github.com/air-verse/air@$air_version"
fi

exec "$air_binary" "$@"
