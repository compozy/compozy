---
id: ET-profile-web-settings-lifecycle-dialogs
area: ET
title: Manage profile lifecycle from Settings with plan-backed dialogs
persona: Ada
journey: J-operate-profiles
expected: Settings lists active profiles with identity and work counts, demotes the archived list and the selection map to disclosure, and every lifecycle dialog renders exactly what its plan endpoint returned — rename tiers, archive paused automations and blocked-by-running, delete enumeration or routing to archive, unarchive reactivation — with a stale plan refused and re-asked rather than executed.
entry_points: Settings → Profiles; create|rename|archive|unarchive|delete dialogs; GET /api/profiles/{name}/rename-plan|archive-plan|delete-plan; POST /api/profiles/{name}/rename|archive|unarchive; DELETE /api/profiles/{name}
qa_status: fail
bug_ids: BUG-20261003-profile-palette-cancel-reopens; BUG-20261003-profile-delete-orphans-automations; BUG-20261003-profile-dialog-survives-back; BUG-20261003-profile-archive-resource-automations; BUG-20261003-profile-rename-repositories-unchecked; BUG-20260906-settings-nav-stale-open-history; BUG-20261003-profile-archive-event-rejected; BUG-20261003-profile-delete-live-stream-owner; BUG-20261003-profile-dialog-validation-toast; BUG-20261003-profile-create-stale-name-error; BUG-20261003-profile-emoji-keyboard-unreachable
fix_status: pending
retest_status: pending
fix_commits: 4760da89f; fb4b8a40a; 8380b94f2; b4ab86b39; a15b2ea62; b4ed8ca18; 74744060b
evidence: docs/qa/evidence/2026-10-02-untested/profile-rename-fixed-sol-ended.json; .compozy/tasks/sessions-stability/memory/profile-navigation-ci.md; docs/qa/evidence/2026-10-02-untested/profile-emoji-fixed-settings-persisted.png; docs/qa/evidence/2026-10-02-untested/profile-emoji-fixed-sol-ended.json
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-profile-cli-lifecycle
---

Flagged by Profiles task 05. The final QA tasks own the real-user walk, evidence, and verdict.

Walk:

1. Open Settings → Profiles and confirm the default read: active list with identity and work counts,
   the separation line as the page's only prose, and both the archived list and the selection map
   closed.
2. Create a profile and provoke each name refusal (empty, already taken, reserved); confirm each is
   reported inline against the field rather than as a toast.
3. Edit an identity — swap an icon for an emoji, then type an invalid hex — and confirm the invalid
   value is reported inline while the previous color stays.
4. Rename a profile that has committed repo folders; confirm the machine tier is informational, repo
   offers are pre-checked, declining one reports the content as dormant afterwards, and renaming
   `default` is refused with the permanence sentence.
5. Archive a profile with a scheduled automation and confirm the paused list; start a session in
   another profile and confirm archiving it is blocked with the running session named as a warning.
6. Unarchive and confirm the reactivation list is reported and that each automation stays paused.
7. Confirm delete appears only on an archived, empty profile, enumerates what will be removed, and
   that a profile still holding work routes to archive instead.
8. Open a lifecycle dialog, change the profile from a terminal so the plan goes stale, then confirm
   the mutation is refused and the dialog re-reads the plan rather than executing the old one.
9. Confirm profile lifecycle actions invoked from the command palette open these same dialogs.

Expected evidence: screenshots of the page default read and each dialog state, plan request/response
pairs alongside the mutation that quoted the revision, the stale-plan refusal, and the resulting
profile list.


### 2026-09-06 targeted navigation re-walk

Selected branch: after archiving the active profile and reloading Profiles, open Settings and choose Profiles while the launcher's General navigation is still pending. The latest section click must win; the archived disclosure stays reachable. E2E-014 passed five unchanged daemon-served repetitions against the repaired Web build. The original CI trace preserves the slower response timing that exposed the race; coordinator regressions cover that ordering deterministically. This retest addresses the linked navigation defect and does not claim a new walk of every lifecycle action in this scenario.

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

