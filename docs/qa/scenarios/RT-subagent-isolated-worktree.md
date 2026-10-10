---
id: RT-subagent-isolated-worktree
area: RT
title: Delegate independent code changes on an isolated branch
persona: Ada
journey: J-15-operate-session-via-cli-api
expected: An isolated child starts at the caller HEAD or requested base, runs in a distinct durable worktree, receives delivery instructions, and exposes pinned Git and PR facts when it settles; admitted worktrees survive all subagent terminal paths and recovery preserves changed work.
entry_points: compozy__subagent_delegate; compozy__subagent_status; compozy session subagents show; compozy worktree deliver
qa_status: untested
bug_ids: none
fix_status: none
retest_status: pending
fix_commits: none
evidence: none
last_report: none
overlaps: RT-subagent-delegate; RT-subagent-restart; ET-web-subagent-card; RT-session-spawn-wake
---

Spec: `.compozy/tasks/agent-collaboration/_spec.md`, Business Rules 10–16. Canonical automated
owners: session subagent suite, daemon subagent integration suite and worktree real-Git suite.
This packet does not authorize QA labs; the controller owns the scenario walk after integration.

1. Commit a starting point, leave a separate uncommitted file, and delegate two tasks with the same
   title and `isolation: worktree`. Confirm distinct branches and checkouts at the caller HEAD;
   neither checkout contains the uncommitted file. Replay one request with the same idempotency key:
   its subagent and checkout identities must remain unchanged.
2. Delegate from an isolated child with shared mode, then worktree mode. Confirm inherited checkout
   for shared mode and a new checkout at that child's HEAD for worktree mode. Explicit `base_ref`
   overrides the default. Shared plus a nonblank base is rejected.
3. Commit in an isolated child and deliver with `compozy worktree deliver`. On settlement, compare
   the wake, native status, HTTP/UDS payload and CLI show/JSON: branch, base SHA, commits ahead,
   dirty-file count and PR URL/state agree. CLI show uses the compact branch/base/ahead/clean line,
   a numbered PR link and an Observed timestamp; unknown facts are omitted. Advance the base ref;
   the snapshot stays pinned.
   A missing forge returns unknown; a successful no-PR lookup returns none. Capture an async
   `subagent.settled` hook and compare its isolation/worktree snapshot with native status.
4. Exercise setup failure and hook denial: have setup write a non-ignored file before failing. No
   child is admitted; setup-failed checkouts and branches are force-removed, including setup output.
   The error is respectively isolation_failed or capability_denied.
5. Restart at each compensation boundary, including before worktree association and before child
   linkage. Confirm recovery finds the run-owned checkout and exact deterministic child. Stop and
   join the unadmitted child before cleanup; retain admitted, dirty or ahead checkouts. A failed
   rollback keeps its anchor and retries successfully on a later boot.
6. Complete, cancel, stop the parent and archive. Confirm admitted worktrees remain available and
   existing worktree removal refusals still apply.
