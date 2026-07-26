# QA Run Report — 2026-07-11 — Goal execution

- **Scope:** Full Goal release-confidence cycle from conversational ingress through controls, observation/editor, context/budget recovery, structured operation, restart/race safety, and the Lumen Notes autonomous collaboration playbook.
- **Cadence tier:** full
- **Build:** current Goal worktree (Codex round-8 `SHIP`) · **Environment:** two fresh isolated labs at `http://127.0.0.1:64489` and `http://127.0.0.1:49728`; browser driver `agent-browser`; playbook `consumer-saas-growth`
- **Started:** 2026-07-11T13:11:57Z · **Status:** blocked — provider compatibility
- **Bootstrap manifests:** `/Users/pedronauck/dev/qa-labs/agh-goal-task-08-20260711-20260711-131157-346225-lab/qa-artifacts/qa/bootstrap-manifest.json` and `/Users/pedronauck/dev/qa-labs/agh-goal-task-08-20260711-alpha-20260711-132716-618585-lab/qa-artifacts/qa/bootstrap-manifest.json`

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Power User / autonomous agent | desktop, flaky or wifi, en-US | CH-043, CH-044 |
| Bruno | Power User | desktop, wifi/flaky, en-US | CH-047, CH-041, CH-042, CH-045 |
| Lea | New User | laptop, wifi, en-US | CH-046 |
| Marina | Casual Reviewer | phone-large, 4G, en-US | CH-048 |
| Sol | Accessibility-Reliant | desktop, screen reader/keyboard, en-US | CH-040 |
| Priya Joshi | Head of Growth | operator kickoff, autonomous collaboration | consumer-saas-growth playbook |

## Flows in Scope

