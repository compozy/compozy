---
id: RT-worktree-web-removal-two-step
area: RT
title: Remove a worktree through the two-step safety flow
persona: Ada
journey: J-worktree-management
expected: A clean worktree removes after one confirm that quotes the record label and states the branch is not deleted. A dirty or unpushed worktree refuses with quantified risk rows (changed files, ± lines, unpushed commits) and offers the assisted exit as the primary; force is a ghost-danger doorway after Cancel that leads to a second confirm re-stating the quantities. A bound idle session is reported as stopping; a session mid-turn blocks removal outright. A branch that also exists on a remote downgrades to an informational note and single-step removal.
entry_points: Workspaces overview → worktree nest → Remove…
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-worktree-cli-lifecycle
---

QA impact: Task 06 adds the `WorktreeRemoveDialog` set. The Phase C walk must confirm the refusal
never becomes a confirm on its own, that the force step names the same numbers the refusal did, and
that removal leaves the branch and the run history intact.
