#!/usr/bin/env bash
# Worktree lifecycle for parallel agent/dev checkouts.
#
#   scripts/worktree.sh new <slug> [--branch <name>] [--base <ref>] [--dir <path>] [--build] [--e2e] [--skip-install]
#   scripts/worktree.sh bootstrap [--build] [--e2e] [--skip-install]
#   scripts/worktree.sh rm <slug-or-path> [--force]
#   scripts/worktree.sh list
#
# `new` creates a sibling worktree (default <repo-parent>/<repo-name>-worktrees/<slug>)
# and bootstraps it. `bootstrap` makes the CURRENT checkout dev-ready: mise tool
# pins, bun install (postinstall links .claude/skills + AGENTS.md), optional
# `make build` (--build) and Playwright chromium (--e2e). Go lanes need no
# per-worktree setup (GOCACHE/GOMODCACHE are shared); `make verify` and the E2E
# lanes queue machine-wide (L-030), scoped lanes are capacity-bounded.
set -euo pipefail

usage() {
  sed -n '2,14p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

die() {
  echo "worktree: $*" >&2
  exit 1
}

repo_root() {
  git rev-parse --show-toplevel 2>/dev/null || die "not inside a git checkout"
}

default_container() {
  local root parent name
  root="$(repo_root)"
  parent="$(dirname "$root")"
  name="$(basename "$root")"
  # main checkout owns the container; linked worktrees resolve their main root
  local common
  common="$(git rev-parse --git-common-dir)"
  common="$(cd "$(dirname "$common")" && pwd)"
  parent="$(dirname "$common")"
  name="$(basename "$common")"
  echo "$parent/$name-worktrees"
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
