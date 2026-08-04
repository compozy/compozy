---
id: ET-running-session-window-close-clean
area: ET
title: Close a running session window without a thread teardown error
persona: Théo
journey: J-11
expected: Closing a window while its session is running removes the window once, leaves the agent work alive, produces no error boundary or product console error, and reopening the same session renders its current transcript.
entry_points: web desktop session window close; web session catalog reopen
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-durable-acp-sessions-20260804-164200-701355-lab/qa-artifacts/qa/evidence/02-running-before-window-close.png;/Users/pedronauck/dev/qa-labs/compozy-durable-acp-sessions-20260804-164200-701355-lab/qa-artifacts/qa/evidence/browser-console-after-close.txt
last_report: docs/qa/reports/2026-08-04-durable-acp-sessions.md
overlaps: ET-web-window-routing-lifecycle; RT-058
---

Added by the 2026-08-04 assistant thread teardown fix. The public observable includes both the clean close and the successful reopen while server-side work continues.

QA 2026-08-04: the window closed during a live Claude turn, the session continued running, and reopening restored the current transcript. Browser error collection stayed empty; only expected SSE cleanup and reopen debug events were logged.
