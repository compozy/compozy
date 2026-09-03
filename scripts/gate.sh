#!/usr/bin/env bash
# Evidence-cached verification gate.
#
#   auto         classify the diff vs merge-base and run only affected lanes;
#                the PR CI run owns full verification
#   full         opt-in local `make verify` (machine-wide lock lives in mage)
#   plan         print the classification and commands without running anything
#   status       print evidence records vs the current tree fingerprint
#   fingerprint  print the current tree fingerprint
#
# Records live in .cache/gate/<lane>.json keyed by a content fingerprint
# (HEAD tree + tracked diff + untracked blob hashes): commits that preserve the
# tree keep records valid; any content or mode edit goes stale.
# Target bash 3.2 (stock macOS) — no associative arrays, no mapfile.

set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

GATE_DIR=".cache/gate"
LOG_DIR="$GATE_DIR/logs"

CURRENT_FINGERPRINT=""
CI_FULL_REASONS=""
GO_SCOPES=""
SDK_GO=0
JS_FILTERS=""
JS_ALL=0
CODEGEN_CHECK=0
TOOLING_TEST=0
BASE_UNUSABLE=0
UNCLASSIFIED_COUNT=0
NO_LANE_COUNT=0
CHANGED_COUNT=0
GATE_SLOT=""

log() { printf '[gate] %s\n' "$*"; }
die() {
  printf '[gate] error: %s\n' "$*" >&2
  exit 2
}

# shellcheck source=scripts/gate_runtime.sh
. scripts/gate_runtime.sh

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
  local base merge_base temp_files
  base="$(resolve_base)"
  merge_base=""
  if [ -n "$base" ]; then
    merge_base="$(git merge-base HEAD "$base" 2>/dev/null || true)"
  fi
  if [ -z "$merge_base" ]; then
    return 1
  fi
  temp_files="$(mktemp "${TMPDIR:-/tmp}/compozy-gate-files.XXXXXX")"
  trap 'rm -f "${temp_files:-}"' EXIT
  git diff --name-only "$merge_base" HEAD >"$temp_files"
  git diff --name-only HEAD >>"$temp_files"
  git ls-files -o --exclude-standard >>"$temp_files"
  sort -u "$temp_files" | sed '/^$/d'
}

