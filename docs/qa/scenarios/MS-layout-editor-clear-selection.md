---
id: MS-layout-editor-clear-selection
area: MS
title: Clear the layout canvas selection with pointer or keyboard
persona: Bruno
journey: J-administer-window-manager
expected: The empty canvas background is a real button named "Clear layout selection"; clicking it or activating it with Enter or Space clears the selected group, node, or floating window without intercepting controls rendered above it.
entry_points: Settings › Layouts; layout canvas background
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-ms-wave2-current-20260730-061842-796290-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: MS-configure-window-manager; ET-layout-editor-split-weights
---

story: As a keyboard operator, I can leave a layout selection without needing a pointer.

qa-impact: 2026-07-28 replaced a click handler on the canvas container with a semantic background button and added keyboard activation coverage. Flag only; the next QA cycle owns live retesting.
