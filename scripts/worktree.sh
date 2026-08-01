#!/usr/bin/env bash
# Worktree lifecycle for parallel agent/dev checkouts.
#
#   scripts/worktree.sh new <slug> [--branch <name>] [--base <ref>] [--dir <path>] [--build] [--e2e] [--skip-install]
#   scripts/worktree.sh bootstrap [--build] [--e2e] [--skip-install]
#   scripts/worktree.sh rm <slug-or-path> [--force]
#   scripts/worktree.sh list
#
# `new` creates a sibling worktree (default <repo-parent>/_worktrees/<slug>),
# replaces shared dirs (.claude .codex .compozy .resources docs) with copies
# from the main checkout, then bootstraps. `bootstrap` makes the CURRENT
# checkout dev-ready: mise tool pins, bun install (postinstall links
# .claude/skills + AGENTS.md), golangci-lint cache seeded from the main
# checkout (the cache is per-worktree), optional `make build` (--build) and
# Playwright chromium (--e2e). GOCACHE/GOMODCACHE are shared; `make verify`
# and the E2E lanes queue machine-wide (L-030), scoped lanes are
# capacity-bounded.
set -euo pipefail

# Dirs wiped in the new worktree and replaced with copies from the main checkout.
SHARED_DIRS=(.claude .codex .compozy .resources docs)

usage() {
  sed -n '2,17p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

die() {
  echo "worktree: $*" >&2
  exit 1
}

repo_root() {
  git rev-parse --show-toplevel 2>/dev/null || die "not inside a git checkout"
}

# Absolute path to the primary checkout (owner of .git), even when invoked
# from a linked worktree.
main_root() {
  local common
  common="$(git rev-parse --git-common-dir)"
  if [[ "$common" != /* ]]; then
    common="$(repo_root)/${common#./}"
  fi
  (cd "$(dirname "$common")" && pwd)
}

default_container() {
  echo "$(dirname "$(main_root)")/_worktrees"
}

# Drop worktree copies of shared/local dirs and replace with main's tree.
sync_shared_dirs_from_main() {
  local dest="$1"
  local src name
  src="$(main_root)"
  dest="$(cd "$dest" && pwd)"
  [ "$src" = "$dest" ] && die "refusing to sync main onto itself"

  for name in "${SHARED_DIRS[@]}"; do
    if [ -e "$dest/$name" ] || [ -L "$dest/$name" ]; then
      rm -rf "$dest/$name"
    fi
    if [ -e "$src/$name" ] || [ -L "$src/$name" ]; then
      echo "worktree: copying $name/ from main ($src)"
      cp -a "$src/$name" "$dest/$name"
    else
      echo "worktree: skip $name/ (not present in main)"
    fi
  done
}

user_cache_base() {
  case "$(uname -s)" in
    Darwin) printf '%s/Library/Caches' "$HOME" ;;
    *) printf '%s' "${XDG_CACHE_HOME:-$HOME/.cache}" ;;
  esac
}

root_cache_digest() {
  if command -v shasum >/dev/null 2>&1; then
    printf '%s' "$1" | shasum -a 256 | cut -c1-16
  else
    printf '%s' "$1" | sha256sum | cut -c1-16
  fi
}

# Warm this checkout's golangci-lint cache from the main checkout's: the cache
# is keyed per worktree root (magefiles/deps_lint.go), so a fresh worktree would
# otherwise pay a cold full-repo analysis. Stale entries are safe
# (content-addressed); best-effort — never fails the bootstrap.
seed_lint_cache() {
  local dest_root="$1" src_root version base src dest tmp
  src_root="$(main_root)"
  [ "$src_root" = "$dest_root" ] && return 0
  if [ -n "${GOLANGCI_LINT_CACHE:-}" ]; then
    echo "worktree: GOLANGCI_LINT_CACHE is set (shared cache) — skipping lint cache seed"
    return 0
  fi
  if ! command -v shasum >/dev/null 2>&1 && ! command -v sha256sum >/dev/null 2>&1; then
    echo "worktree: no sha256 tool — skipping lint cache seed"
    return 0
  fi
  version="$(sed -n 's/^golangci-lint[[:space:]]*=[[:space:]]*"\([^"]*\)".*/\1/p' "$dest_root/mise.toml" 2>/dev/null | head -n1 || true)"
  if [ -z "$version" ]; then
    echo "worktree: no golangci-lint pin in mise.toml — skipping lint cache seed"
    return 0
  fi
  base="$(user_cache_base)/compozy-dev/golangci-lint/${version#v}"
  src="$base/$(root_cache_digest "$src_root")"
  dest="$base/$(root_cache_digest "$dest_root")"
  if [ ! -d "$src" ]; then
    echo "worktree: main checkout has no lint cache yet — skipping lint cache seed"
    return 0
  fi
  if [ -d "$dest" ]; then
    echo "worktree: lint cache already present — skipping lint cache seed"
    return 0
  fi
  echo "worktree: seeding golangci-lint cache from main ($(du -sh "$src" 2>/dev/null | cut -f1 || echo '?'))"
  tmp="$dest.seed.$$"
  rm -rf "$tmp"
  if cp -a "$src" "$tmp" && mv "$tmp" "$dest"; then
    return 0
  fi
  rm -rf "$tmp"
  echo "worktree: lint cache seed failed — first lint run will be cold"
}

bootstrap() {
  local do_build=0 do_e2e=0 skip_install=0
  while [ $# -gt 0 ]; do
    case "$1" in
      --build) do_build=1 ;;
      --e2e) do_e2e=1 ;;
      --skip-install) skip_install=1 ;;
      *) die "unknown bootstrap option: $1" ;;
    esac
    shift
  done
  local root
  root="$(repo_root)"
  cd "$root"

  if command -v mise >/dev/null 2>&1; then
    echo "worktree: mise install (repo tool pins)"
    mise install
  else
    echo "worktree: mise not found — ensure golangci-lint/gotestsum match mise.toml pins manually"
  fi

  seed_lint_cache "$root"

  if [ "$skip_install" -eq 1 ]; then
    echo "worktree: skipping bun install (--skip-install)"
  else
    command -v bun >/dev/null 2>&1 || die "bun is required (https://bun.sh)"
    echo "worktree: bun install (postinstall links .claude/skills + AGENTS.md)"
    bun install
  fi

  if [ "$do_build" -eq 1 ]; then
    echo "worktree: make build"
    make build
  fi

  if [ "$do_e2e" -eq 1 ]; then
    echo "worktree: installing Playwright chromium (shared browser cache)"
    (cd web && bun run test:e2e:install)
  fi

  echo ""
  echo "worktree ready: $root ($(git branch --show-current))"
  echo "  scoped gates : make lint | go test -race ./internal/<pkg>/... | bunx turbo run test --filter=./web"
  echo "  full gate    : make verify  (queues machine-wide behind other worktrees — L-030)"
  echo "  QA hygiene   : make qa-reap after any QA lab run (L-029)"
}