# Paths whose change requires the full PR CI gate even when the local lane is
# narrow. The local gate records the reason and runs the affected lane only.
is_ci_full_trigger() {
  case "$1" in
    go.mod | go.sum | bun.lock* | Makefile | turbo.json | mise.toml | .tool-versions | DESIGN.md | config.toml) return 0 ;;
    package.json | */package.json) return 0 ;;
    magefiles/* | scripts/*) return 0 ;;
    tsconfig*.json | vitest.config.* | knip.json | electron-builder.yml | .bun-version | .golangci* | .oxlintrc* | .oxfmt* | oxfmt*) return 0 ;;
    *.sql | */atlas.sum | atlas* | sqlc*) return 0 ;;
    *openapi*) return 0 ;;
    internal/config/* | packages/ui/src/tokens.css) return 0 ;;
    *) return 1 ;;
  esac
}

# Docs and agent instructions exercise no verify lane; extension and skill resource manifests do.
is_no_lane() {
  case "$1" in
    extensions/* | skills/*) return 1 ;;
		docs/* | packages/site/content/* | .claude/* | .codex/* | .cursor/* | .agents/* | .compozy/* | .github/* | .vscode/* | .deep-review/*) return 0 ;;
		*.md | *.mdc | LICENSE* | .gitignore | .gitattributes | .editorconfig | .repoclone.rc) return 0 ;;
    *) return 1 ;;
  esac
}

classify() {
	local path="$1" pkg
	if is_ci_full_trigger "$path"; then
		CI_FULL_REASONS="${CI_FULL_REASONS}${path}"$'\n'
		case "$path" in
			go.mod | go.sum | config.toml | .golangci*) GO_SCOPES="${GO_SCOPES}./..."$'\n' ;;
			bun.lock* | turbo.json | tsconfig*.json | vitest.config.* | knip.json | electron-builder.yml | .bun-version | .oxlintrc* | .oxfmt* | oxfmt* | package.json | */package.json)
				JS_ALL=1
				;;
			Makefile | mise.toml | .tool-versions | magefiles/* | scripts/*) TOOLING_TEST=1 ;;
			*.sql | */atlas.sum | atlas* | sqlc* | *openapi* | DESIGN.md | packages/ui/src/tokens.css)
				CODEGEN_CHECK=1
				;;
		esac
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
    extensions/*) GO_SCOPES="${GO_SCOPES}./extensions/..."$'\n' ;;
    skills/*) GO_SCOPES="${GO_SCOPES}./skills/..."$'\n' ;;
    tests/*) GO_SCOPES="${GO_SCOPES}./tests/..."$'\n' ;;
    sdk/go/*) SDK_GO=1 ;;
    sdk/typescript/*) JS_FILTERS="${JS_FILTERS}./sdk/typescript"$'\n' ;;
    sdk/react/*) JS_FILTERS="${JS_FILTERS}./sdk/react"$'\n' ;;
    sdk/examples/*/*)
      pkg="${path#sdk/examples/}"
      JS_FILTERS="${JS_FILTERS}./sdk/examples/${pkg%%/*}"$'\n'
      ;;
    web/*) JS_FILTERS="${JS_FILTERS}./web"$'\n' ;;
    packages/*/*)
      pkg="${path#packages/}"
      JS_FILTERS="${JS_FILTERS}./packages/${pkg%%/*}"$'\n'
      ;;
		desktop/*) JS_FILTERS="${JS_FILTERS}./desktop"$'\n' ;;
		magefiles/* | scripts/*) : ;;
		*.go) GO_SCOPES="${GO_SCOPES}./..."$'\n' ;;
		*)
			if ! is_ci_full_trigger "$path"; then
				CI_FULL_REASONS="${CI_FULL_REASONS}unclassified: ${path}"$'\n'
				UNCLASSIFIED_COUNT=$((UNCLASSIFIED_COUNT + 1))
			fi
			;;
	esac
}

classify_all() {
	local files f
	if ! files="$(changed_files)"; then
		CI_FULL_REASONS="${CI_FULL_REASONS}no usable merge base ($(resolve_base))"$'\n'
		BASE_UNUSABLE=1
  fi
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
  local rec="$1" logfile
  logfile="$(record_field "$rec" log)"
  [ -f "$rec" ] &&
    [ "$(record_field "$rec" fingerprint)" = "$CURRENT_FINGERPRINT" ] &&
    [ "$(record_field "$rec" result)" = "pass" ] &&
    [ -n "$logfile" ] &&
    [ -f "$logfile" ]
}

write_record() {
  local id="$1" result="$2" cmd="$3" logfile="$4" duration="$5" rec temp_record
  mkdir -p "$GATE_DIR"
  rec="$(record_path "$id")"
  temp_record="${rec}.tmp.$$"
  printf '{\n  "gate": "%s",\n  "fingerprint": "%s",\n  "result": "%s",\n  "command": "%s",\n  "finished_at": "%s",\n  "duration_s": %s,\n  "log": "%s"\n}\n' \
    "$id" "$CURRENT_FINGERPRINT" "$result" "$cmd" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$duration" "$logfile" \
    >"$temp_record"
  mv "$temp_record" "$rec"
}

run_lane() {
  local id="$1"
  shift
  local cmd_display="$*" rec logfile started duration rc command_rc tee_rc
  local -a pipeline_status
  rec="$(record_path "$id")"
  if record_current "$rec"; then
    log "SKIP $id — evidence current (finished $(record_field "$rec" finished_at), log $(record_field "$rec" log))"
    return 0
  fi
  mkdir -p "$LOG_DIR"
  logfile="$LOG_DIR/$id-$(date +%s)-$$.log"
  log "RUN  $id: $cmd_display"
  started=$(date +%s)
  set +e
  "$@" 2>&1 | tee "$logfile"
  pipeline_status=("${PIPESTATUS[@]}")
  command_rc=${pipeline_status[0]}
  tee_rc=${pipeline_status[1]}
  set -e
  if [ "$command_rc" -ne 0 ]; then
    rc="$command_rc"
  else
    rc="$tee_rc"
  fi
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
  local cpu parallel total_budget p
  cpu="$(sysctl -n hw.ncpu 2>/dev/null || nproc 2>/dev/null || echo 8)"
  parallel="$(go_test_parallel)"
  total_budget=$((cpu / 2))
  p=$((total_budget / parallel))
  if [ "$p" -lt 1 ]; then
    p=1
  fi
  printf '%d\n' "$p"
}

go_test_parallel() { printf '4\n'; }

go_race_gcflags() {
  if [ "${COMPOZY_GO_FULL_CHECKPTR:-0}" = "1" ]; then
    return
  fi
  printf '%s\n' '-gcflags=modernc.org/...=-d=checkptr=0'
}

normalized_go_scopes() {
	if printf '%s' "$GO_SCOPES" | grep -Fxq './...'; then
		printf './...\n'
		return
	fi
	printf '%s' "$GO_SCOPES" | sort -u
}

turbo_concurrency() {
  local cpu maximum concurrency
  cpu="$(sysctl -n hw.ncpu 2>/dev/null || nproc 2>/dev/null || echo 8)"
  maximum="${COMPOZY_GATE_MAX_CONCURRENT:-2}"
	case "$maximum" in
		'' | *[!0-9]* | 0) die "COMPOZY_GATE_MAX_CONCURRENT must be a positive integer" ;;
	esac
  concurrency=$((cpu / maximum / 2))
  [ "$concurrency" -ge 1 ] || concurrency=1
  printf '%d\n' "$concurrency"
}

# Lint and tests narrow to the changed subtrees; the PR CI full gate owns
# cross-package fallout in unchanged dependents.
run_go_lanes() {
	local scope_args gcflags
	scope_args="$(normalized_go_scopes | tr '\n' ' ')"
	gcflags="$(go_race_gcflags)"
	if [ -n "$GO_SCOPES" ]; then
		run_lane go-lint env "COMPOZY_GO_LINT_SCOPES=$scope_args" make go-lint
		# shellcheck disable=SC2086
		run_lane go-test env -u COMPOZY_HOME -u COMPOZY_CONFIG_HOME -u COMPOZY_HTTP_PORT -u COMPOZY_UDS_PATH -u COMPOZY_WEB_API_PROXY_TARGET -u COMPOZY_WEB_DIST_DIR -u TMUX_BRIDGE_SOCKET -u PROVIDER_HOME -u PROVIDER_CODEX_HOME CGO_ENABLED=1 go test -race $gcflags -p "$(go_test_p)" -parallel "$(go_test_parallel)" -timeout 45m $scope_args
  fi
  if [ "$SDK_GO" -eq 1 ]; then
    run_lane sdk-go-test env CGO_ENABLED=1 go -C sdk/go test -race -parallel=4 ./...
  fi
}

run_js_lanes() {
	local filter id
	if [ "$JS_ALL" -eq 1 ]; then
		run_lane js-all env "TURBO_CONCURRENCY=$(turbo_concurrency)" bunx turbo run lint typecheck test
		return
	fi
  for filter in $(printf '%s' "$JS_FILTERS" | sort -u); do
    id="js-$(printf '%s' "$filter" | tr './' '--' | sed 's/^-*//')"
    run_lane "$id" env "TURBO_CONCURRENCY=$(turbo_concurrency)" bunx turbo run lint typecheck test --filter="$filter"
  done
}

