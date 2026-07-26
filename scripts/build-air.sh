#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

build_dir=${AGH_AIR_BUILD_DIR:-"$repo_root/.tmp/air"}
binary="$build_dir/agh"
next_binary="$build_dir/agh.next"

cleanup() {
  local exit_status=$?
  trap - EXIT

  if [[ -e "$next_binary" ]] && ! rm -f -- "$next_binary"; then
    echo "dev: failed to remove incomplete daemon binary $next_binary" >&2
    if ((exit_status == 0)); then
      exit_status=1
    fi
  fi

  exit "$exit_status"
}

trap cleanup EXIT

mkdir -p "$build_dir"
go build -o "$next_binary" ./cmd/agh
mv -f -- "$next_binary" "$binary"