cmd_new() {
  [ $# -ge 1 ] || die "usage: worktree.sh new <slug> [--branch <name>] [--base <ref>] [--dir <path>] [--build] [--e2e] [--skip-install]"
  local slug="$1"
  shift
  [[ "$slug" =~ ^[a-z0-9][a-z0-9._-]*$ ]] || die "slug must be kebab-case: $slug"

  local branch="$slug" base="main" dir="" pass_through=()
  while [ $# -gt 0 ]; do
    case "$1" in
      --branch)
        [ $# -ge 2 ] && [ -n "$2" ] && [[ "$2" != --* ]] || die "--branch requires a value"
        branch="$2"
        shift
        ;;
      --base)
        [ $# -ge 2 ] && [ -n "$2" ] && [[ "$2" != --* ]] || die "--base requires a value"
        base="$2"
        shift
        ;;
      --dir)
        [ $# -ge 2 ] && [ -n "$2" ] && [[ "$2" != --* ]] || die "--dir requires a value"
        dir="$2"
        shift
        ;;
      --build | --e2e | --skip-install) pass_through+=("$1") ;;
      *) die "unknown new option: $1" ;;
    esac
    shift
  done
  if [ -z "$dir" ]; then
    dir="$(default_container)/$slug"
  fi
  [ -e "$dir" ] && die "target already exists: $dir"
  mkdir -p "$(dirname "$dir")"

  if git show-ref --verify --quiet "refs/heads/$branch"; then
    echo "worktree: adding $dir on existing branch $branch"
    git worktree add "$dir" "$branch"
  else
    echo "worktree: adding $dir on new branch $branch (from $base)"
    git worktree add -b "$branch" "$dir" "$base"
  fi

  sync_shared_dirs_from_main "$dir"
  (cd "$dir" && bootstrap ${pass_through[0]:+"${pass_through[@]}"})
}

cmd_rm() {
  [ $# -ge 1 ] || die "usage: worktree.sh rm <slug-or-path> [--force]"
  local target="$1" force=""
  shift
  while [ $# -gt 0 ]; do
    case "$1" in
      --force) force="--force" ;;
      *) die "unknown rm option: $1" ;;
    esac
    shift
  done
  local dir="$target"
  if [ ! -d "$dir" ]; then
    dir="$(default_container)/$target"
  fi
  [ -d "$dir" ] || die "worktree not found: $target"
  # git refuses dirty worktrees without --force; we never override that default.
  git worktree remove ${force:+"$force"} "$dir"
  echo "worktree removed: $dir (branch kept; run 'make qa-reap' if it hosted QA labs)"
}

main() {
  local cmd="${1:-}"
  [ $# -gt 0 ] && shift
  case "$cmd" in
    new) cmd_new "$@" ;;
    bootstrap) bootstrap "$@" ;;
    rm) cmd_rm "$@" ;;
    list) git worktree list ;;
    -h | --help | help | "") usage ;;
    *) die "unknown command: $cmd (try --help)" ;;
  esac
}

main "$@"
