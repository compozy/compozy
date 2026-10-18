#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if (($# > 0)); then
  paths=("$@")
else
  paths=(
    COPY.md
    DESIGN.md
    MIGRATION_GUIDE.md
    PRODUCT.md
    README.md
    RELEASE_BODY.md
    RELEASE_NOTES.md
    SECURITY.md
    .release-notes
    .goreleaser.release-footer.md.tmpl
    docs/_memory/glossary.md
    packages/site
    web
    skills/compozy
    extensions
    sdk
    catalog
    .env.example
    .compozy/loops/implement-tasks/loop.yaml
    cmd/compozy
    cmd/compozy-codegen
    cmd/compozy-manifest-check
    internal/cli
    internal/agentidentity
    internal/store/migration_refusal.go
    internal/update
    desktop/pages
    .github/actions
    .github/workflows
    .goreleaser.yml
    magefiles
    scripts/install.template.sh
    scripts/release-plan-contract.sh
    scripts/sync-design-md.mjs
  )
fi

cd "$repo_root"

# Skip API tokens, never whole lines: adjacent product prose must still fail.
api_identifiers='(?x:
  \bCompozy\.(?:connect|start|__registry)\b
  | \bnew[\t ]+Compozy(?=[\t ]*\()
  | \bclass[\t ]+Compozy(?=[\t ]+(?:extends|implements)\b|[\t ]*\{)
  | \b(?:import|export)[\t ]+(?:type[\t ]+)?\{[\w\s,]*?\bCompozy(?=[\t ]*(?:as\b|[,}]))
  | \b(?:Promise|HeyApiRegistry)<[\t ]*Compozy(?=[\t ]*[>|])
  | \b(?:let|const|var)[\t ]+[\w\x24]+:[\t ]*Compozy(?=[\t ]*[|;=])
  | \bOperationStrategy\.single\(\{\s*root:[\t ]*"Compozy"
  | \bCompozy[\t ]+\w+[\t ]+\x60json:"compozy"\x60
)'
pattern="${api_identifiers}(*SKIP)(*F)|(?<!X-)\bCompozy\b(?! Network)"

set +e
matches="$(rg --multiline --with-filename --line-number --no-heading --pcre2 "$pattern" -- "${paths[@]}" 2>&1)"
status=$?
set -e

if ((status == 1)); then
  echo "Product language is consistent: CompozyOS is the product and compozy is the command."
  exit 0
fi
if ((status != 0)); then
  echo "$matches" >&2
  echo "Product-language scan failed before it could classify the result." >&2
  exit "$status"
fi

echo "$matches" >&2
echo "Retired product-language 'Compozy' found. Use 'CompozyOS'; preserve public SDK identifiers and X-Compozy-* headers." >&2
exit 1
