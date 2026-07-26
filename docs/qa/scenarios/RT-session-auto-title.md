---
id: RT-session-auto-title
area: RT
title: Generate a useful title from the first real task
persona: Bruno
journey: J-17
expected: After the first real user task, the session receives one concise automatic title that survives refresh and is consistent between the workspace list and session topbar.
entry_points: web new session; web session list; web session detail
qa_status: pass
bug_ids: BUG-20260713-background-session-indicator-title
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/rt-agh84-two-workspace-badge-title.dom.txt
last_report: docs/qa/reports/2026-07-13-automation-features.md
overlaps:
---

The title must not expose evaluator language, raw ids, or an empty placeholder.
