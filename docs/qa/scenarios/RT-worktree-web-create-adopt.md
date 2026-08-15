---
id: RT-worktree-web-create-adopt
area: RT
title: Create and adopt worktrees through the desktop shell
persona: Ada
journey: J-worktree-management
expected: Creation is name-first with the generated name in the placeholder only, a live `branch → path` preview, and field-level name-collision, branch-held, and base-ref refusals; after acceptance, Cancel stays live until the exact row reaches ready, then the Web selects it and closes the dialog. A failed setup still completes as ready, while cancellation or materialization failure stays distinct. Selecting a discovered row confirms adoption, validates the linked checkout, refuses the main checkout without touching it, and does not re-run bootstrap.
entry_points: S4 Workspace menu → New worktree; S1|S3 nest → discovered row; S2 overview menu → discovered row Adopt / New worktree footer row; S5 Worktree row/status
qa_status: pass
bug_ids: BUG-20260813-pending-worktree-marked-missing; BUG-20260813-base-ref-accepted-before-validation
fix_status: fixed
retest_status: pass
fix_commits: b6eb94d0; 0d54b6fe
evidence: /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/browser-worktree-create-missing.json; /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/browser-worktree-stream-proof.json; /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/browser-worktree-stream-proof.png; /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/browser-worktree-cancel-complete.json; /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/browser-worktree-adopt.json; /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/base-ref-refusal-fixed.json; /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/browser-base-ref-refusal-fixed.png; /Users/pedronauck/dev/qa-labs/compozy-worktree-scroll-area-20260814-184457-553088-lab/qa-artifacts/qa/web-worktree-scroll-after.png; /Users/pedronauck/dev/qa-labs/compozy-worktree-scroll-area-20260814-184457-553088-lab/qa-artifacts/qa/cli-adopt-worktrees.jsonl; /Users/pedronauck/dev/qa-labs/compozy-worktree-lifecycle-fixes-20260815-004729-655016-lab/qa-artifacts/qa/screenshots/web-create-selected.png; /Users/pedronauck/dev/qa-labs/compozy-pr-410-review-evidence-20260815-055849-475301-lab/qa-artifacts/qa/screenshots/new-worktree-dialog.png; /Users/pedronauck/dev/qa-labs/compozy-pr-410-review-evidence-20260815-055849-475301-lab/qa-artifacts/qa/screenshots/workspaces-current-worktree.png; /Users/pedronauck/dev/qa-labs/compozy-pr-410-review-evidence-20260815-055849-475301-lab/qa-artifacts/qa/screenshots/adopt-worktree-confirmation.png; /Users/pedronauck/dev/qa-labs/compozy-pr-410-review-evidence-20260815-055849-475301-lab/qa-artifacts/qa/screenshots/create-terminal-failure-story.png
last_report: docs/qa/reports/2026-08-15-pr-410-review-evidence.md
overlaps: RT-worktree-web-nested-navigation
---

QA impact: Task 06 adds `WorktreeCreateDialog` (wired to create + create-cancel) and
`WorktreeAdoptDialog`. The Phase C walk must confirm the preview omits the path when the placement
root cannot be derived rather than guessing it, that a refusal clears when its field is edited, and
that adoption leaves an adopted external at its original foreign path.

2026-08-15 S2 redesign (reset to untested): the Workspaces overview is now the Command-Tab
switcher — its worktree nest is an always-visible vertical menu; discovered rows adopt via the
revealed Adopt affordance or ↵, and New worktree is the footer row after the hairline (lone dashed
button when the git workspace has zero worktrees). Dialogs and the S1/S3/S4 legs are unchanged.

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

2026-08-14 focused rewalk (large catalog): the public CLI adopted 18 real linked worktrees and
returned all 18 through the structured list. In the Web overview, New worktree stayed visible
outside the scrolling catalog, opened the existing creation dialog, and Cancel returned cleanly.
The prior end-to-end create/adopt evidence remains valid because this change only moved the shared
entry point into the bounded submenu.

2026-08-14 lifecycle fix flag: re-walk accepted creation through its authoritative `ready` row,
including automatic selection/dialog close, pending cancellation, asynchronous rollback, and the
`ready` plus failed-setup outcome.

2026-08-14 targeted walk: `feature-analytics` reached ready, the dialog closed, the new worktree
became the selected scope, and the selected identity survived a browser refresh.
