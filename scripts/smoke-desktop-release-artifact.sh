#!/usr/bin/env bash
set -euo pipefail

platform=${1:?usage: smoke-desktop-release-artifact.sh <macos|linux> <artifact> <version> <runtime-version> [mount|extract]}
artifact=${2:?release artifact is required}
release_version=${3:?release version is required}
runtime_version=${4:?current channel runtime version is required}
linux_launch_mode=${5:-mount}

if [[ "${platform}" == "linux" \
  && "${linux_launch_mode}" != "mount" \
  && "${linux_launch_mode}" != "extract" ]]; then
  echo "desktop release smoke: unsupported Linux launch mode ${linux_launch_mode}" >&2
  exit 2
fi

smoke_parent=${RUNNER_TEMP:-${TMPDIR:-/tmp}}
# Keep COMPOZY_HOME short enough for macOS sockaddr_un after /var resolves to /private/var.
smoke_root=$(mktemp -d "${smoke_parent}/compozyds.XXXXXX")
smoke_home="${smoke_root}/home/runtime"
qa_output="${smoke_root}/qa-artifacts"
qa_root="${qa_output}/qa"
manifest_path="${qa_root}/bootstrap-manifest.json"
script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(git -C "${GITHUB_WORKSPACE:-$(pwd)}" rev-parse --show-toplevel)
bash "${script_dir}/prepare-desktop-smoke-artifact.sh" "${platform}" "${artifact}"
mkdir -p "${smoke_home}" "${qa_root}/pids"
smoke_home=$(cd "${smoke_home}" && pwd -P)
http_port=$(python3 -c 'import socket; listener = socket.socket(); listener.bind(("127.0.0.1", 0)); print(listener.getsockname()[1]); listener.close()')
uds_path="${smoke_home}/compozyd.sock"
{
  printf '[http]\n'
  printf 'host = "127.0.0.1"\n'
  printf 'port = %s\n\n' "${http_port}"
  printf '[daemon]\n'
  printf 'socket = "%s"\n' "${uds_path}"
} >"${smoke_home}/config.toml"
jq -n \
  --arg qa_output "${qa_output}" \
  --arg compozy_home "${smoke_home}" \
  --arg http_port "${http_port}" \
  --arg uds_path "${uds_path}" \
  '{
    qa_output_path: $qa_output,
    env: {
      QA_OUTPUT_PATH: $qa_output,
      COMPOZY_HOME: $compozy_home,
      COMPOZY_HTTP_PORT: $http_port,
      COMPOZY_UDS_PATH: $uds_path
    }
  }' >"${manifest_path}"
teardown_command=(
  python3
  "${repo_root}/.agents/skills/eng/eng-qa-bootstrap/scripts/teardown-qa-env.py"
  --manifest "${manifest_path}"
  --repo-root "${repo_root}"
)
mount_path=
app_pid=
app_signal_target=

emit_desktop_diagnostics() {
  echo "desktop release smoke: diagnostics root ${smoke_root}" >&2
  for diagnostic_file in \
    "${smoke_home}/app.json" \
    "${smoke_home}/logs/desktop.log" \
    "${smoke_home}/desktop.log"; do
    if [[ -f "${diagnostic_file}" ]]; then
      echo "desktop release smoke: ${diagnostic_file}" >&2
      sed -n '1,240p' "${diagnostic_file}" >&2
    fi
  done
}

stop_desktop_child() {
  [[ -n "${app_pid}" ]] || return 0
  if ! jobs -pr | grep -Fxq "${app_pid}"; then
    wait "${app_pid}" 2>/dev/null || true
    return 0
  fi
  kill -TERM -- "${app_signal_target}" 2>/dev/null || true
  for _ in {1..10}; do
    if ! jobs -pr | grep -Fxq "${app_pid}"; then
      wait "${app_pid}" 2>/dev/null || true
      return 0
    fi
    sleep 1
  done
  kill -KILL -- "${app_signal_target}" 2>/dev/null || true
  wait "${app_pid}" 2>/dev/null || true
}

quarantine_unverified_daemon_record() {
  local reason=$1
  echo "desktop release smoke: refusing to signal an unverified daemon: ${reason}" >&2
  for record in daemon.json daemon.lock; do
    if [[ -e "${smoke_home}/${record}" ]]; then
      mv "${smoke_home}/${record}" "${qa_root}/${record}.unverified"
    fi
  done
}

