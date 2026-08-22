---
id: ET-profile-web-aggregate-create-chip-toast
area: ET
title: State the destination and name the owner when creating under All profiles
persona: Ada
journey: J-operate-profiles
expected: While All profiles is on, every shared creation surface shows a fixed "→ default" destination chip in its default read with no picker attached, the created item is filed in default, the success toast names that owner, and the item then appears owner-labeled in the aggregate listing. Scoped views show neither chip nor toast.
entry_points: new-session composer; Tasks new task; Automation job and trigger forms; agent, bridge, knowledge and MCP install dialogs; command palette session.new
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

1. With a second profile created and All profiles on, open each shared creation surface and confirm
   the "→ default" chip is visible without hovering, and that it is text with no control attached.
2. Create a session from the composer and confirm the toast names default, then confirm the new row
   appears in the aggregate listing carrying the default owner tag.
3. Repeat the creation from the command palette and confirm the same chip and the same toast — no
   surface is exempt.
4. Switch to a real profile and confirm the chip and the owner toast are both absent, and that a
   created item lands in that profile rather than in default.
5. Confirm no creation surface offers a way to change the destination from within the surface.

Expected evidence: screenshots of the chip in each creation surface, the toast after commit, the
owner-labeled row in the aggregate listing, and the request/response pair showing the profile the
creation was filed under.