QA 2026-10-03 rename preview: both repository offers are unexpectedly unchecked. The public
plan returns exactly those paths; Escape preserves the original profile and restores focus.
Tracked as BUG-20261003-profile-rename-repositories-unchecked. See profile-lifecycle-sol-* receipts.

QA 2026-10-03 rename repair: repository offers start checked and retain explicit declines across
name edits and a stale-plan reread. Escape resets transient choices and leaves no partial profile.
Sol's CLI color edit provokes HTTP409; the dialog asks for review before a second confirmation
quotes the new revision. Only the accepted folder moves; UDS, exact notes, Git and reload agree.
BUG-20261003-profile-rename-repositories-unchecked is verified. The project menu exposed a worktree
submenu, so the declined-content hint remains unverified. Full lifecycle charter remains Pending.

QA 2026-10-03 archive walk: the enabled resource job is absent from HTTP/UDS plans and remains
scheduler-registered after Web archive. Unarchive reports no paused automations and retains the
enabled job. The running-session blocker and cancellation behave correctly. Shared backend defect
BUG-20261003-profile-archive-resource-automations is open; fresh repair replay is pending. See
profile-archive-sol-* receipts and the dated report. No actual automation execution is claimed.

QA 2026-10-03 archive repair replay: the enabled job and trigger now appear in the plan and pause
on archive. Unarchive retains both pauses; per-item keyboard reactivation uses the actual owner.
CLI repeat preserves the exact audit list, and a real daemon restart keeps both definitions paused.
BUG-20261003-profile-archive-resource-automations is verified. Evidence: profile-archive-fixed-sol-*
and profile-archive-fixed-cli-* in the dated report. The complete lifecycle charter remains Pending.

QA 2026-10-03 deletion walk: a paused canonical job and trigger are omitted from work counts.
Settings labels their owner Empty and permits deletion, leaving both aggregate automation lists
broken by an absent owner. BUG-20261003-profile-delete-orphans-automations is open.
Browser Back also leaves Archive open above General for at least 12 seconds;
BUG-20261003-profile-dialog-survives-back is open. Invalid color feedback and Escape cancellation pass.
See profile-delete-navigation-sol-* and the dated report; full lifecycle verdict remains Pending.

QA 2026-10-03 deletion repair: canonical jobs/triggers now count as work, including paused
definitions. Web omits Delete for their archived owner, and direct CLI deletion refuses it.
An empty archived profile still deletes successfully with its exact revision; reload and UDS
confirm it is gone while both aggregate catalogs retain the other owner and its automations.
BUG-20261003-profile-delete-orphans-automations is verified; see profile-delete-fixed-sol-*
and the dated report. The full lifecycle charter remains Pending.

QA 2026-10-03 history repair: Archive and unarchive success close on Browser Back and remain
closed on Forward. Reopening Rename drops an abandoned draft. UDS and reload preserve the intended
active profile; BUG-20261003-profile-dialog-survives-back is verified. Palette Create and Archive
hand off to the canonical dialogs, but cancellation retains the flow in the window URL and reload
reopens Archive. Filed BUG-20261003-profile-palette-cancel-reopens. See profile-dialog-back-fixed-sol-*
and the dated report; full lifecycle and spoken screen-reader verdicts remain Pending.

QA 2026-10-03 palette replay repair: the Settings window consumes a valid lifecycle URL intent
when it opens the canonical dialog. Both a retained deep link and a fresh palette invocation stay
canceled after reload, with UDS retaining the active profile. The final real-daemon E2E-017/027
and React Doctor 100/100 pass. BUG-20261003-profile-palette-cancel-reopens is verified; evidence
is profile-palette-cancel-fixed-sol-* and profile-lifecycle-final-*. Full charter remains Pending.
