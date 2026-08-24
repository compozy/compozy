---
id: NB-notification-preset-profile-enablement
area: NB
title: Route notification presets by profile enablement
persona: Dora
journey: J-adopt-extension-profiles
expected: Preset definitions remain shared while each profile defaults to enabled and stores only disabled exceptions; Settings follows the active profile, CLI and HTTP/UDS mutate the same row, and delivery skips a preset only in the disabled profile.
entry_points: web Settings → Hooks, Notification presets panel; compozy --profile <name> notification-preset list|enable|disable; GET /api/notifications/presets?profile=<name>; PUT /api/notifications/presets/{name}/enablement over HTTP and UDS
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: NB-044; NB-045; ET-extension-profile-enablement
---

Flagged by profiles Task 09. The final profiles QA cycle owns the first walk.

2026-08-23 reconciliation (Profiles task 12): the entry points were corrected against runtime truth
before planning the walk. The CLI group is `compozy notification-preset` — `internal/cli/notifications.go:13`
registers `notificationsKey = "notification-preset"` and `:203-243` adds the profile-resolving
`enable|disable` verbs; there is no `notifications preset` path. The web rows render in the Hooks
settings page through `NotificationPresetsPanel` with its `profile` prop
(`web/src/routes/_app/settings/-hooks-settings-page.tsx:62-72`); there is no
`/settings/notifications` route. Status is unchanged — the behavior is new, not re-verified.

Walk:

1. In a non-default profile, list presets and confirm every one reads enabled with no exception row
   stored anywhere yet.
2. Disable one preset with no `--profile`, confirm it acts on the active profile, and confirm the
   other profiles still read it as enabled.
3. Enable and disable the same preset from CLI, HTTP, and UDS in turn and confirm all three mutate
   the same row and return the same effective state.
4. Open the Hooks settings page in each profile and confirm the rows follow the active profile.
5. Trigger a delivery that the preset would produce and confirm it is skipped only in the profile
   where it is disabled.
6. Re-enable it and confirm the exception row disappears rather than being kept as an explicit
   enabled row.

Expected evidence: per-profile list output before and after each mutation, the CLI/HTTP/UDS
responses side by side, screenshots of the panel in two profiles, the delivery outcome in both
profiles, and the stored row count after re-enabling.
