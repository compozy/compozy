#!/usr/bin/env bash
# Pre-push check on the files this branch changes against origin/main. It
# catches the format and lint failures that would otherwise cost a full CI
# round trip, in seconds:
#   js/ts/css/html/json/yaml/md -> oxfmt --check; js/ts -> oxlint --deny-warnings
#   *.go (root module)          -> gofmt -l, then the full .golangci.yml on the
#                                  changed packages (type-aware, unlike the
#                                  edit-time hook)
#
# Findings block the push. Missing tooling skips its tier with a note, because
# CI still enforces every check. Skip the whole hook with
# COMPOZY_SKIP_PRE_PUSH=1 or `git push --no-verify`.
set -uo pipefail

[ "${COMPOZY_SKIP_PRE_PUSH:-}" = "1" ] && exit 0
cd "$(git rev-parse --show-toplevel)" || exit 0

base=$(git merge-base HEAD origin/main 2>/dev/null) || {
  echo "pre-push: origin/main is unknown; skipping checks." >&2
  exit 0
}

fmt_files=()
lint_files=()
go_files=()
while IFS= read -r rel; do
  [ -f "$rel" ] || continue
  case "$rel" in
    *.js | *.jsx | *.ts | *.tsx | *.mjs | *.cjs)
      fmt_files+=("$rel")
      lint_files+=("$rel")
      ;;
    *.css | *.html | *.json | *.jsonc | *.yaml | *.yml | *.md) fmt_files+=("$rel") ;;
    sdk/* | magefiles/* | scripts/*) ;;
    *.go) go_files+=("$rel") ;;
  esac
done < <(git diff --name-only --diff-filter=ACMR "$base" HEAD)

status=0
fail() {
  printf 'pre-push: %s\n' "$1" >&2
  status=1
}

if [ "${#fmt_files[@]}" -gt 0 ]; then
  bunx --no-install oxfmt --check --no-error-on-unmatched-pattern "${fmt_files[@]}" ||
    fail "oxfmt found unformatted files; run bunx oxfmt on them."
fi
if [ "${#lint_files[@]}" -gt 0 ]; then
  bunx --no-install oxlint --deny-warnings --no-error-on-unmatched-pattern "${lint_files[@]}" ||
    fail "oxlint reported findings."
fi

if [ "${#go_files[@]}" -gt 0 ]; then
  unformatted=$(gofmt -l "${go_files[@]}")
  [ -z "$unformatted" ] || fail "gofmt needed on: $unformatted (run make fmt)."

  bin=$(mise which golangci-lint 2>/dev/null || command -v golangci-lint 2>/dev/null) || bin=""
  if [ -n "$bin" ] && "$bin" version 2>/dev/null | grep -q "version 2\."; then
    export GOLANGCI_LINT_CACHE="$PWD/.cache/golangci-hook"
    pkg_dirs=()
    while IFS= read -r dir; do pkg_dirs+=("$dir"); done < <(
      for rel in "${go_files[@]}"; do echo "./$(dirname "$rel")/"; done | sort -u
    )
    "$bin" run --timeout 3m "${pkg_dirs[@]}" ||
      fail "golangci-lint reported findings on the changed packages."
  else
    echo "pre-push: golangci-lint v2 not found; CI will lint Go." >&2
  fi
fi

[ "$status" -eq 0 ] || echo "pre-push: fix the findings above, or push with --no-verify to let CI report them." >&2
exit "$status"
