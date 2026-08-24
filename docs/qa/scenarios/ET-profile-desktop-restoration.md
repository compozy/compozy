---
id: ET-profile-desktop-restoration
area: ET
title: Keep desktops and window layouts per profile
persona: Ada
journey: J-restore-per-profile-state
expected: Each profile keeps its own desktops and window arrangement per workspace; switching profiles restores the target profile's desks exactly as they were left and shows none of the previous profile's windows; a brand-new profile enters on a single seeded default desktop; an archived profile's arrangement is retained and returns untouched on unarchive; deleting a profile counts its saved arrangements in the delete preview and removes every one of them, with other profiles' desks intact.
entry_points: menubar profile switcher; web desktop pager; GET /api/workspaces/{workspace_id}/window-manager?profile=; POST /api/workspaces/{workspace_id}/window-manager/preview?profile=; POST /api/workspaces/{workspace_id}/window-manager/commands?profile=; compozy desktop list; compozy profile delete
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-window-manager-multi-client; ET-window-manager-layout-gestures; ET-profile-switcher-restore
---

Flagged by Profiles task 10. The final QA tasks own the real-user walk, evidence, and verdict.

Walk:

1. In one workspace under `default`, create a second desktop and arrange a couple of windows on it.
2. Switch to a second profile and confirm it enters on a single default desktop with none of
   `default`'s windows on screen.
3. Arrange a different desktop and window layout under the second profile.
4. Switch back and forth between the two profiles and confirm each arrangement returns exactly as it
   was left, in both directions, including which desktop is active.
5. Open a second browser context on the other profile and confirm neither client's windows appear in
   the other, and that neither is force-switched when the peer switches.
6. Archive the second profile, confirm the operator lands on `default`, then unarchive it and confirm
   its arrangement returns untouched.
7. Delete an empty profile that owns saved desks: confirm the delete preview counts them under saved
   desktops, that the count matches what the CLI and the DELETE response report, and that after the
   delete no arrangement of that profile survives while every other profile's desks are unchanged.

Expected evidence: screenshots of each profile's desks before and after a switch, the fresh-profile
clean state, the two-context pair, and the delete preview beside the delete result showing the same
saved-desktop count.
