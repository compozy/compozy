#!/usr/bin/env bash
set -euo pipefail

platform=${1:?usage: smoke-desktop-release-artifact.sh <macos|linux> <artifact> <version> <runtime-version>}
artifact=${2:?release artifact is required}
release_version=${3:?release version is required}
runtime_version=${4:?current channel runtime version is required}

[[ -f "${artifact}" ]] || {
  echo "desktop release smoke: artifact does not exist: ${artifact}" >&2
  exit 1
}

smoke_parent=${RUNNER_TEMP:-${TMPDIR:-/tmp}}
smoke_home=$(mktemp -d "${smoke_parent}/compozy-desktop-smoke-home.XXXXXX")
mount_path=
app_pid=
app_process_group=false

stop_process() {
  local pid=$1
  local signal_target=$pid
  if [[ "${app_process_group}" == "true" && "${pid}" == "${app_pid}" ]]; then
    signal_target=-${pid}
  fi
  if ! kill -0 "${pid}" 2>/dev/null; then
    return 0
  fi
  kill -TERM -- "${signal_target}" 2>/dev/null || true
  for _ in {1..10}; do
    if ! kill -0 "${pid}" 2>/dev/null; then
      return 0
    fi
    sleep 1
  done
  kill -KILL -- "${signal_target}" 2>/dev/null || true
  for _ in {1..5}; do
    if ! kill -0 "${pid}" 2>/dev/null; then
      return 0
    fi
    sleep 1
  done
  echo "desktop release smoke: could not stop process ${pid}" >&2
  return 1
}

cleanup() {
  local status=$?
  trap - EXIT INT TERM

  if [[ -n "${app_pid}" ]] && kill -0 "${app_pid}" 2>/dev/null; then
    stop_process "${app_pid}" || status=1
    wait "${app_pid}" 2>/dev/null || true
  fi
  if [[ -f "${smoke_home}/daemon.json" ]]; then
    local daemon_pid
    daemon_pid=$(jq -er '.pid | select(type == "number" and . > 0)' "${smoke_home}/daemon.json" 2>/dev/null || true)
    if [[ -n "${daemon_pid}" ]]; then
      app_process_group=false
      stop_process "${daemon_pid}" || status=1
    fi
  fi
  if [[ -n "${mount_path}" ]]; then
    hdiutil detach "${mount_path}" -force >/dev/null 2>&1 || true
    rm -rf "${mount_path}"
  fi
  rm -rf "${smoke_home}"
  exit "${status}"
}
trap cleanup EXIT INT TERM

if (
  PATH=/usr/bin:/bin:/usr/sbin:/sbin
  export PATH
  command -v compozy >/dev/null 2>&1
); then
  echo "desktop release smoke: restricted PATH unexpectedly contains compozy" >&2
  exit 1
fi

case "${platform}" in
  macos)
    operator_probe_paths=(/opt/homebrew/bin/compozy /usr/local/bin/compozy)
    ;;
  linux)
    operator_probe_paths=(/usr/local/bin/compozy /usr/bin/compozy)
    ;;
  *)
    echo "desktop release smoke: unsupported platform ${platform}" >&2
    exit 2
    ;;
esac
for operator_binary in "${operator_probe_paths[@]}"; do
  if [[ -x "${operator_binary}" ]]; then
    echo "desktop release smoke: operator runtime is present at ${operator_binary}" >&2
    exit 1
  fi
done

case "${platform}" in
  macos)
    mount_path=$(mktemp -d "${smoke_parent}/compozy-desktop-smoke-mount.XXXXXX")
    hdiutil attach -nobrowse -readonly -mountpoint "${mount_path}" "${artifact}" >/dev/null
    app_path=$(find "${mount_path}" -maxdepth 1 -type d -name '*.app' -print -quit)
    app_binary="${app_path}/Contents/MacOS/compozyos-desktop"
    ;;
  linux)
    app_binary=${artifact}
    ;;
esac

[[ -x "${app_binary}" ]] || {
  echo "desktop release smoke: packaged app executable is missing: ${app_binary}" >&2
  exit 1
}
[[ ! -e "${smoke_home}/bin/compozy" ]] || {
  echo "desktop release smoke: isolated home is not empty" >&2
  exit 1
}

case "${platform}" in
  macos)
    COMPOZY_HOME="${smoke_home}" HOME="${smoke_home}" PATH=/usr/bin:/bin:/usr/sbin:/sbin \
      "${app_binary}" >"${smoke_home}/desktop.log" 2>&1 &
    ;;
  linux)
    app_process_group=true
    setsid env COMPOZY_HOME="${smoke_home}" HOME="${smoke_home}" PATH=/usr/bin:/bin:/usr/sbin:/sbin \
      APPIMAGE_EXTRACT_AND_RUN=1 xvfb-run --auto-servernum "${app_binary}" \
      >"${smoke_home}/desktop.log" 2>&1 &
    ;;
esac
app_pid=$!

deadline=$((SECONDS + 180))
while ((SECONDS < deadline)); do
  if [[ -x "${smoke_home}/bin/compozy" && -s "${smoke_home}/daemon.json" ]]; then
    daemon_pid=$(jq -er '.pid | select(type == "number" and . > 0)' "${smoke_home}/daemon.json" 2>/dev/null || true)
    daemon_port=$(jq -er '.port | select(type == "number" and . > 0)' "${smoke_home}/daemon.json" 2>/dev/null || true)
    if [[ -n "${daemon_pid}" && -n "${daemon_port}" ]]; then
      status=$(curl --fail --silent --show-error "http://127.0.0.1:${daemon_port}/api/status" || true)
      if [[ -n "${status}" ]] \
        && jq -e \
          --arg home "${smoke_home}" \
          --arg runtime_version "${runtime_version}" \
          --argjson pid "${daemon_pid}" \
          '.daemon.pid == $pid
            and .daemon.user_home_dir == $home
            and (.daemon.version | ltrimstr("v")) == $runtime_version' \
          <<<"${status}" >/dev/null \
        && jq -e \
          --arg release_version "${release_version}" \
          '.app_version == $release_version
            and .diagnostic_report.app_version == $release_version' \
          "${smoke_home}/app.json" >/dev/null \
        && jq -e \
          --arg channel "${RELEASE_CHANNEL:?RELEASE_CHANNEL is required}" \
          --arg release_version "${release_version}" \
          --arg runtime_version "${runtime_version}" \
          '.installed_by == "desktop-app"
            and .app_version == $release_version
            and .channel == $channel
            and .runtime_version == $runtime_version' \
          "${smoke_home}/bin/.desktop-provenance.json" >/dev/null; then
        echo "desktop release smoke: PASS (${platform}, ${release_version}, runtime ${runtime_version})"
        exit 0
      fi
    fi
  fi
  if ! kill -0 "${app_pid}" 2>/dev/null; then
    wait "${app_pid}" || true
    echo "desktop release smoke: packaged app exited before provisioning completed" >&2
    sed -n '1,200p' "${smoke_home}/desktop.log" >&2 || true
    exit 1
  fi
  sleep 1
done

echo "desktop release smoke: timed out waiting for the packaged app to provision and start its runtime" >&2
sed -n '1,200p' "${smoke_home}/desktop.log" >&2 || true
exit 1
