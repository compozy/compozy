---
id: ET-profile-aggregate-chip-toast
area: ET
title: State the destination and name the owner when creating under All profiles
persona: Ada
journey: J-scope-work-by-profile
expected: While All profiles is on, every shared creation surface shows a fixed "→ default" destination chip in its default read with no picker attached, the created item is filed in default, the success toast names that owner, and the item then appears owner-labeled in the aggregate listing. Scoped views show neither chip nor toast.
entry_points: new-session composer; Tasks new task; Automation job and trigger forms; worktree creation; Loop run; bridge creation; command palette session.new
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-profile-web-aggregate-owner-surfaces; ET-profile-aggregate-owner-labels
---

Flagged by Profiles task 07. The final QA tasks own the real-user walk, evidence, and verdict.

Walk:

1. With a second profile created and All profiles on, open each selector-bearing surface — session,
   task, automation job, automation trigger, worktree, Loop run, and bridge creation — and confirm the
   "→ default" chip is visible without hovering and is text with no control attached. Agent definitions,
   knowledge, MCP install, and task bridge subscription remain chip-free because they do not declare
   a profile selector.
2. Create one item on each selector-bearing surface and confirm every request files it in `default`,
   each success toast names `default`, and each aggregate row carries the default owner tag.
3. Repeat one supported session path from the command palette and confirm the same chip, request,
   toast, and owner row.
4. Switch to a real profile and confirm the chip and owner toast are absent, and that a created item
   lands in that profile rather than in `default`.
5. Confirm no creation surface offers a way to change the destination from within the surface.

Expected evidence: screenshots of the chip in each selector-bearing creation surface, the chip-free
exceptions, per-surface request/response and toast captures, the owner-labeled aggregate rows, and
the command-palette session request showing the selected profile.
