# QA Run Report — 2026-07-15 — Northstar Pay Capacity Retest

- **Scope:** Release-grade `northstar-pay` one-kickoff collaboration retest for Marketplace Task 11 and scheduler policy A.
- **Build:** Source-freeze tree `55aa6b498297ad483d89d355b3febce60e6d7f68` from detached verification commit `2d129da2809b0edacb9624c317fd9bd54beb7d55` (parent `282366f84`).
- **Environment:** Fresh isolated lab `marketplace-northstar-capacity-final-20260715-20260716-001326-274237`; daemon `http://127.0.0.1:51692`.
- **Provider:** Real Claude Sonnet 5 sessions from an isolated operator home; exactly one in-persona kickoff and no follow-up provider prompt.
- **Observation:** 2026-07-16T00:31:46Z–01:01:47Z; full 30-minute window; no stall.
- **Status:** PASS — behavioral scenario, source-freeze verification, and strict evidence seal.

## Persona and Playbook

| Persona | Role | Workspace | Session |
|---|---|---|---|
| Sofia Mendes | Founder / Product Manager | `ws_e8dd9b7b08652e8b` | `sess-5b0ad67110cdf556` · turn `turn-5e5490ccb5a59074` |

The provider completed the single kickoff normally, held the scheduler activation barrier, refused to invent a partner-timeout decision before the trigger, and moved coordination to `default` when its interactive session correctly lacked presence in task-specific channels. No operator recovery prompt was sent.

## Contract Matrix

| Contract item | Required | Observed | Status |
|---|---:|---:|---|
| Declared agents / differentiated roles | 11 / 11 | 11 / 11 | Pass |
| Declared root Tasks / runs | 12 / 12 | all 12 root Tasks completed | Pass |
| One operator kickoff | exactly 1 | 1 | Pass |
| Provider-backed decision session | at least 1 | 1 live PM session with recorded decisions | Pass |
| Cross-surface objects | at least 3 | 4 Tasks across CLI, API, Web, and runtime | Pass |
| Artifacts used later | at least 2 | pricing contract and canary service reused downstream | Pass |
| Completed disruption probes | 3 | 3 | Pass |
| Non-Markdown deliverables | at least 4 | all declared types valid; 29 valid non-Markdown files | Pass |
| Peer messages | at least 14 | 23 indexed by the auditor | Pass |
| Complete review cycles | at least 2 | 2 | Pass |
| Resolved disagreements | at least 1 | 1 duplicate-work collision resolved | Pass |
| Active channels | at least 5 | 9 | Pass |

## Serial-Capacity Proof

The frontend pool had one task-role session, `sess-264a441966e18cbf`. It processed the three root runs serially:

1. `task-northstar-pay-001` / `run-e5f0c864448cbee3` claimed and completed.
2. `task-northstar-pay-002` / `run-a440c3d23c27afb8` was claimed by the same session while task 004 remained queued.
3. `task-northstar-pay-004` / `run-f10508ac6352685f` was then claimed by the same session and completed.

During the occupied window, CLI and HTTP scheduler status both reported `starved_run_count=0` and `needs_attention_run_count=0`. Runtime JSONL recorded repeated `scheduler.capacity_waiting` decisions for compatible queued work. The final CLI and API snapshots still reported zero starvation and zero attention while autonomous follow-up work continued.

This proves policy A end to end: compatible busy capacity freezes convergence, does not spawn an elastic worker, and releases the serial backlog to the same session when the active lease completes.

## Disruption Recovery

| Probe | Trigger | Recovery | Result |
|---|---|---|---|
| `partner_timeout` | Stale partner replay ETA written after minute 5 | PM gated BR to the fallback banner, kept MX live, and held promotion; CTO confirmed the arm within the recovery window | Pass |
| `pricing_claim_violation` | Explicit “zero fees” message posted to `growth-launch` after minute 12 | Copywriter restored the approved headline/subhead; compliance independently checked shipped source and approved NP-007 | Pass |
| `canary_error_budget_breach` | Urgent task event queued after minute 18 | Release manager verified the 422 budget gate, paused at stage 10, confirmed the 409 pause lock, preserved rollback, and posted the decision to `release-control` | Pass |

