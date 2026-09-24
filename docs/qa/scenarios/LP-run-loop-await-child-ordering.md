---
id: LP-run-loop-await-child-ordering
area: LP
title: Keep ordered run-loop children blocked until terminal
persona: Bruno
journey: J-await-child-loop
expected: A parent Loop with two ordered run-loop nodes in mode await keeps the first node in awaiting_child while its child is live, starts no second child before the first terminates, restores the same child after daemon restart without duplication, and reaches done only after both children succeed. A declared terminal child result retains loop_run_id and status for a downstream template; an undeclared result keeps its scalar output.
entry_points: compozy loop run; compozy loop status; compozy loop runs; HTTP and UDS Loop run detail
qa_status: pass
bug_ids: 671
fix_status: fixed
retest_status: pass
fix_commits: 81a193db8d9822b85616230d6a3a5a85b6663623
evidence: /Users/pedronauck/dev/qa-labs/compozy-pr-672-run-loop-terminal-20260924-194241-052016-lab/qa-artifacts/qa/cli/parent-after-restart.json; /Users/pedronauck/dev/qa-labs/compozy-pr-672-run-loop-terminal-20260924-194241-052016-lab/qa-artifacts/qa/cli/parent-final.json; /Users/pedronauck/dev/qa-labs/compozy-pr-672-run-loop-terminal-20260924-194241-052016-lab/qa-artifacts/qa/cli/http-parent-final.json
last_report: docs/qa/reports/2026-09-24-pr-672-run-loop-terminal.md
overlaps:
---

Use two workspace-authored Loops with no extension, agent, provider, skill, or task dependency. The
child parks on a durable wait. The parent invokes it twice through an authored edge and `mode:
await`. Restart after the first child identity is durable, then release each child through the
public node-resume surface and verify the exact run ordering.

Issue 671 extension: give one awaited node `produces: {loop_run_id: string, status: string}` and
use `{{ .nodes.<id>.output.loop_run_id }}` in a downstream prompt. After child `done` and `no-op`,
verify the persisted node output and rendered prompt use the exact child ID and terminal status.
Repeat without `produces` to verify the scalar terminal marker remains, and confirm validation
rejects an unsupported field or type before starting a child. Validation also rejects constraints
on `loop_run_id` or `status` that terminal values cannot guarantee, such as a status enum containing
only `done`. The 2026-09-24 isolated CLI/HTTP walk confirmed the same first child ID after a
daemon restart, a rendered downstream receipt with its ID and terminal status, and the unchanged
scalar output for the second node without `produces`. Both child runs and the parent reached
`done`; the live definition validator rejected constrained and extra output fields.

Issue: https://github.com/compozy/compozy/issues/386
