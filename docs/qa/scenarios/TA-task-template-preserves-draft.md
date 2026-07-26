---
id: TA-task-template-preserves-draft
area: TA
title: Preserve authored task fields while changing templates
persona: Bruno
journey: J-complete-task-tree
expected: Switching a Create task template or editor mode preserves title and description while updating only preset-owned contract fields, unless the operator explicitly confirms a reset.
entry_points: web Create task modal
qa_status: untested
bug_ids: BUG-20260713-task-template-clears-draft
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-task-template-draft-fixed.dom.txt
last_report: docs/qa/reports/2026-07-13-automation-features.md
overlaps: TA-parent-rollup-completion
---

Exercise title/description plus owner, parent, priority, attempts, and approval fields; presets must not erase fields they do not own.

2026-07-13 retest: title and description survived `Break into steps`, Simple → Advanced → Simple, and the final fresh DOM read. Canonical route coverage exercises every Simple preset and preserves advanced operator-owned fields; the browser modal was cancelled without creating a task.

QA impact 2026-07-14: template selection is now URL-authoritative and applies preset-owned fields only after the search parameter commits. Planning update only; reset to untested without a QA replay.
