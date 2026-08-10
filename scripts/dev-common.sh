#!/usr/bin/env bash
# Shared process and teardown helpers for the development entry scripts
# (scripts/dev.sh, scripts/desktop-dev.sh). Sourced, never executed.
# remove_dev_readiness reads the caller's readiness_dir/readiness_fifo/
# readiness_marker/readiness_fd_open globals; both entry scripts keep the
# readiness channel open on fd 9. stop_owned_daemon expects the caller's
# working directory to be the repo root.

terminate_process() {
  local pid=$1
  local name=$2

  if [[ -z "$pid" ]] || ! kill -0 "$pid" 2>/dev/null; then
    return
  fi

  echo "dev: stopping $name"
  if ! kill -TERM "$pid" 2>/dev/null && kill -0 "$pid" 2>/dev/null; then
    echo "dev: failed to signal $name (pid=$pid)" >&2
  fi
}

wait_for_process() {
  local pid=$1
  local name=$2
  local attempts=0

  if [[ -z "$pid" ]]; then
    return
  fi

  while kill -0 "$pid" 2>/dev/null && ((attempts < 64)); do
    sleep 0.25
    attempts=$((attempts + 1))
  done

  if kill -0 "$pid" 2>/dev/null; then
    echo "dev: $name did not stop after 16s; forcing termination" >&2
    if ! kill -KILL "$pid" 2>/dev/null && kill -0 "$pid" 2>/dev/null; then
      echo "dev: failed to terminate $name (pid=$pid)" >&2
    fi
  fi

  local wait_status=0
  wait "$pid" || wait_status=$?
  if ((wait_status != 0 && wait_status != 130 && wait_status != 137 && wait_status != 143)); then
    echo "dev: $name exited with status $wait_status" >&2
  fi
}

remove_dev_readiness() {
  local cleanup_status=0

  if ((readiness_fd_open == 1)); then
    if ! exec 9>&-; then
      echo "dev: failed to close the readiness channel" >&2
      cleanup_status=1
    fi
    readiness_fd_open=0
  fi
  if [[ -n "$readiness_marker" ]] && [[ -d "$readiness_marker" ]]; then
    if ! rmdir -- "$readiness_marker"; then
      echo "dev: failed to remove readiness marker $readiness_marker" >&2
      cleanup_status=1
    fi
  fi
  if [[ -n "$readiness_fifo" ]] && [[ -p "$readiness_fifo" ]]; then
    if ! rm -f -- "$readiness_fifo"; then
      echo "dev: failed to remove readiness channel $readiness_fifo" >&2
      cleanup_status=1
    fi
  fi
  if [[ -n "$readiness_dir" ]] && [[ -d "$readiness_dir" ]]; then
    if ! rmdir -- "$readiness_dir"; then
      echo "dev: failed to remove readiness directory $readiness_dir" >&2
      cleanup_status=1
    fi
  fi

  return "$cleanup_status"
}

stop_owned_daemon() {
  local state_dir=$1
  local owner_id=$2
  local binary=$3

  if [[ ! -f "$state_dir/dev-owner-id" ]]; then
    return 0
  fi
  go run ./scripts/air-state-lock "$state_dir/dev-owner.lock" -- \
    bash scripts/stop-dev-daemon.sh "$state_dir" "$owner_id" "$binary"
}
