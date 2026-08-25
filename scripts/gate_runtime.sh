#!/usr/bin/env bash
# Machine-capacity and content-fingerprint support for gate.sh.
# Sourced after gate.sh defines log, die, and GATE_SLOT.

gate_slot_root() {
  if [ -n "${COMPOZY_GATE_SLOT_DIR:-}" ]; then
    printf '%s\n' "$COMPOZY_GATE_SLOT_DIR"
  elif [ "$(uname -s)" = "Darwin" ]; then
    printf '%s/Library/Caches/compozy-dev/gate-slots\n' "$HOME"
  else
    printf '%s/compozy-dev/gate-slots\n' "${XDG_CACHE_HOME:-$HOME/.cache}"
  fi
}

release_gate_slot() {
  [ -n "$GATE_SLOT" ] || return 0
  if [ "$(sed -n '1p' "$GATE_SLOT/pid" 2>/dev/null || true)" = "$$" ]; then
    rm -f "$GATE_SLOT/pid"
    rmdir "$GATE_SLOT" 2>/dev/null || true
  fi
  GATE_SLOT=""
}

acquire_gate_slot() {
  local maximum="${COMPOZY_GATE_MAX_CONCURRENT:-2}" root slot owner waited=0
  case "$maximum" in
    '' | *[!0-9]* | 0) die "COMPOZY_GATE_MAX_CONCURRENT must be a positive integer" ;;
  esac
  root="$(gate_slot_root)"
  mkdir -p "$root"
  while :; do
    slot=1
    while [ "$slot" -le "$maximum" ]; do
      GATE_SLOT="$root/slot-$slot"
      if mkdir "$GATE_SLOT" 2>/dev/null; then
        printf '%s\n' "$$" >"$GATE_SLOT/pid"
        printf '%s\n' "$(pwd)" >"$GATE_SLOT/worktree"
        trap release_gate_slot EXIT
        trap 'exit 129' HUP
        trap 'exit 130' INT
        trap 'exit 143' TERM
        log "capacity slot $slot/$maximum acquired"
        return 0
      fi
      owner="$(sed -n '1p' "$GATE_SLOT/pid" 2>/dev/null || true)"
      if [ -z "$owner" ] || ! kill -0 "$owner" 2>/dev/null; then
        rm -f "$GATE_SLOT/pid" "$GATE_SLOT/worktree"
        rmdir "$GATE_SLOT" 2>/dev/null || true
      fi
      slot=$((slot + 1))
    done
    GATE_SLOT=""
    if [ $((waited % 10)) -eq 0 ]; then
      log "affected lanes are waiting for one of $maximum machine capacity slots"
    fi
    sleep 1
    waited=$((waited + 1))
  done
}

tree_fingerprint() (
  local temp_index temp_paths
  temp_index="$(mktemp "${TMPDIR:-/tmp}/compozy-gate-index.XXXXXX")"
  temp_paths="$(mktemp "${TMPDIR:-/tmp}/compozy-gate-paths.XXXXXX")"
  trap 'rm -f "$temp_index" "$temp_paths"' EXIT
  rm -f "$temp_index"
  if git rev-parse --verify --quiet HEAD >/dev/null 2>&1; then
    GIT_INDEX_FILE="$temp_index" git read-tree HEAD
    git diff --name-only -z HEAD -- >"$temp_paths"
  else
    GIT_INDEX_FILE="$temp_index" git read-tree --empty
    : >"$temp_paths"
  fi
  git ls-files -o -z --exclude-standard >>"$temp_paths"
  if [ -s "$temp_paths" ]; then
    GIT_INDEX_FILE="$temp_index" git add -A \
      --pathspec-from-file="$temp_paths" --pathspec-file-nul
  fi
  GIT_INDEX_FILE="$temp_index" git write-tree
)
