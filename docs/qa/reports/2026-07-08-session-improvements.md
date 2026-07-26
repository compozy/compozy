# QA Run Report - 2026-07-08 - session-improvements

- **Scope:** Session Experience Overhaul execution gate: blank-on-return, session open latency,
  live streaming/reconnect, transcript UI language, CLI/API/UDS session parity, and truthful UI
  invariants planned by task 42.
- **Cadence tier:** Full.
- **Build:** local task-43 diff on top of `4ea584c41`; automatic commit disabled.
- **Environment:** deterministic QA lab, daemon target `http://127.0.0.1:60431`.
- **Started:** 2026-07-08T15:20:58-03:00. **Status:** QA execution and final verification
  complete.

## Bootstrap

```text
[QA_BOOTSTRAP]
manifest_path=/Users/pedronauck/dev/qa-labs/agh-session-improvements-20260708-182156-752360-lab/qa-artifacts/qa/bootstrap-manifest.json
lab_root=/Users/pedronauck/dev/qa-labs/agh-session-improvements-20260708-182156-752360-lab
runtime_home=/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/aghqa-07cd636345d5/runtime
base_url=http://127.0.0.1:60431
report_path=/Users/pedronauck/Dev/compozy/agh/docs/qa/reports/2026-07-08-session-improvements.md
health_status=fresh
[/QA_BOOTSTRAP]
```

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Théo | Power User | desktop / wifi-fast / en-US | CH-014, CH-016 |
| Ada | Power User (native-tool) | desktop / wifi-fast / en-US | CH-018 |
| Rafa | Casual User | desktop / wifi-fast / en-US | CH-017, CH-021 |
| Nia | New User | laptop / wifi-fast / en-US | CH-015, CH-019 |
| Sol | Accessibility-Reliant | desktop / wifi-fast / en-US | CH-020 |

## Flows in Scope

- `J-11 Return to a running session` - blank-on-return hero, return paths, snapshot reconnect,
  lifecycle truth, and workspace redirect notice (`../journeys/J-11-return-to-running-session.md`).
- `J-12 Open a session fast` - cold open, deep link, warm remount, and long-history paging
  (`../journeys/J-12-open-session-fast.md`).
- `J-13 Follow a live run` - live streaming, scroll anchoring, composer queue, stop, and clear
  convergence (`../journeys/J-13-follow-a-live-run.md`).
- `J-14 Read a finished transcript` - grouped tool rows, inline I/O, folds, usage truth, clear,
  and gap-free paging (`../journeys/J-14-read-a-finished-transcript.md`).
