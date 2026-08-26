# QA Run Report — 2026-08-26 — Loop issue fixes

- **Scope:** GitHub issues #451, #472, #480, #485, #486, and #489 — Loop recovery, failure policy, completion time, and effective config provenance
- **Cadence tier:** targeted
- **Build:** `151e299e6` plus documentation-format cleanup · **Environment:** fresh isolated lab, `http://127.0.0.1:55115`, CLI/API/Web/runtime required, no provider required
- **Started:** 2026-08-26T20:07:13Z · **Status:** behavior complete; exact-head CI pending

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | desktop / wifi-fast / en-US | CH-loop-effective-config-truth |
| Ada | Power User | desktop / wifi-fast / en-US | CH-loop-effective-config-truth, CH-loop-terminal-time-recovery |
| Lea | New User | laptop / wifi-fast / en-US | CH-008 |

## Flows in Scope

- `J-configure-and-run-loop` — reuse reviewed settings, explain each winning value, and enforce the selected failure policy (`../journeys/J-configure-and-run-loop.md`)
- `J-loop-terminal-recovery` — preserve exact terminal time and isolate invalid persisted history during recovery (`../journeys/J-loop-terminal-recovery.md`)
- `J-02` — preview a Loop without creating work, used as the adjacent canary (`../journeys/J-02-dry-run-preview.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-loop-effective-config-truth | J-configure-and-run-loop / LP-effective-config-provenance | Bruno + Ada | Feature Tour | Pass | | |
| 2 | CH-loop-effective-config-truth | J-configure-and-run-loop / LP-halt-on-node-failure | Bruno + Ada | Feature Tour | Fixed | BUG-20260826-halt-rerun-busy | `151e299e6` |
| 3 | CH-loop-terminal-time-recovery | J-loop-terminal-recovery / LP-terminal-completion-time | Ada | Interrupt Tour | Pass | | |
| 4 | CH-loop-terminal-time-recovery | J-loop-terminal-recovery / LP-invalid-snapshot-boot-isolation | Ada | Interrupt Tour | Blocked (needs human verify) | Public corruption fixture unavailable | |
| 5 | CH-008 | J-02 / LP-006 | Lea | Garbage Tour | Pass | | |
| 6 | CH-008 | J-02 / LP-007 | Lea | Garbage Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-loop-effective-config-truth — Bruno + Ada

- **Ran:** 2026-08-26T20:09Z–20:16Z (box respected: yes)
- **Findings:** Built-in and stored Loop sources were visible before admission. Explicit `0` and
  `false` survived dry-run resolution with `per_run` provenance, and changing current config did
  not rewrite the run's pinned sources. The halt run failed once in generation 1. An operator rerun
  from `load_tasks` admitted generation 2 through six downstream nodes, including nodes that had
  never materialized. A later read still showed exactly two generations.
- **Independent reads:** CLI admission and status, HTTP run detail, native status tool, and Web run
  list after refresh.
- **Bugs filed/updated:** BUG-20260826-halt-rerun-busy → verified.
- **Scenarios settled:** LP-effective-config-provenance → pass; LP-halt-on-node-failure → fixed.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-loop-issues-fixes-rerun-20260826-200713-569291-lab/qa-artifacts/qa/logs/explicit-rerun-current-head.json`;
  `/Users/pedronauck/dev/qa-labs/compozy-loop-issues-fixes-rerun-20260826-200713-569291-lab/qa-artifacts/qa/logs/api-status-current-head.json`.
- **Paper cuts:** None.

### CH-loop-terminal-time-recovery — Ada

- **Ran:** 2026-08-26T20:10Z–20:15Z (box respected: yes)
- **Findings:** The rerun's live status omitted `completed_at`. After its deterministic failure,
  CLI and HTTP exposed `2026-08-26T20:10:50.647475Z`. Web displayed `Failed`, `2 rounds`,
  and a `1m 13s` duration that remained unchanged across refresh while relative age advanced.
- **Scenarios settled:** LP-terminal-completion-time → pass; LP-invalid-snapshot-boot-isolation →
  blocked-verify.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-loop-issues-fixes-rerun-20260826-200713-569291-lab/qa-artifacts/qa/logs/terminal-status-current-head.json`;
  `/Users/pedronauck/dev/qa-labs/compozy-loop-issues-fixes-rerun-20260826-200713-569291-lab/qa-artifacts/qa/screenshots/loop-rerun-terminal-refresh.png`.
- **Blocked boundary:** The public product has no operator-owned way to persist a deliberately
  invalid definition snapshot. Store-backed integration coverage cannot become a persona-level pass.

### CH-008 — Lea

- **Ran:** 2026-08-26T20:16Z–20:17Z (box respected: yes)
- **Findings:** Empty `slug` showed `slug is required to run this loop.` without rendering a
  plan. With `slug=missing-qa`, Web rendered a generation-1 plan with 13 nodes; CLI preserved
  explicit zero/false overrides. A fresh run list still contained only the pre-existing run.
- **Scenarios settled:** LP-006 → pass; LP-007 → pass.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-loop-issues-fixes-rerun-20260826-200713-569291-lab/qa-artifacts/qa/logs/web-dry-run-canaries.json`.
- **Paper cuts:** None.

## What Was Fixed

- BUG-20260826-halt-rerun-busy: rerun admission now distinguishes selected pending history from
  active work and traverses the authored graph through unmaterialized nodes. Regression coverage:
  `TestCoordinatorOperatorRerunPlanner` and
  `TestServiceTimeTravelShouldPreserveHistoryContracts`. Retest passed on `151e299e6`.

## Paper Cuts

None.

## Runtime Errors Observed

- Expected deterministic payload failure: `No task set matched
  .compozy/tasks/missing-qa/task_*.md.` It is the journey fixture, not a product defect.

## Human Verifications Needed

- LP-invalid-snapshot-boot-isolation: provide an operator-owned fixture that persists one invalid
  definition snapshot beside one healthy run. Restart the isolated daemon, verify readiness, then
  read both runs through CLI and HTTP. The invalid run must terminalize with its structured snapshot
  cause while the healthy run restores normally.

## Decisions for a Human

None.

## Edge Probes

- Explicit `0` and `false` config values: preserved with source attribution.
- Unknown config field: rejected before run creation.
- Missing required Web input: inline error, no plan, no run.
- Rerun across pending, unmaterialized descendants: admitted only inside the selected closure.
- Terminal Web refresh: durable duration and generation count preserved.

## Experiential Lenses

| Journey | Usability | Accessibility | Perceived performance | Compatibility | Error recovery | Production parity |
|---|---|---|---|---|---|---|
| J-configure-and-run-loop | pass | pass — existing labeled controls and structured CLI output | pass | pass — CLI plus current Chrome Web leg | pass — deterministic error named the corrective action and explicit rerun succeeded | friction — local production build and real daemon, but no packaged desktop |
| J-loop-terminal-recovery | pass | pass — status is textual, not color-only | pass | pass — CLI, HTTP, and current Chrome agreed | pass for rerun; public invalid-snapshot injection remains blocked | friction — isolated local daemon, no provider required |

## Learnings

- Taxonomy coverage: functional truth, error/recovery, continuity, and cross-surface consistency are in scope. Responsive and accessibility checks are not applicable to the changed data contracts; Web layout is unchanged. Production-provider coverage is deliberately skipped because these bounded journeys require no provider.
- A halted graph can contain valid pending history beyond the failed node. Rerun admission must reason
  about the selected graph closure, not treat the word `pending` as proof of live execution.

## Exit Gates

- Focused Go regression command passed before rebase:
  `go test -race ./internal/loop -run '^(TestCoordinatorOperatorRerunPlanner|TestServiceTimeTravelShouldPreserveHistoryContracts)$'`.
- Fresh current-head `make build` passed in the isolated lab.
- The operator explicitly deferred the scoped local `make gate` rerun to PR CI.
- PR #492 exact-head CI is pending. Its first frontend lane found only MDX format drift in
  `guardrails.mdx` and `running.mdx`; the official formatter corrected both. The second frontend
  pass then exposed Loop mock fixtures that had not co-shipped the newly required
  `effective_config` and `request_expire_after` fields. The fixtures now satisfy the generated
  contract, and the focused Web typecheck passed before the next push.
- Final CI evidence will be recorded at
  `/Users/pedronauck/dev/qa-labs/compozy-loop-issues-fixes-rerun-20260826-200713-569291-lab/qa-artifacts/qa/final-make-verify.log`.
- Strict evidence audit covered one run id across CLI, API, Web, and runtime. Its only remaining
  blocker is C14, the deliberately deferred local gate; it will be settled with exact-head PR CI
  evidence after push.
- Lab teardown:
  `/Users/pedronauck/dev/qa-labs/compozy-loop-issues-fixes-rerun-20260826-200713-569291-lab/qa-artifacts/qa/teardown.json`
  reports `"clean": true`, with the daemon and Web process stopped and no survivors.

## Final Status

**BLOCKED** — behavior is ready with one documented public-fixture verification block. Delivery remains not ready
until the formatter fix and QA write-back are pushed and every required check on PR #492's new head
is green.
