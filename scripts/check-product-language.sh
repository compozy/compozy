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
    desktop/src-tauri/pages
    desktop/src-tauri/tauri.conf.json
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

set +e
matches="$(rg --line-number --no-heading --pcre2 '(?<!X-)\bCompozy\b(?! Network)' -- "${paths[@]}" 2>&1)"
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
echo "Retired product-language 'Compozy' found. Use 'CompozyOS'; preserve only 'Compozy Network' and X-Compozy-* headers." >&2
exit 1
