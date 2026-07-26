#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

binary=${1:-./.tmp/air/agh}
if [[ ! -x "$binary" ]]; then
  echo "dev: daemon binary is not executable: $binary" >&2
  exit 1
fi

: "${AGH_AIR_DEV_RUN_ID:?AGH_AIR_DEV_RUN_ID must identify the owning development session}"

state_dir=${AGH_AIR_STATE_DIR:-}
if [[ -z "$state_dir" ]]; then
  state_dir=$(go run ./scripts/air-state-dir)
fi
running_binary_id_file="$state_dir/running-binary-id"
owner_id_file="$state_dir/dev-owner-id"
mkdir -p "$state_dir"

if [[ ${AGH_AIR_STATE_LOCK_HELD:-} != 1 ]]; then
  export AGH_AIR_STATE_LOCK_HELD=1
  exec go run ./scripts/air-state-lock "$state_dir/dev-owner.lock" -- bash "$0" "$binary"
fi

atomic_write() {
  local destination=$1
  local value=$2
  local temporary="${destination}.tmp.$$"

  if ! printf '%s\n' "$value" > "$temporary"; then
    echo "dev: failed to write temporary state $temporary" >&2
    return 1
  fi
  if ! mv -f -- "$temporary" "$destination"; then
    echo "dev: failed to publish state $destination" >&2
    if ! rm -f -- "$temporary"; then
      echo "dev: failed to remove temporary state $temporary" >&2
    fi
    return 1
  fi
}

if ! checksum_output=$(cksum "$binary"); then
  echo "dev: failed to identify daemon binary $binary" >&2
  exit 1
fi
read -r checksum byte_count _ <<< "$checksum_output"
binary_id="$checksum:$byte_count"

replace_daemon_locked() {
  local running_binary_id=
  if [[ -f "$running_binary_id_file" ]]; then
    running_binary_id=$(<"$running_binary_id_file")
  fi
  local owner_id=
  if [[ -f "$owner_id_file" ]]; then
    owner_id=$(<"$owner_id_file")
  fi
  if [[ "$binary_id" == "$running_binary_id" && "$owner_id" == "$AGH_AIR_DEV_RUN_ID" ]]; then
    echo "dev: build failed or produced unchanged daemon bytes; keeping the current daemon"
    return 0
  fi

  local stop_output=
  if stop_output=$("$binary" daemon stop 2>&1); then
    echo "dev: stopped the existing daemon for development takeover"
  elif [[ "$stop_output" != *"cli: daemon is not running"* ]]; then
    echo "dev: could not take over the existing daemon" >&2
    printf '%s\n' "$stop_output" >&2
    return 1
  fi

  if ! atomic_write "$owner_id_file" "$AGH_AIR_DEV_RUN_ID"; then
    return 1
  fi

  echo "dev: starting the rebuilt daemon"
  if ! "$binary" daemon start; then
    return 1
  fi

  atomic_write "$running_binary_id_file" "$binary_id"
}

if ! replace_daemon_locked; then
  exit 1
fi

if [[ -n ${AGH_AIR_READY_FIFO:-} || -n ${AGH_AIR_READY_MARKER:-} ]]; then
  : "${AGH_AIR_READY_FIFO:?AGH_AIR_READY_FIFO must identify the development readiness channel}"
  : "${AGH_AIR_READY_MARKER:?AGH_AIR_READY_MARKER must identify the one-shot readiness marker}"
  bash "$repo_root/scripts/dev-readiness.sh" ready \
    "$AGH_AIR_READY_FIFO" "$AGH_AIR_DEV_RUN_ID" "$AGH_AIR_READY_MARKER" "$binary_id"
fi
