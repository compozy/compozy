---
id: ET-profile-web-settings-lifecycle-dialogs
area: ET
title: Manage profile lifecycle from Settings with plan-backed dialogs
persona: Ada
journey: J-operate-profiles
expected: Settings lists active profiles with identity and work counts, demotes the archived list and the selection map to disclosure, and every lifecycle dialog renders exactly what its plan endpoint returned — rename tiers, archive paused automations and blocked-by-running, delete enumeration or routing to archive, unarchive reactivation — with a stale plan refused and re-asked rather than executed.
entry_points: Settings → Profiles; create|rename|archive|unarchive|delete dialogs; GET /api/profiles/{name}/rename-plan|archive-plan|delete-plan; POST /api/profiles/{name}/rename|archive|unarchive; DELETE /api/profiles/{name}
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
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