run_support_lanes() {
	if [ "$CODEGEN_CHECK" -eq 1 ]; then
		run_lane codegen-check make codegen-check
		export COMPOZY_CODEGEN_CHECKED=1
	fi
	if [ "$TOOLING_TEST" -eq 1 ]; then
		run_lane gate-integration env CGO_ENABLED=1 go test -race -p "$(go_test_p)" -parallel "$(go_test_parallel)" -timeout 15m -tags=integration ./scripts
		run_lane mage-test env CGO_ENABLED=1 go test -race -p "$(go_test_p)" -parallel "$(go_test_parallel)" -timeout 15m -tags=mage ./magefiles
	fi
}

print_classification() {
  log "changed files: $CHANGED_COUNT (base: $(resolve_base))"
	if [ -n "$CI_FULL_REASONS" ]; then
		log "changes requiring the full PR CI gate:"
		printf '%s' "$CI_FULL_REASONS" | sort -u | sed 's/^/[gate]   /'
  fi
	if [ -n "$GO_SCOPES" ]; then
		log "go scopes: $(normalized_go_scopes | tr '\n' ' ')"
  fi
  if [ "$SDK_GO" -eq 1 ]; then
    log "sdk/go lane: separate module (go -C sdk/go)"
  fi
	if [ -n "$JS_FILTERS" ]; then
		log "js filters: $(printf '%s' "$JS_FILTERS" | sort -u | tr '\n' ' ')"
	fi
	if [ "$JS_ALL" -eq 1 ]; then
		log "js lane: all workspaces"
	fi
	if [ "$CODEGEN_CHECK" -eq 1 ]; then
		log "codegen lane: make codegen-check"
	fi
	if [ "$TOOLING_TEST" -eq 1 ]; then
		log "tooling lanes: gate integration + mage tests"
	fi
  if [ "$NO_LANE_COUNT" -gt 0 ]; then
    log "no-lane (docs/instructions/CI): $NO_LANE_COUNT files"
  fi
}

