---
id: ET-web-sessions-catalog-modal
area: ET
title: Open the sessions catalog as a global modal from dock and palette
persona: Bruno
journey:
expected: Dock Sessions and ⌘K Toggle sessions open one centered Dialog over the desk with scrim; filter and recent/all views list live catalog truth; selecting a session opens its window and closes the modal; Escape/scrim dismisses without changing windows; compact and floating share the same modal chrome.
entry_points: web dock Sessions; ⌘K Toggle sessions; os-sessions-modal
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-desktop-shell-lifecycle; ET-web-command-palette-shortcuts
---

story: As a builder, I can jump between sessions from a global catalog modal without a persistent side rail.

qa-impact: 2026-07-22 — Sessions catalog hard-cut from floating rail/Sheet to shell-level Dialog overlay (parity with ⌘K). `railOpen` removed from persisted desktop doc. Flag only; the next QA cycle owns live retesting.