- `J-26 Start, converge, and control a conversational Goal` (`../journeys/J-26-converge-and-control-goal.md`)
- `J-27 Observe Goal truth and author a snapshot-pinned Goal node` (`../journeys/J-27-observe-and-author-goal.md`)
- `J-28 Recover from context pressure and budget boundaries truthfully` (`../journeys/J-28-recover-context-and-budget.md`)
- `J-29 Operate and recover a Goal without UI-only shortcuts` (`../journeys/J-29-operate-and-recover-goal.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---:|---|---|---|---|---|---|---|
| 1 | CH-044 | J-29 / GL-029..032,036..037,039 | Ada | Interrupt Tour | Blocked (needs human verify) | Required live provider rejects `gpt-5.6-sol` before the first agent decision | |
| 2 | CH-042 | J-28 / GL-021,038,040 | Bruno | Network Tour | Blocked (needs human verify) | Required live provider rejects `gpt-5.6-sol` before the first agent decision | |
| 3 | CH-041 | J-28 / GL-017..020,040 | Bruno | Interrupt Tour | Blocked (needs human verify) | Required live provider rejects `gpt-5.6-sol` before the first agent decision | |
| 4 | CH-047 | J-26 / GL-005..008,010..012 | Bruno | Interrupt Tour | Blocked (needs human verify) | Required live provider rejects `gpt-5.6-sol` before the first agent decision | |
| 5 | CH-043 | J-29 / GL-025..028,034,036 | Ada | Feature Tour | Blocked (needs human verify) | Required live provider rejects `gpt-5.6-sol` before the first agent decision | |
| 6 | CH-046 | J-26 / GL-001..004,009..013 | Lea | Feature Tour | Blocked (needs human verify) | Required live provider rejects `gpt-5.6-sol` before the first agent decision | |
| 7 | CH-048 | J-27 / GL-014..016,035 | Marina | Feature Tour | Blocked (needs human verify) | Required live provider rejects `gpt-5.6-sol` before the first agent decision | |
| 8 | CH-040 | J-27 / GL-014..015,022,033,035 | Sol | Back-Button Tour | Blocked (needs human verify) | Required live provider rejects `gpt-5.6-sol` before the first agent decision | |
| 9 | CH-045 | J-27 / GL-022..024 | Bruno | Feature Tour | Blocked (needs human verify) | Required live provider rejects `gpt-5.6-sol` before the first agent decision | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Real-Scenario Playbook

- **Playbook:** `consumer-saas-growth` (Lumen Notes activation sprint)
- **Materialized:** 4 workspaces, 7 differentiated agents, 4 channels, 11 open tasks, 5 knowledge files.
- **Required deliverables:** 2 TSX pages, 1 TSX component, 2 TS modules, 2 TS tests, 1 SQL migration, 1 runbook, 1 decision spec.
- **Required collaboration:** 12 peer messages, 3 review cycles, 1 resolved disagreement, 3 active channels.
- **Provider requirement:** live provider-backed session required; an unreachable provider produces BLOCKED, never PASS.

### Provider attempts

- Pass 1 opened live Codex session `sess-dd69a2fb7c3490e7` with `gpt-5.6-sol`, but the single kickoff returned HTTP 400: Codex CLI `0.144.1` was too old for the model. Strict audit returned exit 2; teardown completed with `clean: true` at `/Users/pedronauck/dev/qa-labs/agh-goal-task-08-20260711-20260711-131157-346225-lab/qa-artifacts/qa/teardown.json`.
- Pass 2 used a fresh lab and an isolated command override supplying Codex CLI `0.145.0-alpha.4` plus `codex-acp` `0.16.0`. Session `sess-97ad54efedba6242` opened, but the single kickoff returned the same HTTP 400 before any agent decision. Strict audit returned exit 2; teardown completed with `clean: true` at `/Users/pedronauck/dev/qa-labs/agh-goal-task-08-20260711-alpha-20260711-132716-618585-lab/qa-artifacts/qa/teardown.json`.
- The second pass materialized 4 workspaces, 7 workspace agents, 11 queued task runs, and all four playbook collaboration channels before kickoff. The browser reached the served Web app; evidence is indexed at `qa-artifacts/qa/screenshots/web-home.png` in the second lab.

## What Was Fixed

- **BUG-0033:** daemon boot started recovered Loop coordinators before observer/session reconciliation completed. A shared atomic gate now suppresses the recurring scheduler and Loop hook/watch observers until `bootFinalize`; the initial backstop runs after the barrier and the recurring scheduler starts last. The gate regression, canonical parked-watch restart E2E, and full 70-case runtime lane pass.
- **BUG-0034:** task recovery co-committed `task.recovered` but did not fan its exact identity out to already-connected subscribers. The immediate store transaction now returns the committed `EventRecord`, which is published directly to the domain observer and every live stream. GlobalDB identity coverage, Manager coverage with two subscribers plus a neighboring same-type record, and the two-tab browser E2E pass.
- QA helper path resolution now follows the nested `.agents/skills/agh/{agh-qa-bootstrap,real-scenario-qa}` layout. The smoke test and playbook loader validation pass.
- Seven stale browser-E2E assumptions were aligned with current product contracts: settled-turn folding, the standalone MCP route, nested Skills detail routes, MSW's non-2xx behavior, task-stream readiness, current Vault list/filter semantics, and current Storybook bootstrap diagnostics. These are test-infrastructure corrections, not product bug entries.

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|
| Priya Joshi | Playbook kickoff | Session creation succeeds, but the provider rejects the selected model only when the first prompt is sent | Blocks the entire live collaboration tour | Provider compatibility blocker; no product verdict inferred |
| Operator | Direct browser diagnostics | A direct Playwright invocation can serve stale embedded assets unless `AGH_WEB_DIST_DIR` points at the freshly built `web/dist` | Misleading local failures | The authoritative `make test-e2e-web` lane sets the correct runtime contract |

## Runtime Errors Observed

- Pre-session infrastructure correction: bootstrap path resolution expected the former flat real-scenario skill path. Fixed the helper to resolve `.agents/skills/agh/real-scenario-qa`; smoke test and playbook validation passed before this lab was created.
- Provider boundary: public `codex-acp` `0.16.0` rejects `gpt-5.6-sol` even when launched alongside the newest available Codex alpha (`0.145.0-alpha.4`). Both exact kickoff streams and provider-attempt records are retained in their lab evidence roots. Per the real-scenario contract, this lane is BLOCKED rather than PASS.

## Human Verifications Needed

- Re-run CH-046..CH-045 in a fresh isolated lab after the public `codex-acp` path accepts `gpt-5.6-sol`. Do not substitute a different model: the operator explicitly selected Codex / `gpt-5.6-sol` / maximum reasoning.
- Complete the Lumen Notes collaboration playbook through its deliverables, peer-message minimum, review cycles, disagreement resolution, and channel activity requirements.
- Independently reread the resulting CLI/HTTP/UDS/native/Web state and transition GL-001..GL-040 only from the captured public-surface evidence.

## Decisions for a Human

None. This is an external provider compatibility prerequisite, not a product-scope decision.

## Learnings

- Coordinator activation belongs after durable boot reconciliation. Combining recovery and activation in one boot helper creates a hidden phase-order race even when each component is correct in isolation.
- A co-committed transactional event is not automatically live-delivered. The owning transaction must return its exact immutable event identity; reconstructing it later from a cursor and type is not concurrency-safe proof.
- Automated E2E lanes can find and verify product regressions, but they do not replace the Task 08 persona charters or the live-provider playbook contract.

## Automated Verification

- `make test-e2e-runtime` — passed all 70 runtime cases after BUG-0033.
- `make test-e2e-web` — passed all 62 Playwright cases after BUG-0034 and fixture-contract remediation.
- `go test -race ./internal/task/...` — passed all 467 task package tests.
- `make lint` — passed with zero issues.
- Full monorepo `make verify` — the pre-round-9 fix loop passed with exit code 0; a fresh post-remediation run is pending. The Vite chunk-size advisory is the known non-blocking build advisory; lint reported no warnings or errors.

## AGH Impact Audit

- **Native tools:** No native tool IDs, toolsets, descriptors, schemas, digests, risk flags, or capability gates changed. Checked the Task recovery and Loop boot fixes; they alter delivery/activation timing behind existing task and Loop surfaces only.
- **Extensibility and hooks:** Existing `task.recovered` observer delivery now receives the exact record returned by the committing transaction, identical to live SSE subscribers. Loop hook/watch observers share the boot-ready coordinator gate with the scheduler. No hook ID, capability, bundle, registry, bridge SDK, MCP sidecar, config lifecycle, or watch-event contract changed.
- **Workspace data isolation:** Both fixes preserve existing scopes. `task.recovered` is emitted only from the exact task record loaded by task ID after its transactional cursor, and existing stream authorization remains the subscriber boundary. Loop recovery continues to use workspace-scoped durable cursors and the canonical cross-workspace E2E negative.
- **Official AGH skill:** No update required for BUG-0033/0029. Public commands, tool IDs, hook names, Goal behavior, and operational guidance are unchanged; only correctness of existing boot and event-delivery contracts changed.

## Final Status

- **Exit gate:** PENDING — post-round-9 full monorepo `make verify` has not run yet; runtime 70/70, browser 62/62, task package 467/467, boot-order 3/3, and lint zero issues are green.
- **Issues by user impact:** 2 Blocks-Completion product defects found and fixed (BUG-0033, BUG-0034); 1 external provider compatibility blocker remains.
- **Coverage:** 0/4 journeys walked; 9/9 persona charters blocked before the first agent decision. GL-001..GL-040 are `blocked-verify`, not passed.
- **Verdict:** **BLOCKED.** Automated backbones are green, but Task 08's required live-provider charter execution and real-scenario collaboration contract are incomplete.

```yaml qa-bootstrap
manifest_paths:
  - /Users/pedronauck/dev/qa-labs/agh-goal-task-08-20260711-20260711-131157-346225-lab/qa-artifacts/qa/bootstrap-manifest.json
  - /Users/pedronauck/dev/qa-labs/agh-goal-task-08-20260711-alpha-20260711-132716-618585-lab/qa-artifacts/qa/bootstrap-manifest.json
lab_roots:
  - /Users/pedronauck/dev/qa-labs/agh-goal-task-08-20260711-20260711-131157-346225-lab
  - /Users/pedronauck/dev/qa-labs/agh-goal-task-08-20260711-alpha-20260711-132716-618585-lab
runtime_homes:
  - /Users/pedronauck/dev/qa-labs/agh-goal-task-08-20260711-20260711-131157-346225-lab/.agh
  - /Users/pedronauck/dev/qa-labs/agh-goal-task-08-20260711-alpha-20260711-132716-618585-lab/.agh
base_urls:
  - http://127.0.0.1:64489
  - http://127.0.0.1:49728
verification:
  runtime_e2e: pass-70
  browser_e2e: pass-62
  task_race: pass-467
  make_verify: pending-post-round-9
teardown:
  - /Users/pedronauck/dev/qa-labs/agh-goal-task-08-20260711-20260711-131157-346225-lab/qa-artifacts/qa/teardown.json
  - /Users/pedronauck/dev/qa-labs/agh-goal-task-08-20260711-alpha-20260711-132716-618585-lab/qa-artifacts/qa/teardown.json
teardown_clean: true
blocker: public codex-acp 0.16.0 rejects gpt-5.6-sol before the first agent decision
```
