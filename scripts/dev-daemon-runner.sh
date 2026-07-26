#!/usr/bin/env bash

set -euo pipefail

if (($# < 1)); then
  echo "usage: scripts/dev-daemon-runner.sh <air-version> [air-args...]" >&2
  exit 2
fi

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

air_version=$1
shift

air_pid=
air_state_dir=$(go run ./scripts/air-state-dir)
air_build_dir="$repo_root/.tmp/air"
dev_run_id="dev-daemon-$$-$(date +%s)"
export AGH_AIR_BUILD_DIR=$air_build_dir
export AGH_AIR_STATE_DIR=$air_state_dir
export AGH_AIR_DEV_RUN_ID=$dev_run_id

stop_owned_daemon() {
  go run ./scripts/air-state-lock "$air_state_dir/dev-owner.lock" -- \
    bash scripts/stop-dev-daemon.sh "$air_state_dir" "$dev_run_id" "$air_build_dir/agh"
}

cleanup() {
  local exit_status=$?
  trap - EXIT INT TERM

  if [[ -n "$air_pid" ]] && kill -0 "$air_pid" 2>/dev/null; then
    if ! kill -TERM "$air_pid" 2>/dev/null && kill -0 "$air_pid" 2>/dev/null; then
      echo "dev: failed to signal Air (pid=$air_pid)" >&2
      exit_status=1
    fi
  fi

  if [[ -n "$air_pid" ]]; then
    local air_status=0
    wait "$air_pid" || air_status=$?
    if ((air_status != 0 && air_status != 130 && air_status != 143)); then
      echo "dev: Air exited with status $air_status" >&2
      if ((exit_status == 0)); then
        exit_status=$air_status
      fi
    fi
  fi

  if ! stop_owned_daemon && ((exit_status == 0)); then
    exit_status=1
  fi

  exit "$exit_status"
}

trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

bash scripts/run-air.sh "$air_version" "$@" &
air_pid=$!

air_status=0
wait "$air_pid" || air_status=$?
air_pid=
exit "$air_status"
