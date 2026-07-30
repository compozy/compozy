#!/usr/bin/env bash
# Evidence-cached verification gate.
#
#   auto         classify the diff vs merge-base and run only affected lanes;
#                sensitive paths escalate to the full gate
#   full         the full `make verify` (machine-wide lock lives in mage)
#   plan         print the classification and commands without running anything
#   status       print evidence records vs the current tree fingerprint
#   fingerprint  print the current tree fingerprint
#
# Records live in .cache/gate/<lane>.json keyed by a content fingerprint
# (HEAD tree + index tree + hashes of modified/untracked files): commits and
# rebases that keep content identical keep records valid; any edit goes stale.
# Target bash 3.2 (stock macOS) — no associative arrays, no mapfile.

set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

GATE_DIR=".cache/gate"
LOG_DIR="$GATE_DIR/logs"

CURRENT_FINGERPRINT=""
FULL_REASONS=""
GO_SCOPES=""
JS_FILTERS=""
NO_LANE_COUNT=0
CHANGED_COUNT=0

log() { printf '[gate] %s\n' "$*"; }
die() {
  printf '[gate] error: %s\n' "$*" >&2
  exit 2
}

hash_stdin() {
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 | cut -d' ' -f1
  else
    sha256sum | cut -d' ' -f1
  fi
}

tree_fingerprint() {
  {
    git rev-parse 'HEAD^{tree}' 2>/dev/null || printf 'no-head\n'
    git write-tree 2>/dev/null || printf 'no-index\n'
    git ls-files -m -o --exclude-standard -z | sort -zu | while IFS= read -r -d '' path; do
      case "$path" in
        "$GATE_DIR"/*) continue ;;
      esac
      if [ -f "$path" ]; then
        printf '%s=' "$path"
        hash_stdin <"$path"
      else
        printf '%s=absent\n' "$path"
      fi
    done
  } | hash_stdin
}

resolve_base() {
  if [ -n "${GATE_BASE:-}" ]; then
    printf '%s\n' "$GATE_BASE"
    return
  fi
  if git rev-parse --verify --quiet origin/main >/dev/null 2>&1; then
    printf 'origin/main\n'
    return
  fi
  if git rev-parse --verify --quiet main >/dev/null 2>&1; then
    printf 'main\n'
    return
  fi
  printf '\n'
}

changed_files() {
  local base merge_base
  base="$(resolve_base)"
  {
    if [ -n "$base" ]; then
      merge_base="$(git merge-base HEAD "$base" 2>/dev/null || true)"
      if [ -n "$merge_base" ]; then
        git diff --name-only "$merge_base" HEAD
      fi
    fi
    git diff --name-only HEAD
    git ls-files -o --exclude-standard
  } | sort -u | sed '/^$/d'
}

# Paths whose change invalidates cross-cutting assumptions (codegen, deps,
# build tooling, schema/contract/config surfaces): fail safe by running full.
is_full_trigger() {
  case "$1" in
    go.mod | go.sum | bun.lock* | Makefile | turbo.json | mise.toml | .tool-versions | DESIGN.md) return 0 ;;
    package.json | */package.json) return 0 ;;
    magefiles/* | scripts/*) return 0 ;;
    tsconfig*.json | .golangci* | .oxlintrc* | .oxfmt* | oxfmt*) return 0 ;;
    *.sql | */atlas.sum | atlas* | sqlc*) return 0 ;;
    *openapi*) return 0 ;;
    internal/config/* | packages/ui/src/tokens.css) return 0 ;;
    *) return 1 ;;
  esac
}

# Docs, agent instructions, and CI definitions exercise no verify lane.
is_no_lane() {
  case "$1" in
    docs/* | .claude/* | .codex/* | .agents/* | .compozy/* | .github/* | .deep-review/*) return 0 ;;
    *.md | *.mdc | LICENSE* | .gitignore | .gitattributes | .editorconfig) return 0 ;;
    *) return 1 ;;
  esac
}

classify() {
  local path="$1" pkg
  if is_full_trigger "$path"; then
    FULL_REASONS="${FULL_REASONS}${path}"$'\n'
    return
  fi
  if is_no_lane "$path"; then
    NO_LANE_COUNT=$((NO_LANE_COUNT + 1))
    return
  fi
  case "$path" in
    internal/*/*)
      pkg="${path#internal/}"
      GO_SCOPES="${GO_SCOPES}./internal/${pkg%%/*}/..."$'\n'
      ;;
    internal/*) GO_SCOPES="${GO_SCOPES}./internal/..."$'\n' ;;
    cmd/*) GO_SCOPES="${GO_SCOPES}./cmd/..."$'\n' ;;
    tests/*) GO_SCOPES="${GO_SCOPES}./tests/..."$'\n' ;;
    web/*) JS_FILTERS="${JS_FILTERS}./web"$'\n' ;;
    packages/*/*)
      pkg="${path#packages/}"
      JS_FILTERS="${JS_FILTERS}./packages/${pkg%%/*}"$'\n'
      ;;
    *) FULL_REASONS="${FULL_REASONS}unclassified: ${path}"$'\n' ;;
  esac
}

