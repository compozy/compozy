---
id: LP-run-loop-await-child-ordering
area: LP
title: Keep ordered run-loop children blocked until terminal
persona: Bruno
journey: J-await-child-loop
expected: A parent Loop with two ordered run-loop nodes in mode await keeps the first node in awaiting_child while its child is live, starts no second child before the first terminates, restores the same child after daemon restart without duplication, and reaches done only after both children succeed.
entry_points: compozy loop run; compozy loop status; compozy loop runs; HTTP and UDS Loop run detail
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /home/franciscpd/dev/qa-labs/compozy-run-loop-await-child-20260813-201607-890089-lab/qa-artifacts/qa/parent-after-restart.json; /home/franciscpd/dev/qa-labs/compozy-run-loop-await-child-20260813-201607-890089-lab/qa-artifacts/qa/final-summary.json
last_report: docs/qa/reports/2026-08-13-run-loop-await-child.md
overlaps:
---

Use two workspace-authored Loops with no extension, agent, provider, skill, or task dependency. The
child parks on a durable wait. The parent invokes it twice through an authored edge and `mode:
await`. Restart after the first child identity is durable, then release each child through the
public node-resume surface and verify the exact run ordering.

Issue: https://github.com/compozy/compozy/issues/386
