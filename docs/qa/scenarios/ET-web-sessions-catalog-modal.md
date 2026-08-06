---
id: ET-web-sessions-catalog-modal
area: ET
title: Open the sessions catalog as a global modal from dock and palette
persona: Bruno
journey: J-operate-desktop-shell
expected: Dock Sessions and ⌘K Toggle sessions open one centered Dialog over the desk with scrim; filter and recent/all views list live catalog truth; selecting a session opens its window and closes the modal; Escape/scrim dismisses without changing windows; compact and floating share the same modal chrome.
entry_points: web dock Sessions; ⌘K Toggle sessions; os-sessions-modal
qa_status: pass
bug_ids: BUG-20260805-session-delete-dialog-disappears
fix_status: fixed
retest_status: pass
fix_commits: PR-309-coderabbit-remediation
evidence: /Users/pedronauck/dev/qa-labs/compozy-session-sidebar-parent-20260806-212647-734931-lab/qa-artifacts/qa/journey-log.jsonl
last_report: docs/qa/reports/2026-08-05-session-archive-coderabbit.md
overlaps: ET-web-desktop-shell-lifecycle; ET-web-command-palette-shortcuts
---

story: As a builder, I can jump between sessions from a global catalog modal without a persistent side rail.

qa-impact: 2026-07-22 — Sessions catalog hard-cut from floating rail/Sheet to shell-level Dialog overlay (parity with ⌘K). `railOpen` removed from persisted desktop doc. Flag only; the next QA cycle owns live retesting.

QA impact 2026-08-04: session-row actions and archived separation changed the shared catalog list.
The modal open/select/dismiss behavior remains the canary for row-menu event isolation.

QA completion 2026-08-05: the live modal opened over the desk, row-menu actions did not select or
close it, Escape dismissed the menu with focus return, and selecting the row opened the session.
Desktop and narrow captures preserved the centered modal chrome and Archived disclosure.

QA impact 2026-08-05: the modal's shared delete confirmation now stays visible while deletion is
pending. Reset for focused confirmation that nested dismissal remains blocked without closing the catalog.

QA finding 2026-08-05: confirming a delayed deletion closed both nested dialogs before the request
settled. See `BUG-20260805-session-delete-dialog-disappears`.

QA completion 2026-08-05: opening and confirming deletion retained the global catalog under the
confirmation. Escape was rejected while pending; successful deletion returned to the still-open,
updated catalog.
2026-08-06 session-sidebar impact flag: the modal body is now the shared SessionList component and renders provenance threads (children nested under their parent with a count toggle); the Recent back affordance moved into the All pane. Reset to untested for the next QA cycle.

2026-08-06 re-walked live after the SessionList extraction: palette 'Toggle sessions' opens the modal, provenance thread (chip=5) renders in both panes, row click opens/focuses the session window. Evidence: lab journey-log.jsonl. Verdict: pass.
