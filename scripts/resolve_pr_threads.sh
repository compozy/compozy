#!/usr/bin/env bash
set -euo pipefail

# resolve_pr_threads.sh — Resolve/Unresolve GitHub PR review threads from exported AI docs
#
# Generic, reusable script. It scans issue markdown files for a "RESOLVED" marker
# (or uses an explicit thread IDs file) and calls GitHub's GraphQL API via `gh`.
#
# Requirements:
# - GitHub CLI (`gh`) authenticated with repo scope
#
# Usage:
#   scripts/resolve_pr_threads.sh \
#     --issues-dir ai-docs/reviews-pr-259/issues \
#     [--action resolve|unresolve] \
#     [--status-pattern "^\\*\\*Status:\\*\\* - \\[[xX]\\] RESOLVED( ✓)?$"] \
#     [--threads-file /path/to/thread_ids.txt] \
#     [--changed] \
#     [--dry-run] \
#     [--delay-ms 300]
#
# Examples:
#   # Resolve all marked threads for PR 259
#   scripts/resolve_pr_threads.sh --issues-dir ai-docs/reviews-pr-259/issues
#
#   # Unresolve (reopen) marked threads
#   scripts/resolve_pr_threads.sh --issues-dir ai-docs/reviews-pr-259/issues --action unresolve
#
#   # Only process issue files changed in git (staged or working tree)
#   scripts/resolve_pr_threads.sh --issues-dir ai-docs/reviews-pr-259/issues --changed
#
#   # Provide explicit thread IDs (one per line)
#   scripts/resolve_pr_threads.sh --threads-file /tmp/threads.txt

issues_dir=""
action="resolve"            # resolve|unresolve
status_pattern='^\*\*Status:\*\* - \[[xX]\] RESOLVED( ✓)?$'
threads_file=""
only_changed=false
dry_run=false
delay_ms=300

die() { echo "Error: $*" >&2; exit 1; }

need_cmd() { command -v "$1" >/dev/null 2>&1 || die "Missing required command: $1"; }

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --issues-dir)        issues_dir=${2:-}; shift 2 ;;
      --action)            action=${2:-}; shift 2 ;;
      --status-pattern)    status_pattern=${2:-}; shift 2 ;;
      --threads-file)      threads_file=${2:-}; shift 2 ;;
      --changed)           only_changed=true; shift ;;
      --dry-run)           dry_run=true; shift ;;
      --delay-ms)          delay_ms=${2:-300}; shift 2 ;;
      -h|--help)
        sed -n '1,80p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
      *) die "Unknown arg: $1" ;;
    esac
  done
}

validate_args() {
  [[ -n "$threads_file" || -n "$issues_dir" ]] || die "Provide --threads-file or --issues-dir"
  [[ "$action" =~ ^(resolve|unresolve)$ ]] || die "--action must be resolve|unresolve"
  if [[ -n "$issues_dir" && ! -d "$issues_dir" ]]; then
    die "Issues directory not found: $issues_dir"
  fi
  need_cmd gh
  need_cmd grep
  need_cmd sed
}

msleep() { perl -e 'select(undef,undef,undef, $ARGV[0]/1000.0)' "$1" 2>/dev/null || sleep 0.3; }

extract_threads_from_file() {
  # stdout: thread IDs found in the file
  local f="$1"
  # Format 1: "Thread ID: `PRRT_xxx`"
  grep -Eo 'Thread ID: `[^`]+`' "$f" | sed -E 's/.*`([^`]+)`.*/\1/' || true
  # Format 2: gh example: threadId='PRRT_xxx'
  grep -Eo "threadId='[^']+'" "$f" | sed -E "s/.*'([^']+)'.*/\1/" || true
}

gather_threads_from_issues() {
  local files=()
  if $only_changed; then
    # Collect changed issue files under the directory (staged + unstaged)
    while IFS= read -r path; do
      [[ -f "$path" ]] && files+=("$path")
    done < <(git ls-files -m -o --exclude-standard "$issues_dir" | grep -E '/issue_.*\.md$' || true)
  else
    while IFS= read -r path; do files+=("$path"); done < <(find "$issues_dir" -type f -name 'issue_*.md' | sort)
  fi

  local ids=()
  for f in "${files[@]}"; do
    # Check status line matches pattern
    if grep -Eq "$status_pattern" "$f"; then
      echo "✅ RESOLVED: $(basename "$f")"
      while IFS= read -r id; do
        [[ -n "$id" ]] && ids+=("$id")
      done < <(extract_threads_from_file "$f" | sort -u)
    fi
  done
  printf "%s\n" "${ids[@]}" | sort -u
}

gather_threads() {
  if [[ -n "$threads_file" ]]; then
    grep -Eo 'PRRT_[A-Za-z0-9_-]+' "$threads_file" | sort -u
  else
    gather_threads_from_issues
  fi
}

do_call() {
  local id="$1"
  if [[ "$action" == "resolve" ]]; then
    gh api graphql \
      -f query='mutation($threadId: ID!) { resolveReviewThread(input: { threadId: $threadId }) { thread { isResolved } } }' \
      -F threadId="$id"
  else
    gh api graphql \
      -f query='mutation($threadId: ID!) { unresolveReviewThread(input: { threadId: $threadId }) { thread { isResolved } } }' \
      -F threadId="$id"
  fi
}

main() {
  parse_args "$@"; validate_args

  echo "🔧 Mode: $action"
  [[ -n "$issues_dir" ]] && echo "📁 Issues dir: $issues_dir"
  [[ -n "$threads_file" ]] && echo "🧾 Threads file: $threads_file"
  echo "🔎 Status pattern: $status_pattern"
  echo "🧪 Dry run: $dry_run | 🔁 Changed only: $only_changed | ⏱ Delay: ${delay_ms}ms"

  mapfile -t ids < <(gather_threads)
  echo "\n📋 Found ${#ids[@]} thread(s) to process\n"

  local ok=0 fail=0
  for id in "${ids[@]}"; do
    echo "📡 ${action^} thread: $id"
    if $dry_run; then
      echo "   (dry-run) Skipping API call"
    else
      if do_call "$id"; then
        echo "   ✅ Success"
        ((ok++))
      else
        echo "   ❌ Failed"
        ((fail++))
      fi
      msleep "$delay_ms"
    fi
  done

  echo "\n📊 Summary: OK=$ok  FAIL=$fail  TOTAL=${#ids[@]}"
  if (( fail == 0 )); then
    echo "🎉 Completed without errors"
  else
    echo "⚠️  Some operations failed — see log above"
  fi
}

main "$@"
