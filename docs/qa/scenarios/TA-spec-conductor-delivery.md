---
id: TA-spec-conductor-delivery
area: TA
title: Deliver a two-slice spec through spec_conductor
persona: Bruno
journey: J-deliver-through-agents
expected: A spec_conductor session delivers a two-slice spec from an untracked spec directory through one integration worktree, recovers from restarts without repeating a slice, refuses to replace an active conductor, and runs reviewers read-only.
entry_points: compozy session new --agent spec_conductor; compozy session prompt; compozy session status; compozy__worktree_list; git log --grep Spec-Slice
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-upgrade-spec-cycle-hard-cut
---

Spec QA-001, covering US-001, US-005.AC-2, US-005.EC-1 and child reporting.

Use a fixture spec with `_slices.md` and a gitignored spec directory. Confirm children read it by
absolute path from their own worktrees, and each dispatch states provider, model and reasoning effort.

Force these cases and record each result:

1. Five send-backs on one slice, each starting from a monitor wake, with no hop limit failure.
2. A restart right after a squash and before the `_run.md` checkpoint. Git history wins, and the slice
   is not integrated twice.
3. A restart after worktree cleanup.
4. A replacement conductor while the previous one is active. It reports the owner and stops.
5. A spec without `_slices.md`. Nothing is dispatched until the operator accepts proposed slices.
6. A `reviewer` asked to edit source. It refuses and writes only its findings file.

Record the send-back count, since early worker stops are a known risk of delivering without `/goal`.
