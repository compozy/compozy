---
id: ET-hidden-window-live-resources
area: ET
title: Suspend hidden window live resources
persona: Bruno
journey: J-operate-desktop-shell
expected: Task, Loop, Network, Bridge, Marketplace, and dashboard windows stop their own streams, polling, and elapsed clocks when minimized, off-desktop, inside an inactive tab stack, or document-hidden; visible sibling windows stay current, and every restored window reconciles current server state without a manual reload.
entry_points: Web OS shell; window minimize and restore; desktop switch; tab-stack activation; browser background and foreground
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-08-11-frontend-performance/two-live-windows-visible.png;docs/qa/evidence/2026-08-11-frontend-performance/task-restored-cursor-reconnect.png;docs/qa/evidence/2026-08-11-frontend-performance/hidden-window-resources.har
last_report: docs/qa/reports/2026-08-11-frontend-performance.md
overlaps: RT-visible-session-streaming
---

Created for the 2026-08-11 frontend performance remediation. The observable is browser resource ownership plus truthful catch-up, not daemon-side work cancellation.

QA 2026-08-11: minimizing the live task window closed its task EventSource while the visible Network window stayed current. Restoring the task opened one cursor-based stream at `after_sequence=3`; document background/foreground then reconciled both visible windows without a reload or duplicate event.
