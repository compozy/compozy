---
id: RT-worktree-web-create-adopt
area: RT
title: Create and adopt worktrees through the desktop shell
persona: Ada
journey: J-worktree-management
expected: Creation is name-first with the generated name in the placeholder only, a live `branch → path` preview, and three refusals that land on their own field — name collision, branch held elsewhere (offering "Select that worktree instead"), and base ref not found. After the request is accepted the row is pending and Cancel stays live; cancelling unwinds the creation daemon-side and removes the row. Selecting a discovered row opens the adoption confirm naming the validation and stating bootstrap is not re-run; a directory whose metadata resolves into the main checkout is refused and left untouched.
entry_points: Workspace menu → New worktree; nest → discovered row
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-worktree-web-nested-navigation
---

QA impact: Task 06 adds `WorktreeCreateDialog` (wired to create + create-cancel) and
`WorktreeAdoptDialog`. The Phase C walk must confirm the preview omits the path when the placement
root cannot be derived rather than guessing it, that a refusal clears when its field is edited, and
that adoption leaves an adopted external at its original foreign path.
