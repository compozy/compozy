---
id: ET-resource-only-extension-dev
area: ET
title: Iterate on a resource-only extension without a build toolchain
persona: Bruno
journey: J-extension-dev-lifecycle
expected: A native extension that declares only static agents, skills, loops, automation, or layouts builds without package.json or go.mod and without running build or describe commands; dev, reload, and watch publish changed resources only to the selected workspace; and an invalid edit leaves the last-good generation active.
entry_points: `compozy extension build <dir>`; `compozy extension dev <dir> --workspace <ref>`; `compozy extension reload <name> <dir> --workspace <ref>`; `compozy extension dev <dir> --watch`; `GET /api/agents?workspace=<ref>`
qa_status: pass
bug_ids: BUG-20261002-resource-manifest-paths
fix_status: fixed
retest_status: pass
fix_commits: 8df0742f9
evidence: docs/qa/evidence/2026-10-02-untested/passive-reload-assertions.json; docs/qa/evidence/2026-10-02-untested/passive-watch-assertions.json; docs/qa/evidence/2026-10-02-untested/passive-batuta-resources.json; docs/qa/evidence/2026-10-02-untested/passive-removed-extension.json; /Users/pedronauck/dev/qa-labs/compozy-pr-423-resource-only-watch-20260817-185100-670194-lab/qa-artifacts/qa/generated-manifest.toml; /Users/pedronauck/dev/qa-labs/compozy-pr-423-resource-only-watch-20260817-185100-670194-lab/qa-artifacts/qa/watch-reload.jsonl; /Users/pedronauck/dev/qa-labs/compozy-pr-423-resource-only-watch-20260817-185100-670194-lab/qa-artifacts/qa/workspace-skills.json; /Users/pedronauck/dev/qa-labs/compozy-pr-423-resource-only-watch-20260817-185100-670194-lab/qa-artifacts/qa/extensions-after-invalid.json; /Users/pedronauck/dev/qa-labs/compozy-pr-423-resource-only-watch-20260817-185100-670194-lab/qa-artifacts/qa/qa-audit-report.json; /Users/pedronauck/dev/qa-labs/compozy-pr-423-resource-only-watch-20260817-185100-670194-lab/qa-artifacts/qa/teardown.json
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-extension-dev-reload-loop; ET-agent-plugin-dev-reload
---

A native extension that declares only static agents, skills, loops, automation, or layouts builds without package.json or go.mod and without running build or describe commands; dev, reload, and watch publish changed resources only to the selected workspace; and an invalid edit leaves the last-good generation active.

Walk each listed public entry point, then reload and read the stored result independently. Exercise rejection and recovery with the same workspace and profile to confirm that unrelated state remains intact.

2026-09-27 scope update: retired product surfaces were removed from this active scenario. Historical evidence remains in the dated reports; this revised contract requires a fresh walk.
