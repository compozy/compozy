---
id: RT-worktree-web-create-adopt
area: RT
title: Create and adopt worktrees through the desktop shell
persona: Ada
journey: J-worktree-management
expected: Creation is name-first with the generated name in the placeholder only, a live `branch → path` preview, and three refusals that land on their own field — name collision, branch held elsewhere (offering "Select that worktree instead"), and base ref not found. After the request is accepted the row is pending and Cancel stays live; cancelling unwinds the creation daemon-side and removes the row. Selecting a discovered row opens the adoption confirm naming the validation and stating bootstrap is not re-run; a directory whose metadata resolves into the main checkout is refused and left untouched.
entry_points: S4 Workspace menu → New worktree; S1|S2|S3 nest → discovered row; S5 Worktree row/status
qa_status: untested
bug_ids: BUG-20260813-pending-worktree-marked-missing; BUG-20260813-base-ref-accepted-before-validation
fix_status: fixed
retest_status: pass
fix_commits: b6eb94d0; 0d54b6fe
evidence: /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/browser-worktree-create-missing.json; /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/browser-worktree-stream-proof.json; /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/browser-worktree-stream-proof.png; /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/browser-worktree-cancel-complete.json; /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/browser-worktree-adopt.json; /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/base-ref-refusal-fixed.json; /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/browser-base-ref-refusal-fixed.png
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
affordance through the catalog stream, and appeared in `git worktree list`. A configured slow setup
proved cancellation removes the pending checkout and branch. A real externally-created linked
worktree was discovered and adopted in place without bootstrap. Missing-base submission initially
exposed acceptance before validation; the fixed replay returned `base_ref_not_found` to the Base
ref field and created no row. Branch-holder and main-checkout adoption refusals remain in this
charter.

2026-08-14 behavior change: discovered rows are now labeled with the branch-derived name adoption
would mint (detached checkouts carry `detached-<short-sha>`) instead of the directory basename
(BR-4) — an external checkout whose directory happens to be named like the workspace no longer
reads as the workspace itself. Covered by the discovery merge suite (discovery_test.go); labels
flow to web, CLI, and native-tool listings from the same daemon field.

2026-08-14 behavior change (S2 side submenu): reaching New worktree and discovered rows from the
menubar now goes through the workspace row's hover side submenu instead of a click-expanded inline
nest. Selecting a discovered row still opens the adoption confirm, and the menu closes when the
dialog opens. E2E helpers updated: openWorkspaceNest hovers the workspace row and waits for the
submenu container before touching nest entries.

2026-08-14 layout: New worktree is compact (`sm` 560). Checkout paths truncate with a tooltip.
Adopt refusal uses the unframed ruled confirm chrome. Verbs and refusals are unchanged.
