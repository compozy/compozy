# QA Run Report — 2026-08-17 — resource-only-extension-dev

- **Scope:** Issue #421 resource-only extension build, dev, reload, watch-compatible generations, workspace resource projection, last-good recovery, and code-backed compatibility canary
- **Cadence tier:** targeted
- **Build:** 5c5789e3 + local `fix/resource-only-extension-dev` diff · **Environment:** isolated CLI/API/runtime lab `compozy-resource-only-extension-dev-20260817-020712-286410-lab`; browser unavailable and outside this CLI/API charter
- **Started:** 2026-08-17T02:07:12-03:00 · **Status:** pass

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | desktop / wifi-fast / en-US | CH-resource-only-extension-dev |

## Flows in Scope

- `J-extension-dev-lifecycle` — iterate on immutable workspace-scoped extension generations and preserve last-good behavior (`../journeys/J-extension-dev-lifecycle.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-resource-only-extension-dev | J-extension-dev-lifecycle / ET-resource-only-extension-dev | Bruno | Feature Tour | Pass | | |
| 2 | CH-resource-only-extension-dev | J-extension-dev-lifecycle / ET-extension-dev-reload-loop | Bruno | Feature Tour | Pass | adjacent code-backed canary | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-resource-only-extension-dev — Bruno

- **Ran:** 2026-08-17T02:09:11Z → 2026-08-17T02:14:00Z (box respected: yes)
- **Findings:**
  - No functional divergence. A native manifest plus static agent built without a language project, linked without install trust, and appeared as a workspace-owned agent through the public HTTP catalog.
  - Repeating the build preserved generation `562afc3a…`; changing the prompt produced `cd0ca6ff…`; malformed YAML failed before activation and the API still returned generation two from `cd0ca6ff…`.
  - The global agent catalog never exposed the workspace agent, and removing the dev link removed it from a fresh workspace catalog read.
  - The code-backed Go scaffold remained active and `ext__code_canary__search` returned `No results for alpha` through the public invoke API.
  - After source freeze, the lab was reused from 2026-08-17T02:51:21Z to 02:54:32Z. The current binary reproduced deterministic build `aa93e277…`, workspace-only API and native-tool projection, valid reload `6542fc8f…`, last-good retention after malformed YAML, and clean removal.
- **Bugs filed/updated:** none
- **Scenarios settled:** ET-resource-only-extension-dev → pass; ET-extension-dev-reload-loop → pass
- **Paper cuts:** none
- **Surprises:** the malformed agent diagnostic correctly named both the staged resource and YAML parser location while leaving the prior generation healthy.
- **Suggested next charter:** add a watch-process interruption charter if watch debounce behavior changes independently of the shared reload path.

## What Was Fixed

- Workspace development resources are now collected alongside enabled global extensions and retain their workspace ownership through agents, skills, Loops, automation, and layouts.
- Automation jobs and triggers bind their domain scope and workspace ID before validation; layout record and spec IDs remain identical.
- Staged or inactive development links no longer project a global fallback or abort unrelated resource synchronization.
- `compozy__workspace_describe` now consumes the workspace-aware agent catalog, matching the HTTP workspace surfaces.
- Sources with neither a supported toolchain nor a native manifest now receive an actionable authoring diagnostic.

## Paper Cuts

None.

## Runtime Errors Observed

- Expected negative probe: malformed `AGENT.md` returned a staged static-resource validation error; the active extension and workspace agent remained healthy.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- The public workspace agent catalog is a useful independent read path for passive extension generations; global catalog absence simultaneously proves the isolation boundary.
- A code-backed extension can share the same isolated daemon as the passive kit and gives a cheap compatibility canary for the pre-existing toolchain lane.

## Final Status

**PASS** — issue #421 is ready for owner review. No PR, push, or commit was created.

- Canonical E2E: `CGO_ENABLED=1 go test -race -tags=integration ./internal/daemon -run '^TestDaemonE2EExtensionAuthoringShouldCompleteTheDevelopmentLoopWithoutTrustPrompts$' -count=1` — pass.
- Final `make verify` evidence: `GOTOOLCHAIN=go1.26.4 GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=tag.gpgSign GIT_CONFIG_VALUE_0=false make gate` escalated to the full verifier and passed.
- Closing gate: the same environment with `make gate-full` reported current passing evidence; `make gate-status` recorded fingerprint `fb9e5e013540a5cc8a7d9478fae3b07a5aa8d078` as `pass CURRENT` at 2026-08-17T03:33:57Z.
- QA evidence: `qa-artifacts/qa/journey-log.jsonl` and `qa-artifacts/qa/qa-audit-report.json` in the isolated lab.
- Teardown: `qa-artifacts/qa/teardown.json` records `clean: true` and no surviving processes.
- Findings: 0 open bugs, 0 paper cuts, 0 human verifications, 0 human decisions.
