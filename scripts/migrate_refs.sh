#!/usr/bin/env bash
set -euo pipefail

# Compozy: Legacy directives → ID-based references migration (docs/examples/READMEs)
# - Scans for occurrences of $ref/$use/$merge and resource:: patterns
# - Applies conservative replacements aligned with tasks/prd-refs/0001/0003
# - Prints all modified files for review. Idempotent.

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

YES=${YES:-false}
DRY_RUN=${DRY_RUN:-false}

usage() {
  cat <<EOF
Usage: scripts/migrate_refs.sh [--yes|-y] [--dry-run]

Environment variables:
  YES=true      Non-interactive mode (assume yes)
  DRY_RUN=true  Show changes without writing (requires 'git' diff)
EOF
}

while [[ ${1:-} =~ ^- ]]; do
  case "$1" in
    --yes|-y) YES=true ; shift ;;
    --dry-run) DRY_RUN=true ; shift ;;
    --help|-h) usage ; exit 0 ;;
    *) echo "Unknown flag: $1" ; usage ; exit 1 ;;
  esac
done

echo "Scanning for legacy directive usage under docs/, examples/, and engine/*/README.md ..."

# Build file list (markdown + yaml) across docs/examples and engine READMEs
MATCHED_FILES=$( (
  grep -rl --include=*.{md,mdx,yml,yaml} -E '(\$ref|\$use|\$merge|resource::|local::)' docs/ examples/ 2>/dev/null || true
  find engine -type f -maxdepth 3 -name 'README.md' -print0 | xargs -0 rg -l -N -S '(\$ref|\$use|\$merge|resource::|local::|pkg/ref|LoadConfigWithEvaluator|ref\.Evaluator)' -- 2>/dev/null || true
) | sort -u )

if [[ -z "${MATCHED_FILES}" ]]; then
  echo "No files with legacy directive syntax found."
  exit 0
fi

echo "The following files contain legacy syntax:"
echo "${MATCHED_FILES}" | sed 's/^/ - /'

if [[ "$YES" != true ]]; then
  echo
  read -rp "Proceed with automated replacements? [y/N] " CONFIRM
  if [[ ! "${CONFIRM}" =~ ^[Yy]$ ]]; then
    echo "Aborted by user."
    exit 1
  fi
fi

