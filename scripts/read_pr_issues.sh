#!/usr/bin/env bash
set -euo pipefail

# read_pr_issues.sh — Read and display PR review issues in a concatenating way.
#
# This script reads issue files from ai-docs/reviews-pr-XXX directories and displays
# them as one continuous output with clear separators for easy review while working on PR fixes.
#
# Usage:
#   scripts/read_pr_issues.sh \
#     --pr 277 \
#     --type issue \
#     --from 1 \
#     --to 10
#
#   # Or read all issues of a type:
#   scripts/read_pr_issues.sh \
#     --pr 277 \
#     --type issue \
#     --all
#
# Valid types: issue, duplicated, outside, nitpick

pr_number=""
issue_type=""
from_issue=""
to_issue=""
read_all=false

die() { echo "Error: $*" >&2; exit 1; }

usage() {
  cat <<'EOF'
Usage: scripts/read_pr_issues.sh --pr PR_NUMBER --type TYPE [--from START --to END | --all]

Arguments:
  --pr PR_NUMBER    Pull request number (e.g., 277)
  --type TYPE       Type of items: issue, duplicated, outside, nitpick
  --from START      Start issue number (inclusive)
  --to END          End issue number (inclusive)
  --all             Read all issues of the specified type
  -h, --help        Show this help

Examples:
  # Read issues 1-10 from PR 277
  scripts/read_pr_issues.sh --pr 277 --type issue --from 1 --to 10

  # Read all issues from PR 277
  scripts/read_pr_issues.sh --pr 277 --type issue --all

  # Read duplicated comments 5-15 from PR 277
  scripts/read_pr_issues.sh --pr 277 --type duplicated --from 5 --to 15
EOF
  exit 0
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --pr)
        [[ $# -ge 2 && ${2:0:2} != -- ]] || die "--pr requires a value"
        pr_number=$2
        shift 2
        ;;
      --type)
        [[ $# -ge 2 && ${2:0:2} != -- ]] || die "--type requires a value"
        issue_type=$2
        shift 2
        ;;
      --from)
        [[ $# -ge 2 && ${2:0:2} != -- ]] || die "--from requires a value"
        from_issue=$2
        shift 2
        ;;
      --to)
        [[ $# -ge 2 && ${2:0:2} != -- ]] || die "--to requires a value"
        to_issue=$2
        shift 2
        ;;
      --all) read_all=true; shift ;;
      -h|--help) usage ;;
      *) die "Unknown arg: $1" ;;
    esac
  done
}

validate_args() {
  [[ -n "$pr_number" ]] || die "Provide --pr"
  [[ -n "$issue_type" ]] || die "Provide --type"

  # Validate issue type
  case "$issue_type" in
    issue|duplicated|outside|nitpick) ;;
    *) die "Invalid --type: $issue_type. Must be one of: issue, duplicated, outside, nitpick" ;;
  esac

  if $read_all; then
    [[ -z "$from_issue" && -z "$to_issue" ]] || die "--all cannot be used with --from or --to"
  else
    [[ -n "$from_issue" ]] || die "Provide --from (or use --all)"
    [[ -n "$to_issue" ]] || die "Provide --to (or use --all)"
    [[ "$from_issue" =~ ^[0-9]+$ ]] || die "--from must be a number"
    [[ "$to_issue" =~ ^[0-9]+$ ]] || die "--to must be a number"
    if (( 10#$from_issue > 10#$to_issue )); then
      die "--from cannot be greater than --to"
    fi
  fi
}

zero_pad() {
  printf "%03d" "$1"
}

get_directory_name() {
  local type="$1"
  case "$type" in
    issue) echo "issues" ;;
    duplicated) echo "duplicated" ;;
    outside) echo "outside" ;;
    nitpick) echo "nitpicks" ;;
    *) die "Unknown type: $type" ;;
  esac
}

print_separator() {
  echo
  echo "$(printf '━%.0s' {1..80})"
  echo
}

display_file() {
  local file="$1"
  local title="$2"

  print_separator
  echo "📄 $title"
  print_separator
  cat "$file"
  echo
}

main() {
  parse_args "$@"
  validate_args

  local pr_dir="ai-docs/reviews-pr-$pr_number"
  local dir_name
  dir_name=$(get_directory_name "$issue_type")
  local target_dir="$pr_dir/$dir_name"

  [[ -d "$pr_dir" ]] || die "PR directory not found: $pr_dir"
  [[ -d "$target_dir" ]] || die "Target directory not found: $target_dir"

  echo "📁 Reading $issue_type files from $target_dir"
  echo

  if $read_all; then
    # Read all files in the directory
    local first=true
    for file in "$target_dir"/*.md; do
      [[ -f "$file" ]] || continue

      if $first; then
        first=false
      else
        print_separator
      fi

      local basename
      basename=$(basename "$file" .md)
      display_file "$file" "$basename"
    done
  else
    # Read specific range
    # Files are named: NNN-<file_path>.md (e.g., 001-engine_project_validators.go.md)
    local first=true
    for (( num=10#$from_issue; num<=10#$to_issue; num++ )); do
      local padded
      padded=$(zero_pad "$num")

      # Find file matching the number prefix
      local files
      files=("$target_dir/${padded}"-*.md)

      if [[ ! -f "${files[0]}" ]]; then
        echo "⚠️  No file found with prefix: ${padded}- in $target_dir"
        continue
      fi

      # Use the first matching file (should only be one)
      local file="${files[0]}"

      if $first; then
        first=false
      else
        print_separator
      fi

      local basename
      basename=$(basename "$file" .md)
      display_file "$file" "$basename (Issue $num)"
    done
  fi

  print_separator
  echo "✅ Reading completed."
}

main "$@"
