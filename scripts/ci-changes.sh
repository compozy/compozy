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
    .github/workflows/ci.yml | .github/workflows/go-integration.yml | .github/actions/* | scripts/ci-changes.sh | scripts/ci-workspaces.py | scripts/ci-frontend.py | scripts/ci-turbo-report.py)
      echo "backend web desktop ci_infra"
      return
      ;;
    .agents/skills/eng/eng-test-conventions/scripts/*)
      # CI runs these convention checkers inside the Go lanes.
      echo "backend"
      return
      ;;
    CLAUDE.md | AGENTS.md | */CLAUDE.md | */AGENTS.md | \
      .agents/* | .claude/* | .codex/* | .cursor/* | .vscode/* | .compozy/* | .husky/* | infra/* | \
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
        unclassified) ci_infra=true; echo "No CI rule matches $path; verifying every area." >&2 ;;
      esac
    done
  done
  printf 'backend=%s\nweb=%s\ndesktop=%s\nci_infra=%s\n' "$backend" "$web" "$desktop" "$ci_infra"
}

# Validate the workflow's needs projection against the selection contract.
require_lanes() {
  jq -e --arg kind "${1:-fast}" '
    . as $needs | .changes.outputs as $o |
    ($o.backend == "true") as $backend |
    ($o.web == "true") as $web |
    ($o.desktop == "true") as $desktop |
    ($o.heavy == "true") as $heavy |
    {
      changes: true, preflight: true, frontend: $web,
      "go-static": $backend, "go-test": $backend, "sdk-go-test": $backend,
      "go-build": $backend, "go-peer-darwin": $backend,
      desktop: ($desktop or $web),
      # Release owns the identical reusable lane and gates its dry-run on it.
      "go-integration": ($heavy and $backend and ($o.release_pr != "true")),
      "go-terminal-windows": ($heavy and $backend),
      "e2e-runtime": ($heavy and ($backend or $web)),
      "e2e-web": ($heavy and ($backend or $web)),
      "e2e-desktop": ($heavy and ($backend or $web or $desktop))
    } as $expected |
    (if $kind == "fast" then
      ["changes", "preflight", "frontend", "go-static", "go-test", "sdk-go-test", "go-build", "go-peer-darwin", "desktop"]
     elif $kind == "heavy" then
      ["changes", "preflight", "go-integration", "go-terminal-windows", "e2e-runtime", "e2e-web", "e2e-desktop"]
     else ["invalid-gate-kind"] end) as $required |
    ($required | all(. as $key | $needs | has($key))) and
    ($needs.changes.result == "success") and
    ($needs.preflight.result == "success") and
    ([$o.backend, $o.web, $o.desktop, $o.heavy] |
      all(. == "true" or . == "false")) and
    ($needs | to_entries | all(
      .key as $key |
      ($expected | has($key)) and
      (.value.result == "success" or
        (.value.result == "skipped" and $expected[$key] == false))
    ))
  ' >/dev/null
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
.github/workflows/go-integration.yml|backend web desktop ci_infra
.github/actions/setup-go/action.yml|backend web desktop ci_infra
scripts/ci-changes.sh|backend web desktop ci_infra
.github/workflows/release.yml|backend
.github/PULL_REQUEST_TEMPLATE.md|meta
.agents/skills/eng/eng-delivery/SKILL.md|meta
.agents/skills/eng/eng-test-conventions/scripts/check-test-conventions.py|backend
.compozy/agents/merge-gate/AGENT.md|meta
CLAUDE.md|meta
internal/CLAUDE.md|meta
web/AGENTS.md|meta
docs/qa/scenarios/TA-merge-gate.md|meta
infra/observability/compose.yaml|meta
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
packages/brand/src/routes/index.tsx|web
bun.lock|web
catalog/icons/github.svg|web
catalog/marketplaces.json|backend web desktop unclassified
some-new-dir/file.txt|backend web desktop unclassified
EOF
  if ((failures > 0)); then
    echo "ci-changes self-test: $failures classification(s) drifted" >&2
    return 1
  fi
  local projection
  projection='{"changes":{"result":"success","outputs":{"backend":"false","web":"true","desktop":"false","heavy":"false"}},"preflight":{"result":"success"},"frontend":{"result":"success"}}'
  projection=$(jq '. + {"go-static":{"result":"skipped"},"go-test":{"result":"skipped"},"sdk-go-test":{"result":"skipped"},"go-build":{"result":"skipped"},"go-peer-darwin":{"result":"skipped"},"desktop":{"result":"success"}}' <<<"$projection")
  require_lanes <<<"$projection"
  if jq 'del(.frontend)' <<<"$projection" | require_lanes; then
    echo "missing frontend was accepted" >&2; return 1
  fi
  if jq '.frontend.result = "skipped"' <<<"$projection" | require_lanes; then
    echo "required frontend skip was accepted" >&2; return 1
  fi
  jq '.changes.outputs.web = "false" | .frontend.result = "skipped"' <<<"$projection" | require_lanes
  for result in failure cancelled; do
    if jq --arg result "$result" '.frontend.result = $result' <<<"$projection" | require_lanes; then
      echo "frontend $result was accepted" >&2; return 1
    fi
  done
  if jq '.preflight.result = "skipped"' <<<"$projection" | require_lanes; then
    echo "preflight skip was accepted" >&2; return 1
  fi
  projection=$(jq '.changes.outputs.backend = "true" | .changes.outputs.heavy = "true" |
    {changes, preflight, "go-integration": {result: "success"},
     "go-terminal-windows": {result: "success"}, "e2e-runtime": {result: "success"},
     "e2e-web": {result: "success"}, "e2e-desktop": {result: "success"}}' <<<"$projection")
  require_lanes heavy <<<"$projection"
  if jq 'del(."go-integration")' <<<"$projection" | require_lanes heavy; then
    echo "missing integration was accepted" >&2; return 1
  fi
  if jq '."go-integration".result = "skipped"' <<<"$projection" | require_lanes heavy; then
    echo "required integration skip was accepted" >&2; return 1
  fi
  # Release PRs run the same integration shards through Release's thin call.
  jq '.changes.outputs.release_pr = "true" | ."go-integration".result = "skipped"' <<<"$projection" | require_lanes heavy
  local scenario expected job skipped
  while IFS='|' read -r scenario expected; do
    projection=$(jq --arg scenario "$scenario" '
      .changes.outputs = {backend: "false", web: "false", desktop: "false", heavy: "true"} |
      if $scenario == "disabled" then .changes.outputs.heavy = "false"
      else .changes.outputs[$scenario] = "true" end |
      ."go-integration".result = "success" | ."go-terminal-windows".result = "success" |
      ."e2e-runtime".result = "success" | ."e2e-web".result = "success" | ."e2e-desktop".result = "success"
    ' <<<"$projection")
    for job in go-integration go-terminal-windows e2e-runtime e2e-web e2e-desktop; do
      skipped=$(jq --arg job "$job" '.[$job].result = "skipped"' <<<"$projection")
      if [[ " $expected " == *" $job "* ]]; then
        if require_lanes heavy <<<"$skipped"; then
          echo "required $scenario/$job skip was accepted" >&2; return 1
        fi
      else
        require_lanes heavy <<<"$skipped" || return 1
      fi
    done
  done <<'HEAVY'
web|e2e-runtime e2e-web e2e-desktop
desktop|e2e-desktop
disabled|
HEAVY
  jq 'with_entries(if .key == "changes" or .key == "preflight" then . else .value.result = "skipped" end)' <<<"$projection" | require_lanes heavy
  python3 - <<'CHECK'
import importlib.util
import json
import tempfile
import os
import runpy
import subprocess
from pathlib import Path
from unittest.mock import patch
spec = importlib.util.spec_from_file_location("ci_workspaces", "scripts/ci-workspaces.py")
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)
assert "compozy-web" in module.select(["packages/ui/src/index.ts"])
assert "@compozy/site" in module.select(["internal/config/config.go"])
assert "@compozy/lint-plugins" in module.select(["packages/ui/src/primitives.ts"])
assert module.select(["unknown/input"]) == module.select(["package.json"])
assert "compozy-web" in module.select(["web/deleted.ts"])
assert module.select(["internal/session/manager.go"]) == []
assert module.select(["AGENTS.md", ".agents/agents/example.md", "docs/_memory/lessons/example.md"]) == []
assert "@compozy/site" in module.select(["docs/_memory/glossary.md"])
web_lanes = module.frontend_lanes(['compozy-web', '@compozy/ui'])
assert [lane['shard'] for lane in web_lanes if lane['lane'] == 'web-tests'] == ['1/4', '2/4', '3/4', '4/4']
assert any(lane['lane'] == 'web' for lane in web_lanes)
assert not any(lane['lane'] == 'web-tests' for lane in module.frontend_lanes(['@compozy/ui']))
assert not any(lane['lane'] == 'web-tests' for lane in module.frontend_lanes(['@compozy/site']))
assert len([lane for lane in module.frontend_lanes(['*']) if lane['lane'] == 'web-tests']) == 4
with tempfile.TemporaryDirectory() as directory:
    root = Path(directory)
    (root / 'package.json').write_text('{"workspaces": []}')
    (root / 'turbo.json').write_text('{"tasks": {"test": "invalid"}}')
    assert module.select(['changed.ts'], root) == ['*']
    (root / 'package.json').write_text('{"workspaces": ["lint-plugins"]}')
    (root / 'lint-plugins').mkdir()
    (root / 'lint-plugins/package.json').write_text('{"name": "@compozy/lint-plugins"}')
    (root / '.turbo/runs').mkdir(parents=True)
    frontend = str(Path('scripts/ci-frontend.py').resolve())
    report = str(Path('scripts/ci-turbo-report.py').resolve())
    previous = Path.cwd()
    try:
        os.chdir(root)
        with patch.dict(os.environ, CI_WORKSPACES='["@compozy/lint-plugins"]'), patch('sys.argv', [frontend, 'web']):
            with patch('subprocess.run'):
                try:
                    runpy.run_path(frontend)
                    raise AssertionError('missing Turbo receipt accepted')
                except RuntimeError:
                    pass
            def write_receipt(*args, **kwargs):
                (root / '.turbo/runs/current.json').write_text('{"tasks": []}')
            (root / '.turbo/runs/older.json').write_text('invalid stale receipt')
            with patch('subprocess.run', side_effect=write_receipt):
                runpy.run_path(frontend)
            runpy.run_path(report)
            (root / '.turbo/runs/current.json').write_text(json.dumps({'tasks': [{
                'taskId': '@compozy/lint-plugins#test', 'package': '@compozy/lint-plugins',
                'task': 'test', 'cache': {'status': 'HIT'}}]}))
            with patch('subprocess.run') as audit:
                runpy.run_path(report)
                audit.assert_called_once_with(['bunx', '--no-install', 'turbo', 'run', 'test',
                    '--filter=@compozy/lint-plugins', '--cache=', '--concurrency=2'], check=True)
            failure = subprocess.CalledProcessError(7, ['turbo', 'test'])
            with patch('subprocess.run', side_effect=failure):
                try:
                    runpy.run_path(report)
                    raise AssertionError('failed cold audit was accepted')
                except subprocess.CalledProcessError as error:
                    assert error is failure
            with patch('subprocess.run'):
                try:
                    runpy.run_path(frontend)
                    raise AssertionError('stale Turbo receipt accepted')
                except RuntimeError:
                    pass
            try:
                runpy.run_path(report)
                raise AssertionError('empty current-run manifest accepted')
            except RuntimeError:
                pass
        (root / 'web').mkdir()
        (root / 'web/package.json').write_text('{"name": "compozy-web"}')
        (root / 'package.json').write_text('{"workspaces": ["web", "lint-plugins"]}')
        receipt_number = 0
        def write_fresh_receipt(*args, **kwargs):
            global receipt_number
            receipt_number += 1
            (root / f'.turbo/runs/run-{receipt_number}.json').write_text('{"tasks": []}')
        with patch.dict(os.environ, CI_WORKSPACES='["compozy-web", "@compozy/lint-plugins"]'):
            with patch('sys.argv', [frontend, 'web']), patch('subprocess.run', side_effect=write_fresh_receipt) as common:
                runpy.run_path(frontend)
                commands = [call.args[0] for call in common.call_args_list]
                assert any('test' in command and '--filter=@compozy/lint-plugins' in command for command in commands)
                assert not any('test' in command and '--filter=compozy-web' in command for command in commands)
                assert any('typecheck' in command and '--filter=compozy-web' in command for command in commands)
                assert len(json.loads((root / '.turbo/ci-receipts.json').read_text())) == 3
            for shard in ['1/4', '2/4', '3/4', '4/4']:
                with patch.dict(os.environ, CI_WEB_SHARD=shard), patch('sys.argv', [frontend, 'web-tests']):
                    with patch('subprocess.run', side_effect=write_fresh_receipt) as execute:
                        runpy.run_path(frontend)
                    execute.assert_called_once_with(('bunx', '--no-install', 'turbo', 'run', 'test',
                        '--filter=compozy-web', '--concurrency=2', '--log-order=stream', '--summarize',
                        '--', '--shard=' + shard, '--maxWorkers=75%', '--retry=2'), check=True)
                    receipts = json.loads((root / '.turbo/ci-receipts.json').read_text())
                    assert len(receipts) == 1
                    Path(receipts[0]).write_text(json.dumps({'tasks': [{
                        'taskId': 'compozy-web#test', 'package': 'compozy-web',
                        'task': 'test', 'cache': {'status': 'HIT'}}]}))
                    with patch('subprocess.run') as audit:
                        runpy.run_path(report)
                    audit.assert_called_once_with(['bunx', '--no-install', 'turbo', 'run', 'test',
                        '--filter=compozy-web', '--cache=', '--concurrency=2',
                        '--', '--shard=' + shard, '--maxWorkers=75%', '--retry=2'], check=True)
                    for effect in [None, subprocess.CalledProcessError(7, ['turbo', 'test'])]:
                        with patch('subprocess.run', side_effect=effect):
                            try:
                                runpy.run_path(frontend)
                                raise AssertionError('missing receipt or failed shard was accepted')
                            except (RuntimeError, subprocess.CalledProcessError):
                                pass
                    def write_two_receipts(*args, **kwargs):
                        write_fresh_receipt()
                        write_fresh_receipt()
                    with patch('subprocess.run', side_effect=write_two_receipts):
                        try:
                            runpy.run_path(frontend)
                            raise AssertionError('multiple shard receipts were accepted')
                        except RuntimeError:
                            pass
        with patch.dict(os.environ, CI_WORKSPACES='["@compozy/lint-plugins"]'), patch('sys.argv', [frontend, 'web-tests']):
            with patch('subprocess.run') as execute:
                runpy.run_path(frontend)
            execute.assert_not_called()
    finally:
        os.chdir(previous)
CHECK
  echo "ci-changes self-test: classification, closure and gate contracts hold"
}

if [[ "${1:-}" == "--self-test" ]]; then
  self_test
elif [[ "${1:-}" == "--classify" ]]; then
  classify "$2"
elif [[ "${1:-}" == "--require-lanes" ]]; then
  require_lanes "${2:-fast}"
else
  summarize
fi
