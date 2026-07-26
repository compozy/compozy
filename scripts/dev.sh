#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

: "${AIR_VERSION:?AIR_VERSION must be set by the development command}"

air_pid=
vite_pid=
dev_web_dir=
readiness_dir=
readiness_fifo=
readiness_marker=
readiness_fd_open=0
air_state_dir=$(go run ./scripts/air-state-dir)
air_build_dir="$repo_root/.tmp/air"
dev_run_id="dev-$$-$(date +%s)"
export AGH_AIR_BUILD_DIR=$air_build_dir
export AGH_AIR_STATE_DIR=$air_state_dir
export AGH_AIR_DEV_RUN_ID=$dev_run_id

configured_api_proxy_target() {
  local config_json
  if ! config_json=$(go run ./cmd/agh config show -o json); then
    echo "dev: failed to resolve the daemon HTTP endpoint from the active config" >&2
    return 1
  fi
  printf '%s' "$config_json" | bun -e '
    const payload = JSON.parse(await Bun.stdin.text());
    const host = String(payload.config?.http?.host ?? "").trim();
    const port = Number(payload.config?.http?.port);
    if (!host || !Number.isInteger(port) || port < 1 || port > 65535) {
      throw new Error("active config does not contain a valid http.host and http.port");
    }
    const connectHost = host === "0.0.0.0" ? "127.0.0.1" : host === "::" ? "::1" : host;
    const urlHost = connectHost.includes(":") && !connectHost.startsWith("[")
      ? `[${connectHost}]`
      : connectHost;
    process.stdout.write(`http://${urlHost}:${port}`);
  '
}

if [[ -n ${AGH_WEB_API_PROXY_TARGET:-} ]]; then
  api_proxy_target=$AGH_WEB_API_PROXY_TARGET
else
  api_proxy_target=$(configured_api_proxy_target)
fi
export AGH_WEB_API_PROXY_TARGET=$api_proxy_target

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

remove_dev_web_redirect() {
  if [[ -z "$dev_web_dir" ]]; then
    return
  fi

  local redirect_file="$dev_web_dir/index.html"
  if [[ -e "$redirect_file" ]] && ! rm -f -- "$redirect_file"; then
    echo "dev: failed to remove $redirect_file" >&2
  fi
  if [[ -d "$dev_web_dir" ]] && ! rmdir -- "$dev_web_dir"; then
    echo "dev: failed to remove $dev_web_dir" >&2
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
  local binary="$air_build_dir/agh"

  go run ./scripts/air-state-lock "$air_state_dir/dev-owner.lock" -- \
    bash scripts/stop-dev-daemon.sh "$air_state_dir" "$dev_run_id" "$binary"
}

cleanup() {
  local exit_status=$?
  trap - EXIT INT TERM

  remove_dev_web_redirect
  terminate_process "$vite_pid" "Vite"
  terminate_process "$air_pid" "Air"
  wait_for_process "$vite_pid" "Vite"
  wait_for_process "$air_pid" "Air"
  if ! remove_dev_readiness && ((exit_status == 0)); then
    exit_status=1
  fi
  if ! stop_owned_daemon && ((exit_status == 0)); then
    exit_status=1
  fi

  exit "$exit_status"
}

trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

if [[ -n ${AGH_WEB_PORT:-} ]]; then
  web_port=$(bun scripts/find-dev-port.mjs "$AGH_WEB_PORT" --strict)
else
  web_port=$(bun scripts/find-dev-port.mjs 3000)
fi
web_url="http://localhost:$web_port"

dev_web_parent="$repo_root/.tmp"
mkdir -p "$dev_web_parent"
if ! dev_web_dir=$(mktemp -d "$dev_web_parent/dev-web-redirect.XXXXXX"); then
  echo "dev: failed to allocate the web redirect directory" >&2
  exit 1
fi
{
  printf '%s\n' '<!doctype html>' '<meta charset="utf-8">'
  printf '<meta http-equiv="refresh" content="0;url=%s/">\n' "$web_url"
  printf '%s\n' '<title>AGH development server</title>' '<script>'
  printf 'window.location.replace("%s" + window.location.pathname + window.location.search + window.location.hash);\n' "$web_url"
  printf '%s\n' '</script>'
  printf '<p>Open <a href="%s/">%s</a>.</p>\n' "$web_url" "$web_url"
} > "$dev_web_dir/index.html"
export AGH_WEB_DIST_DIR=$dev_web_dir

if ! readiness_dir=$(mktemp -d "$dev_web_parent/dev-readiness.XXXXXX"); then
  echo "dev: failed to allocate the readiness directory" >&2
  exit 1
fi
readiness_fifo="$readiness_dir/events.fifo"
readiness_marker="$readiness_dir/ready-sent"
if ! mkfifo "$readiness_fifo"; then
  echo "dev: failed to create the readiness channel $readiness_fifo" >&2
  exit 1
fi
if ! exec 9<> "$readiness_fifo"; then
  echo "dev: failed to open the readiness channel $readiness_fifo" >&2
  exit 1
fi
readiness_fd_open=1
export AGH_AIR_READY_FIFO=$readiness_fifo
export AGH_AIR_READY_MARKER=$readiness_marker

echo "dev: live web UI: $web_url"
echo "dev: daemon web routes will redirect to the live UI"
echo "dev: API traffic will be proxied to the daemon on $api_proxy_target"

bash scripts/run-air-with-events.sh "$readiness_fifo" "$dev_run_id" "$AIR_VERSION" -c .air.toml &
air_pid=$!

ready_binary_id=
ready_status=0
ready_binary_id=$(bash scripts/dev-readiness.sh await "$readiness_fifo" "$dev_run_id") || ready_status=$?
if ((ready_status != 0)); then
  echo "dev: daemon did not become ready for the current Air run" >&2
  exit "$ready_status"
fi
echo "dev: daemon ready for current Air build ($ready_binary_id)"

bun run --cwd web dev:raw -- --port "$web_port" --strictPort &
vite_pid=$!

while :; do
  if ! kill -0 "$air_pid" 2>/dev/null; then
    air_status=0
    wait "$air_pid" || air_status=$?
    air_pid=
    echo "dev: Air exited with status $air_status" >&2
    exit "$air_status"
  fi
  if ! kill -0 "$vite_pid" 2>/dev/null; then
    vite_status=0
    wait "$vite_pid" || vite_status=$?
    vite_pid=
    echo "dev: Vite exited with status $vite_status" >&2
    exit "$vite_status"
  fi
  sleep 0.25
done
