#!/usr/bin/env bash
set -euo pipefail

# resolve_pr_issues.sh — Mark exported PR review issues as resolved.
#
# Given a PR export directory (ai-docs/reviews-pr-XXX) and an inclusive range
# of issue numbers, this script:
#   1. Updates each issue markdown file to mark the status as resolved.
#   2. Resolves the corresponding GitHub review threads via `gh`.
#   3. Updates the `_summary.md` checklist to reflect the resolved issues and
#      refreshes the resolved/unresolved counters.
#
# Usage:
#   scripts/resolve_pr_issues.sh \
#     --pr-dir ai-docs/reviews-pr-277 \
#     --from 11 \
#     --to 22 \
#     [--dry-run]
#
# Requirements:
#   - GitHub CLI (`gh`) authenticated with repo scope (unless --dry-run)
#   - Python 3 (for safe in-place edits)

pr_dir=""
from_issue=""
to_issue=""
dry_run=false

die() { echo "Error: $*" >&2; exit 1; }

need_cmd() { command -v "$1" >/dev/null 2>&1 || die "Missing required command: $1"; }

usage() {
  sed -n '1,60p' "$0" | sed 's/^# \{0,1\}//'
  exit 0
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --pr-dir) pr_dir=${2:-}; shift 2 ;;
      --from)   from_issue=${2:-}; shift 2 ;;
      --to)     to_issue=${2:-}; shift 2 ;;
      --dry-run) dry_run=true; shift ;;
      -h|--help) usage ;;
      *) die "Unknown arg: $1" ;;
    esac
  done
}

validate_args() {
  [[ -n "$pr_dir" ]] || die "Provide --pr-dir"
  [[ -n "$from_issue" ]] || die "Provide --from"
  [[ -n "$to_issue" ]] || die "Provide --to"
  [[ "$from_issue" =~ ^[0-9]+$ ]] || die "--from must be a number"
  [[ "$to_issue" =~ ^[0-9]+$ ]] || die "--to must be a number"
  if (( 10#$from_issue > 10#$to_issue )); then
    die "--from cannot be greater than --to"
  fi
  [[ -d "$pr_dir" ]] || die "PR directory not found: $pr_dir"
  [[ -f "$pr_dir/_summary.md" ]] || die "Summary file not found in $pr_dir"
  [[ -d "$pr_dir/issues" ]] || die "Issues directory not found in $pr_dir"
  need_cmd python3
  if ! $dry_run; then
    need_cmd gh
  fi
}

zero_pad() {
  printf "%03d" "$1"
}

extract_thread_ids() {
  local file="$1"
  grep -Eo 'Thread ID: `[^`]+`' "$file" | sed -E 's/.*`([^`]+)`.*/\1/' || true
  grep -Eo "threadId='[^']+'" "$file" | sed -E "s/.*'([^']+)'.*/\1/" || true
}

mark_issue_resolved() {
  local file="$1"
  python3 - "$file" <<'PY'
import re, sys
path = sys.argv[1]
text = open(path, encoding="utf-8").read()
replacement = r"\1[x] RESOLVED"
new_text, count = re.subn(r"(\*\*Status:\*\*\s*-\s*)\[[xX ]\]\s*(?:UNRESOLVED|RESOLVED(?:\s*✓)?)", replacement, text, count=1)
if count:
    with open(path, "w", encoding="utf-8") as fh:
        fh.write(new_text)
PY
}

update_summary_checkbox() {
  local summary="$1"
  local issue="$2"
  local padded="$3"
  python3 - "$summary" "$issue" "$padded" <<'PY'
import re, sys
summary, issue_num, padded = sys.argv[1:4]
text = open(summary, encoding="utf-8").read()
pattern = rf"(- \[)[ xX](\] \[Issue {int(issue_num)}\]\(issues/issue_{padded}\.md\))"
new_text = re.sub(pattern, r"\1x\2", text, count=1)
if new_text != text:
    with open(summary, "w", encoding="utf-8") as fh:
        fh.write(new_text)
PY
}

refresh_summary_counts() {
  local summary="$1"
  python3 - "$summary" <<'PY'
import re, sys
summary_path = sys.argv[1]
text = open(summary_path, encoding="utf-8").read()
resolved = len(re.findall(r"- \[[xX]\] \[Issue \d+\]\(issues/issue_\d+\.md\)", text))
unresolved = len(re.findall(r"- \[ \] \[Issue \d+\]\(issues/issue_\d+\.md\)", text))
text = re.sub(r"(\*\*Resolved issues:\*\*\s*)(\d+)", lambda m: m.group(1) + str(resolved), text)
text = re.sub(r"(\*\*Unresolved issues:\*\*\s*)(\d+)", lambda m: m.group(1) + str(unresolved), text)
with open(summary_path, "w", encoding="utf-8") as fh:
    fh.write(text)
PY
}

resolve_threads() {
  local issue_file="$1"
  local ids=()
  while IFS= read -r id; do
    [[ -n "$id" ]] && ids+=("$id")
  done < <(extract_thread_ids "$issue_file" | sort -u)

  if [[ ${#ids[@]} -eq 0 ]]; then
    echo "   ⚠️  No thread IDs found in $(basename "$issue_file")"
    return
  fi

  for id in "${ids[@]}"; do
    echo "   📡 Resolving thread $id"
    if $dry_run; then
      echo "     (dry-run) skipping gh api call"
    else
      gh api graphql \
        -f query='mutation($threadId: ID!) { resolveReviewThread(input: { threadId: $threadId }) { thread { isResolved } } }' \
        -F threadId="$id"
    fi
  done
}

main() {
  parse_args "$@"
  validate_args

  local summary_file="$pr_dir/_summary.md"
  local issues_dir="$pr_dir/issues"

  echo "📁 PR dir: $pr_dir"
  echo "🔢 Range: ${from_issue}-${to_issue}"
  echo "🧪 Dry run: $dry_run"

  for (( num=10#$from_issue; num<=10#$to_issue; num++ )); do
    local padded
    padded=$(zero_pad "$num")
    local issue_file="${issues_dir}/issue_${padded}.md"

    if [[ ! -f "$issue_file" ]]; then
      echo "❌ Missing issue file: $issue_file"
      continue
    fi

    echo "➡️  Processing issue_${padded}.md"
    mark_issue_resolved "$issue_file"
    update_summary_checkbox "$summary_file" "$num" "$padded"
    resolve_threads "$issue_file"
  done

  refresh_summary_counts "$summary_file"
  echo "✅ Completed."
}

main "$@"
