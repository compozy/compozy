#!/usr/bin/env bash
# Classify changed repository paths into the CI areas that must verify them.
#
# Usage:
#   scripts/ci-changes.sh < changed-paths   # prints backend=, web=, desktop=, ci_infra=
#   scripts/ci-changes.sh --self-test       # checks the classification table below
#
# Every path lands in at least one bucket. "meta" paths (agent instructions,
# workspace agents, docs, editor config) need only the always-on preflight
# lane. A path no rule recognizes enables every area: an unknown path must
# never let verification skip silently. ci_infra marks changes to the CI
# pipeline itself, which run the full pipeline on the pull request.
set -euo pipefail

# classify prints the space-separated areas for one repository-relative path.
classify() {
  case "$1" in
    .github/workflows/ci.yml | .github/actions/* | scripts/ci-changes.sh)
      echo "backend web desktop ci_infra"
      return
      ;;
    .agents/skills/eng/eng-test-conventions/scripts/*)
      # CI runs these convention checkers inside the Go lanes.
      echo "backend"
      return
      ;;
    CLAUDE.md | AGENTS.md | */CLAUDE.md | */AGENTS.md | \
      .agents/* | .claude/* | .codex/* | .cursor/* | .vscode/* | .compozy/* | .husky/* | \
      docs/_memory/* | docs/design/* | docs/gtm/* | docs/qa/* | docs/research/* | docs/runbooks/* | docs/superpowers/* | \
      .github/ISSUE_TEMPLATE/* | .github/PULL_REQUEST_TEMPLATE.md | .github/CONTRIBUTING.md | .github/CODEOWNERS | \
      CHANGELOG.md | COPY.md | DESIGN.md | MIGRATION_GUIDE.md | PRODUCT.md | SECURITY.md | LICENSE | imgs/* | \
      .air.toml | .claudeignore | .coderabbit.yaml | .commitlintrc.yaml | .deep-review.yaml | .demo-video.yaml | \
      .editorconfig | .env.example | .gitattributes | .gitignore | .ignore | .repoclone.rc | mise.toml | \
      skills-lock.json | skeeper.lock)
      echo "meta"
      return
      ;;
    desktop/*)
      echo "desktop"
      return
      ;;
    Makefile)
      echo "backend desktop"
      return
      ;;
    openapi/*)
      # The OpenAPI document generates both the Go handlers and the TS clients.
      echo "backend web"
      return
      ;;
    *.go | go.mod | go.sum | */go.mod | */go.sum | .golangci.yml | .goreleaser.yml | .goreleaser.*.tmpl | \
      .pr-release | cliff.toml | RELEASE_BODY.md | RELEASE_NOTES.md | config.toml | \
      .github/* | .release-notes/* | cmd/* | extensions/* | internal/* | magefiles/* | skills/*)
      echo "backend"
      return
      ;;
    .bun-version | .oxfmtrc.json | .oxlintrc.json | bun.lock | package.json | tsconfig.json | tsconfig.base.json | \
      turbo.json | vitest.config.ts | knip.json | README.md | \
      web/* | packages/* | scripts/* | sdk/* | lint-plugins/* | patches/* | catalog/icons/* | docs/release/*)
      echo "web"
      return
      ;;
  esac
  echo "backend web desktop unclassified"
}

summarize() {
  local backend=false web=false desktop=false ci_infra=false path area
  while IFS= read -r path; do
    [[ -z "$path" ]] && continue
    for area in $(classify "$path"); do
      case "$area" in
        backend) backend=true ;;
        web) web=true ;;
        desktop) desktop=true ;;
        ci_infra) ci_infra=true ;;
        unclassified) echo "No CI rule matches $path; verifying every area." >&2 ;;
      esac
    done
  done
  printf 'backend=%s\nweb=%s\ndesktop=%s\nci_infra=%s\n' "$backend" "$web" "$desktop" "$ci_infra"
}

self_test() {
  local failures=0 path want got
  while IFS='|' read -r path want; do
    [[ -z "$path" ]] && continue
    got=$(classify "$path" 2>/dev/null)
    if [[ "$got" != "$want" ]]; then
      echo "classify $path: got '$got', want '$want'" >&2
      failures=$((failures + 1))
    fi
  done <<'EOF'
.github/workflows/ci.yml|backend web desktop ci_infra
.github/actions/setup-go/action.yml|backend web desktop ci_infra
scripts/ci-changes.sh|backend web desktop ci_infra
.github/workflows/release.yml|backend
.github/PULL_REQUEST_TEMPLATE.md|meta
.agents/skills/eng/eng-delivery/SKILL.md|meta
.agents/skills/eng/eng-test-conventions/scripts/check-test-conventions.py|backend
.compozy/agents/merge-train/AGENT.md|meta
CLAUDE.md|meta
internal/CLAUDE.md|meta
web/AGENTS.md|meta
docs/qa/scenarios/TA-merge-train-gate.md|meta
docs/release/homebrew-beta-notice.md|web
README.md|web
DESIGN.md|meta
skills/compozy/SKILL.md|backend
internal/session/manager.go|backend
main.go|backend
sdk/go/go.mod|backend
sdk/go/client.go|backend
sdk/typescript/src/index.ts|web
openapi/compozy.yaml|backend web
Makefile|backend desktop
desktop/src/main.ts|desktop
web/src/app.tsx|web
packages/ui/src/index.ts|web
bun.lock|web
catalog/icons/github.svg|web
catalog/marketplaces.json|backend web desktop unclassified
some-new-dir/file.txt|backend web desktop unclassified
EOF
  if ((failures > 0)); then
    echo "ci-changes self-test: $failures classification(s) drifted" >&2
    return 1
  fi
  echo "ci-changes self-test: classification table holds"
}

if [[ "${1:-}" == "--self-test" ]]; then
  self_test
else
  summarize
fi
