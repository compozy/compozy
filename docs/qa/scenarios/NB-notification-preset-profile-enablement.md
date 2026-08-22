---
id: NB-notification-preset-profile-enablement
area: NB
title: Route notification presets by profile enablement
persona: Bruno
journey: J-23
expected: Preset definitions remain shared while each profile defaults to enabled and stores only disabled exceptions; Settings follows the active profile, CLI and HTTP/UDS mutate the same row, and delivery skips a preset only in the disabled profile.
entry_points: /settings/notifications; compozy --profile <name> notifications preset enable|disable; GET /api/notifications/presets?profile=<name>; PUT /api/notifications/presets/{name}/enablement
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: NB-044; NB-045
---

Flagged by profiles Task 09. The final profiles QA cycle owns the first walk.