An optional `exec-signal` cross-post from the release-manager task-role session failed because that session had no local peer in the channel. The equivalent CLI diagnostic was the expected `network: local peer not found`; the hosted native tool intentionally exposed only the safe `tool_backend_failed` mask. This is the established security boundary, not a new product defect, and `release-control` satisfied the playbook recovery contract.

## Deliverable Validation

- `npm test`: 2 files, 17 tests passed.
- `npm run build`: Next.js production build, typecheck, and route generation passed.
- `go test ./...` in `services/canary-control`: 10 tests across 3 packages passed.
- `sh -n workspaces/platform-control/scripts/verify-partner-replay.sh`: passed.
- Web Tasks and Network views rendered the root-task completions, disruption decisions, and collaboration history with no browser console or page errors.

The evidence auditor initially counted only `.test.ts` and ignored the valid React Vitest artifact `test/hero-claim-variants.test.tsx`. The canonical QA bootstrap smoke suite reproduced the false negative, and the auditor now accepts both `.test.ts` and `.test.tsx` for the `ts_test` contract. The regression passes.

## Evidence

- Lab root: `/Users/pedronauck/dev/qa-labs/agh-marketplace-northstar-capacity-final-20260715-20260716-001326-274237-lab`.
- Provider stream: `qa-artifacts/qa/operator-kickoff.jsonl`.
- Serial handoff: `qa-artifacts/qa/cli/tasks-serial-handoff.json` and `qa-artifacts/qa/api/tasks-serial-handoff.json`.
- Capacity windows: `qa-artifacts/qa/cli/scheduler-capacity-window-2.json`, `qa-artifacts/qa/api/scheduler-capacity-window-2.json`, and `qa-artifacts/qa/runtime/scheduler-capacity-waiting.jsonl`.
- Final state: `qa-artifacts/qa/cli/tasks-final.json`, `qa-artifacts/qa/api/tasks-final.json`, and `qa-artifacts/qa/observation-window.json`.
- Web evidence: `qa-artifacts/qa/web/tasks-final.png` and `qa-artifacts/qa/web/network-final.png`.
- Recovery evidence: `qa-artifacts/qa/cli/growth-launch-pricing-recovery.json`, `pricing-compliance-verdict.json`, `release-control-canary-recovery.json`, and `canary-recovery-task.json`.
- Pre-seal audit: `qa-artifacts/qa/qa-audit-report.json`; its deferred C12/C14 checks are closed by this final report and source-freeze gate.
- Full source-freeze gate: `/Users/pedronauck/dev/qa-labs/agh-marketplace-northstar-capacity-final-20260715-20260716-001326-274237-lab/qa-artifacts/qa/final-make-verify.log`.

## Process Envelope

- Bootstrap manifest: `qa-artifacts/qa/bootstrap-manifest.json`.
- Runtime workspace: `/Users/pedronauck/dev/qa-labs/agh-marketplace-northstar-capacity-final-20260715-20260716-001326-274237-lab/project`; the repository root was never registered into the lab.
- Browser session `northstar-capacity` was closed; the unrelated `agh-network-qa` session was untouched.
- Mandatory teardown completed at `2026-07-16T01:10:09Z`.
- `qa-artifacts/qa/teardown.json` records `clean=true`, `survivors=[]`, and termination of the registered daemon and observer PIDs.

## AGH Impact Audit

- **Native tools:** No Marketplace/native-tool contract changed in this retest. Hosted `agh__network_send` safe error masking was checked against the more specific CLI diagnostic and remains unchanged.
- **Extensibility and hooks:** No new extension, hook, capability, bundle, registry, bridge, MCP sidecar, or config lifecycle surface. Scheduler documentation now distinguishes compatible capacity wait from starvation.
- **Workspace data isolation:** All Tasks, runs, sessions, Network messages, CLI/API reads, Web caches, and disruption evidence used only `ws_e8dd9b7b08652e8b`; no cross-workspace object appeared.
- **Official AGH skill:** The task-orchestration references document serial capacity waiting and durable starvation semantics; no tool ID or public command changed.

## Final Seal

Behavioral verdict: **PASS**. Source-freeze verdict: **PASS**. `make verify` passed every monorepo gate against tree `55aa6b498297ad483d89d355b3febce60e6d7f68`, including 15,769 race-enabled Go tests, Web build, Go build, and package boundaries. Strict verdict: **PASS**.
