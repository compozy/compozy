---
id: RT-worktree-web-removal-two-step
area: RT
title: Remove a worktree through the two-step safety flow
persona: Ada
journey: J-worktree-management
expected: A clean worktree removes after one confirm that quotes the record label and states the branch is not deleted. A dirty or unpushed worktree refuses with quantified risk rows (changed files, ± lines, unpushed commits) and offers the assisted exit as the primary; force is a ghost-danger doorway after Cancel that leads to a second confirm re-stating the quantities. A bound idle session is reported as stopping; a session mid-turn blocks removal outright. A branch that also exists on a remote downgrades to an informational note and single-step removal.
entry_points: S15 Workspaces overview → worktree nest → Remove
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/cli-worktree-dirty-exit-remove.jsonl; web/e2e/__tests__/worktrees.spec.ts
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: RT-worktree-cli-lifecycle
---

QA impact: Task 06 adds the `WorktreeRemoveDialog` set. The Phase C walk must confirm the refusal
never becomes a confirm on its own, that the force step names the same numbers the refusal did, and
that removal leaves the branch and the run history intact.

2026-08-14 layout: the Path fact truncates with a tooltip when the absolute checkout is longer
than the confirm dialog. Removal stages and copy are unchanged.


2026-09-16 bulk-removal impact: both menubar and overview expose bounded selection and individual
missing-record resolution. Verify mixed and missing-only selections, Escape/Shift/select-all,
active/foreign/archived/discovered exclusions, immutable targets during live updates, per-item
partial results and failed-only retry. Metadata dismissal preserves replacements and history;
repeated dismissal and refresh/restart stay clean. No force or session stop is implicit.
Current run: `docs/qa/reports/2026-09-16-worktree-bulk-delete.md` (targeted rendered and API evidence recorded; final CI pending).

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.

2026-10-19 #813 impact: row cleanup now follows the row owner instead of the active profile, so a
missing row owned by another profile keeps **Clean up missing record…** enabled (it acts with that
owner's selector). A refused cleanup (archived or unknown owner, active session) stays visible as a
disabled item naming the reason. Missing rows join ready rows in selection mode. Single remove, the
missing-record dialog and the bulk confirmation offer an unchecked **Also delete local branch(es)**
opt-in; the daemon keeps the root checkout's HEAD branch, the default branch and unmerged branches
(reported per row; single cleanup offers **Delete anyway** for unmerged). Targeted isolated-lab walk
on 2026-10-19 (headless Chromium against a disposable daemon and throwaway repository): cross-owner
missing dismissal with branch deletion, archived-owner disabled reason, single removal with an
unmerged refusal then forced retry, and a mixed ready+missing bulk batch with per-row branch outcomes
all passed; Git state was verified after each step and the lab teardown was clean. Evidence is in
the PR description.

2026-10-19 merge-train review follow-up: when the post-cleanup branch read fails and the
response has `reason: delete_failed` without a branch name, single and bulk cleanup must report
that branch deletion failed and its result is unconfirmed. It must never report that the worktree
had no branch. The lifecycle notification regression passes in the existing `use-worktrees` suite;
full integration validation remains owned by current-head PR CI.
