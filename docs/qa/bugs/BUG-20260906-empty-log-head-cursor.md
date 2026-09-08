# BUG-20260906-empty-log-head-cursor: Empty live log streams skip older-timestamp events

- **Status:** fixed — real SQLite/HTTP integration passed
- **Impact:** Missing-Data
- **Severity:** Major · **Priority:** P1
- **Scenarios:** RT-visible-session-streaming
- **Found:** 2026-09-06 · **Report:** docs/qa/reports/2026-09-06-sessions-stability.md

When a live-only log stream opened with an empty durable head, it synthesized the connection time as its cursor. A subsequently persisted event with an earlier timestamp was then filtered out despite having a new durable sequence. Keep the empty cursor at sequence zero; subsequent delivery follows durable sequence order. Explicit legacy timestamp cursors retain their boundary translation.

Invariant: an empty-head live-only stream delivers the first newly persisted event regardless of its timestamp. Owning layer: public HTTP log stream over the real observer/SQLite store. Canonical suite: TestHTTPLogsStreamFromEmptyHead in internal/api/httpapi/httpapi_integration_test.go. A transparent observer probe waits for the actual empty head query; the test then writes an event dated 2000 and verifies its SSE type, positive sequence and exact timestamp. No fabricated query results or timing sleeps are used.

The existing HTTP fixture first reproduced the skipped event before the repair (`.cache/sessions-final-empty-log-cursor-red.log`). Permanent coverage lives only at the stronger integration owner. Final real HTTP/UDS stream selections pass (12.803s / 4.784s): `.cache/sessions-final-gate-stream-integration-green.log`.