validate_daemon_identity_for_teardown() {
  local record="${smoke_home}/daemon.json"
  [[ -f "${record}" ]] || return 0
  local daemon_pid recorded_start
  daemon_pid=$(jq -er '.pid | select(type == "number" and . > 0)' "${record}") || {
    quarantine_unverified_daemon_record "daemon.json has no valid pid"
    return 1
  }
  recorded_start=$(jq -er '.started_at | select(type == "string" and length > 0)' "${record}") || {
    quarantine_unverified_daemon_record "daemon.json has no process start time"
    return 1
  }
  if ! python3 "${script_dir}/verify-process-start.py" "${daemon_pid}" "${recorded_start}"; then
    quarantine_unverified_daemon_record "pid/start-time identity does not match the live process"
    return 1
  fi
}

cleanup() {
  local status=$?
  trap - EXIT INT TERM

  validate_daemon_identity_for_teardown || status=1
  stop_desktop_child
  if ! "${teardown_command[@]}"; then
    status=1
  fi
  if ! jq -e '.clean == true' "${qa_root}/teardown.json" >/dev/null 2>&1; then
    echo "desktop release smoke: teardown evidence is missing or not clean" >&2
    status=1
  fi
  if [[ -n "${mount_path}" ]]; then
    hdiutil detach "${mount_path}" -force >/dev/null 2>&1 || true
    rm -rf "${mount_path}"
  fi
  echo "desktop release smoke: teardown evidence ${qa_root}/teardown.json"
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
    app_binary="${app_path}/Contents/MacOS/CompozyOS"
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
    if [[ "${linux_launch_mode}" == "mount" ]]; then
      command -v ldconfig >/dev/null || {
        echo "desktop release smoke: ldconfig is required to verify libfuse2 absence" >&2
        exit 1
      }
      libfuse_cache=$(ldconfig -p)
      if grep 'libfuse\.so\.2' <<<"${libfuse_cache}" >/dev/null; then
        echo "desktop release smoke: mount verification requires a host without libfuse2" >&2
        exit 1
      fi
      setsid env -u APPIMAGE_EXTRACT_AND_RUN \
        COMPOZY_HOME="${smoke_home}" HOME="${smoke_home}" PATH=/usr/bin:/bin:/usr/sbin:/sbin \
        xvfb-run --auto-servernum "${app_binary}" >"${smoke_home}/desktop.log" 2>&1 &
    else
      setsid env COMPOZY_HOME="${smoke_home}" HOME="${smoke_home}" PATH=/usr/bin:/bin:/usr/sbin:/sbin \
        APPIMAGE_EXTRACT_AND_RUN=1 xvfb-run --auto-servernum "${app_binary}" \
        >"${smoke_home}/desktop.log" 2>&1 &
    fi
    ;;
esac
app_pid=$!
app_signal_target=${app_pid}
if [[ "${platform}" == "linux" ]]; then
  app_signal_target=-${app_pid}
fi
printf '%s\n' "${app_pid}" >"${qa_root}/pids/desktop.pid"

deadline=$((SECONDS + 180))
while ((SECONDS < deadline)); do
  if [[ -x "${smoke_home}/bin/compozy" && -s "${smoke_home}/daemon.json" ]]; then
    daemon_pid=$(jq -er '.pid | select(type == "number" and . > 0)' "${smoke_home}/daemon.json" 2>/dev/null || true)
    daemon_port=$(jq -er '.port | select(type == "number" and . > 0)' "${smoke_home}/daemon.json" 2>/dev/null || true)
    if [[ -n "${daemon_pid}" && -n "${daemon_port}" ]]; then
      status=$(curl --connect-timeout 2 --max-time 5 --fail --silent --show-error \
        "http://127.0.0.1:${daemon_port}/api/status" || true)
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
    emit_desktop_diagnostics
    exit 1
  fi
  if [[ -s "${smoke_home}/app.json" ]] \
    && jq -e '.state == "error" or .diagnostic_report.boot_phase == "error"' \
      "${smoke_home}/app.json" >/dev/null 2>&1; then
    echo "desktop release smoke: packaged app reported a terminal startup error" >&2
    emit_desktop_diagnostics
    exit 1
  fi
  sleep 1
done

echo "desktop release smoke: timed out waiting for the packaged app to provision and start its runtime" >&2
emit_desktop_diagnostics
exit 1