MODIFIED=()
while IFS= read -r file; do
  [[ -z "$file" ]] && continue
  # Create a backup for safety
  cp "$file" "$file.bak"

  # Replace common demo patterns (conservative)
  # $use: agent(resource::agent::#(id=="X")) → agent: X
  sed -E -i '' \
    's/\$use:[[:space:]]*agent\(resource::agent::#\(id=="([^"]+)"\)\)/agent: \1/g' "$file"
  # local:: variant
  sed -E -i '' \
    's/\$use:[[:space:]]*agent\(local::agents\.#\(id=="([^"]+)"\)\)/agent: \1/g' "$file"

  # $use: tool(resource::tool::#(id=="X")) → tool: X
  sed -E -i '' \
    's/\$use:[[:space:]]*tool\(resource::tool::#\(id=="([^"]+)"\)\)/tool: \1/g' "$file"
  # local:: variant
  sed -E -i '' \
    's/\$use:[[:space:]]*tool\(local::tools\.#\(id=="([^"]+)"\)\)/tool: \1/g' "$file"

  # Single-quoted variants
  sed -E -i '' \
    "s/\$use:[[:space:]]*agent\(resource::agent::#\(id==\\'([^\\']+)\\'\)\)/agent: \\1/g" "$file"
  sed -E -i '' \
    "s/\$use:[[:space:]]*tool\(resource::tool::#\(id==\\'([^\\']+)\\'\)\)/tool: \\1/g" "$file"
  # local:: single-quoted variants
  sed -E -i '' \
    "s/\$use:[[:space:]]*agent\(local::agents\.#\(id==\\'([^\\']+)\\'\)\)/agent: \\1/g" "$file"
  sed -E -i '' \
    "s/\$use:[[:space:]]*tool\(local::tools\.#\(id==\\'([^\\']+)\\'\)\)/tool: \\1/g" "$file"

  # Generic $use fallback: $use: agent(X) -> agent: X ; $use: tool(X) -> tool: X
  sed -E -i '' \
    's/\$use:[[:space:]]*agent\(([^)]+)\)/agent: \1/g' "$file"
  sed -E -i '' \
    's/\$use:[[:space:]]*tool\(([^)]+)\)/tool: \1/g' "$file"

  # $ref: resource::<type>::#(id=="X") → X (context-dependent; keeps example minimal)
  sed -E -i '' \
    's/\$ref:[[:space:]]*resource::(agent|tool|workflow|schema|memory)::#\(id=="([^"]+)"\)/\2/g' "$file"
  # local:: double-quoted
  sed -E -i '' \
    's/\$ref:[[:space:]]*local::(agents|tools|workflows|schemas|memory|tasks)\.#\(id=="([^"]+)"\)/\2/g' "$file"
  # Single-quoted $ref
  sed -E -i '' \
    "s/\$ref:[[:space:]]*resource::(agent|tool|workflow|schema|memory)::#\(id==\\'([^\\']+)\\'\)/\\2/g" "$file"
  # local:: single-quoted
  sed -E -i '' \
    "s/\$ref:[[:space:]]*local::(agents|tools|workflows|schemas|memory|tasks)\.#\(id==\\'([^\\']+)\\'\)/\\2/g" "$file"

  # Fallback: transform '- $ref: VALUE' → '- VALUE' and 'key:\n  $ref: VALUE' → 'key: VALUE'
  sed -E -i '' \
    's/^([[:space:]]*)-([[:space:]]*)\$ref:[[:space:]]*(.+)$/\1- \3/g' "$file"
  sed -E -i '' \
    's/^([[:space:]]*)([a-zA-Z0-9_]+):[[:space:]]*\n\1[[:space:]]*\$ref:[[:space:]]*(.+)$/\1\2: \3/g' "$file"

  # Remove $merge lines (legacy, no replacement)
  sed -E -i '' \
    '/^[[:space:]]*\$merge:/d' "$file"

  # resource::<type>::#(id=="X") → X in inline text or examples
  sed -E -i '' \
    's/resource::(agent|tool|workflow|schema|memory)::#\(id=="([^"]+)"\)/\2/g' "$file"
  sed -E -i '' \
    "s/resource::(agent|tool|workflow|schema|memory)::#\(id==\\'([^\\']+)\\'\)/\\2/g" "$file"
  # local:: inline text replacements
  # local:: double-quoted (supports id="X" or id=="X")
  sed -E -i '' \
    's/local::(agents|tools|workflows|schemas|memory|tasks)\.#\(id==?"([^"]+)"\)/\2/g' "$file"
  # local:: single-quoted (supports id='X' or id=='X')
  sed -E -i '' \
    "s/local::(agents|tools|workflows|schemas|memory|tasks)\.#\(id==?'([^']+)'\)/\\2/g" "$file"

  # Explicit key context fixes: 'agent:' and 'tool:' lines with local:: refs → plain IDs
  sed -E -i '' \
    's/^([[:space:]]*agent:[[:space:]]*)local::agents\.#\(id==?"([^"]+)"\)/\1\2/g' "$file"
  sed -E -i '' \
    "s/^([[:space:]]*agent:[[:space:]]*)local::agents\\.\#\\(id==?'([^']+)'\\)/\\1\\2/g" "$file"
  sed -E -i '' \
    's/^([[:space:]]*tool:[[:space:]]*)local::tools\.#\(id==?"([^"]+)"\)/\1\2/g' "$file"
  sed -E -i '' \
    "s/^([[:space:]]*tool:[[:space:]]*)local::tools\\.\#\\(id==?'([^']+)'\\)/\\1\\2/g" "$file"

  # Specific README deprecations
  # - Remove/replace pkg/ref evaluator references and legacy function names
  sed -E -i '' \
    -e 's/LoadConfigWithEvaluator\[/LoadConfig\[/g' \
    -e 's/ref\\.Evaluator/ID-based compile\/link/g' \
    -e 's/pkg\/ref/ResourceStore/g' \
    "$file"

  # Autoload README phrasing adjustments (inline text)
  sed -E -i '' \
    -e 's/Resource resolution for `pkg\/ref` integration/ID-based selectors with ResourceStore/g' \
    -e 's/\*\*Resource Resolution\*\*: Integration with `pkg\/ref` for `resource::` scope references/**Resource Linking**: ID-based selectors compiled via ResourceStore/g' \
    -e 's/Now you can use resource:: references in configurations/Use ID-based selectors in configurations/g' \
    -e 's/Example: resource::agent\/code-assistant/Example: agent: code-assistant/g' \
    "$file"

  # Generic docs statement: replace narrative mentions of resource:: scope
  sed -E -i '' \
    -e 's/`resource::`/ID-based/g' \
    -e 's/resource:: scope references/ID-based selectors/g' \
    "$file"

  # Track modified files (diff non-empty)
  if ! diff -q "$file.bak" "$file" >/dev/null 2>&1; then
    MODIFIED+=("$file")
    if [[ "$DRY_RUN" == true ]]; then
      echo "--- $file (proposed)" && git --no-pager diff -- "$file" || true
      # Revert changes in dry-run, keep backup cleanup logic consistent
      mv "$file.bak" "$file"
    else
      rm -f "$file.bak"
    fi
  else
    rm -f "$file.bak"
  fi
done <<< "$MATCHED_FILES"

echo
if ((${#MODIFIED[@]})); then
  echo "Modified files:"
  for f in "${MODIFIED[@]}"; do
    echo " - $f"
  done
  if [[ "$DRY_RUN" == true ]]; then
    echo "(dry-run) No files written."
  else
    echo
    echo "Review the changes."
  fi
else
  echo "No changes were necessary."
fi
