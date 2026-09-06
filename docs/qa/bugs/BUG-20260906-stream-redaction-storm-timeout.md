# BUG-20260906-stream-redaction-storm-timeout: Redaction delays a large streamed response

- **Status:** fixed locally — unchanged real daemon stress journey passed; new-head CI pending
- **Impact:** Performance
- **Severity:** Major · **Priority:** P1
- **Scenarios:** RT-visible-session-streaming
- **Found:** 2026-09-06 · **Report:** docs/qa/reports/2026-09-06-sessions-stability.md

Runtime CI exhausted the 60s prompt response deadline during the ACP storm of 50,000 chunks. A one-core local run reproduced the failure; CPU sampling identified repeated canonical secret-assignment regex scans. The regex requires a colon or equals sign, so scanning text with neither separator cannot change its output. Applying that necessary condition reduced the local journey to 54.72s, but CI on head 500a93afe still exceeded its deadline. That first improvement was insufficient.

A new 12-second sample showed the remaining provider-token scans dominating active CPU. The redactor now derives each provider expression's required literal prefix from Go's regex parser before adding the existing word-boundary assertion. A missing prefix proves that expression cannot match. Expressions without a required prefix still run, and the original expressions, order, replacements, entropy checks and dynamic-secret protection remain unchanged.

The unchanged real one-core storm passes in 16.83s (51.92s immediately before this change), preserving complete durable bytes, terminal completion and one slow-watcher degradation record. The canonical redaction race suite passes; 11,840 before/after output hashes match, including every provider prefix, boundary variants, and both heuristic settings. Ten sequential benchmark samples per case show median reductions of 73.7% for 64KiB plaintext, 42.4% for its JSON envelope and 67.5% for a short provider message. No timeout, event count or assertion was weakened. The integrated report links the retained failures, profiles, output comparisons and real runs.
