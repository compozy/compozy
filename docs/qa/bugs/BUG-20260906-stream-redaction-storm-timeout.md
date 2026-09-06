# BUG-20260906-stream-redaction-storm-timeout: Redaction delays a large streamed response

- **Status:** fixed — unchanged real daemon stress journey passed
- **Impact:** Performance
- **Severity:** Major · **Priority:** P1
- **Scenarios:** RT-visible-session-streaming
- **Found:** 2026-09-06 · **Report:** docs/qa/reports/2026-09-06-sessions-stability.md

Runtime CI exhausted the 60s prompt response deadline during the ACP storm of 50,000 chunks. A one-core local run reproduced the failure; CPU sampling identified repeated canonical secret-assignment regex scans. The regex requires a colon or equals sign, so scanning text with neither separator cannot change its output. The redactor now applies that necessary condition while preserving the same rules, ordering and dynamic-secret protection.

The unchanged real test now passes in 54.72s with one core, including complete durable bytes, terminal completion and one slow-watcher degradation record. The existing redaction suite passes under the race detector; 800 before/after output hashes match. The canonical benchmark's 64KiB plaintext median improves from 42.21ms to 29.46ms across 10 sequential one-core samples. JSON still takes its required scan. No timeout, event count or assertion was weakened. The integrated report links the retained failure, profile, success, output proof and benchmark evidence.
