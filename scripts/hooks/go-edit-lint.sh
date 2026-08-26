#!/usr/bin/env bash
# Edit-time Go check shared by the Claude Code, Codex, and Cursor hooks.
#
# Payload shapes (stdin JSON):
#   Claude Code PostToolUse  -> .tool_input.file_path
#   Codex PostToolUse        -> .tool_input.file_path, or the raw patch in
#                               .tool_input.command ("*** Update File: <path>")
#   Cursor afterFileEdit     -> top-level .file_path (no .tool_input)
#
# Tiers:
#   1. `golangci-lint fmt -E gofmt,golines` on the edited file(s) only.
#      goimports is deliberately excluded at edit time: it strips imports
#      added ahead of the code that will use them (in-progress edits).
#   2. The syntax-only linters from .golangci.yml on the edited package(s)
#      (no typecheck -> ~1s even on the heaviest packages).
#
# Modes (GO_EDIT_LINT_MODE overrides; default derives from the payload):
#   check -> report findings on stderr and exit 2 so the agent fixes them
#            (Claude Code and Codex feed exit-2 stderr back to the model).
#   fix   -> apply the formatters in place and always exit 0; Cursor's
#            afterFileEdit has no feedback channel, so reporting is useless
#            there and rewriting the file is the only useful action.
#
# Deliberately fail-open: missing tooling, foreign paths, or a busy lock skip
# the check (exit 0) — `make gate` and PR CI own enforcement.
set -uo pipefail

payload=$(cat)
command -v jq >/dev/null 2>&1 || exit 0

mode="${GO_EDIT_LINT_MODE:-}"
if [ -z "$mode" ]; then
  if jq -e '.tool_input | type == "object"' <<<"$payload" >/dev/null 2>&1; then
    mode="check"
  else
    mode="fix"
  fi
fi

candidates=$(jq -r '
  [
    (.tool_input.file_path? // empty),
    (.tool_input.path? // empty),
    (.file_path? // empty)
  ] | .[]' <<<"$payload" 2>/dev/null)
if jq -e '.tool_name == "apply_patch" and (.tool_input.command | type == "string")' <<<"$payload" >/dev/null 2>&1; then
  patch_paths=$(jq -r '.tool_input.command' <<<"$payload" |
    sed -n -e 's/^\*\*\* Update File: //p' -e 's/^\*\*\* Add File: //p')
  candidates=$(printf '%s\n%s' "$candidates" "$patch_paths")
fi
[ -n "$(printf '%s' "$candidates" | tr -d '[:space:]')" ] || exit 0

root="${CLAUDE_PROJECT_DIR:-}"
if [ -z "$root" ]; then
  root=$(jq -r '.cwd // .workspace_roots[0]? // empty' <<<"$payload" 2>/dev/null)
  [ -d "$root" ] || root=""
fi
if [ -z "$root" ]; then
  first=$(printf '%s\n' "$candidates" | sed '/^$/d' | head -n 1)
  probe_dir=$(dirname "$first")
  [ -d "$probe_dir" ] || probe_dir="$PWD"
  root=$(git -C "$probe_dir" rev-parse --show-toplevel 2>/dev/null) || root="$PWD"
fi
cd "$root" 2>/dev/null || exit 0

files=()
while IFS= read -r candidate; do
  [ -n "$candidate" ] || continue
  case "$candidate" in
    /*) rel="${candidate#"$root"/}" ;;
    *) rel="$candidate" ;;
  esac
  case "$rel" in
    /*) continue ;;
    *.go) ;;
    *) continue ;;
  esac
  case "$rel" in
    sdk/* | magefiles/* | scripts/*) continue ;;
  esac
  [ -f "$rel" ] || continue
  case " ${files[*]-} " in
    *" $rel "*) ;;
    *) files+=("$rel") ;;
  esac
done <<<"$candidates"
[ "${#files[@]}" -gt 0 ] || exit 0

bin=$(mise which golangci-lint 2>/dev/null || command -v golangci-lint 2>/dev/null) || exit 0
"$bin" version 2>/dev/null | grep -q "version 2\." || exit 0

export GOLANGCI_LINT_CACHE="$root/.cache/golangci-hook"
mkdir -p "$GOLANGCI_LINT_CACHE" 2>/dev/null || exit 0

# One check at a time per checkout; a busy lock skips (the next edit re-checks).
# A lock older than 5 minutes is stale (killed run) and gets replaced.
lock="$GOLANGCI_LINT_CACHE/.lock"
if ! mkdir "$lock" 2>/dev/null; then
  now=$(date +%s)
  lock_mtime=$(stat -f %m "$lock" 2>/dev/null || stat -c %Y "$lock" 2>/dev/null || echo "$now")
  if [ $((now - lock_mtime)) -lt 300 ]; then
    exit 0
  fi
  rm -rf "$lock" 2>/dev/null
  mkdir "$lock" 2>/dev/null || exit 0
fi
trap 'rmdir "$lock" 2>/dev/null' EXIT

issues=""

if [ "$mode" = "fix" ]; then
  "$bin" fmt -E gofmt,golines "${files[@]}" >/dev/null 2>&1
else
  fmt_out=$("$bin" fmt --diff -E gofmt,golines "${files[@]}" 2>&1)
  if [ $? -ne 0 ] && [ -n "$fmt_out" ]; then
    issues="formatting drift (gofmt/golines) — fix now or it fails the gate:
$fmt_out
"
  fi
fi

# Intersection of .golangci.yml linters.enable with golangci's [fast] set:
# syntax-only, no typecheck, safe on in-progress code.
fast_linters="dogsled,funlen,gochecknoinits,gocyclo,ineffassign,lll,misspell,nakedret,nolintlint,whitespace"
pkg_dirs=$(for rel in "${files[@]}"; do dirname "$rel"; done | sort -u)
if [ "$mode" = "check" ]; then
  while IFS= read -r pkg_dir; do
    [ -n "$pkg_dir" ] || continue
    run_out=$("$bin" run --enable-only "$fast_linters" --concurrency 4 --timeout 40s "./$pkg_dir/" 2>&1)
    run_rc=$?
    if [ "$run_rc" -ne 0 ] && [ -n "$run_out" ]; then
      case "$run_out" in
        *"no go files to analyze"*) ;;
        *)
          issues="${issues}fast linters on ./$pkg_dir/:
$run_out
"
          ;;
      esac
    fi
  done <<<"$pkg_dirs"
fi

if [ -n "$issues" ]; then
  printf 'golangci edit-time check (fast tier; the full gate still owns type-aware linters):\n%s' "$issues" | head -c 4000 >&2
  exit 2
fi
exit 0
