---
id: LP-runs-roster-server-ordering
area: LP
title: Rank the runs roster by what needs a human, on the server
persona: Ada
journey: J-operate-loop-run-headless
expected: Every runs-list read returns needs-you runs first, then active, then terminal, with the ordering applied before pagination so a run that needs a human never falls off page one; each item carries progress (round, steps done, steps total) always and an attention object (kind, count, since) only when something is actually waiting; CLI columns, HTTP and UDS responses agree on the same persisted state, a malformed cursor is a field-addressed 400 invalid_cursor, and a run id from another workspace resolves to 404 rather than an empty success.
entry_points: compozy loop runs; compozy loop runs --loop <loop-name> --status <status> -o json; GET /api/workspaces/:workspace_id/loop-runs over HTTP and UDS; /docs/cli/loop/runs
qa_status: pass
bug_ids: BUG-20260719-autonomous-progress-unobservable
fix_status: fixed
retest_status: pass — runtime-owned observer followed catalog advancement and matched all 11 terminal Tasks
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-runtime-20260821-1126-20260821-112711-004724-lab/qa-artifacts/qa/headless/roster-limit1.json; /Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-runtime-20260821-1126-20260821-112711-004724-lab/qa-artifacts/qa/headless/roster-page2.json; /Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-runtime-20260821-1126-20260821-112711-004724-lab/qa-artifacts/qa/observation-summary.json
last_report: docs/qa/reports/2026-08-21-loop-task-legibility.md
overlaps: LP-run-read-agent-journey; LP-web-runs-roster-rerank; LP-web-runs-breadcrumb; GL-016
---

story: As an agent watching a workspace I ask for the runs list and the first thing I get back is whatever needs a human — I never have to page through terminal runs to find the run that is blocked.

Ordering is the contract, not a convenience. Walk it with more runs than one page holds and at least one needs-you run seeded late, so a client-side sort would put it on page two: the server has to rank before it paginates or the roster lies at exactly the moment it matters. Compare the same persisted state over CLI, HTTP and UDS and confirm the order is identical on all three.

`progress` is served on every item and never recounted by a client (Safety Invariant 12); `attention` is absent — not zero, not empty — when nothing waits, so absence is the signal. Confirm both shapes on the same run as it moves from active to parked to terminal.

This scenario carries the runtime-owned progress surface that `BUG-20260719-autonomous-progress-unobservable` has been missing for six reproductions: an observer reading `progress` from this list must be able to distinguish real advancement from a stall without tailing any log.

QA impact 2026-08-21: Task 03 extended the runs list with server-owned ordering, attention and progress. Planning flag only; the loop's QA phase owns the real-daemon walk and evidence.

QA result 2026-08-21: a needs-you run created last ranked first before the `limit=1` cut; every
row carried progress and terminal rows omitted attention. Cursor paging returned each run once.
The roster assertions passed, but the scenario is `blocked-decision`: the one-kickoff observer still
uses a lab-local journey log instead of the public runtime reads and therefore cannot satisfy the
closure contract for `BUG-20260719-autonomous-progress-unobservable`.

QA closure 2026-08-21: the public-read observer remained healthy while independent catalog captures
advanced from 4 to 11 completed Tasks, recorded eight durable transitions, and matched every final
Task id/status. Together with the earlier server-ordering, paging, progress, and attention walk,
this resolves the row's only blocked decision and the scenario now passes. Evidence:
`/Users/pedronauck/dev/qa-labs/compozy-loop-legibility-observer-closure-20260821-130214-633585-lab/qa-artifacts/qa/observer-catalog-comparison.json`.
