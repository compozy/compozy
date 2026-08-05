---
id: ET-web-sessions-catalog-modal
area: ET
title: Open the sessions catalog as a global modal from dock and palette
persona: Bruno
journey: J-operate-desktop-shell
expected: Dock Sessions and ⌘K Toggle sessions open one centered Dialog over the desk with scrim; filter and recent/all views list live catalog truth; selecting a session opens its window and closes the modal; Escape/scrim dismisses without changing windows; compact and floating share the same modal chrome.
entry_points: web dock Sessions; ⌘K Toggle sessions; os-sessions-modal
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-session-archive-20260805-031044-743468-lab/qa-artifacts/qa/journey-log.jsonl;/Users/pedronauck/dev/qa-labs/compozy-session-archive-20260805-031044-743468-lab/qa-artifacts/qa/screenshots/session-catalog-desktop.png;/Users/pedronauck/dev/qa-labs/compozy-session-archive-20260805-031044-743468-lab/qa-artifacts/qa/screenshots/session-catalog-narrow.png
last_report: docs/qa/reports/2026-08-04-session-archive.md
overlaps: ET-web-desktop-shell-lifecycle; ET-web-command-palette-shortcuts
---

story: As a builder, I can jump between sessions from a global catalog modal without a persistent side rail.

qa-impact: 2026-07-22 — Sessions catalog hard-cut from floating rail/Sheet to shell-level Dialog overlay (parity with ⌘K). `railOpen` removed from persisted desktop doc. Flag only; the next QA cycle owns live retesting.

QA impact 2026-08-04: session-row actions and archived separation changed the shared catalog list.
The modal open/select/dismiss behavior remains the canary for row-menu event isolation.

QA completion 2026-08-05: the live modal opened over the desk, row-menu actions did not select or
close it, Escape dismissed the menu with focus return, and selecting the row opened the session.
Desktop and narrow captures preserved the centered modal chrome and Archived disclosure.
