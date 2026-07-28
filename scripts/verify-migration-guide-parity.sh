#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
root_guide="$repo_root/MIGRATION_GUIDE.md"
site_guide="$repo_root/packages/site/content/runtime/migration/index.mdx"
scratch="$(mktemp -d "${TMPDIR:-/tmp}/compozy-migration-guide.XXXXXX")"
trap 'rm -rf -- "$scratch"' EXIT

required_ids=(
  command-map
  config-translation
  config-migrator
  skills
  deferred-and-removed
  license
  go-library
  domain-and-clean-state
)

normalize_guide() {
  local source="$1"
  local destination="$2"

  awk '
    NR == 1 && $0 == "---" { frontmatter = 1; next }
    frontmatter && $0 == "---" { frontmatter = 0; next }
    frontmatter { next }
    /^<!-- site-only:start -->$/ { site_only = 1; next }
    /^<!-- site-only:end -->$/ { site_only = 0; next }
    site_only { next }
    /^import[[:space:]].*;?$/ { next }
    {
      sub(/\r$/, "")
      sub(/[[:space:]]+$/, "")
      print
    }
  ' "$source" >"$destination"

  while [[ -s "$destination" ]] && [[ "$(head -n 1 "$destination")" == "" ]]; do
    tail -n +2 "$destination" >"$destination.next"
    mv "$destination.next" "$destination"
  done
  while [[ -s "$destination" ]] && [[ "$(tail -n 1 "$destination")" == "" ]]; do
    sed '$d' "$destination" >"$destination.next"
    mv "$destination.next" "$destination"
  done
}

for guide in "$root_guide" "$site_guide"; do
  if [[ ! -f "$guide" ]]; then
    echo "migration-guide-check: missing guide: $guide" >&2
    exit 1
  fi
done

normalize_guide "$root_guide" "$scratch/root.md"
normalize_guide "$site_guide" "$scratch/site.md"

for section_id in "${required_ids[@]}"; do
  marker="<a id=\"$section_id\"></a>"
  for normalized in "$scratch/root.md" "$scratch/site.md"; do
    count="$(grep -Fxc "$marker" "$normalized" || true)"
    if [[ "$count" != "1" ]]; then
      echo "migration-guide-check: $normalized has $count occurrences of required section id $section_id; want 1" >&2
      exit 1
    fi
  done
done

if ! diff -u \
  --label MIGRATION_GUIDE.md \
  --label packages/site/content/runtime/migration/index.mdx \
  "$scratch/root.md" \
  "$scratch/site.md"; then
  echo "migration-guide-check: normalized guide content differs" >&2
  exit 1
fi

echo "migration-guide-check: root and site guides match across all eight required sections"
