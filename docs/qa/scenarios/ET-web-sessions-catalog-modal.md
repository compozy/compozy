---
id: ET-web-sessions-catalog-modal
area: ET
title: Open the sessions catalog as a global modal from menu and palette
persona: Bruno
journey: J-operate-desktop-shell
expected: The Session menu and ⌘K Toggle sessions open one centered Dialog over the desk with scrim; filter and recent/all views list live catalog truth; selecting a session opens its window and closes the modal; Escape/scrim dismisses without changing windows; compact and floating share the same modal chrome.
entry_points: Session menu Toggle sessions; ⌘K Toggle sessions; os-sessions-modal
qa_status: pass
bug_ids: BUG-20260805-session-delete-dialog-disappears
fix_status: fixed
retest_status: pass
fix_commits: PR-309-coderabbit-remediation
evidence: /Users/pedronauck/dev/qa-labs/compozy-pr-327-coderabbit-20260806-224025-123277-lab/qa-artifacts/qa/journey-log.jsonl; docs/qa/evidence/2026-08-06-pr-327-coderabbit/catalog-filter-ancestor-path.png; docs/qa/evidence/2026-08-06-pr-327-coderabbit/catalog-group-collapsed.png; docs/qa/evidence/2026-08-24-eng-136/session-menu-catalog.png
last_report: docs/qa/reports/2026-08-24-eng-136.md
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

2026-08-06 PR 327 CodeRabbit impact: the shared catalog now preserves cyclic sessions, restores complete ancestor paths while filtering, and removes collapsed thread/group bodies from keyboard and assistive-technology navigation. Reset for a targeted live re-walk.

2026-08-06 PR 327 completion: Dock Sessions rendered four live sessions, and filtering for the grandchild kept the root and intermediate parent visible. Collapsed thread and agent bodies became inert while their toggles stayed reachable; expansion restored the rows, the catalog dismissed cleanly, and reopening after reload returned current runtime truth. Verdict: pass.

QA impact 2026-08-24 ENG-136: the dock Sessions action is now contextual — it starts the new-session flow when no session window exists and focuses the most-recent session window otherwise. The catalog remains owned by the Session menu and palette, so this scenario is reset for a focused re-walk.

QA completion 2026-08-24: the Session menu opened the shell catalog and dismissed cleanly in the dedicated ENG-136 walk; the compact E2E-020 path opened the same catalog through the command palette. Evidence: `docs/qa/evidence/2026-08-24-eng-136/session-menu-catalog.png`. Verdict: pass.