classify_all() {
  local files f
  files="$(changed_files)"
  if [ -z "$files" ]; then
    return 0
  fi
  while IFS= read -r f; do
    if [ -n "$f" ]; then
      CHANGED_COUNT=$((CHANGED_COUNT + 1))
      classify "$f"
    fi
  done <<EOF
$files
EOF
}

record_path() { printf '%s/%s.json\n' "$GATE_DIR" "$1"; }

record_field() {
  sed -n 's/.*"'"$2"'": "\([^"]*\)".*/\1/p' "$1" 2>/dev/null | head -n1
}

record_current() {
  local rec="$1"
  [ -f "$rec" ] &&
    [ "$(record_field "$rec" fingerprint)" = "$CURRENT_FINGERPRINT" ] &&
    [ "$(record_field "$rec" result)" = "pass" ]
}

write_record() {
  local id="$1" result="$2" cmd="$3" logfile="$4" duration="$5"
  mkdir -p "$GATE_DIR"
  printf '{\n  "gate": "%s",\n  "fingerprint": "%s",\n  "result": "%s",\n  "command": "%s",\n  "finished_at": "%s",\n  "duration_s": %s,\n  "log": "%s"\n}\n' \
    "$id" "$CURRENT_FINGERPRINT" "$result" "$cmd" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$duration" "$logfile" \
    >"$(record_path "$id")"
}

run_lane() {
  local id="$1"
  shift
  local cmd_display="$*" rec logfile started duration rc
  rec="$(record_path "$id")"
  if record_current "$rec"; then
    log "SKIP $id — evidence current (finished $(record_field "$rec" finished_at), log $(record_field "$rec" log))"
    return 0
  fi
  mkdir -p "$LOG_DIR"
  logfile="$LOG_DIR/$id-$(date +%s).log"
  log "RUN  $id: $cmd_display"
  started=$(date +%s)
  set +e
  "$@" 2>&1 | tee "$logfile"
  rc=${PIPESTATUS[0]}
  set -e
  duration=$(($(date +%s) - started))
  if [ "$rc" -eq 0 ]; then
    write_record "$id" pass "$cmd_display" "$logfile" "$duration"
    log "PASS $id (${duration}s) — record $rec"
  else
    write_record "$id" fail "$cmd_display" "$logfile" "$duration"
    log "FAIL $id (${duration}s, exit $rc) — log $logfile"
  fi
  return "$rc"
}

go_test_p() {
  if [ -n "${COMPOZY_GO_TEST_P:-}" ]; then
    printf '%s\n' "$COMPOZY_GO_TEST_P"
    return
  fi
  local cpu p
  cpu="$(sysctl -n hw.ncpu 2>/dev/null || nproc 2>/dev/null || echo 8)"
  p=$((cpu / 8))
  if [ "$p" -lt 1 ]; then
    p=1
  fi
  printf '%d\n' "$p"
}

run_go_lanes() {
  local scope_args
  scope_args="$(printf '%s' "$GO_SCOPES" | sort -u | tr '\n' ' ')"
  run_lane go-lint make go-lint
  # shellcheck disable=SC2086
  run_lane go-test env CGO_ENABLED=1 go test -race -p "$(go_test_p)" $scope_args
}

