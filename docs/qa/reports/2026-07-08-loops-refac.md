# QA Run Report - 2026-07-08 - loops-refac

- **Scope:** `loops-refac` targeted QA execution for the Task 15 plan: gated loop run-agent sessions, `ext__dev_cycle__import_tasks`, watch-events phase A/B/C scenario rows, and the reviews-watch gating re-walk.
- **Cadence tier:** targeted + e2e-web inclusion
- **Build:** `bd21f7e0` + dirty worktree carrying the loops-refac implementation batch and the BUG-0021 local fix.
- **Environment:** fresh isolated `agh-qa-bootstrap` lab at `http://127.0.0.1:64424`; BUG-0021 retest lab at `http://127.0.0.1:58521`.
- **Started:** 2026-07-08T13:01:52Z
- **Status:** failed / not ready to ship. BUG-0023 remains open and blocks `software-delivery` plus direct import-tool parity. Watch-events phase rows were skipped honestly because the runtime/web substrate is not present in this worktree.

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | desktop / wifi-fast / en-US | CH-026, CH-022, CH-023, CH-025, CH-005 |
| Ada | Power User (native-tool) | desktop / wifi-fast / en-US | CH-027, CH-024 |

## Flows in Scope

- `J-01 Run a default dev-cycle Loop to a verified finish` - software-delivery under the policy gate (`../journeys/J-01-arrive-and-use-run.md`)
- `J-07 Operate a Loop end-to-end as an autonomous agent via native tools` - direct import-tool parity (`../journeys/J-07-agent-operated-run.md`)
- `J-08 Run a watch-source Loop that self-corrects and concludes on its own` - reviews-watch gating re-walk (`../journeys/J-08-watch-and-maintain.md`)
- `J-16 Author, park, wake, and recover a daemon-internal watch-events loop` - watch-events phase A/B/C scenarios (`../journeys/J-16-watch-events-wake.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-026 | J-01 / LP-003 | Bruno | Feature | Fail | BUG-0023; BUG-0021 | pending local diff |
| 2 | CH-026 | J-01 / LP-046 | Bruno | Feature | Fail | BUG-0023; BUG-0021 | pending local diff |
| 3 | CH-027 | J-07 / LP-045 | Ada | Feature | Fail | BUG-0023 | |
| 4 | CH-022 | J-16 / LP-040 | Bruno | Feature | Skipped | upstream watch-events substrate absent | |
| 5 | CH-022 | J-16 / LP-043 | Bruno | Feature | Skipped | upstream watch-events substrate absent | |
| 6 | CH-022 | J-16 / LP-044 | Bruno | Feature | Skipped | upstream watch-events substrate absent | |
| 7 | CH-023 | J-16 / LP-041 | Bruno | Interrupt | Skipped | upstream watch-events substrate absent | |
| 8 | CH-024 | J-16 / LP-042 | Ada | Feature | Skipped | upstream watch-events substrate absent | |
| 9 | CH-025 | J-16 / LP-047 | Bruno | Feature | Skipped | upstream phase B/C substrate absent | |
| 10 | CH-025 | J-16 / LP-048 | Bruno | Feature | Skipped | upstream phase B/C substrate absent | |
| 11 | CH-025 | J-16 / LP-049 | Bruno | Feature | Skipped | upstream phase B/C substrate absent | |
| 12 | CH-025 | J-16 / LP-050 | Bruno | Feature | Skipped | upstream phase B/C substrate absent | |
| 13 | CH-005 | J-08 / LP-029 | Bruno | Interrupt | Blocked (needs human verify) | real CodeRabbit review event/account seed required | |

Status legend: `Pass | Fail | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`.

## Bootstrap

Fresh isolated lab created with `agh-qa-bootstrap`:

- `SCENARIO_SLUG`: `loops-refac-task-16-20260708-130224-745100`
- `WORKSPACE_PATH`: `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260708-130224-745100-lab`
- `QA_OUTPUT_PATH`: `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260708-130224-745100-lab/qa-artifacts`
- `BOOTSTRAP_MANIFEST`: `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260708-130224-745100-lab/qa-artifacts/qa/bootstrap-manifest.json`
- `BOOTSTRAP_ENV`: `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260708-130224-745100-lab/qa-artifacts/qa/bootstrap.env`
- `AGH_HOME`: `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/aghqa-71059cd70f44/runtime`
- `AGH_HTTP_PORT`: `64424`
- `AGH_UDS_PATH`: `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/aghqa-71059cd70f44/runtime/aghd.sock`
- `TMUX_BRIDGE_SOCKET`: `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/aghqa-71059cd70f44/runtime/tmux-bridge.sock`
- `AGH_WEB_API_PROXY_TARGET`: `http://127.0.0.1:64424`
- `PROVIDER_HOME`: `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/aghqa-71059cd70f44/provider`
- `PROVIDER_CODEX_HOME`: `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/aghqa-71059cd70f44/provider/.codex`
- `BROWSER_MODE`: `agent-browser`
- `BROWSER_BLOCKER`: `browser-use skill not found in CODEX_HOME plugin cache`
- `SCENARIO_CONTRACT`: `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260708-130224-745100-lab/qa-artifacts/qa/scenario-contract.json`
- `BEHAVIORAL_CHARTER`: `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260708-130224-745100-lab/qa-artifacts/qa/behavioral-scenario-charter.yaml`
- `JOURNEY_LOG`: `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260708-130224-745100-lab/qa-artifacts/qa/journey-log.jsonl`
- `PROVIDER_ATTEMPT`: `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260708-130224-745100-lab/qa-artifacts/qa/provider-attempt.json`
- `AUDIT_COMMAND`: `/Users/pedronauck/Dev/compozy/agh2/.agents/skills/real-scenario-qa/scripts/audit-qa-evidence.py`
- `REUSED_LAB`: `false`

Bootstrap validation evidence:

- Manifest read successfully and reports `schema_version: 1`, `status.reused_lab: false`, `status.health: fresh`.
- Required files present under `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260708-130224-745100-lab/qa-artifacts/qa`: `bootstrap-manifest.json`, `bootstrap.env`, `scenario-contract.json`, `behavioral-scenario-charter.yaml`, `journey-log.jsonl`, `provider-attempt.json`.
- Runtime home exists and is isolated: `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/aghqa-71059cd70f44/runtime`.
- Provider home exists and is isolated: `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/aghqa-71059cd70f44/provider`.

```text
[QA_BOOTSTRAP]
manifest_path=/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260708-130224-745100-lab/qa-artifacts/qa/bootstrap-manifest.json
lab_root=/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260708-130224-745100-lab
runtime_home=/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/aghqa-71059cd70f44/runtime
base_url=http://127.0.0.1:64424
verification_report=/Users/pedronauck/Dev/compozy/agh2/docs/qa/reports/2026-07-08-loops-refac.md
health_status=fresh
[/QA_BOOTSTRAP]
```

BUG-0021 retest lab:

- `SCENARIO_SLUG`: `loops-refac-task-16-bug0021-retest-20260708-132925-584673`
- `WORKSPACE_PATH`: `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-bug0021-retest-20260708-132925-584673-lab`
- `QA_OUTPUT_PATH`: `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-bug0021-retest-20260708-132925-584673-lab/qa-artifacts`
- `BOOTSTRAP_MANIFEST`: `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-bug0021-retest-20260708-132925-584673-lab/qa-artifacts/qa/bootstrap-manifest.json`
- `AGH_HOME`: `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/aghqa-58517f70ef91/runtime`
- `AGH_HTTP_PORT`: `58521`
- `AGH_UDS_PATH`: `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/aghqa-58517f70ef91/runtime/aghd.sock`
- `TMUX_BRIDGE_SOCKET`: `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/aghqa-58517f70ef91/runtime/tmux-bridge.sock`
- `AGH_WEB_API_PROXY_TARGET`: `http://127.0.0.1:58521`

```text
[QA_BOOTSTRAP]
manifest_path=/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-bug0021-retest-20260708-132925-584673-lab/qa-artifacts/qa/bootstrap-manifest.json
lab_root=/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-bug0021-retest-20260708-132925-584673-lab
runtime_home=/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/aghqa-58517f70ef91/runtime
base_url=http://127.0.0.1:58521
verification_report=/Users/pedronauck/Dev/compozy/agh2/docs/qa/reports/2026-07-08-loops-refac.md
health_status=fresh
[/QA_BOOTSTRAP]
```

## Pre-Run Baseline

- Scoped tracker rows were `untested`: `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260708-130224-745100-lab/qa-artifacts/qa/logs/baseline-state-rows.csv` (13 rows: `LP-003`, `LP-029`, `LP-040..LP-050`).
- Upstream workstream status showed `task_08`, `task_10`, `task_11`, `task_12`, `task_13`, and `task_14` still `pending`: `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260708-130224-745100-lab/qa-artifacts/qa/logs/baseline-upstream-task-status.txt`.
- Runtime/surface grep for `SourceWatchEvents|EventSubscription|SupportedWatchEvents|watch_events|watch-events` under `internal/loop`, `internal/daemon`, `internal/store/globaldb`, `web/src/systems/loops`, and site loop docs produced no matches: `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260708-130224-745100-lab/qa-artifacts/qa/logs/baseline-watch-events-runtime-rg.txt` (0 bytes; `rg` exit code 1).
- WS2/WS3 code is present in the current worktree (`ext__dev_cycle__import_tasks`, `AllowedToolsOverride`, shared narrowing helpers): `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260708-130224-745100-lab/qa-artifacts/qa/logs/baseline-ws2-ws3-runtime-rg.txt`.

## Scenario Evidence

- `LP-003` software-delivery did not reach verified done. Initial run failed before `load_tasks` with BUG-0021; after the local fix, the run reached `load_tasks` and failed on BUG-0023:
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260708-130224-745100-lab/qa-artifacts/qa/logs/software-delivery-run-start.json`
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260708-130224-745100-lab/qa-artifacts/qa/logs/software-delivery-slug-input-g1-run.json`
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-bug0021-retest-20260708-132925-584673-lab/qa-artifacts/qa/logs/software-delivery-run-status-2.json`
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-bug0021-retest-20260708-132925-584673-lab/qa-artifacts/qa/logs/load-tasks-g2-task-get.json`
- `LP-046` gated session posture could not be observed because no run-agent session was reached. The initial blocker was BUG-0021; the remaining blocker is BUG-0023 before `execute_task`:
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260708-130224-745100-lab/qa-artifacts/qa/logs/software-delivery-dry-run.json`
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-bug0021-retest-20260708-132925-584673-lab/qa-artifacts/qa/logs/load-tasks-g2-run-list.json`
- `LP-045` direct import-tool parity failed with BUG-0023:
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260708-130224-745100-lab/qa-artifacts/qa/logs/import-tasks-tool-info.json`
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260708-130224-745100-lab/qa-artifacts/qa/logs/import-tasks-valid.json`
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260708-130224-745100-lab/qa-artifacts/qa/logs/daemon-error-logs-after-import-failure.jsonl`
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260708-130224-745100-lab/qa-artifacts/qa/logs/tool-search-import.json`
- `LP-029` reviews-watch was started and remained armed on the watch-source. The gating re-walk could not reach `fix_batch` without a real CodeRabbit review event/account seed:
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260708-130224-745100-lab/qa-artifacts/qa/logs/reviews-watch-run-start.json`
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260708-130224-745100-lab/qa-artifacts/qa/logs/reviews-watch-run-status-2.json`
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260708-130224-745100-lab/qa-artifacts/qa/logs/loop-list-after-runs.json`
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260708-130224-745100-lab/qa-artifacts/qa/logs/logs-after-reviews-watch-observe.json`
- `LP-040..LP-044` and `LP-047..LP-050` were skipped with evidence because the watch-events runtime/web substrate is not present and upstream tasks are not landed:
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260708-130224-745100-lab/qa-artifacts/qa/logs/baseline-upstream-task-status.txt`
  - `/Users/pedronauck/dev/qa-labs/agh-loops-refac-task-16-20260708-130224-745100-lab/qa-artifacts/qa/logs/baseline-watch-events-runtime-rg.txt`

## What Was Fixed

- BUG-0021 was fixed in the worktree by making `source/input` nodes coordinator-owned and resolving their `input_ref` directly from `loop.Run.Inputs` instead of queuing them as loop-action work.
- Regression:
  - Red-before: `go test -race ./internal/loop -run '^TestCoordinatorRunnerShouldResolveInputSourceNodes$' -count=1` failed because the input source queued a node run.
  - Green-after: same command passed after the fix.
  - Scoped package: `go test -race ./internal/loop -count=1` passed.
- Live retest: second lab run advanced past `slug_input` and failed later at `load_tasks`, proving the original BUG-0021 failure mode is gone.

## Runtime Errors Observed

- BUG-0023 direct tool call: `tool_denied`, `source_policy_result: denied`, `reason_codes: ["source_disabled"]`.
- BUG-0023 loop action call after BUG-0021 fix: `unknown_action_kind: loop: unknown action kind; tool "ext__dev_cycle__import_tasks" not found`.
- Pre-fix first-lab daemon restart was blocked by the exhausted coordinator state from BUG-0021: `task "loop.looprun-d8712cd01653cc23.coordinator" exhausted max_attempts=3`.
- CLI observation command mismatch: `agh observe events` is not a valid command in this build; durable task/run/log evidence was captured with `agh task get`, `agh task run list`, `agh loop status`, `agh loop list`, and `agh logs` instead.

## Human Verifications Needed

- `LP-029` needs a real authenticated CodeRabbit review event/account seed before `reviews-watch` can reach `fix_batch` and expose the gated run-agent session posture.
- `LP-043`/`LP-044` browser e2e cannot run until the watch-events authoring form and parked run-detail read-model exist. The bootstrap also reported `BROWSER_BLOCKER=browser-use skill not found in CODEX_HOME plugin cache`.

## Decisions for a Human

- Treat BUG-0023 as the remaining P0 blocker before re-running Task 16 to completion. It affects both direct agent-manageability and the default `software-delivery` loop.
- Keep watch-events rows skipped, not failed, until tasks 8/10/11/12/13/14 land. The Task 15 plan explicitly allowed honest blocked/skipped verdicts for unshipped phases.
- Decide whether to create a follow-up task for the pre-fix daemon restart behavior caused by an exhausted coordinator. It was observed only after BUG-0021 had already produced an unrecoverable first-lab run.

## AGH Impact Audit

- **Native tools:** no new tool IDs or schemas introduced by this QA run. Checked `agh tool info/search/invoke`, `agh loop run/status/list`, `agh task get`, `agh task run list`, and `agh logs`. BUG-0023 shows `ext__dev_cycle__import_tasks` is discoverable but not usable/routable from required public surfaces.
- **Extensibility and hooks:** no new extension/hook contracts were added. BUG-0023 is an existing extension-tool routing/source-policy gap. BUG-0021 changes loop coordinator evaluation for existing `source/input` nodes only.
- **Workspace data isolation:** QA used fresh isolated runtime homes and non-default ports. The BUG-0021 code path resolves run-local `Inputs` inside the existing workspace-scoped `Run`; no new cross-workspace datum or cache was introduced.
- **Official AGH skill:** no public behavior docs or bundled skill changes in this task. A future BUG-0023 fix may require `skills/agh/` updates if the public loop/tool operation contract changes.

## Learnings

- Task 15's phase B/C warning was correct: `LP-047..LP-050` are not executable before tasks 13/14 land.
- `source/input` must be part of the coordinator-owned source family. Treating it as a worker/action node corrupts the loop graph before action execution begins.
- QA state can move out of `untested` even when the result is skipped/blocked, as long as the report carries machine-checkable evidence for the verdict.

## Final Status

Task 16 execution did not reach a green completion gate:

- All Task 15 scoped rows were consumed and moved out of `untested`.
- BUG-0023 remains open and blocks `LP-003`, `LP-045`, and `LP-046`.
- BUG-0021 is fixed and verified in the working tree, with commit pending because auto-commit is disabled.
- `make verify` result is recorded in this report after the final gate run.
