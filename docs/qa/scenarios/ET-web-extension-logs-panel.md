---
id: ET-web-extension-logs-panel
area: ET
title: Follow redacted extension logs from the web detail
persona: Bruno
journey: J-extension-dev-lifecycle
expected: The extension detail logs panel reads one fresh `{stream_epoch, logs}` snapshot for the selected `(name, workspace)` instance before following SSE, pairs every resumed `after` cursor with that epoch, appends only same-epoch `extension_log` deltas, atomically replaces retained rows on `extension_log_reset` (including an empty reset), keeps the Query cache authoritative across reconnect/pause, and closes the EventSource when the panel unmounts or the instance changes.
entry_points: /marketplace/extension/$entryId (Logs panel); `GET /api/extensions/{name}/logs?workspace=&follow=1&after=&stream_epoch=`
qa_status: pass
bug_ids: BUG-20260729-global-extension-log-workspace-scope; BUG-20260812-workspace-extension-detail-missing
fix_status: fixed
retest_status: pass
fix_commits: 72170640
evidence: /Users/pedronauck/dev/qa-labs/compozy-frontend-performance-review-20260812-051926-937324-lab/qa-artifacts/qa/web-extension-logs-pass.png; /Users/pedronauck/dev/qa-labs/compozy-frontend-performance-review-20260812-051926-937324-lab/qa-artifacts/qa/api-extension-logs.json; /Users/pedronauck/dev/qa-labs/compozy-frontend-performance-review-20260812-051926-937324-lab/qa-artifacts/qa/teardown.json
last_report: docs/qa/reports/2026-08-11-frontend-performance.md
overlaps: ET-web-extension-detail
---

Added by ext-improvs Task 08. Exercise a dev-linked extension that writes to stderr: confirm the
initial history read, live append, a forced reconnect (kill and restart the instance) leaving no
duplicate or reordered rows, the paused state retaining rows, and no cross-workspace leakage when
the active workspace changes.

QA impact 2026-07-29: new surface. Never verified against a real daemon.

Visual recapture was explicitly waived by the operator on 2026-07-29. Functional browser coverage
is the implementation evidence for this run.

QA verdict 2026-07-29: PASS after fixing the global-instance scope regression. The live browser
first reproduced the workspace-scoped `not dev linked` error while the global HTTP read returned
200, then the same tab rendered the global empty log state without an error. Dev-overlay reconnect,
sequence, redaction, and workspace isolation remain covered by the daemon-served E2E lane.

QA impact 2026-08-12: the public log cursor now includes `stream_epoch`, and ring replacement is an
atomic SSE reset instead of a frontend recovery guess. Historical evidence remains valid for scope and
redaction only; replay the updated epoch/reset behavior before restoring `pass`.

QA result 2026-08-12: the first browser pass found
`BUG-20260812-workspace-extension-detail-missing`. After the scoped Marketplace projection fix, the
same deep link rendered `epoch-two-observed`; pausing Follow retained the row, and a page reload read
the same canonical snapshot before following again. The independent HTTP read matched the Web row,
the browser reported no page errors, and teardown was clean.
