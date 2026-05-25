#!/usr/bin/env bash
# check-setup.sh — READ-ONLY pr-release wiring verifier for a consuming repo.
#
# Inspects only. Performs no writes, no git mutations, no network or GitHub
# API calls. Run from the consuming repository root:
#   bash skills/releasepr/scripts/check-setup.sh
#
# Exit code: 0 if no FAIL rows, 1 if any FAIL. WARN does not fail the run.

set -u

pass=0
warn=0
fail=0

ok()   { printf 'PASS  %s\n' "$1"; pass=$((pass + 1)); }
warning() { printf 'WARN  %s\n' "$1"; warn=$((warn + 1)); }
bad()  { printf 'FAIL  %s\n' "$1"; fail=$((fail + 1)); }

echo "pr-release setup check (read-only)"
echo "----------------------------------"

# 1. Binary resolvable.
bin=""
if command -v pr-release >/dev/null 2>&1; then
  bin="pr-release"
elif [ -x "./bin/pr-release" ]; then
  bin="./bin/pr-release"
elif command -v releasepr >/dev/null 2>&1; then
  bin="releasepr"
fi
if [ -n "$bin" ]; then
  ver="$("$bin" version 2>/dev/null | grep -m1 '^Version:' | tr -d '\t' || true)"
  ok "pr-release binary found ($bin)${ver:+ - $ver}"
else
  bad "pr-release binary not found (install it; see references/setup.md)"
fi

# 2. GitHub token presence (presence only — value never printed/validated).
if [ -n "${GITHUB_TOKEN:-}" ] || [ -n "${RELEASE_TOKEN:-}" ] || \
   [ -n "${PR_RELEASE_GITHUB_TOKEN:-}" ] || [ -n "${COMPOZY_RELEASE_GITHUB_TOKEN:-}" ]; then
  ok "a GitHub token environment variable is set"
else
  warning "no GitHub token env var set (needed for GitHub operations; OK for local dry-run prep)"
fi

# 3. Config file (optional).
if [ -f ".pr-release.yaml" ]; then
  ok "config file found: .pr-release.yaml"
elif [ -f ".compozy-release.yaml" ]; then
  ok "config file found: .compozy-release.yaml (legacy name)"
else
  warning "no .pr-release.yaml (optional; defaults + env/remote detection apply)"
fi

# 4. Owner/repo resolvable.
if [ -n "${GITHUB_REPOSITORY:-}" ]; then
  ok "GITHUB_REPOSITORY is set ($GITHUB_REPOSITORY)"
elif git remote get-url origin >/dev/null 2>&1; then
  origin="$(git remote get-url origin 2>/dev/null)"
  ok "origin remote present for owner/repo detection ($origin)"
else
  bad "cannot resolve owner/repo: set GITHUB_REPOSITORY or add an origin remote"
fi

# 5. Conventional commits since last tag (advisory only).
if git rev-parse --git-dir >/dev/null 2>&1; then
  last_tag="$(git describe --tags --abbrev=0 2>/dev/null || true)"
  if [ -n "$last_tag" ]; then
    range="${last_tag}..HEAD"
  else
    range="HEAD"
  fi
  conv="$(git log "$range" --pretty=format:%s 2>/dev/null \
    | grep -Eic '^(feat|fix|perf|refactor|build|chore|docs|test|ci|style)(\(.+\))?!?: ' || true)"
  if [ "${conv:-0}" -gt 0 ] 2>/dev/null; then
    ok "$conv conventional commit(s) since ${last_tag:-repo start}"
  else
    warning "no conventional commits since ${last_tag:-repo start} (no version bump; use --force to release anyway)"
  fi
else
  bad "not inside a git repository"
fi

# 6. A release workflow referencing pr-release.
if [ -d ".github/workflows" ] && \
   grep -rilqE 'pr-release|releasepr' .github/workflows 2>/dev/null; then
  ok "a workflow under .github/workflows references pr-release"
else
  warning "no .github/workflows file references pr-release (copy assets/release.yml.template)"
fi

echo "----------------------------------"
printf 'Summary: %d PASS, %d WARN, %d FAIL\n' "$pass" "$warn" "$fail"
if [ "$fail" -gt 0 ]; then
  echo "See skills/releasepr/references/setup.md and troubleshooting.md to resolve FAILs."
  exit 1
fi
exit 0
