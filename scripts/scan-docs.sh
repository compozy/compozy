#!/usr/bin/env bash
set -euo pipefail

echo "Scanning docs and tool READMEs for legacy @compozy/tool- references..."

# Scan docs (md/mdx) excluding native-tools migration doc
matches_docs=$(rg -n "@compozy/tool-" docs -g "**/*.{md,mdx}" -S --hidden -g "!native-tools.md" || true)

# Scan only tool package READMEs (exclude root README and package.json)
matches_tools=$(rg -n "@compozy/tool-" tools/*/README.md -S --hidden || true)

matches="${matches_docs}
${matches_tools}"

# Trim whitespace to detect real matches
trimmed=$(printf "%s" "$matches" | tr -d '\n\r\t ')
if [[ -n "$trimmed" ]]; then
  echo "Found legacy references:\n$matches" >&2
  exit 1
fi
echo "OK: no legacy references found."
