---
id: ET-layout-editor-shortcut-recorder
area: ET
title: Record, reset and de-conflict window-manager shortcuts
persona: Bruno
journey: J-administer-window-manager
expected: Clicking a chord records the next real key combination; a bare key or a modifier-only press is refused with a spoken reason; recording an action's shipped default clears the override instead of storing a copy; only overrides are persisted; two overrides on one chord block the save and say so, while an override landing on another action's shipped chord is stored and explained as shadowing; Escape cancels recording; reset returns one action or all of them to the shipped keymap.
entry_points: Settings › Layouts › Shortcuts
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: MS-configure-window-manager
---

story: As an operator, I can change a window shortcut and be told immediately if the daemon will not accept the result.

qa-impact: 2026-07-24 new behavior, replacing a raw JSON textarea. The two conflict classes are distinguished because `CanonicalShortcuts` rejects duplicate overrides but accepts an override that shadows a default. Flag only; the next QA cycle owns live testing.
