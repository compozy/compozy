---
id: ET-profile-desktop-restoration
area: ET
title: Keep desktops and window layouts per profile
persona: Ada
journey: J-restore-per-profile-state
expected: Each profile keeps its own desktops and window arrangement per workspace; switching profiles restores the target profile's desks exactly as they were left and shows none of the previous profile's windows; a session of another profile opened under All profiles closes from the desk when the operator switches into the desk's profile, while a deep link opened under a selected profile keeps its owner banner; a brand-new profile enters on a single seeded default desktop; an archived profile's arrangement is retained and returns untouched on unarchive; deleting a profile counts its saved arrangements in the delete preview and removes every one of them, with other profiles' desks intact.
entry_points: dock-foot profile switcher; web topbar pager; GET /api/workspaces/{workspace_id}/window-manager?profile=; POST /api/workspaces/{workspace_id}/window-manager/preview?profile=; POST /api/workspaces/{workspace_id}/window-manager/commands?profile=; compozy desktop list; compozy profile delete
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report: docs/qa/reports/2026-10-02-untested.md
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
5. Turn on All profiles, open a session owned by the second profile, then select the remembered
   profile whose desk you are on. Confirm that session's window closes, the remembered profile's own
   windows stay, the second profile's desk still shows its own windows when you switch to it, and a
   link to the second profile's session opened while the remembered profile is selected shows the
   owner banner instead of closing.
6. Open a second browser context on the other profile and confirm neither client's windows appear in
   the other, and that neither is force-switched when the peer switches.
7. Archive the second profile, confirm the operator lands on `default`, then unarchive it and confirm
   its arrangement returns untouched.
8. Delete an empty profile that owns saved desks: confirm the delete preview counts them under saved
   desktops, that the count matches what the CLI and the DELETE response report, and that after the
   delete no arrangement of that profile survives while every other profile's desks are unchanged.

Review follow-up #1221: during step 5, leave the project while session-owner reads are pending,
then return and confirm the unfinished cleanup resumes. Repeat with a failed profile-selection
request: after rollback, no pending or retried close may run, and the restored profile's desks
must not be swept. The existing `use-desktop-chrome.test.tsx` hook suite covers both interleavings;
the real-browser replay of these additions remains for CI (no local E2E in this dispatch).

Impact: Web cleanup lifecycle only. Native tools, HTTP/UDS contracts, hooks, extensions, config,
persisted workspace/profile isolation, and `skills/compozy/` have no contract changes.

Expected evidence: screenshots of each profile's desks before and after a switch, the fresh-profile
clean state, the two-context pair, and the delete preview beside the delete result showing the same
saved-desktop count.

qa-impact: 2026-09-30 shell rail v2 (flat topbar, left dock rail, gutterless tiling, browser-tab deck, light/dark theme). The profile switcher moved from the menubar tray to the dock foot (above the theme toggle and Settings) and the pager to the topbar tray. Scenario already untested; entry points updated.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.

QA 2026-11-03 (#1184, step 5 only): in an isolated lab with profiles `xray` and `yankee`, a
`xray` session opened under All profiles was stored on `yankee`'s desk and stayed there with
"This session belongs to xray" after switching to `yankee`. After the fix the switch closes it
(the daemon snapshot for `yankee` no longer lists it), `yankee`'s own session window stays,
switching to `xray` restores both of its windows, and a deep link to a `xray` session while
`yankee` is selected still shows the owner banner. The remaining steps stay unwalked.
