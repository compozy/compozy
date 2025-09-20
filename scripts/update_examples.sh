#!/usr/bin/env bash
set -euo pipefail

# Update Compozy examples: remove $use directives and simplify task route $ref where safe.
# Note: Schema $ref replacements are handled manually per-file to avoid YAML corruption.

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
EX_DIR="$ROOT_DIR/examples"

find "$EX_DIR" -type f -name "*.yaml" | while IFS= read -r file; do
  # macOS/BSD sed needs a separate arg for -i backup suffix
  if sed --version >/dev/null 2>&1; then
    SED=(sed -E -i '')
  else
    SED=(sed -E -i '')
  fi
  # Replace $use: agent(local::agents.#(id="X")) -> agent: X
  "${SED[@]}" -e 's/\$use:\s*agent\(local::agents\.#\(id==?"([^")]+)"\)\)/agent: \1/g' "$file"
  "${SED[@]}" -e 's/\$use:\s*agent\(resource::agent::#\(id==?"([^")]+)"\)\)/agent: \1/g' "$file"

  # Replace $use: tool(local::tools.#(id="X")) -> tool: X
  "${SED[@]}" -e 's/\$use:\s*tool\(local::tools\.#\(id==?"([^")]+)"\)\)/tool: \1/g' "$file"
done

echo "Updated \$use directives in examples. Review schema \$ref manually if present."
