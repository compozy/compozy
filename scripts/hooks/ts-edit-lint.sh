#!/usr/bin/env bash
# TypeScript/web runner for edit-lint.sh. Contract: cwd = repo root, argv =
# [--mode <check|fix>] followed by repo-relative paths; findings print to
# stdout with exit 2; every other path exits 0 (fail-open — the gate owns
# enforcement).
#
# Mirrors the repo lint surface (root package.json `lint` + lint-staged):
#   1. `oxfmt` auto-applies on js/ts and css/html/json/jsonc/yaml/md; the
#      .oxfmtrc.json ignorePatterns decide skips, so generated and vendored
#      files pass through untouched.
#   2. check mode only: `oxlint --deny-warnings` with the root .oxlintrc.json
#      (jsPlugins included) on the edited js/ts files — the same flag the gate
#      runs. Typecheck stays with the gate, like the Go runner's type-aware
#      tier.
set -uo pipefail

mode="check"
if [ "${1:-}" = "--mode" ]; then
  mode="${2:-check}"
  shift 2 2>/dev/null || exit 0
fi

fmt_files=()
lint_files=()
for rel in "$@"; do
  case "$rel" in
    *node_modules/*) continue ;;
  esac
  [ -f "$rel" ] || continue
  case "$rel" in
    *.js | *.jsx | *.ts | *.tsx | *.mjs | *.cjs | *.mts | *.cts)
      fmt_files+=("$rel")
      lint_files+=("$rel")
      ;;
    *.css | *.html | *.json | *.jsonc | *.yaml | *.yml | *.md)
      fmt_files+=("$rel")
      ;;
  esac
done
[ "${#fmt_files[@]}" -gt 0 ] || exit 0

oxfmt_bin="node_modules/.bin/oxfmt"
oxlint_bin="node_modules/.bin/oxlint"

reformatted=""
if [ -x "$oxfmt_bin" ]; then
  before=$(cksum "${fmt_files[@]}" 2>/dev/null)
  "$oxfmt_bin" --no-error-on-unmatched-pattern "${fmt_files[@]}" >/dev/null 2>&1
  after=$(cksum "${fmt_files[@]}" 2>/dev/null)
  if [ "$before" != "$after" ]; then
    reformatted=$(printf '%s\n%s\n' "$before" "$after" | sort | uniq -u | awk '{print $NF}' | sort -u | tr '\n' ' ')
    reformatted="${reformatted% }"
  fi
fi

[ "$mode" = "check" ] || exit 0
[ "${#lint_files[@]}" -gt 0 ] || exit 0
[ -x "$oxlint_bin" ] || exit 0

run_out=$("$oxlint_bin" --deny-warnings "${lint_files[@]}" 2>&1)
run_rc=$?
if [ "$run_rc" -eq 0 ] || [ -z "$run_out" ]; then
  exit 0
fi

issues="oxlint --deny-warnings:
$run_out
"
if [ -n "$reformatted" ]; then
  issues="oxfmt auto-applied on: $reformatted — re-read those files before editing them again.
$issues"
fi
printf '%s' "$issues"
exit 2
