---
id: LP-run-loop-await-child-ordering
area: LP
title: Keep ordered run-loop children blocked until terminal
persona: Bruno
journey: J-await-child-loop
expected: A parent Loop with two ordered run-loop nodes in mode await keeps the first node in awaiting_child while its child is live, starts no second child before the first terminates, restores the same child after daemon restart without duplication, and reaches done only after both children succeed. A declared terminal child result retains loop_run_id and status for a downstream template; an undeclared result keeps its scalar output.
entry_points: compozy loop run; compozy loop status; compozy loop runs; HTTP and UDS Loop run detail
qa_status: pass
bug_ids: 671; 675
fix_status: fixed
retest_status: pass
fix_commits: 81a193db8d9822b85616230d6a3a5a85b6663623
evidence: docs/qa/evidence/2026-09-24-pr-672-run-loop-terminal/parent-after-restart.json; docs/qa/evidence/2026-09-24-pr-672-run-loop-terminal/parent-final.json; docs/qa/evidence/2026-09-24-pr-672-run-loop-terminal/http-parent-final.json
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

Recovery extension: pause a parent after its awaited child fails, rerun that exact child from its
failed QA node without changing the candidate inputs, then resume the parent. Verify the settled
parent node adopts the recovered child's terminal result and exact ID before delivery; no second
review child is created. Repeat after changing the candidate or marking an upstream producer for
rerun and verify a fresh child is created. A different workspace, parent, node task, detached child,
nonterminal child, or timeout failure must never supply reusable proof. This extension requires a
new public-interface replay; the older ordering evidence does not verify recovery adoption.

Immutable candidate proof extension: before the original review, capture `worktree_id`, the
complete selected `include_paths`, and scoped exit-plan `fingerprint`, and supply its JSON serialization as
`inputs.reviewed_worktree` to an optional JSON string input on the child. Retain that exact persisted
proof during recovery. A recovered successful child may be adopted only while the registered
worktree repository, branch, HEAD, selected file bytes and selected index state still match.
Repeat with changed selected content, a new HEAD, or changed selected staged state and verify
normal review execution; repeat without proof and verify the same safe rerun. Never replace the
original fingerprint with a fresh digest to authorize adoption. The domain Git verifier and
Loop/SQLite ownership suites provide focused evidence; the representative public managed-session
replay remains owned by the current reported-issues QA report.
