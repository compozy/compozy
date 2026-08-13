---
id: RT-worktree-web-create-adopt
area: RT
title: Create and adopt worktrees through the desktop shell
persona: Ada
journey: J-worktree-management
expected: Creation is name-first with the generated name in the placeholder only, a live `branch → path` preview, and three refusals that land on their own field — name collision, branch held elsewhere (offering "Select that worktree instead"), and base ref not found. After the request is accepted the row is pending and Cancel stays live; cancelling unwinds the creation daemon-side and removes the row. Selecting a discovered row opens the adoption confirm naming the validation and stating bootstrap is not re-run; a directory whose metadata resolves into the main checkout is refused and left untouched.
entry_points: S4 Workspace menu → New worktree; S1|S2|S3 nest → discovered row; S5 Worktree row/status
qa_status: untested
bug_ids: BUG-20260813-pending-worktree-marked-missing
fix_status: fixed
retest_status: passed
fix_commits: pending Task 10 QA remediation commit
evidence: /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/browser-worktree-create-missing.json;/Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/browser-worktree-stream-proof.json;/Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/browser-worktree-stream-proof.png
last_report: docs/qa/reports/2026-08-13-worktree-support.md
overlaps: RT-worktree-web-nested-navigation
---

QA impact: Task 06 adds `WorktreeCreateDialog` (wired to create + create-cancel) and
`WorktreeAdoptDialog`. The Phase C walk must confirm the preview omits the path when the placement
root cannot be derived rather than guessing it, that a refusal clears when its field is edited, and
that adoption leaves an adopted external at its original foreign path.

2026-08-13 fix replay: the first live create exposed a catalog race that changed the accepted
`pending` row to `missing` before checkout materialization. After restricting missing-path
reconciliation to stable `ready` rows, a clean browser replay reached `ready`, removed the pending
affordance through the catalog stream, and appeared in `git worktree list`. The remaining refusal,
cancel, and adoption cases continue in the Task 10 charter.
