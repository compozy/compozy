---
status: resolved
file: internal/core/run/internal/acpshared/reusable_agent_lifecycle.go
line: 248
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4092828776,nitpick_hash:45152813b21c
review_hash: 45152813b21c
source_review_id: "4092828776"
source_review_submitted_at: "2026-04-10T22:56:04Z"
---

# Issue 008: JSON decode errors are silently swallowed.
## Review Comment

Both `decodeRunAgentToolInput` and `decodeRunAgentToolResult` discard JSON unmarshal errors. This is likely intentional for resilience against malformed tool payloads, but consider logging at debug level to aid troubleshooting when tool results don't emit expected lifecycle events.

Also applies to: 266-286

---

## Triage

- Decision: `invalid`
- Notes:
  - These decode helpers are intentionally best-effort parsers for optional lifecycle decoration extracted from tool blocks. Malformed JSON is treated as "no lifecycle metadata available" so the parent session can continue normally.
  - Adding debug logging here would require threading a logger into pure decode helpers or emitting high-volume hot-path noise for malformed/partial payloads, without a concrete correctness failure to address.
  - The current behavior is resilient by design: lifecycle events are skipped when the payload is unusable, but the session stream remains intact.
  - Resolution: analysis complete; no code change required.
