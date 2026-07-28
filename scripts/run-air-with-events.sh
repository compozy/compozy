#!/usr/bin/env bash

set -euo pipefail

if (($# < 3)); then
  echo "usage: scripts/run-air-with-events.sh <readiness-fifo> <run-id> <air-version> [air-args...]" >&2
  exit 2
fi

readiness_fifo=$1
run_id=$2
air_version=$3
shift 3

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
air_pid=
requested_status=0

forward_requested_signal() {
  local signal_name=TERM

  if ((requested_status == 130)); then
    signal_name=INT
  fi
  if [[ -n "$air_pid" ]] && kill -0 "$air_pid" 2>/dev/null; then
    if ! kill "-$signal_name" "$air_pid" 2>/dev/null && kill -0 "$air_pid" 2>/dev/null; then
      echo "dev: failed to forward $signal_name to Air (pid=$air_pid)" >&2
    fi
  fi
}

trap 'requested_status=130' INT
trap 'requested_status=143' TERM

bash "$repo_root/scripts/run-air.sh" "$air_version" "$@" &
air_pid=$!

air_status=0
wait "$air_pid" || air_status=$?
if ((requested_status != 0)); then
  forward_requested_signal
fi
if kill -0 "$air_pid" 2>/dev/null; then
  wait "$air_pid" || air_status=$?
fi
if ((requested_status != 0)); then
  air_status=$requested_status
fi

if ! bash "$repo_root/scripts/dev-readiness.sh" air-exit "$readiness_fifo" "$run_id" "$air_status"; then
  echo "dev: failed to publish Air exit status $air_status" >&2
  if ((air_status == 0)); then
    air_status=1
  fi
fi

exit "$air_status"
