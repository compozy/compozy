#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

# shellcheck source=scripts/dev-common.sh
source "$repo_root/scripts/dev-common.sh"

if [[ -z ${COMPOZY_MARKETPLACE_CATALOG_BASE_URL:-} ]]; then
  export COMPOZY_MARKETPLACE_CATALOG_BASE_URL="file://${repo_root}/catalog"
fi

: "${AIR_VERSION:?AIR_VERSION must be set by the development command}"

air_pid=
app_pid=
readiness_dir=
readiness_fifo=
readiness_marker=
readiness_fd_open=0
compozy_home=${COMPOZY_HOME:-"$HOME/.compozy"}
export COMPOZY_HOME=$compozy_home
air_state_dir=$(go run ./scripts/air-state-dir)
air_build_dir="$repo_root/.tmp/air"
dev_run_id="desktop-dev-$$-$(date +%s)"
web_dist_dir="$repo_root/web/dist"
export COMPOZY_AIR_BUILD_DIR=$air_build_dir
export COMPOZY_AIR_STATE_DIR=$air_state_dir
export COMPOZY_AIR_DEV_RUN_ID=$dev_run_id

port_listening() {
  local host=$1
  local port=$2

  (exec 3<> "/dev/tcp/$host/$port") 2>/dev/null
}

home_daemon_recorded_on_port() {
  local port=$1
  local info_file="$compozy_home/daemon.json"

  [[ -f "$info_file" ]] || return 1
  local recorded
  recorded=$(bun -e '
    const info = JSON.parse(await Bun.stdin.text());
    if (!Number.isInteger(info.pid) || info.pid <= 0 || !Number.isInteger(info.port)) {
      throw new Error("daemon.json does not contain a valid pid and port");
    }
    process.stdout.write(`${info.pid}\t${info.port}`);
  ' < "$info_file") || return 1
  local pid=${recorded%%$'\t'*}
  local recorded_port=${recorded##*$'\t'}
  [[ "$recorded_port" == "$port" ]] || return 1
  kill -0 "$pid" 2>/dev/null
}

built_desktop_shell_binary() {
  bun run --cwd desktop build:runtime >&2
  bun run --cwd desktop build:main >&2
  printf '%s' "$repo_root/desktop/node_modules/.bin/electron"
}

cleanup() {
  local exit_status=$?
  trap - EXIT INT TERM

  terminate_process "$app_pid" "the desktop shell"
  terminate_process "$air_pid" "Air"
  wait_for_process "$app_pid" "the desktop shell"
  wait_for_process "$air_pid" "Air"
  if ! remove_dev_readiness && ((exit_status == 0)); then
    exit_status=1
  fi
  if ! stop_owned_daemon "$air_state_dir" "$dev_run_id" "$air_build_dir/compozy" && ((exit_status == 0)); then
    exit_status=1
  fi

  exit "$exit_status"
}

trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

if [[ ! -f "$web_dist_dir/index.html" ]]; then
  echo "desktop-dev: web bundle missing at $web_dist_dir — run 'make web-build' first" >&2
  exit 1
fi
export COMPOZY_WEB_DIST_DIR=$web_dist_dir

if ! endpoint=$(go run ./scripts/dev-http-endpoint); then
  echo "desktop-dev: failed to resolve the daemon HTTP endpoint from the active config" >&2
  exit 1
fi
http_host=${endpoint%%$'\t'*}
http_port=${endpoint##*$'\t'}

if port_listening "$http_host" "$http_port" && ! home_daemon_recorded_on_port "$http_port"; then
  echo "desktop-dev: another process is already listening on $http_host:$http_port and it is not the daemon recorded in $compozy_home/daemon.json" >&2
  echo "desktop-dev: inspect it with: lsof -iTCP:$http_port -sTCP:LISTEN" >&2
  echo "desktop-dev: stop that daemon (for example another checkout's 'make dev') or set a different [http] port in $compozy_home/config.toml" >&2
  exit 1
fi

echo "desktop-dev: building the desktop shell"
app_binary=$(built_desktop_shell_binary)
if [[ ! -x "$app_binary" ]]; then
  echo "desktop-dev: desktop shell binary is not executable: $app_binary" >&2
  exit 1
fi

dev_tmp_parent="$repo_root/.tmp"
mkdir -p "$dev_tmp_parent"
if ! readiness_dir=$(mktemp -d "$dev_tmp_parent/dev-readiness.XXXXXX"); then
  echo "desktop-dev: failed to allocate the readiness directory" >&2
  exit 1
fi
readiness_fifo="$readiness_dir/events.fifo"
readiness_marker="$readiness_dir/ready-sent"
if ! mkfifo "$readiness_fifo"; then
  echo "desktop-dev: failed to create the readiness channel $readiness_fifo" >&2
  exit 1
fi
if ! exec 9<> "$readiness_fifo"; then
  echo "desktop-dev: failed to open the readiness channel $readiness_fifo" >&2
  exit 1
fi
readiness_fd_open=1
export COMPOZY_AIR_READY_FIFO=$readiness_fifo
export COMPOZY_AIR_READY_MARKER=$readiness_marker

echo "desktop-dev: COMPOZY_HOME is $compozy_home"
echo "desktop-dev: daemon web routes serve the built web bundle at $web_dist_dir"

bash scripts/run-air-with-events.sh "$readiness_fifo" "$dev_run_id" "$AIR_VERSION" -c .air.toml &
air_pid=$!

ready_binary_id=
ready_status=0
ready_binary_id=$(bash scripts/dev-readiness.sh await "$readiness_fifo" "$dev_run_id") || ready_status=$?
if ((ready_status != 0)); then
  echo "desktop-dev: daemon did not become ready for the current Air run" >&2
  exit "$ready_status"
fi
echo "desktop-dev: daemon ready for current Air build ($ready_binary_id)"

COMPOZY_DESKTOP_BUNDLE_ROOT="$repo_root/desktop/.artifacts/runtime" \
  "$app_binary" "$repo_root/desktop" &
app_pid=$!
echo "desktop-dev: desktop shell launched (quit the app or press Ctrl-C to stop the stack)"

while :; do
  if ! kill -0 "$air_pid" 2>/dev/null; then
    air_status=0
    wait "$air_pid" || air_status=$?
    air_pid=
    echo "desktop-dev: Air exited with status $air_status" >&2
    exit "$air_status"
  fi
  if ! kill -0 "$app_pid" 2>/dev/null; then
    app_status=0
    wait "$app_pid" || app_status=$?
    app_pid=
    if ((app_status == 0)); then
      echo "desktop-dev: desktop shell exited; stopping the development stack"
    else
      echo "desktop-dev: desktop shell exited with status $app_status" >&2
    fi
    exit "$app_status"
  fi
  sleep 0.25
done
