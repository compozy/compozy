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

No product bugs or paper cuts remained in this targeted walk. Existing unrelated scenario coverage is not re-certified.

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
