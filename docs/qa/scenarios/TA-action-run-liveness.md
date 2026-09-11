---
id: TA-action-run-liveness
area: TA
title: Keep long-running Loop nodes alive without a hidden clock
persona: Ada
journey: J-bound-runaway-work
expected: A Loop node without an authored timeout stays live while fresh work evidence remains. The Loop silence window raises attention. Separately, configured session supervision can stop expired work after stop_grace; only verified process-tree exit permits bounded recovery of an authoritative task binding, preserving committed files and consuming max_attempts.
entry_points: `compozy loop status --run-id <run-id> -o json`; Loop node inventory and event history over CLI/HTTP/UDS; `loops.defaults.delivery.liveness.silence_window`
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-ta-replay-20260730-062156-531636-lab/qa-artifacts/qa; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/loop/adjacent-safety-tests.json
last_report:
overlaps: LP-days-long-node-no-clock; LP-crash-death-resume; TA-lease-recovery-attempt-budget
---

QA impact 2026-08-02: the Loop node lifecycle hard cut removes the inherited 7m30s action kill,
`node_timeout`/`no_progress` lease-failure path, and
`task.orchestration.action_run_timeout`. The former pass evidence no longer proves the product
contract. Task 13 owns the isolated public-surface walk after Tasks 07–11 expose the node inventory,
attention events, and current configuration reference.

Forensic evidence contract (SD-006) — each item cites timestamp, exact command, observed output:

- A clock advanced by days while a node with no authored timeout remains live and no duration
  failure event is emitted.
- Fresh activity, an in-flight tool, and transport presence each count as evidence without a
  synthetic heartbeat becoming progress.
- Silence raises one attention flag, new evidence clears it, and neither transition interrupts
  the prompt or frees its lease.
- An authored node timeout bounds that node; session supervision is independently controlled by
  fresh work evidence, `quiet_after` and `stop_grace`.

Issue 616 verification slice (CI only): `TestManagerIntegrationSupervisedWorkRecovery` freezes a
disposable ACP process group with bounded supervision timings and requires process-tree exit before
a linked task attempt appears. `TestGlobalDBSupervisedRecovery` owns attempt exhaustion, prior lease
recovery accounting and the Loop cell/binding transition. `TestSharedSessionStopOperation` owns
receipt replay and explicit-stop exclusion. Run the integration through the required
`Supervised work recovery (integration)` CI lane; no host sessions or local QA labs are used.
The original reporter's 41-minute measurements remain unverified. A successful bounded fixture does
not establish exactly-once external effects or remeasure the production default timers.
