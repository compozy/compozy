# QA Run Report — 2026-10-28 — Archive projects

- **Scope:** Issue #1005 archive/unarchive and project removal.
- **Cadence tier:** targeted
- **Build:** feat/issue-1005-archive-projects (working tree)
- **Environment:** isolated local daemon and production web bundle; no application mocks.
- **Status:** in-progress

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status |
|---|---|---|---|---|---|
| 1 | CH-archive-projects | Archive round-trip, public surfaces and history | Bruno | Feature Tour | Pass |
| 2 | CH-archive-projects | Picker, Settings, busy refusal and removal | Bruno | Feature Tour | Pass |

## Session Debriefs

### CH-archive-projects — Bruno

- CLI and native tools archived/unarchived real projects with the same ID. CLI `--archived` returned only archived registrations.
- HTTP returned `workspace_busy` and named an active unbound session. After a verified stop, HTTP archive succeeded, default workspace/session catalogs excluded the project, and explicit history remained byte-for-byte equivalent as JSON. A new session returned HTTP 409 `workspace_archived`. UDS unarchive restored the same ID.
- A named second desktop and its complete public layout snapshot were identical before, during and after archive.
- Chromium/Playwright against the production bundle walked Settings archive refusal, Remove warning, Archive instead, refresh persistence and Unarchive. The picker context menu archived a project and removed its tile. Settings restored it. Remove was canceled once, then confirmed for a disposable project; its README remained on disk.
- The initial attached-browser driver did not deliver UI actions reliably. A clean isolated Chromium repeated the journey successfully with no page errors; only that run supplies browser verdicts.
- Fresh-lab session creation initially lacked a provider default. Setting the lab's default provider resolved that setup prerequisite; no provider prompt or inference was needed for this lifecycle journey.

The first review exposed incomplete resource-backed automation admission, implicit aggregate filtering, SDK call compatibility and missing-root retention. These were repaired with owning-suite regressions. Existing unrelated scenario coverage is not re-certified.

## Verification

Targeted store suites prove lossless upgrade, archive identity, stopped history, active-session/Loop/automation refusal, and overlay reactivation guards. Full local race/E2E and `make gate` are excluded by the operator's parallel-executor rule; current-head CI is the delivery gate.

## Final Status

Targeted runtime and Web journeys passed. PR delivery remains in progress until current-head CI and both external reviews complete. The screenshots are published in the PR gallery; local captures are retained in the isolated run evidence.

## Checks

- Focused Go workspace, store, core API, CLI, native-tool and overview suites: passed.
- Web interaction and registry suites: 54 tests passed.
- Root Turborepo filtered web typecheck/build and lint: passed; lint reports zero warnings/errors.
- React Doctor diff: 100/100, no issues.
- Codegen-check: passed with repository-pinned Bun 1.4.0.
- Production build retains pre-existing CSS `::highlight`, chunk-size and dynamic-import warnings; no unrelated build configuration was changed.

## Review remediation

- Consolidated the unpublished schema change into one generated migration, 00155; no archive triggers remain.
- Archive and work admission use the same SQLite write transaction boundary. Session/Loop and legacy/resource-backed automation writes check the archived state before committing. Runtime overrides participate in effective-enabled checks.
- Added real boot recovery coverage for retained stopped sessions and terminal Loops, automation manager restart/definition reload, disabled-overlay removal, unarchive/re-enable and resource-backed automation refusal.
- Missing folders stay visible with Folder not found in Settings and the picker, preserving registration and history. New sessions return workspace_root_missing; read-only history remains available. Renaming the fixture folder away and back proved flag restoration and identical history. Archive/Unarchive also passed while the folder was absent.
- Preserved the exact Go SDK `ListWorkspaces(ctx)` method type and added `ListWorkspacesWithParams(ctx, params)`; real HTTP and method-value coverage verify compatibility.
- React Doctor's CI complexity finding was repaired by splitting the project menu and blocker rendering. The pinned CI version (0.9.17) reports 100/100 on the full branch diff.

## Final-schema QA pass

A fresh isolated lab repeated CLI/UDS, HTTP and native archive/unarchive on migration 00155, preserving the same ID, history and layout. The final production bundle repeated all four screenshot journeys without page errors. The first lab was torn down with `clean: true` before this pass.

The two additional Web suites identified by CI passed (59 tests): Settings now runs with its real QueryClient provider, and the adapter preserves the unfiltered request URL. The native metadata inventory and historical alias upgrade suites passed. The single CI-failing worktree overview removal scenario passed locally (7.6 seconds); no test changes or deadline increases were made.

The strict QA evidence auditor has one remaining C14 finding: it requires a successful full local gate record. The operator explicitly prohibits that run on this shared machine, so it was not fabricated or bypassed. The runtime/browser observations above passed; current-head CI remains the delivery gate.

## Missing-folder and recovery follow-up

The live rename/restore journey passed with the same workspace ID and identical history, including Archive/Unarchive while absent and a typed new-session refusal. Six refreshed screenshots cover Settings, picker, refusal, removal, and both missing-folder views. The updated production bundle completed the archive/remove journeys with no page errors. A stale embedded bundle during a lab restart was corrected by restoring the lab-only web-dist override before recording the successful run.

Focused suites also prove that archived non-stopped disk metadata can be reconciled without admitting new work, and that new config/package automations for archived targets are deferred with a warning while unrelated definitions sync. Unarchive permits those definitions on the next sync. Default task queries now filter persisted archive state without reading project folders, and pagination verifies the exact returned identities.
