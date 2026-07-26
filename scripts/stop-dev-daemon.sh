#!/usr/bin/env bash

set -euo pipefail

if (($# != 3)); then
  echo "usage: scripts/stop-dev-daemon.sh <state-dir> <owner-id> <daemon-binary>" >&2
  exit 2
fi

state_dir=$1
expected_owner_id=$2
binary=$3
owner_id_file="$state_dir/dev-owner-id"

if [[ ! -f "$owner_id_file" ]] || [[ "$(<"$owner_id_file")" != "$expected_owner_id" ]]; then
  exit 0
fi
if [[ ! -x "$binary" ]]; then
  echo "dev: cannot stop the owned daemon because $binary is unavailable" >&2
  exit 1
fi

stop_output=
if stop_output=$("$binary" daemon stop 2>&1); then
  echo "dev: stopped the daemon owned by this development session"
elif [[ "$stop_output" != *"cli: daemon is not running"* ]]; then
  echo "dev: failed to stop the daemon owned by this development session" >&2
  printf '%s\n' "$stop_output" >&2
  exit 1
fi

if [[ -f "$owner_id_file" ]] && [[ "$(<"$owner_id_file")" == "$expected_owner_id" ]]; then
  if ! rm -f -- "$owner_id_file"; then
    echo "dev: failed to clear daemon ownership state" >&2
    exit 1
  fi
fi
