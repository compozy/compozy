---
id: RT-worktree-cli-lifecycle
area: RT
title: Manage a worktree lifecycle through structured CLI output
persona: Ada
journey: J-worktree-management
expected: Creating, cancelling, adopting, listing, inspecting, refreshing status, executing or cancelling an assisted exit, refusing an unsafe removal, forcing removal, and dismissing through `compozy worktree` never falls back to the workspace root; every `<ref>` verb behaves the same for a name or ID, returns truthful structured output and deterministic exit codes, and remove → dismiss makes the old name reusable without losing ID-addressed history.
entry_points: compozy worktree list|create|cancel|adopt|inspect|status|exit|commit|push|pr|exit-cancel|remove|dismiss -o json|jsonl
qa_status: pass
bug_ids: BUG-20260814-worktree-mutation-output-loses-identity
fix_status: fixed
retest_status: pass
fix_commits: working-tree
evidence: /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/cli-worktree-create.jsonl; /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/cli-worktree-dirty-exit-remove.jsonl; /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/cli-worktree-force-remove.json; internal/daemon/daemon_worktree_e2e_integration_test.go; /Users/pedronauck/dev/qa-labs/compozy-worktree-lifecycle-fixes-20260815-004729-655016-lab/qa-artifacts/qa/logs/cli-retest-status-by-name.json; /Users/pedronauck/dev/qa-labs/compozy-worktree-lifecycle-fixes-20260815-004729-655016-lab/qa-artifacts/qa/logs/cli-retest-remove-by-name.json; /Users/pedronauck/dev/qa-labs/compozy-worktree-lifecycle-fixes-20260815-004729-655016-lab/qa-artifacts/qa/logs/cli-retest-dismiss-by-name.json; /Users/pedronauck/dev/qa-labs/compozy-worktree-lifecycle-fixes-20260815-004729-655016-lab/qa-artifacts/qa/logs/cli-retest-recreate-same-name.json; /Users/pedronauck/dev/qa-labs/compozy-worktree-lifecycle-fixes-20260815-004729-655016-lab/qa-artifacts/qa/logs/cli-retest-old-id-tombstone.json
last_report: docs/qa/reports/2026-08-14-worktree-lifecycle-fixes.md
overlaps: RT-worktree-api-surface-parity
---

QA impact: Task 02 adds the public worktree CLI lifecycle. The Phase C walk must cover an empty
list, pending-to-ready creation, cached and refreshed status, the dirty two-step removal refusal,
creation cancellation, adoption retry, forced removal, tombstone dismissal, and structured not-found behavior.

QA impact: Task 05 adds assisted exit verbs. The Phase C walk must verify the suggested action,
blocked reasons, operation id, cancellation, and exactly one successful completion action.

2026-08-14 lifecycle fix flag: cross a name through inspect, status, create cancel, exit actions,
exit cancel, remove, and dismiss; then recreate the dismissed name and confirm the old row remains
readable by ID.

2026-08-14 targeted walk and governed replay: status, remove, and dismiss accepted the name and
returned the canonical row identity. The catalog name was reused with the intentionally retained
Git branch, producing a distinct ready row while the dismissed predecessor remained readable by ID.
