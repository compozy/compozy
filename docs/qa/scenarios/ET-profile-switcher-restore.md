---
id: ET-profile-switcher-restore
area: ET
title: Create, switch, and restore profiles from the dock-foot switcher
persona: Ada
journey: J-operate-profiles
expected: The switcher is a neutral icon button while only default exists, becomes an identity element once a second profile is created, switches through the canonical selection route, answers the boundary question in one sentence, offers the All-profiles state, and restores each project's remembered profile on return without ever force-switching an already-open client.
entry_points: dock-foot profile switcher; Create profile… dialog; command palette Profiles view; profile.use; GET|PUT /api/profiles/selection; GET /api/logs/stream?component=profile
qa_status: fail
bug_ids: BUG-20261003-profile-project-restoration; BUG-20261003-profile-archive-event-rejected; BUG-20261003-profile-delete-live-stream-owner; BUG-20261003-profile-dialog-validation-toast; BUG-20261003-profile-create-stale-name-error; BUG-20261003-profile-emoji-keyboard-unreachable
fix_status: pending
retest_status: pending
fix_commits: 4760da89f; fb4b8a40a; 8380b94f2; b4ab86b39
evidence: docs/qa/evidence/2026-10-02-untested/profile-project-entry-final-sol-ended.json; docs/qa/evidence/2026-10-02-untested/profile-selection-cli-updates-map.png; docs/qa/evidence/2026-10-02-untested/profile-global-independent-memory.png; /Users/pedronauck/dev/qa-labs/compozy-profiles-final-20260826-081429-551001-lab/qa-artifacts/qa/quiet-profile-trigger.png; /Users/pedronauck/dev/qa-labs/compozy-profiles-final-20260826-081429-551001-lab/qa-artifacts/qa/global-profile-restored.png; /Users/pedronauck/dev/qa-labs/compozy-profiles-final-20260826-081429-551001-lab/qa-artifacts/qa/workspace-profile-restored.png; /Users/pedronauck/dev/qa-labs/compozy-profiles-final-20260826-081429-551001-lab/qa-artifacts/qa/all-profiles-layered-mark.png; docs/qa/evidence/2026-10-02-untested/profile-emoji-fixed-settings-persisted.png; docs/qa/evidence/2026-10-02-untested/profile-emoji-fixed-sol-ended.json
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-profile-selection-precedence; ET-profile-palette-view; MS-web-menubar-global-scope-toggle
---

Flagged by Profiles task 05. The final QA tasks own the real-user walk, evidence, and verdict.

Walk:

1. On a fresh home, confirm the switcher renders as a quiet icon button with no name or color, and
   that nothing else in the product mentions profiles.
2. Create a second profile from the switcher, choosing an icon and a color in the picker; confirm the
   switcher becomes an identity element showing the exact icon and color that were chosen (never a
   generic placeholder glyph), both on the trigger and on the menu rows, and the new profile is
   active.
3. Switch profiles and confirm listings refilter, the boundary sentence is present verbatim, and the
   project picker lists the same projects in every profile.
4. Leave the project, return, and confirm the remembered profile is restored; repeat for the Global
   lens and confirm it keeps its own slot.
5. With the browser open on one profile, run `compozy profile use <other>` in a terminal. Confirm the
   remembered choice updates in Settings while the open client keeps showing the profile the operator
   was already looking at.
6. Turn on All profiles, confirm the neutral layered mark replaces the identity, then leave and
   return to a project and confirm it lands on a real profile rather than the aggregate.

Expected evidence: screenshots of the quiet and plural states, the switcher menu, selection-route
request/response pairs, terminal transcript for the cross-surface switch, and the restored state after
re-entering each project.

QA 2026-08-26: Passed in an isolated lab. Global and workspace selections restored independently,
the open browser resisted an external CLI switch, and All profiles returned to a real profile after
re-entry.

qa-impact: 2026-09-30 shell rail v2. The profile switcher moved from the menubar tray to the dock foot (above the theme toggle and Settings). Reset to re-walk switching, creation and restoration from its new home.

QA 2026-10-03: a CLI-driven archive lost its lifecycle audit and left the open profile stale.
The archive audit repair now delivers the named event and sweeps the live browser to default.
A separate delete replay persists its event but loses the recovery stream while desktop authority
reconnects, leaving the removed profile visible. Full Settings/switcher walks remain Pending.

QA 2026-10-03 recovery replay: the global profile stream now survives desktop reconnection.
A CLI deletion reaches the open browser, which returns to default without reload. UDS confirms
the event and deleted identity; the original catalog is retained and recovery survives refresh.
The archive repair is commit 4760da89f. Both recovery defects are verified, while the complete
Settings/switcher charter remains Pending. See profile-delete-fixed-* and the dated report.

QA 2026-10-03 keyboard walk: navigation and cancellation preserve focus and leave no partial
profile. A reserved-name refusal also emits a technical toast beside the inline error; tracked as
BUG-20261003-profile-dialog-validation-toast. Full lifecycle and spoken screen-reader legs remain Pending.

QA 2026-10-03 feedback replay: reserved and duplicate-name refusals now stay inline without a
raw error toast. Blank-name and server refusals track the current input, and corrected creation
is independently read and survives reload. Both feedback bugs are verified. Emoji Enter selection
works and persists, but arrow navigation stays on the first result;
BUG-20261003-profile-emoji-keyboard-unreachable remains open. Full lifecycle and spoken
screen-reader legs remain Pending. See the dated report
and profile-create-name-error-sol-* receipts.

QA 2026-10-03 emoji repair: both identity dialogs now preserve the picker's arrow navigation.
Sol chooses Open book, cancels Create without leaving a profile, and saves the emoji on reading-room.
UDS and reload retain the identity and color. Escape restores focus in both dialogs.
BUG-20261003-profile-emoji-keyboard-unreachable is verified; the complete lifecycle/restore charter
and spoken screen-reader output remain Pending. See profile-emoji-fixed-sol-* evidence.

QA 2026-10-03 project restoration: Sol switches Studio Operations → Editorial by keyboard.
The browser carries navigation-notes although Editorial resolves to default through the CLI.
Reload restores default. BUG-20261003-profile-project-restoration is open; the 59-frame
profile-switcher-restore-sol recording is stopped before investigation. The complete charter
and spoken screen-reader output remain Pending.

QA 2026-10-03 final selection replay: BUG-20261003-profile-project-restoration is verified.
Sol confirms project entry, aggregate exit, external CLI updates in the Settings map without
forcing the open client, independent Global/project selections on reload, and Back after a
switch. The 235-frame profile-project-entry-final-sol recording is stopped before review.
E2E-013/015/020 pass on the final production build. All keyboard and public-interface legs
of this scenario are now covered across the linked replays; spoken VoiceOver output remains
unverified because the native connector failed to launch it twice (-10005). After recording
the repair commit, this row can settle as blocked-verify for that remaining human leg.

Human verification remaining: with VoiceOver running, traverse the quiet/plural trigger,
profile options and All profiles using Tab/arrow keys, confirm active name/symbol and
selection announcements without relying on color, then switch projects and confirm the
restored identity is announced. The existing keyboard evidence does not claim spoken output.
