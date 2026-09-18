---
id: RT-worktree-web-missing-resolution
area: RT
title: Resolve a worktree whose directory disappeared out of band
persona: Ada
journey: J-worktree-management
expected: A worktree removed outside Compozy is reported as missing with its history preserved and never cascades into session or task deletion. The resolution dialog states history preservation before either choice and offers two legs — Dismiss record, which drops the entry only and renders the idempotent no-op outcome verbatim; and It's back, which re-verifies the recorded path and restores that same record to ready when the identical repository is found there. A different repository at that path stays refused and the record stays missing.
entry_points: S1 Workspace menu or workspace overview → worktree actions → Clean up missing record
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/browser-worktree-create-missing.json; web/e2e/__tests__/worktrees.spec.ts
last_report: docs/qa/reports/2026-08-13-worktree-support.md
overlaps: RT-worktree-web-removal-two-step
---

QA impact: Task 06 adds `WorktreeMissingResolutionDialog`. Restore runs through the idempotent adopt
operation, so the Phase C walk must confirm the restored record keeps its original id rather than
minting a second one, and that dismissal leaves bound sessions and task runs readable.

2026-08-14 layout: Expected path truncates with a tooltip. Resolution legs and copy are unchanged.

The menubar nest and Workspaces overview both expose **Clean up missing record…**, opening the
same dismissal/restore dialog. Both also offer selection mode for missing-only and mixed batches.


2026-09-16 bulk-removal impact: both menubar and overview expose bounded selection and individual
missing-record resolution. Verify mixed and missing-only selections, Escape/Shift/select-all,
active/foreign/archived/discovered exclusions, immutable targets during live updates, per-item
partial results and failed-only retry. Metadata dismissal preserves replacements and history;
repeated dismissal and refresh/restart stay clean. No force or session stop is implicit.
Current run: `docs/qa/reports/2026-09-16-worktree-bulk-delete.md` (targeted rendered and API evidence recorded; final CI pending).
