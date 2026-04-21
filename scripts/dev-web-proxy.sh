#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/.." && pwd)"
DAEMON_HOME="${COMPOZY_DEV_HOME:-${REPO_ROOT}/.tmp/dev-home}"
DAEMON_HTTP_PORT="${COMPOZY_DAEMON_HTTP_PORT:-2123}"
WEB_DEV_PROXY="${COMPOZY_WEB_DEV_PROXY:-http://127.0.0.1:3000}"

mkdir -p "${DAEMON_HOME}"
cd "${REPO_ROOT}"

cleanup() {
  if [[ -n "${daemon_pid:-}" ]]; then
    kill "${daemon_pid}" 2>/dev/null || true
    wait "${daemon_pid}" 2>/dev/null || true
  fi
  if [[ -n "${vite_pid:-}" ]]; then
    kill "${vite_pid}" 2>/dev/null || true
    wait "${vite_pid}" 2>/dev/null || true
  fi
}

trap cleanup EXIT INT TERM

bun run --cwd web dev &
vite_pid=$!

env \
  HOME="${DAEMON_HOME}" \
  COMPOZY_DAEMON_HTTP_PORT="${DAEMON_HTTP_PORT}" \
  COMPOZY_WEB_DEV_PROXY="${WEB_DEV_PROXY}" \
  ./bin/compozy daemon start --foreground &
daemon_pid=$!

status=0
while kill -0 "${daemon_pid}" 2>/dev/null && kill -0 "${vite_pid}" 2>/dev/null; do
  sleep 1
done

if ! kill -0 "${daemon_pid}" 2>/dev/null; then
  wait "${daemon_pid}" || status=$?
fi

if ! kill -0 "${vite_pid}" 2>/dev/null && [[ "${status}" -eq 0 ]]; then
  wait "${vite_pid}" || status=$?
fi

exit "${status}"
