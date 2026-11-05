#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Public surfaces scanned by default.
public_paths=(
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

# PRODUCT_LANGUAGE_PATH_AS names the repository-relative path a single explicit fixture stands for,
# so the path-scoped rules below classify it as that file.
explicit=0
if (($# > 0)); then
  explicit=1
  paths=("$@")
else
  paths=("${public_paths[@]}")
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

failed=0
if ((status == 1)); then
  :
elif ((status != 0)); then
  echo "$matches" >&2
  echo "Product-language scan failed before it could classify the result." >&2
  exit "$status"
else
  echo "$matches" >&2
  echo "Retired product-language 'Compozy' found. Use 'CompozyOS'; preserve public SDK identifiers and X-Compozy-* headers." >&2
  failed=1
fi

# Retired agent-kind vocabulary (#1199). The space form "stays on" is ordinary English and is not
# matched. The v0.5.1 stays_on aliases are removed (#1239). Rule A keeps the retired word out of
# production Go except the files that migrate persisted user state or explain rejections; Rule B
# keeps it off public surfaces.
retired_kind='(?i)stays[-_]?on'
# Rule B skips immutable history identifiers only: the shipped stays-on-hardening spec, its branch,
# reports and artifacts, dated reports, and content-addressed QA scenario, charter and bug IDs.
history_refs='(?x:
  stays-on-hardening
  | \d{4}-\d{2}-\d{2}-stays-on-[\w-]+\.md
  | \bBUG-\d{8}-[\w-]*stays-on[\w-]*
  | \b(?:MS|CH)-[\w-]*stays-on[\w-]*
)'
rule_b_pattern="${history_refs}(*SKIP)(*F)|${retired_kind}"

# Production Go files that may name stays_on: the user-state migrations and the rejection messages
# that point at always_on.
rule_a_allowed=(
  internal/config/agent_kind_retired.go
  internal/config/agent_kind_rewrite.go
)

rule_a_in_scope() {
  local rel="$1"
  [[ "$rel" == *.go && "$rel" != *_test.go && "$rel" != */testdata/* && "$rel" != testdata/* ]] || return 1
  [[ "$rel" == internal/* || "$rel" == cmd/* || "$rel" == sdk/go/* ]] || return 1
  [[ "$rel" == sdk/go/client/* ]] && return 1
  local allowed
  for allowed in "${rule_a_allowed[@]}"; do
    [[ "$rel" == "$allowed" ]] && return 1
  done
  return 0
}

rule_b_in_scope() {
  local rel="$1" root
  for root in "${public_paths[@]}" docs/qa/scenarios; do
    [[ "$rel" == "$root" || "$rel" == "$root"/* ]] && return 0
  done
  return 1
}

rule_b_allowed() {
  local rel="$1"
  case "$rel" in
    RELEASE_NOTES.md | RELEASE_BODY.md | CHANGELOG.md | .release-notes/archive/* | .release-notes/always-on-aliases-removed-v0.5.2.md) return 0 ;;
    sdk/client/* | sdk/go/client/* | openapi/compozy.json | internal/tools/builtin/testdata/native-tool-catalog.json) return 0 ;;
    docs/_memory/glossary.md | COPY.md | MIGRATION_GUIDE.md | packages/site/content/docs/migration/index.mdx) return 0 ;;
    skills/compozy/references/agent-definitions.md | packages/site/content/docs/agents/definitions.mdx) return 0 ;;
    internal/cli/*_test.go | extensions/*_test.go) return 0 ;;
    web/*/__tests__/* | web/*.test.*) return 0 ;;
    web/proto/review/*) return 0 ;;
    scripts/check-product-language.sh | scripts/product_language_integration_test.go) return 0 ;;
  esac
  local allowed
  for allowed in "${rule_a_allowed[@]}"; do
    [[ "$rel" == "$allowed" ]] && return 0
  done
  return 1
}

relative_path() {
  local file="$1"
  if ((explicit)) && [[ -n "${PRODUCT_LANGUAGE_PATH_AS:-}" ]]; then
    printf '%s\n' "$PRODUCT_LANGUAGE_PATH_AS"
    return
  fi
  printf '%s\n' "${file#./}"
}

kind_findings=()
if ((explicit)); then
  rule_a_paths=("${paths[@]}")
  rule_b_paths=("${paths[@]}")
else
  rule_a_paths=(internal cmd sdk/go)
  rule_b_paths=("${paths[@]}" docs/qa/scenarios)
fi

# rg exits 1 for no match; any higher status means the scan itself failed, so fail closed.
kind_scan() {
  local label="$1" output status
  shift
  set +e
  output="$(rg --files-with-matches --pcre2 "$@" 2>&1)"
  status=$?
  set -e
  if ((status > 1)); then
    echo "$output" >&2
    echo "Retired agent-kind scan ($label) failed before it could classify the result." >&2
    exit "$status"
  fi
  ((status == 0)) && printf '%s\n' "$output"
  return 0
}

rule_a_files="$(kind_scan "Rule A" --glob '*.go' "${retired_kind}|\bCanonicalAgentKind\b" -- "${rule_a_paths[@]}")"
while IFS= read -r file; do
  [[ -n "$file" ]] || continue
  rel="$(relative_path "$file")"
  rule_a_in_scope "$rel" || continue
  kind_findings+=("Rule A: $file ($rel) is production Go that names stays_on or calls CanonicalAgentKind outside the migration and rejection files")
done <<<"$rule_a_files"

rule_b_files="$(kind_scan "Rule B" "$rule_b_pattern" -- "${rule_b_paths[@]}")"
while IFS= read -r file; do
  [[ -n "$file" ]] || continue
  rel="$(relative_path "$file")"
  rule_b_in_scope "$rel" || continue
  rule_b_allowed "$rel" && continue
  kind_findings+=("Rule B: $file ($rel) uses the retired stays-on vocabulary")
done <<<"$rule_b_files"

if ((${#kind_findings[@]} > 0)); then
  printf '%s\n' "${kind_findings[@]}" >&2
  echo "Retired agent-kind vocabulary found. Use always_on / Always-on. Production Go may name stays_on only in the migration and rejection files; public surfaces only in the listed migration docs, release notes, tests and history identifiers." >&2
  failed=1
fi

if ((failed)); then
  exit 1
fi
echo "Product language is consistent: CompozyOS is the product and compozy is the command."
