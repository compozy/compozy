---
id: ET-layout-editor-load-saved-layout
area: ET
title: Loading a saved layout over unapplied edits asks first
persona: Bruno
journey: J-administer-window-manager
expected: Loading a saved layout while the editor is clean replaces the draft straight away; loading one while the draft has unapplied edits asks before discarding them and leaves the draft untouched on cancel; deleting a saved layout asks first and, once confirmed, clears every field of the removed record rather than leaving its scope, screen shape and overflow behind; a card's thumbnail is the stored layout's own geometry.
entry_points: Settings › Layouts › Saved layouts
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: MS-configure-window-manager
---

story: As an operator, loading or deleting a saved layout never silently destroys work I have not applied yet.

qa-impact: 2026-07-24 fixes a defect: selecting a profile replaced the whole draft with no confirmation and no dirty check, and a delete left three of the removed record's fields seeded into the next one. Flag only; the next QA cycle owns live testing.

QA impact 2026-07-25 (deep-review remediation): saved-layout mutations now use the profile's scope
identity and current expected version, and cache updates target the exact workspace. Flag only; the
next QA cycle owns save, load, overwrite, and delete retesting across workspaces.