- `J-15 Operate a session via CLI/API` - CLI, HTTP, and UDS parity with raw/transcript streams and
  bounded reads (`../journeys/J-15-operate-session-via-cli-api.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-014 | J-11 / RT-045 RT-043 RT-023 RT-015 RT-024 RT-041 | Théo | Interrupt Tour | Fixed | BUG-0020 | pending local diff |
| 2 | CH-018 | J-15 / RT-050 RT-051 RT-023 RT-022 RT-012 RT-042 | Ada | Feature / strategy-based | Pass | | |
| 3 | CH-016 | J-13 / RT-054 RT-058 RT-059 RT-018 RT-019 RT-020 RT-013 | Théo | Multi-Tab Tour | Pass | | |
| 4 | CH-017 | J-14 / RT-048 RT-049 RT-053 RT-055 RT-056 RT-057 RT-060 | Rafa | Feature Tour | Pass | | |
| 5 | CH-021 | J-14 / RT-047 RT-052 RT-022 RT-017 | Rafa | Garbage Tour | Pass | | |
| 6 | CH-015 | J-12 / RT-046 RT-047 RT-040 RT-012 RT-044 | Nia | Network Tour | Pass | | |
| 7 | CH-020 | J-13 / RT-054 RT-058 RT-057 RT-048 RT-059 | Sol | Back-Button Tour | Pass | | |
| 8 | CH-019 | J-11 / RT-010 RT-015 RT-021 RT-041 | Nia | Back-Button Tour | Pass after fix | BUG-0020 | pending local diff |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`.

## Evidence Index

Lean evidence root: `docs/qa/evidence/2026-07-08-session-improvements/`.

| Evidence | Purpose |
|---|---|
| `browser-running-session.png` | Baseline live session render before the workspace-switch return walk. |
| `browser-post-onboarding.png` | Workspace mismatch notice offers Switch back instead of silently losing context. |
| `browser-running-after-switchback.png`, `browser-switchback-error-full.png`, `browser-switchback-retry.png` | BUG-0020 reproduction evidence. |
| `browser-switchback-fixed-clean-primary.png` | Post-fix switch-back route renders the running transcript. |
| `http-running-detail.json`, `http-events-running.json`, `http-sessions-primary.json` | Independent HTTP read path for live session detail/list/events. |
| `http-stream-transcript-stale.sse`, `http-stream-raw-finished.sse` | Transcript and raw SSE stream evidence. |
| `http-tool-transcript.json`, `http-usage-tool-session.json`, `http-empty-secondary-transcript.json` | Transcript UI language, usage truth, and true-empty checks. |

Bulky Playwright artifacts remained under `.tmp/playwright/test-results/`; the task report cites only
the stable repo evidence and command outcomes.

## Automated Lanes

| Command | Status | Evidence |
|---|---|---|
| `rtk bunx turbo run test --filter=./web -- src/systems/session/adapters/__tests__/session-api.test.ts src/components/assistant-ui/__tests__/session-thread.test.tsx src/systems/session/components/__tests__/session-chat-runtime-provider.test.tsx` | Pass | 3 files, 96 tests passed. |
| `rtk make web-build` | Pass | Vite build and `tsgo --noEmit` passed after the type refinement; chunk-size warning is pre-existing and non-fatal for this command. |
| `rtk make test-e2e-runtime` | Pass | Required daemon/runtime E2E lane passed earlier in this task run. |
| `rtk bun run --cwd web test:e2e:daemon-served:raw --grep "operator cancels a running prompt"` | Pass | 1 Playwright test passed after the clear-dialog test-id fix. |
| `rtk make test-e2e-web` | Pass | Full daemon-served Playwright lane passed: 62 tests, 0 failed. |
| `rtk make verify` | Pass | Final monorepo gate passed with exit 0 after the `golines` formatting fix; the run included the full `codegen-check -> bun-lint -> bun-typecheck -> bun-test -> web-build -> fmt -> lint -> test -> build -> boundaries` pipeline. |

## What Was Fixed

| Bug | Impact | Fix | Regression / replay evidence |
|---|---|---|---|
| BUG-0020 | Blocks-Completion | `ReadonlyThreadProvider` now remounts on transcript identity and virtualized rows render only up to the provider's committed message count; AGH errored tool parts validate through a sanitized dynamic-tool copy while preserving raw tool payloads; clear dialog/button test IDs match the E2E contract. | Focused web suite 96/96; `browser-switchback-fixed-clean-primary.png`; focused Playwright clear/delete test; full `make test-e2e-web` 62/62. |

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|
| Théo | CH-014 return path | The first clean browser permalink was opened from the wrong active workspace and redirected to the agent list. | dull | Re-ran from `qa-session-primary`; fixed evidence is `browser-switchback-fixed-clean-primary.png`. |
| QA operator | E2E-web rerun | `make test-e2e-web` serves `web/dist`, so source fixes were not visible until `make web-build` refreshed the bundle. | sharp | Recorded in task memory; rebuilt before final E2E-web evidence. |

## Runtime Errors Observed

- BUG-0020 initially crashed the session route with
  `useClientLookup: Index 3 out of bounds (length: 3)` in `<ThreadMessageComponent>`.
- A later E2E-web pass exposed AI SDK validation rejecting a persisted AGH `tool-*` error part that
  carried raw `output`; the production normalizer now validates a sanitized copy and restores the
  original AGH part for rendering.

## Human Verifications Needed

- None. Browser persona paths, CLI/API/UDS read paths, and E2E lanes reached terminal automated
  evidence. No external provider credential or screen-reader transcript was required for this task.

## Decisions for a Human

- None.

## Learnings

- Browser plugin in-app failed in this environment with `sandboxCwd must be an absolute file URI`;
  the task used the `agent-browser` fallback.
- `make test-e2e-web` is not a source-build command. Rebuild `web/dist` after web source changes
  before interpreting daemon-served Playwright evidence.
- Persisted AGH tool parts are the product contract for session rendering; AI SDK validation must
  be adapted through a validation copy, not by dropping AGH's raw tool payload.

## Final Status

- **Exit gate:** Pass.
- **Issues by user impact:** Blocks-Completion 1 fixed and retested (BUG-0020); Trust-Damage 0;
  Data-Loss 0; Friction 0; Cosmetic 0.
- **Coverage:** 8/8 charters terminal; RT-010, RT-012, RT-013, RT-015, RT-017, RT-018, RT-019,
  RT-020, RT-021, RT-022, RT-023, RT-024, RT-040, RT-041, RT-042, RT-043, RT-044, RT-045, RT-046,
  RT-047, RT-048, RT-049, RT-050, RT-051, RT-052, RT-053, RT-054, RT-055, RT-056, RT-057, RT-058,
  RT-059, and RT-060 written back to `state.csv`.
- **Verdict:** Release-ready for the session-improvements program. QA execution lanes and final
  monorepo verification are green; no truthful-UI blocker remains.
