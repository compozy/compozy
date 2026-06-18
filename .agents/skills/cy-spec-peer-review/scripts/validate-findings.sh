#!/usr/bin/env bash
set -euo pipefail

# Keep sibling peer-review validator copies in sync.

usage() {
  printf 'usage: %s --kind implementation|techspec --round N --path FILE\n' "$0" >&2
}

kind=""
round=""
path=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --kind)
      kind="${2:-}"
      shift 2
      ;;
    --round)
      round="${2:-}"
      shift 2
      ;;
    --path)
      path="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      printf 'unknown argument: %s\n' "$1" >&2
      usage
      exit 2
      ;;
  esac
done

if [[ -z "$kind" || -z "$round" || -z "$path" ]]; then
  usage
  exit 2
fi

if [[ "$kind" != "implementation" && "$kind" != "techspec" ]]; then
  printf 'invalid --kind: %s\n' "$kind" >&2
  exit 2
fi

if [[ ! "$round" =~ ^[0-9]+$ ]]; then
  printf 'invalid --round: %s\n' "$round" >&2
  exit 2
fi

if [[ ! -s "$path" ]]; then
  printf 'findings file missing or empty: %s\n' "$path" >&2
  exit 1
fi

require_line() {
  local pattern="$1"
  local description="$2"
  if ! grep -Eq "$pattern" "$path"; then
    printf 'missing %s in %s\n' "$description" "$path" >&2
    exit 1
  fi
}

require_absent() {
  local pattern="$1"
  local description="$2"
  if grep -Eiq "$pattern" "$path"; then
    printf 'forbidden placeholder detected (%s) in %s\n' "$description" "$path" >&2
    exit 1
  fi
}

first_line="$(sed -n '1p' "$path")"
if [[ "$first_line" != "---" ]]; then
  printf 'findings file must start with YAML frontmatter: %s\n' "$path" >&2
  exit 1
fi

frontmatter_delimiter_count="$(grep -c '^---$' "$path")"
if [[ "$frontmatter_delimiter_count" -lt 2 ]]; then
  printf 'findings file must close YAML frontmatter: %s\n' "$path" >&2
  exit 1
fi

section_body() {
  local heading="$1"
  awk -v heading="$heading" '
    $0 == heading {in_section = 1; next}
    in_section && /^# / {exit}
    in_section {print}
  ' "$path"
}

require_section_item_or_none() {
  local heading="$1"
  local prefix="$2"
  local body
  body="$(section_body "$heading")"
  if printf '%s\n' "$body" | grep -Eq '^[[:space:]]*None\.[[:space:]]*$'; then
    return
  fi
  if ! printf '%s\n' "$body" | grep -Eq "^## ${prefix}-[0-9][0-9][0-9][[:space:]]"; then
    printf 'section %s must contain None. or at least one %s-NNN item in %s\n' "$heading" "$prefix" "$path" >&2
    exit 1
  fi
}

require_line '^schema_version:[[:space:]]*1[[:space:]]*$' 'schema_version: 1'
require_line "^review_kind:[[:space:]]*$kind[[:space:]]*$" "review_kind: $kind"
require_line "^round:[[:space:]]*$round[[:space:]]*$" "round: $round"
require_line '^reviewer_runtime:[[:space:]]*[^[:space:]].*$' 'reviewer_runtime'
require_line '^reviewer_model:[[:space:]]*[^[:space:]].*$' 'reviewer_model'
require_line '^generated_at:[[:space:]]*[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(Z|[+-][0-9]{2}:?[0-9]{2})[[:space:]]*$' 'generated_at ISO-8601 timestamp'
require_line '^# Summary[[:space:]]*$' '# Summary'
require_line '^# Blockers[[:space:]]*$' '# Blockers'
require_line '^# Nits[[:space:]]*$' '# Nits'
require_line '^# Evidence[[:space:]]*$' '# Evidence'
require_line '^# Deferred Or Follow-Up[[:space:]]*$' '# Deferred Or Follow-Up'

if [[ "$kind" == "implementation" ]]; then
  require_line '^verdict:[[:space:]]*(SHIP|FIX_BEFORE_SHIP|REWORK)[[:space:]]*$' 'implementation verdict'
  require_line '^# Risks[[:space:]]*$' '# Risks'
  require_section_item_or_none '# Risks' 'R'
else
  require_line '^readiness:[[:space:]]*(READY|BLOCKED|NEEDS_REWORK)[[:space:]]*$' 'techspec readiness'
fi

require_section_item_or_none '# Blockers' 'B'
require_section_item_or_none '# Nits' 'N'
require_absent '^[[:space:]]*(TBD|TODO|PLACEHOLDER)\.?[[:space:]]*$' 'bare TBD/TODO/PLACEHOLDER section body'
printf 'findings validation passed: %s\n' "$path"
