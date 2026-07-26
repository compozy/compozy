#!/usr/bin/env bash

set -euo pipefail

usage() {
  echo "usage: scripts/dev-readiness.sh <await|ready|air-exit> <args...>" >&2
  exit 2
}

require_fifo() {
  local fifo_path=$1
  if [[ ! -p "$fifo_path" ]]; then
    echo "dev: readiness channel is not a FIFO: $fifo_path" >&2
    return 1
  fi
}

write_event() {
  local fifo_path=$1
  local event_kind=$2
  local run_id=$3
  local payload=$4

  require_fifo "$fifo_path"
  printf '%s\t%s\t%s\n' "$event_kind" "$run_id" "$payload" > "$fifo_path"
}

await_readiness() {
  local fifo_path=$1
  local expected_run_id=$2
  local event_kind=
  local event_run_id=
  local payload=

  require_fifo "$fifo_path"
  while IFS=$'\t' read -r event_kind event_run_id payload; do
    if [[ "$event_run_id" != "$expected_run_id" ]]; then
      continue
    fi
    case "$event_kind" in
      ready)
        if [[ -z "$payload" ]]; then
          echo "dev: daemon readiness event did not include a binary identity" >&2
          return 1
        fi
        printf '%s\n' "$payload"
        return 0
        ;;
      air-exit)
        if [[ ! "$payload" =~ ^[0-9]+$ ]] || ((payload < 0 || payload > 255)); then
          echo "dev: Air exit event contained an invalid status: $payload" >&2
          return 1
        fi
        echo "dev: Air exited with status $payload before daemon readiness" >&2
        if ((payload == 0)); then
          return 1
        fi
        return "$payload"
        ;;
      *)
        echo "dev: unknown readiness event for the current run: $event_kind" >&2
        return 1
        ;;
    esac
  done < "$fifo_path"

  echo "dev: readiness channel closed before the daemon became ready" >&2
  return 1
}

publish_ready() {
  local fifo_path=$1
  local run_id=$2
  local marker_path=$3
  local binary_id=$4

  if [[ -d "$marker_path" ]]; then
    return 0
  fi
  if ! mkdir -- "$marker_path"; then
    if [[ -d "$marker_path" ]]; then
      return 0
    fi
    echo "dev: failed to claim readiness marker $marker_path" >&2
    return 1
  fi
  if ! write_event "$fifo_path" ready "$run_id" "$binary_id"; then
    if ! rmdir -- "$marker_path"; then
      echo "dev: failed to release readiness marker $marker_path" >&2
    fi
    return 1
  fi
}

if (($# < 1)); then
  usage
fi

command_name=$1
shift
case "$command_name" in
  await)
    if (($# != 2)); then
      usage
    fi
    await_readiness "$1" "$2"
    ;;
  ready)
    if (($# != 4)); then
      usage
    fi
    publish_ready "$1" "$2" "$3" "$4"
    ;;
  air-exit)
    if (($# != 3)); then
      usage
    fi
    write_event "$1" air-exit "$2" "$3"
    ;;
  *)
    usage
    ;;
esac