cmd_auto() {
	classify_all
	if [ "$CHANGED_COUNT" -eq 0 ] && [ -z "$CI_FULL_REASONS" ]; then
		log "clean tree vs base — nothing to gate"
		return 0
	fi
	print_classification
	if [ "$BASE_UNUSABLE" -eq 1 ]; then
		die "cannot classify the local gate without a merge base; fetch the base or set GATE_BASE"
	fi
	if [ "$UNCLASSIFIED_COUNT" -gt 0 ]; then
		die "cannot safely classify $UNCLASSIFIED_COUNT changed path(s); add an affected-lane mapping"
	fi
	if [ -n "$GO_SCOPES" ] || [ -n "$JS_FILTERS" ] || [ "$JS_ALL" -eq 1 ] || [ "$SDK_GO" -eq 1 ] || [ "$CODEGEN_CHECK" -eq 1 ] || [ "$TOOLING_TEST" -eq 1 ]; then
		acquire_gate_slot
	fi
	run_support_lanes
	if [ -z "$GO_SCOPES" ] && [ -z "$JS_FILTERS" ] && [ "$JS_ALL" -eq 0 ] && [ "$SDK_GO" -eq 0 ] && [ "$CODEGEN_CHECK" -eq 0 ] && [ "$TOOLING_TEST" -eq 0 ]; then
		log "docs/instructions only — no gate required"
		return 0
  fi
  if [ -n "$GO_SCOPES" ] || [ "$SDK_GO" -eq 1 ]; then
    run_go_lanes
  fi
	if [ -n "$JS_FILTERS" ] || [ "$JS_ALL" -eq 1 ]; then
		run_js_lanes
	fi
	log "all affected local lanes passed — the PR CI run owns full verification"
}

cmd_full() {
  run_lane full make verify
}

cmd_plan() {
	classify_all
	log "fingerprint: $CURRENT_FINGERPRINT"
	if [ "$CHANGED_COUNT" -eq 0 ] && [ -z "$CI_FULL_REASONS" ]; then
		log "clean tree vs base — nothing to gate"
		return 0
	fi
	print_classification
	if [ "$BASE_UNUSABLE" -eq 1 ]; then
		log "would stop: fetch the base or set GATE_BASE before running the local gate"
		return 0
	fi
	if [ "$UNCLASSIFIED_COUNT" -gt 0 ]; then
		log "would stop: add affected-lane mappings for unclassified paths"
		return 0
	fi
	if [ "$CODEGEN_CHECK" -eq 1 ]; then
		log "would run: make codegen-check"
	fi
	if [ "$TOOLING_TEST" -eq 1 ]; then
		log "would run: CGO_ENABLED=1 go test -race -p $(go_test_p) -parallel $(go_test_parallel) -timeout 15m -tags=integration ./scripts"
		log "would run: CGO_ENABLED=1 go test -race -p $(go_test_p) -parallel $(go_test_parallel) -timeout 15m -tags=mage ./magefiles"
	fi
	if [ -n "$GO_SCOPES" ]; then
		local scopes gcflags
		scopes="$(normalized_go_scopes | tr '\n' ' ')"
		gcflags="$(go_race_gcflags)"
		log "would run: env COMPOZY_GO_LINT_SCOPES='$scopes' make go-lint && CGO_ENABLED=1 go test -race $gcflags -p $(go_test_p) -parallel $(go_test_parallel) -timeout 45m $scopes"
  fi
  if [ "$SDK_GO" -eq 1 ]; then
    log "would run: CGO_ENABLED=1 go -C sdk/go test -race -parallel=4 ./..."
  fi
	if [ "$JS_ALL" -eq 1 ]; then
		log "would run: TURBO_CONCURRENCY=$(turbo_concurrency) bunx turbo run lint typecheck test"
	elif [ -n "$JS_FILTERS" ]; then
    local filter
    for filter in $(printf '%s' "$JS_FILTERS" | sort -u); do
      log "would run: TURBO_CONCURRENCY=$(turbo_concurrency) bunx turbo run lint typecheck test --filter=$filter"
    done
  fi
	if [ -z "$GO_SCOPES" ] && [ -z "$JS_FILTERS" ] && [ "$JS_ALL" -eq 0 ] && [ "$SDK_GO" -eq 0 ] && [ "$CODEGEN_CHECK" -eq 0 ] && [ "$TOOLING_TEST" -eq 0 ]; then
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
    if record_current "$rec"; then
      state="CURRENT-PASS"
    elif [ "$(record_field "$rec" fingerprint)" = "$CURRENT_FINGERPRINT" ]; then
      state="CURRENT-FAIL"
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
