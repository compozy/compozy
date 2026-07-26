# QA Run Report — 2026-07-15 — Northstar Pay

- **Scope:** release-grade `northstar-pay` autonomous collaboration canary alongside the Marketplace exit cycle.
- **Cadence tier:** targeted release-grade real scenario.
- **Build:** `4f73c7066c5f3c7234aac251476b89ca715d9301` plus the open Task 10/11 worktree.
- **Environment:** fresh isolated lab `marketplace-northstar-20260715-20260715-114240-757254`; daemon `http://127.0.0.1:58785`.
- **Provider:** isolated provider home; real Claude session; exactly one in-persona kickoff and no follow-up provider prompt.
- **Started:** 2026-07-15T11:45:08Z · **Status:** BLOCKED

## Persona and Playbook

| Persona | Base | Device / Network / Locale | Session |
|---|---|---|---|
| Sofia Mendes | Northstar Pay product manager | operator session / isolated runtime / en-US | `sess-7b184da0d3d4a217` · turn `turn-6827f16027ff6faf` |

## Session Result

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue |
|---|---|---|---|---|---|---|
| 1 | Materialized `northstar-pay` behavioral charter | declared 11-agent, 12-task launch scenario | Sofia Mendes | autonomous collaboration | Blocked | [BUG-0028](../bugs/BUG-0028.md) |

## Contract Matrix

| Contract item | Required | Observed | Status |
|---|---:|---:|---|
| Declared agents | 11 | 11 materialized | Pass (seed only) |
| Declared task roots | 12 | 12 materialized | Pass (intent only) |
| Declared channels | 10 | 10 materialized | Pass (seed only) |
| One operator kickoff | exactly 1 | 1 | Pass |
| Provider-backed sessions with decisions | At least 1 | 1 healthy PM turn | Pass |
| Task runs | 12 | 0 | Blocked |
| Same objects observed through CLI/API/Web/runtime | At least 3 surfaces | 0 complete objects | Blocked |
| Artifacts used later | At least 2 | 0 | Blocked |
| Completed disruption probes | At least 3 | 0 | Blocked |
| Non-Markdown deliverables | At least 4 | 0 | Blocked |
| Peer messages | At least 14 | 0 | Blocked |
| Complete review cycles | At least 2 | 0 | Blocked |
| Resolved disagreements | At least 1 | 0 | Blocked |
| Active channels in journey log | At least 5 | 0 | Blocked |

## Runtime Observation

- The single Product Manager turn completed normally and posted one launch-room status mentioning the CTO. No second prompt was sent.
- All 12 declared Tasks remained `ready` with zero runs after the five-minute stall threshold. Scheduler diagnostics showed `paused=false`, `queued_run_count=0`, and `active_claim_count=0`.
- The current Tasks contract is explicit: creation saves intent; it does not enqueue execution. The playbook bootstrap created the 12 intents but never started their runs.
- The seeded launch brief, pricing guardrails, and risk memo were written under the lab-root `knowledge/` directory. Agent workspaces are separate named roots under `workspaces/`, and the seeder did not materialize the declared knowledge associations there. The Product Manager therefore searched its workspace, memory, and knowledge surfaces without finding the inputs.
- The Product Manager saw only the two Tasks owned by its workspace. This is correct workspace isolation, not evidence that the other ten Tasks were missing.

## Root-Cause Classification

This pass isolates the repeated stall to the QA playbook/bootstrap contract rather than the production scheduler:

1. The harness materialized task intents but omitted the public start/enqueue boundary required by the Tasks contract.
2. The seeder wrote knowledge outside the roots visible to the assigned workspaces and ignored the playbook's knowledge associations.

The runtime was healthy and unpaused; it had no queued work to claim. The next pass must correct the harness, bootstrap a fresh lab, enqueue the declared Tasks in a controlled pre-kickoff sequence, and repeat the one-kickoff scenario. It must not mutate this stalled lab into a passing run.

## Strict Audit

- Report: `/Users/pedronauck/dev/qa-labs/agh-marketplace-northstar-20260715-20260715-114240-757254-lab/qa-artifacts/qa/qa-audit-report.json`
- Summary: `/Users/pedronauck/dev/qa-labs/agh-marketplace-northstar-20260715-20260715-114240-757254-lab/qa-artifacts/qa/qa-audit-report.md`
- Blocking checks: C6 (0/12 task runs), C7/C8 (missing cross-surface journey evidence), C10/C11 (no reused artifacts or disruption probes), C16 (no declared deliverables), and C17 (no collaboration/review/disagreement/channel activity).
- C14 is expected for this intermediate failed pass: the single final `make verify` gate belongs to the completed Marketplace source freeze, not to a known-blocked lab.

## Process Envelope

- Manifest: `/Users/pedronauck/dev/qa-labs/agh-marketplace-northstar-20260715-20260715-114240-757254-lab/qa-artifacts/qa/bootstrap-manifest.json`.
- Scenario contract: `/Users/pedronauck/dev/qa-labs/agh-marketplace-northstar-20260715-20260715-114240-757254-lab/qa-artifacts/qa/scenario-contract.json`.
- Behavioral charter: `/Users/pedronauck/dev/qa-labs/agh-marketplace-northstar-20260715-20260715-114240-757254-lab/qa-artifacts/qa/behavioral-scenario-charter.yaml`.
- Diagnostic note: `/Users/pedronauck/dev/qa-labs/agh-marketplace-northstar-20260715-20260715-114240-757254-lab/qa-artifacts/qa/notes/bug-0028-marketplace-retest.json`.
- Mandatory teardown evidence: `/Users/pedronauck/dev/qa-labs/agh-marketplace-northstar-20260715-20260715-114240-757254-lab/qa-artifacts/qa/teardown.json` records `clean=true`, no survivors, and completion at `2026-07-15T14:55:02Z`.

## Final Status

**BLOCKED — BUG-0028 reproduced for a third independent pass.** The real-provider kickoff was healthy, but the QA harness neither made the declared knowledge readable to assigned workspaces nor started the 12 saved task intents. No product-scheduler change is justified by this evidence. A fresh post-fix playbook run is required; this lab remains the immutable failing witness.

[QA_BOOTSTRAP]
manifest_path=/Users/pedronauck/dev/qa-labs/agh-marketplace-northstar-20260715-20260715-114240-757254-lab/qa-artifacts/qa/bootstrap-manifest.json
lab_root=/Users/pedronauck/dev/qa-labs/agh-marketplace-northstar-20260715-20260715-114240-757254-lab
runtime_home=/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/aghqa-ee20e019b6aa/runtime
base_url=http://127.0.0.1:58785
verification_report=/Users/pedronauck/Dev/compozy/agh/docs/qa/reports/2026-07-15-northstar-pay.md
health_status=fresh
teardown_path=/Users/pedronauck/dev/qa-labs/agh-marketplace-northstar-20260715-20260715-114240-757254-lab/qa-artifacts/qa/teardown.json
[/QA_BOOTSTRAP]
