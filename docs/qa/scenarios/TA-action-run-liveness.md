---
id: TA-action-run-liveness
area: TA
title: Keep long-running Loop nodes alive without a hidden clock
persona: Ada
journey: J-bound-runaway-work
expected: A Loop node runs for days when the definition has no timeout, fresh ACP activity, an in-flight tool, or a present transport counts as life, silence only raises a self-clearing attention flag, and only an authored node timeout may end the work by duration.
entry_points: `compozy loop runs show <run-id> -o json`; Loop node inventory and event history over CLI/HTTP/UDS; `loops.defaults.delivery.liveness.silence_window`
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
- An explicitly authored node timeout remains the only duration-based terminal path.
