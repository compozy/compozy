---
id: RT-worktree-web-missing-resolution
area: RT
title: Resolve a worktree whose directory disappeared out of band
persona: Ada
journey: J-worktree-management
expected: A worktree removed outside Compozy is reported as missing with its history preserved and never cascades into session or task deletion. The resolution dialog states history preservation before either choice and offers two legs — Dismiss record, which drops the entry only and renders the idempotent no-op outcome verbatim; and It's back, which re-verifies the recorded path and restores that same record to ready when the identical repository is found there. A different repository at that path stays refused and the record stays missing.
entry_points: Workspaces overview → worktree nest → Resolve…
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-worktree-web-removal-two-step
---

QA impact: Task 06 adds `WorktreeMissingResolutionDialog`. Restore runs through the idempotent adopt
operation, so the Phase C walk must confirm the restored record keeps its original id rather than
minting a second one, and that dismissal leaves bound sessions and task runs readable.
