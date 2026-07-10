#!/usr/bin/env bash
# dispatch-slices.sh — run N explorer slices in parallel via `compozy exec`,
# wait for all, and report per-slice exit codes. Bundled with the
# agent-exploration skill. Zero external dependencies (native bash + the
# `compozy` binary).
#
# Usage:
#   dispatch-slices.sh \
#     --ide <ide> --model <model> --reasoning <effort> \
#     [--logs <dir>] [--bin <path>] \
#     -- <prompt-file> [<prompt-file>...]
#
# Required:
#   --ide        Compozy runtime (e.g. claude, codex, cursor-agent, ...).
#   --model      Model name (e.g. opus, 'grok-4.5[effort=high,fast=true]').
#   --reasoning  Reasoning effort: low | medium | high | xhigh.
#
# Optional:
#   --logs       Directory for per-slice .out/.err/.exit files. Default ./dispatch-logs.
#   --bin        Path to the compozy binary. Defaults to $COMPOZY_BIN or `compozy` on PATH.
#
# Positional (after --): 1 to 8 prompt files. Each file's basename
# (without .txt/.md) becomes the slice id used for log file naming.
#
# Behaviour:
#   - Each prompt is dispatched via:
#       $BIN exec --agent explorer --ide ... --model ... --reasoning-effort ... --prompt-file <file>
#   - All slices run in parallel via shell job control (& + wait $pid).
#   - Per-slice stdout/stderr/exit are captured under --logs.
#   - The script blocks until every slice has exited, then prints a summary.
#   - Exits 0 only when every slice exited 0; otherwise exits 1.
#   - SIGINT/SIGTERM kills running child processes before exiting.
set -uo pipefail

IDE=""
MODEL=""
REASONING=""
LOGS_DIR="./dispatch-logs"
BIN="${COMPOZY_BIN:-compozy}"

print_help() {
  sed -n '2,32p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

while [ $# -gt 0 ]; do
  case "$1" in
    --ide)        IDE="${2:-}"; shift 2 ;;
    --ide=*)      IDE="${1#--ide=}"; shift ;;
    --model)      MODEL="${2:-}"; shift 2 ;;
    --model=*)    MODEL="${1#--model=}"; shift ;;
    --reasoning)  REASONING="${2:-}"; shift 2 ;;
    --reasoning=*) REASONING="${1#--reasoning=}"; shift ;;
    --logs)       LOGS_DIR="${2:-}"; shift 2 ;;
    --logs=*)     LOGS_DIR="${1#--logs=}"; shift ;;
    --bin)        BIN="${2:-}"; shift 2 ;;
    --bin=*)      BIN="${1#--bin=}"; shift ;;
    --)           shift; break ;;
    -h|--help)    print_help; exit 0 ;;
    *)            echo "ERROR: unknown argument: $1" >&2; exit 2 ;;
  esac
done

if [ -z "$IDE" ] || [ -z "$MODEL" ] || [ -z "$REASONING" ]; then
  echo "ERROR: --ide, --model, --reasoning are required" >&2
  exit 2
fi

if [ "$#" -lt 1 ] || [ "$#" -gt 8 ]; then
  echo "ERROR: pass between 1 and 8 prompt files after --" >&2
  exit 2
fi

if ! command -v "$BIN" >/dev/null 2>&1 && [ ! -x "$BIN" ]; then
  echo "ERROR: compozy binary not found at '$BIN' (override via --bin or \$COMPOZY_BIN)" >&2
  exit 2
fi

mkdir -p "$LOGS_DIR"

PIDS=()
SLUGS=()

cleanup() {
  if [ "${#PIDS[@]}" -gt 0 ]; then
    echo "interrupted; killing ${#PIDS[@]} child process(es)..." >&2
    kill "${PIDS[@]}" 2>/dev/null || true
  fi
  exit 130
}
trap cleanup INT TERM

START_TS=$(date +%s)
echo "dispatch: $# slice(s) via $BIN exec --agent explorer --ide $IDE --model $MODEL --reasoning-effort $REASONING"
echo "logs:     $LOGS_DIR"

for PROMPT_FILE in "$@"; do
  if [ ! -f "$PROMPT_FILE" ]; then
    echo "ERROR: prompt file not found: $PROMPT_FILE" >&2
    exit 2
  fi
  SLUG="$(basename "$PROMPT_FILE")"
  SLUG="${SLUG%.txt}"
  SLUG="${SLUG%.md}"
  SLUGS+=("$SLUG")

  OUT="$LOGS_DIR/$SLUG.out"
  ERR="$LOGS_DIR/$SLUG.err"
  rm -f "$LOGS_DIR/$SLUG.exit"

  COMPOZY_NO_UPDATE_NOTIFIER=1 "$BIN" exec \
    --agent explorer \
    --ide "$IDE" \
    --model "$MODEL" \
    --reasoning-effort "$REASONING" \
    --prompt-file "$PROMPT_FILE" \
    >"$OUT" 2>"$ERR" &

  PID=$!
  PIDS+=("$PID")
  echo "  dispatched: $SLUG  pid=$PID"
done

FAILED=0
TOTAL=${#PIDS[@]}
for i in "${!PIDS[@]}"; do
  PID="${PIDS[$i]}"
  SLUG="${SLUGS[$i]}"
  if wait "$PID"; then
    EXIT_CODE=0
  else
    EXIT_CODE=$?
    FAILED=$((FAILED + 1))
  fi
  echo "$EXIT_CODE" > "$LOGS_DIR/$SLUG.exit"
  echo "  exited:     $SLUG  rc=$EXIT_CODE"
done

ELAPSED=$(( $(date +%s) - START_TS ))
echo "summary: total=${ELAPSED}s  ok=$((TOTAL - FAILED))/${TOTAL}  failed=${FAILED}/${TOTAL}"

# Clear trap so a clean exit does not trigger cleanup.
trap - INT TERM

if [ "$FAILED" -gt 0 ]; then
  exit 1
fi
exit 0