run_js_lanes() {
  local filter id
  for filter in $(printf '%s' "$JS_FILTERS" | sort -u); do
    id="js-$(printf '%s' "$filter" | tr './' '--' | sed 's/^-*//')"
    run_lane "$id" bunx turbo run lint typecheck test --filter="$filter"
  done
}

print_classification() {
  log "changed files: $CHANGED_COUNT (base: $(resolve_base))"
  if [ -n "$FULL_REASONS" ]; then
    log "full-gate escalation triggers:"
    printf '%s' "$FULL_REASONS" | sort -u | sed 's/^/[gate]   /'
  fi
  if [ -n "$GO_SCOPES" ]; then
    log "go scopes: $(printf '%s' "$GO_SCOPES" | sort -u | tr '\n' ' ')"
  fi
  if [ -n "$JS_FILTERS" ]; then
    log "js filters: $(printf '%s' "$JS_FILTERS" | sort -u | tr '\n' ' ')"
  fi
  if [ "$NO_LANE_COUNT" -gt 0 ]; then
    log "no-lane (docs/instructions/CI): $NO_LANE_COUNT files"
  fi
}

cmd_auto() {
  classify_all
  if [ "$CHANGED_COUNT" -eq 0 ]; then
    log "clean tree vs base — nothing to gate"
    return 0
  fi
  print_classification
  if [ -n "$FULL_REASONS" ]; then
    log "escalating to full gate"
    cmd_full
    return
  fi
  if [ -z "$GO_SCOPES" ] && [ -z "$JS_FILTERS" ]; then
    log "docs/instructions only — no gate required"
    return 0
  fi
  if [ -n "$GO_SCOPES" ]; then
    run_go_lanes
  fi
  if [ -n "$JS_FILTERS" ]; then
    run_js_lanes
  fi
  log "all affected lanes passed — full gate ('make gate-full') still owns completion"
}

cmd_full() {
  run_lane full make verify
}

cmd_plan() {
  classify_all
  log "fingerprint: $CURRENT_FINGERPRINT"
  if [ "$CHANGED_COUNT" -eq 0 ]; then
    log "clean tree vs base — nothing to gate"
    return 0
  fi
  print_classification
  if [ -n "$FULL_REASONS" ]; then
    log "would run: make verify (full escalation)"
    return 0
  fi
  if [ -n "$GO_SCOPES" ]; then
    log "would run: make go-lint && CGO_ENABLED=1 go test -race -p $(go_test_p) $(printf '%s' "$GO_SCOPES" | sort -u | tr '\n' ' ')"
  fi
  if [ -n "$JS_FILTERS" ]; then
    local filter
    for filter in $(printf '%s' "$JS_FILTERS" | sort -u); do
      log "would run: bunx turbo run lint typecheck test --filter=$filter"
    done
  fi
  if [ -z "$GO_SCOPES" ] && [ -z "$JS_FILTERS" ]; then
    log "would run: nothing (docs/instructions only)"
  fi
}

cmd_status() {
  log "fingerprint: $CURRENT_FINGERPRINT"
  local rec found=0 state
  for rec in "$GATE_DIR"/*.json; do
    [ -f "$rec" ] || continue
    found=1
    state="STALE"
    if [ "$(record_field "$rec" fingerprint)" = "$CURRENT_FINGERPRINT" ]; then
      state="CURRENT"
    fi
    printf '[gate] %-12s %-4s %-7s finished=%s log=%s\n' \
      "$(record_field "$rec" gate)" "$(record_field "$rec" result)" "$state" \
      "$(record_field "$rec" finished_at)" "$(record_field "$rec" log)"
  done
  if [ "$found" -eq 0 ]; then
    log "no evidence records — run 'make gate' or 'make gate-full'"
  fi
}

main() {
  CURRENT_FINGERPRINT="$(tree_fingerprint)"
  case "${1:-auto}" in
    auto) cmd_auto ;;
    full) cmd_full ;;
    plan) cmd_plan ;;
    status) cmd_status ;;
    fingerprint) printf '%s\n' "$CURRENT_FINGERPRINT" ;;
    *) die "usage: gate.sh [auto|full|plan|status|fingerprint]" ;;
  esac
}

main "$@"
