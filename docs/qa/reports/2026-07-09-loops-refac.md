# QA Run Report - 2026-07-09 - loops-refac

- **Scope:** `loops-refac` targeted QA continuation for Task 16: re-walk the watch-events rows that returned to `untested` after the phase A/B/C substrate landed, and retest BUG-0023-blocked WS2/WS3 rows when the fix-loop permits.
- **Cadence tier:** targeted + e2e-web inclusion
- **Build:** `0958aab5` + dirty worktree carrying the current loops-refac implementation batch.
- **Environment:** fresh isolated `agh-qa-bootstrap` lab at `http://127.0.0.1:49189`.
- **Started:** 2026-07-09T03:47:07Z
- **Status:** complete; final `make verify` passed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | desktop / wifi-fast / en-US | CH-022, CH-023, CH-025, CH-026, CH-005 |
| Ada | Power User (native-tool) | desktop / wifi-fast / en-US | CH-024, CH-027 |

## Flows in Scope

- `J-01 Run a default dev-cycle Loop to a verified finish` - software-delivery under the policy gate (`../journeys/J-01-arrive-and-use-run.md`)
- `J-07 Operate a Loop end-to-end as an autonomous agent via native tools` - direct import-tool parity (`../journeys/J-07-agent-operated-run.md`)
- `J-08 Run a watch-source Loop that self-corrects and concludes on its own` - reviews-watch gating re-walk (`../journeys/J-08-watch-and-maintain.md`)
- `J-16 Author, park, wake, and recover a daemon-internal watch-events loop` - watch-events phase A/B/C scenarios (`../journeys/J-16-watch-events-wake.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-026 | J-01 / LP-003 | Bruno | Feature | Blocked (needs human verify) | BUG-0023 and BUG-0021 fixed; full terminal run now needs configured isolated-lab provider | pending manual commit |
| 2 | CH-026 | J-01 / LP-046 | Bruno | Feature | Blocked (needs human verify) | Gating regressions pass; live loop-spawned provider session needs configured isolated-lab provider | pending manual commit |
| 3 | CH-027 | J-07 / LP-045 | Ada | Feature | Fixed | BUG-0023 fixed; import tool callable and workspace-relative patterns anchored | pending manual commit |
| 4 | CH-022 | J-16 / LP-040 | Bruno | Feature | Pass | | |
| 5 | CH-022 | J-16 / LP-043 | Bruno | Feature | Pass | | |
| 6 | CH-022 | J-16 / LP-044 | Bruno | Feature | Pass | | |
| 7 | CH-023 | J-16 / LP-041 | Bruno | Interrupt | Pass | | |
| 8 | CH-024 | J-16 / LP-042 | Ada | Feature | Pass | | |
| 9 | CH-025 | J-16 / LP-047 | Bruno | Feature | Pass | | |
| 10 | CH-025 | J-16 / LP-048 | Bruno | Feature | Pass | | |
| 11 | CH-025 | J-16 / LP-049 | Bruno | Feature | Pass | | |
| 12 | CH-025 | J-16 / LP-050 | Bruno | Feature | Pass | | |
| 13 | CH-005 | J-08 / LP-029 | Bruno | Interrupt | Blocked (needs human verify) | BUG-0022 fixed; git repo + real CodeRabbit review event/account seed required to reach `fix_batch` | pending manual commit |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Bootstrap

Fresh isolated lab created with `agh-qa-bootstrap`:

- `SCENARIO_SLUG`: `loops-refac-task-16-20260709-20260709-034751-043179`
- `WORKSPACE_PATH`: `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab`
- `QA_OUTPUT_PATH`: `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts`
- `BOOTSTRAP_MANIFEST`: `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa/bootstrap-manifest.json`
- `BOOTSTRAP_ENV`: `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa/bootstrap.env`
- `AGH_HOME`: `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/aghqa-6873f014d4f1/runtime`
- `AGH_HTTP_PORT`: `49189`
- `AGH_UDS_PATH`: `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/aghqa-6873f014d4f1/runtime/aghd.sock`
- `TMUX_BRIDGE_SOCKET`: `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/aghqa-6873f014d4f1/runtime/tmux-bridge.sock`
- `AGH_WEB_API_PROXY_TARGET`: `http://127.0.0.1:49189`
- `PROVIDER_HOME`: `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/aghqa-6873f014d4f1/provider`
- `PROVIDER_CODEX_HOME`: `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/aghqa-6873f014d4f1/provider/.codex`
- `BROWSER_MODE`: `agent-browser`
- `BROWSER_BLOCKER`: `browser-use skill not found in CODEX_HOME plugin cache`
- `SCENARIO_CONTRACT`: `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa/scenario-contract.json`
- `BEHAVIORAL_CHARTER`: `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa/behavioral-scenario-charter.yaml`
- `JOURNEY_LOG`: `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa/journey-log.jsonl`
- `PROVIDER_ATTEMPT`: `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa/provider-attempt.json`
- `AUDIT_COMMAND`: `/Users/pedronauck/Dev/compozy/agh2/.agents/skills/real-scenario-qa/scripts/audit-qa-evidence.py`
- `REUSED_LAB`: `false`

Bootstrap validation evidence:

- Manifest read successfully and reports `schema_version: 1`, `status.reused_lab: false`, `status.health: fresh`.
- Required files present under `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa`: `bootstrap-manifest.json`, `bootstrap.env`, `scenario-contract.json`, `behavioral-scenario-charter.yaml`, `journey-log.jsonl`, `provider-attempt.json`.
- Runtime home exists and is isolated: `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/aghqa-6873f014d4f1/runtime`.
- Provider home exists and is isolated: `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/aghqa-6873f014d4f1/provider`.

```text
[QA_BOOTSTRAP]
manifest_path=/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa/bootstrap-manifest.json
lab_root=/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab
runtime_home=/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/aghqa-6873f014d4f1/runtime
base_url=http://127.0.0.1:49189
verification_report=/Users/pedronauck/Dev/compozy/agh2/docs/qa/reports/2026-07-09-loops-refac.md
health_status=fresh
[/QA_BOOTSTRAP]
```

## Pre-Run Baseline

- Current tracker scope before this run:
  - `LP-040`, `LP-041`, `LP-042`, `LP-043`, `LP-044`, `LP-047`, `LP-048`, `LP-049`, `LP-050` are `untested` and must move to terminal verdicts in this report.
  - `LP-003`, `LP-045`, `LP-046` carry prior `fail` verdicts linked to BUG-0023/BUG-0021; BUG-0023 is rechecked by this run's fix-loop.
  - `LP-029` carries prior `blocked-verify` because a real authenticated CodeRabbit review event/account seed is required to reach `fix_batch`.

## Scenario Evidence

Lab scratch evidence stays in the fresh lab and is indexed here by absolute path.

### BUG-0023 / Import Tool Retest

- Before-fix reproduction:
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa/logs/bug0020-tool-info.json`
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa/logs/bug0020-tool-invoke-valid.json`
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa/logs/import-tasks-relative-pattern.json`
- After-fix retest:
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa/logs/bug0020-tool-info-after-fix.json`
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa/logs/bug0020-tool-invoke-after-fix.json`
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa/logs/import-tasks-info-after-relative-fix.json`
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa/logs/import-tasks-relative-pattern-after-fix.json`
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa/logs/import-tasks-empty-pattern-after-fix.json`

### Software Delivery and Gating

- `software-delivery` after import-policy fix:
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa/logs/software-delivery-after-relative-fix-run.json`
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa/logs/software-delivery-after-relative-fix-http-run.json`
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa/logs/software-delivery-after-relative-fix-task-list.json`
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa/logs/software-delivery-after-relative-fix-execute-task-get.json`
- Gating regression lane:
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa/logs/session-gating-regressions.jsonl`

### Watch Events Runtime

- Phase A wake + restart replay:
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa/logs/watch-events-runtime-e2e.jsonl`
- Phase B/C supported contracts, coordinator rows, and `event.post_record` redaction:
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa/logs/watch-events-phase-bc-tests.jsonl`

### Web E2E

- Playwright onboarding:
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa/logs/web-playwright-onboarding-complete.json`
- Watch-events authoring form, no `force: true` actionability overrides:
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa/logs/web-watch-events-editor-playwright.json`
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa/screenshots/web-watch-events-editor-playwright.png`
- Run-detail parked read-model panel:
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa/logs/web-watch-events-run-detail-playwright.json`
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa/screenshots/web-watch-events-run-detail-playwright.png`
- Component and codec regression lane:
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa/logs/web-watch-events-vitest-relpath.log`

### Reviews Watch Smoke

- `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa/logs/reviews-watch-inspect-20260709.json`
- `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa/logs/reviews-watch-20260709-run.json`
- `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa/logs/reviews-watch-20260709-status.json`
- `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa/logs/reviews-watch-20260709-status-after-poll.json`
- `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa/logs/reviews-watch-20260709-stop.json`
- BUG-0022 before/after evidence:
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa/logs/reviews-watch-bug0022-before-after.log`
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa/logs/reviews-watch-after-fix-run.json`
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa/logs/reviews-watch-after-fix-status-after-poll.json`
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa/logs/reviews-watch-after-fix-stop.json`

### Focused Verification Before Final Gate

- `go test -race ./internal/daemon -run TestDaemonExtensionToolProvider -count=1` - passed.
- `go test -race ./internal/daemon -run 'TestDaemonNativeRuntimePolicyResolver/Should_trust_bundled_extension_read_only_tools_in_default_runtime_policy' -count=1` - passed.
- `make build` - passed.
- `go test -json -race -tags=integration ./internal/daemon -run TestDaemonE2ELoopWatchEventsShouldWakeAndRecover` - passed; evidence in `watch-events-runtime-e2e.jsonl`.
- `go test -json -race ./internal/daemon ./internal/loop -run "TestLoopWatchEventsObserverShould|TestSupportedWatchEventsShouldExposeSupportedContracts|TestWatchEventsSessionConstraintsShouldResolveStreamKeys|TestWatchEventsSessionStreamsShouldNormalizeDynamicKeys"` - passed; evidence in `watch-events-phase-bc-tests.jsonl`.
- `bunx turbo run test --filter=./web -- src/systems/loops/components/__tests__/loop-editor-watch-events.test.tsx src/systems/loops/components/__tests__/loop-run-page.test.tsx src/systems/loops/lib/__tests__/codec.test.ts` - passed; evidence in `web-watch-events-vitest-relpath.log`.
- `go test -json -race ./internal/session ./internal/daemon -run "TestAllowedToolsOverridePolicyHelpers|TestCreateAllowedToolsOverrideNarrowsAgentProfile|TestSessionPolicyGate|TestDaemonNativeRuntimePolicyResolver/Should_resolve_full_default_projection"` - passed; evidence in `session-gating-regressions.jsonl`.
- `go test -race ./internal/loop -run 'TestCoordinatorRunnerWatchSource/Should_render_watch_source_spec_templates_before_polling' -count=1` - passed.

## What Was Fixed

- BUG-0023 is fixed in the worktree:
  - `internal/daemon/tool_policy_resolver.go` now trusts enabled bundled extension source grants for the default runtime policy while keeping marketplace/MCP/non-bundled sources conservative.
  - `internal/daemon/native_extension_tool_provider.go` wraps extension tool calls so `ext__dev_cycle__import_tasks` resolves relative `pattern` values against the request workspace root and rejects `../` escapes with `scope_mismatch`.
  - Boot wiring passes the extension registry into the native tool policy resolver.
- Regression coverage:
  - `internal/daemon/native_tools_test.go`: bundled read-only extension tools are trusted by default policy.
  - `internal/daemon/native_extension_tool_provider_test.go`: workspace-root relative import anchoring and escape rejection.
- BUG-0022 is fixed in the worktree:
  - `internal/loop/coordinator_watch.go` now renders `node.watch` with the runtime namespace before calling the watch-source poller.
  - `internal/loop/control_plan.go` passes generation, resolved graph, topology, and prior outputs into the watch-source evaluator so rendering has the same context as action params.
  - `internal/loop/coordinator_watch_test.go` covers a `watch.pr: "{{ .inputs.pr }}"` regression that would have sent a raw template to the extension before the fix.

## Paper Cuts

- The daemon-served web bundle returned route-not-found for the editor deep link during browser QA, so the web E2E pass used the isolated Vite dev server with `AGH_WEB_API_PROXY_TARGET=http://127.0.0.1:49189`. This is recorded as QA logistics, not a Task 16 product bug, because the target UI and proxy-backed API path were exercised successfully.
- The run-detail Playwright fixture intentionally fulfilled SSE with an empty route; the browser console recorded a stream error while the read-model panel still rendered and asserted correctly.

## Runtime Errors Observed

- `LP-003` / `LP-046`: after BUG-0023 and BUG-0021 fixes, `software-delivery` advances past `load_tasks` into `execute_task`, then stops at the isolated-lab provider prerequisite: `agent provider is required; run agh install or set agent.provider/defaults.provider`. This blocks full provider-backed terminal verification in this lab and is not filed as a product bug.
- `LP-029`: initial `reviews-watch` smoke exposed BUG-0022 (`watch.pr` sent as raw `{{ .inputs.pr }}` to the extension). After the fix, the run advances past template rendering and reaches the next environment prerequisite: the lab workspace is not a git repository and lacks the real CodeRabbit/GitHub context needed for an end-to-end wake. That remaining blocker is not filed as a product bug.

## Human Verifications Needed

- `LP-003`: rerun `software-delivery` to terminal done with an isolated lab whose provider config is intentionally present.
- `LP-046`: observe the live loop-spawned provider-backed session posture after the provider-backed `execute_task` session is created; regression evidence already covers sandbox/permission/allowed_tools narrowing and widening rejection.
- `LP-029`: seed or connect an authenticated CodeRabbit review event/account from a real git workspace so `reviews-watch` can wake into `fetch_issues -> fix_batch` and exercise the gated `review_fixer` session.

## Decisions for a Human

None yet.

## AGH Impact Audit

- **Native tools:** Changed native-tool policy behavior, not IDs or schemas. `ext__dev_cycle__import_tasks` remains the same tool ID and descriptor, but enabled bundled extension source grants now resolve as trusted and callable. Descriptor/schema digests did not change. Evidence: BUG-0023 after-fix tool info/invoke logs plus daemon policy regression tests.
- **Extensibility and hooks:** Changed daemon routing for bundled extension tools by adding source-trust handling and daemon-side workspace-root anchoring for the dev-cycle import tool. Watch-source specs now render runtime templates before extension polling. No hook IDs, capabilities, bundle manifests, MCP sidecars, config keys, or bridge SDKs changed. Watch-events phase B/C consumed existing hook/event families and filed no new extensibility bug.
- **Workspace data isolation:** Relative `ext__dev_cycle__import_tasks` patterns are now scoped to the request workspace root via the daemon workspace resolver; escaping patterns are rejected with `scope_mismatch`. Watch-events runtime lanes exercised workspace-scoped readers and cross-workspace non-match behavior. No new persisted data class was introduced.
- **Official AGH skill:** No `skills/agh/` update required for this fix-loop. Public behavior added by tasks 12/14 was already represented in the workstream docs/skill updates; this task changed daemon routing/policy and QA evidence only.

## Learnings

- Use a real CSV parser for `docs/qa/state.csv`; Task 16 revalidated all rows at 16 columns after updates.
- For web QA in isolated labs, prefer `make web-dev` with `AGH_WEB_API_PROXY_TARGET` from the bootstrap manifest when daemon-served static assets are stale or unavailable.
- `AB-009` remains useful for a future live-daemon browser seed; Task 16 covered actual wake/replay through runtime E2E and covered the UI-bearing changes through Playwright/component tests.
- `watch-source` `watch` fields have the same runtime-template contract as action params; raw `node.WatchSpec` should not cross the extension boundary.

## Final Status

- `docs/qa/state.csv` updated: all Task 16 scope rows moved out of `untested` or preserved as explicit `blocked-verify` with fresh evidence.
- `docs/qa/bugs/BUG-0023.md` updated to `fixed`.
- `docs/qa/bugs/BUG-0022.md` filed and marked `fixed`.
- Final `make verify` passed after the fix-loop and lint corrections.
  - Evidence: `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260709-20260709-034751-043179-lab/qa-artifacts/qa/logs/make-verify-final.log`
  - Result: `DONE 12689 tests, 2 skipped in 396.066s`; `OK: all package boundaries respected`.
