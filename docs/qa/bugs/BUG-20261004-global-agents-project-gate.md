# BUG-20261004-global-agents-project-gate: Global Agents requires a project to show shared definitions

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Lea
- **Journey Step:** J-scope-global-across-workspaces, find a shared user-layer resource
- **Scenarios:** MS-global-scope-no-workspace-work
- **Found:** 2026-10-04 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

Opening Agents with Global enabled displays No project selected and Choose a project to see
its agents. A shared user-layer agent is present and resolves through Global CLI/HTTP reads,
and appears in both project catalogs, but Lea cannot reach it in the Global Web catalog.

## Reproduction

- **Charter:** CH-profile-global-phase-zero · **Tour:** Feature Tour
- **Environment:** real Chrome, 1512 × 862, Wi-Fi, en-US; isolated daemon at port 50727.

1. Create a user-layer agent through the public resource CLI.
2. Enable Global and open Agents from the dock.
3. Read the shared agent through the Global CLI/API and open Agents in two project views.

Expected: shared definitions remain browsable in Global without registering or choosing a folder.
Actual: only the Global Web catalog shows the project gate.

## Evidence

Under docs/qa/evidence/2026-10-02-untested/:
- global-work-fixed-lea-global-agent-panel.json and global-work-fixed-lea-agents-global.png.
- global-work-user-agent-resolve-global.json and global-work-user-agent-http-global.json.
- global-work-fixed-lea-agent-studio.png and global-work-fixed-lea-agent-editorial.png.
- global-work-fixed-lea-ended.json closes the persona before engineering diagnosis.
- Recording: /Users/pedronauck/.config/browser-harness/agent-workspace/recordings/global-work-fixed-lea.

## Fix

- Root cause: the fleet route, hook and view require a nonempty runtime workspace, and the paginated
  catalog endpoint also requires workspace even though definition reads support Global. Global
  correctly has no runtime workspace after the home-workspace removal.
- Repair: reuse effective Global definition resolution and server-owned pagination. Global
  session aggregates retain explicit profile scope and count matching agent names across projects;
  workspace selectors still narrow definitions and metrics. Web distinguishes pending scope from
  resolved Global, and detail/list reads share the same scoped query factories. Existing workspace
  request and cursor shapes remain compatible.
- Fix commit: 60ddd98e12e10731a7f98d8dedca20ff88f2e459.
- Regression test: owning fleet catalog suite and query boundary, followed by a fresh Lea replay.

## Verification

Fresh Lea replay passes: Global list/detail, same-profile reload, two project canaries, exact
HTTP/UDS counts, server pagination and cross-workspace cursor refusal. All temporary resources and
sessions are removed and the public baselines match. Receipts use global-agents-; the 35-frame
global-agents-fixed-lea recording is closed. The existing core, Session Manager and SQLite suites
fail before repair and pass afterward; 88 focused Web tests and root lint/typecheck/build pass.
Commit and broader delivery gate remain pending. Separate shell-transition observations are
retained in the report for dedicated reproduction, without changing this catalog verdict.


## Verified delivery — 2026-10-05

Fix commit: 60ddd98e12e10731a7f98d8dedca20ff88f2e459. Original-persona replay and the owning checks pass.
The warning-free affected gate passes, and the commit tree exactly matches
f1493b4c0a97be585cc4fe2e60b941df61a20ba4. Receipts: qa-catalog-delivery-gate-6.json
and qa-catalog-repairs-commit-identity.json in docs/qa/evidence/2026-10-02-untested/.
