#!/bin/sh
set -eu

if ! command -v openbox >/dev/null 2>&1; then
  echo "Desktop E2E on Linux requires openbox." >&2
  exit 1
fi

openbox --sm-disable >/dev/null 2>&1 &
window_manager_pid=$!
cleanup() {
  kill "$window_manager_pid" 2>/dev/null || true
  wait "$window_manager_pid" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

bun run --cwd desktop test:e2e
