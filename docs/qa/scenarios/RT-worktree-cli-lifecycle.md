---
id: RT-worktree-cli-lifecycle
area: RT
title: Manage a worktree lifecycle through structured CLI output
persona: Ada
journey: J-worktree-management
expected: Creating, listing, inspecting, refreshing status, executing or cancelling an assisted exit, refusing an unsafe removal, forcing removal, and dismissing a worktree through `compozy worktree` returns truthful structured output, deterministic exit codes, and never falls back to the workspace root.
entry_points: compozy worktree create|list|inspect|status|exit|commit|push|pr|exit-cancel|remove|dismiss -o json
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-worktree-api-surface-parity
---

QA impact: Task 02 adds the public worktree CLI lifecycle. The Phase C walk must cover an empty
list, pending-to-ready creation, cached and refreshed status, the dirty two-step removal refusal,
forced removal, tombstone dismissal, and structured not-found behavior.

QA impact: Task 05 adds assisted exit verbs. The Phase C walk must verify the suggested action,
blocked reasons, operation id, cancellation, and exactly one successful completion action.
