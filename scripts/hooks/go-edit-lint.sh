#!/usr/bin/env bash
# Go runner for edit-lint.sh. Contract: cwd = repo root, argv = [--mode
# <check|fix>] followed by repo-relative *.go paths; findings print to stdout
# with exit 2; every other path exits 0 (fail-open — the gate owns enforcement).
#
# Tiers:
#   1. `golangci-lint fmt -E gofmt,golines` auto-applies in every mode: the fix
#      is deterministic and an agent cannot pre-compute gofmt alignment, so
#      reporting drift would only buy a wasted round-trip. goimports is
#      deliberately excluded at edit time: it strips imports added ahead of the
#      code that will use them (in-progress edits).
#   2. check mode only: the syntax-only linters from .golangci.yml on the
#      edited package(s) (no typecheck -> ~1s even on the heaviest packages),
#      run after the formatters so lll/whitespace drift they own never blocks.
set -uo pipefail

mode="check"
if [ "${1:-}" = "--mode" ]; then
  mode="${2:-check}"
  shift 2 2>/dev/null || exit 0
fi

files=()
for rel in "$@"; do
  case "$rel" in
    sdk/* | magefiles/* | scripts/*) continue ;;
    *.go) ;;
    *) continue ;;
  esac
  [ -f "$rel" ] || continue
  files+=("$rel")
done
[ "${#files[@]}" -gt 0 ] || exit 0

bin=$(mise which golangci-lint 2>/dev/null || command -v golangci-lint 2>/dev/null) || exit 0
"$bin" version 2>/dev/null | grep -q "version 2\." || exit 0

export GOLANGCI_LINT_CACHE="$PWD/.cache/golangci-hook"
mkdir -p "$GOLANGCI_LINT_CACHE" 2>/dev/null || exit 0

# One golangci invocation at a time per checkout; a busy lock skips (the next
# edit re-checks). A lock older than 5 minutes is stale (killed run).
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

before=$(cksum "${files[@]}" 2>/dev/null)
"$bin" fmt -E gofmt,golines "${files[@]}" >/dev/null 2>&1
after=$(cksum "${files[@]}" 2>/dev/null)
reformatted=""
if [ "$before" != "$after" ]; then
  reformatted=$(printf '%s\n%s\n' "$before" "$after" | sort | uniq -u | awk '{print $NF}' | sort -u | tr '\n' ' ')
  reformatted="${reformatted% }"
fi

[ "$mode" = "check" ] || exit 0

# Intersection of .golangci.yml linters.enable with golangci's [fast] set:
# syntax-only, no typecheck, safe on in-progress code.
fast_linters="dogsled,funlen,gochecknoinits,gocyclo,ineffassign,lll,misspell,nakedret,nolintlint,whitespace"
issues=""
pkg_dirs=$(for rel in "${files[@]}"; do dirname "$rel"; done | sort -u)
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

[ -n "$issues" ] || exit 0
if [ -n "$reformatted" ]; then
  issues="gofmt/golines auto-applied on: $reformatted — re-read those files before editing them again.
$issues"
fi
printf '%s' "$issues"
exit 2
