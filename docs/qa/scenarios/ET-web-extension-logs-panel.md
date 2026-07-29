---
id: ET-web-extension-logs-panel
area: ET
title: Follow redacted extension logs from the web detail
persona: Bruno
journey: J-extension-dev-lifecycle
expected: The extension detail logs panel reads `GET /api/extensions/{name}/logs` for the selected `(name, workspace)` instance, follows it over SSE using only the named `extension_log` event, keeps sequences monotonic without duplicates across a reconnect, retains rendered lines while reconnecting or paused, exposes an operator-controlled Follow switch, announces connection status without announcing individual lines, and closes the EventSource when the panel unmounts or the instance changes.
entry_points: /marketplace/extension/$entryId (Logs panel); `GET /api/extensions/{name}/logs?workspace=&follow=1&after=`
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: web/src/systems/extensions/hooks/use-extension-logs.ts; web/src/systems/extensions/components/extension-log-panel.tsx; web/e2e/__tests__/extensions.spec.ts
last_report:
overlaps: ET-web-extension-detail
---

Added by ext-improvs Task 08. Exercise a dev-linked extension that writes to stderr: confirm the
initial history read, live append, a forced reconnect (kill and restart the instance) leaving no
duplicate or reordered rows, the paused state retaining rows, and no cross-workspace leakage when
the active workspace changes.

QA impact 2026-07-29: new surface. Never verified against a real daemon.

Visual recapture was explicitly waived by the operator on 2026-07-29. Functional browser coverage
remains the implementation evidence; this scenario stays `untested` for the next QA cycle.
