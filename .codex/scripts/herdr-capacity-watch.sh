#!/usr/bin/env bash
# Watch Herdr Codex panes for model-capacity stalls and unstick them.
#
# On capacity: if the pane chrome shows an active/paused goal
# ("Pursuing goal" / "Goal paused"), send "/goal resume"; otherwise send
# "continue".
#
# Usage (leave running in an open terminal tab; Herdr server must be up):
#   .codex/scripts/herdr-capacity-watch.sh
#
# Env overrides:
#   INTERVAL_SECS   poll interval (default 300)
#   COOLDOWN_SECS   per-pane action cooldown (default 900)
#   READ_LINES      pane read line window (default 120)
#   MATCH_TEXT      capacity message to detect
#   RESUME_CMD      command when a goal is active/paused (default "/goal resume")
#   CONTINUE_CMD    command when no goal chrome is present (default "continue")
#   HERDR_BIN       herdr binary path (default: herdr on PATH)
#   STATE_DIR       cooldown state directory

set -euo pipefail

INTERVAL_SECS="${INTERVAL_SECS:-300}"
COOLDOWN_SECS="${COOLDOWN_SECS:-900}"
READ_LINES="${READ_LINES:-120}"
MATCH_TEXT="${MATCH_TEXT:-Selected model is at capacity. Please try a different model.}"
RESUME_CMD="${RESUME_CMD:-/goal resume}"
CONTINUE_CMD="${CONTINUE_CMD:-continue}"
HERDR_BIN="${HERDR_BIN:-herdr}"
STATE_DIR="${STATE_DIR:-${TMPDIR:-/tmp}/herdr-capacity-watch}"

log() {
  printf '%s %s\n' "$(date -u +'%Y-%m-%dT%H:%M:%SZ')" "$*"
}

cleanup() {
  log "stopped"
  exit 0
}
trap cleanup INT TERM

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    log "error: required command not found: $1"
    exit 1
  fi
}

cooldown_path() {
  local pane_id="$1"
  local key
  # bash 3.2 / macOS: no associative arrays; encode pane ids for filenames
  key="$(printf '%s' "$pane_id" | python3 -c 'import sys,hashlib; print(hashlib.sha1(sys.stdin.buffer.read()).hexdigest())')"
  printf '%s/%s\n' "$STATE_DIR" "$key"
}

server_running() {
  local out
  out="$("$HERDR_BIN" status 2>&1 || true)"
  printf '%s\n' "$out" | grep -q 'status: running'
}

codex_pane_ids() {
  "$HERDR_BIN" agent list 2>/dev/null | python3 -c '
import json, sys
try:
    payload = json.load(sys.stdin)
except json.JSONDecodeError as exc:
    print(f"json decode failed: {exc}", file=sys.stderr)
    sys.exit(1)
agents = payload.get("result", {}).get("agents", [])
for agent in agents:
    if agent.get("agent") != "codex":
        continue
    pane_id = agent.get("pane_id")
    if pane_id:
        print(pane_id)
'
}

# Returns 0 when the visible Codex status chrome shows an active or paused goal.
# Prefer --source visible: recent scrollback can contain transcript lines like
# "• Goal paused Objective: ..." that are not the corner chrome.
has_active_goal() {
  local pane_id="$1"
  local visible
  if ! visible="$("$HERDR_BIN" pane read "$pane_id" --source visible --format text 2>/dev/null)"; then
    return 1
  fi
  # Status chrome lives on the bottom status line:
  #   Pursuing goal (1h 27m)
  #   Goal paused (/goal resume)
  printf '%s' "$visible" | python3 -c '
import sys
lines = [ln.rstrip() for ln in sys.stdin.read().splitlines() if ln.strip()]
tail = "\n".join(lines[-4:]) if lines else ""
if "Pursuing goal" in tail or "Goal paused (/goal resume)" in tail:
    raise SystemExit(0)
raise SystemExit(1)
'
}

pick_unstick_cmd() {
  local pane_id="$1"
  if has_active_goal "$pane_id"; then
    printf '%s\n' "$RESUME_CMD"
  else
    printf '%s\n' "$CONTINUE_CMD"
  fi
}

in_cooldown() {
  local pane_id="$1"
  local path last now
  path="$(cooldown_path "$pane_id")"
  if [[ ! -f "$path" ]]; then
    return 1
  fi
  last="$(cat "$path" 2>/dev/null || echo 0)"
  case "$last" in
    ''|*[!0-9]*) last=0 ;;
  esac
  now="$(date +%s)"
  if (( now - last < COOLDOWN_SECS )); then
    return 0
  fi
  return 1
}

mark_action() {
  local pane_id="$1"
  local path
  path="$(cooldown_path "$pane_id")"
  mkdir -p "$STATE_DIR"
  date +%s >"$path"
}

scan_once() {
  local pane_id screen cmd

  if ! server_running; then
    log "herdr server not running; sleeping ${INTERVAL_SECS}s"
    return 0
  fi

  local panes=""
  local list_out
  if ! list_out="$(codex_pane_ids)"; then
    log "error: failed to list codex agents"
    return 0
  fi

  local count=0
  while IFS= read -r pane_id; do
    [[ -n "$pane_id" ]] || continue
    if [[ -n "$panes" ]]; then
      panes="$panes $pane_id"
    else
      panes="$pane_id"
    fi
    count=$((count + 1))
  done <<<"$list_out"

  if (( count == 0 )); then
    log "no codex panes; sleeping ${INTERVAL_SECS}s"
    return 0
  fi

  log "scan ${count} codex pane(s): ${panes}"

  for pane_id in $panes; do
    if ! screen="$("$HERDR_BIN" pane read "$pane_id" --source recent --lines "$READ_LINES" --format text 2>/dev/null)"; then
      log "error: pane read failed pane=${pane_id}"
      continue
    fi

    if [[ "$screen" != *"$MATCH_TEXT"* ]]; then
      continue
    fi

    if in_cooldown "$pane_id"; then
      log "capacity hit pane=${pane_id} (cooldown active, skip)"
      continue
    fi

    cmd="$(pick_unstick_cmd "$pane_id")"
    if [[ "$cmd" == "$RESUME_CMD" ]]; then
      log "capacity hit pane=${pane_id} goal=active; sending: ${cmd}"
    else
      log "capacity hit pane=${pane_id} goal=none; sending: ${cmd}"
    fi

    if "$HERDR_BIN" pane run "$pane_id" "$cmd"; then
      mark_action "$pane_id"
      log "unstuck pane=${pane_id} cmd=$(printf '%q' "$cmd")"
    else
      log "error: pane run failed pane=${pane_id} cmd=$(printf '%q' "$cmd")"
    fi
  done
}

require_cmd "$HERDR_BIN"
require_cmd python3
mkdir -p "$STATE_DIR"

log "herdr capacity watch started interval=${INTERVAL_SECS}s cooldown=${COOLDOWN_SECS}s resume=$(printf '%q' "$RESUME_CMD") continue=$(printf '%q' "$CONTINUE_CMD")"

while true; do
  scan_once || true
  sleep "$INTERVAL_SECS"
done
