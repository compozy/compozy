---
status: resolved
file: internal/core/agent/client_test.go
line: 1486
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4093340073,nitpick_hash:63d3097b6393
review_hash: 63d3097b6393
source_review_id: "4093340073"
source_review_submitted_at: "2026-04-11T03:34:14Z"
---

# Issue 004: This hook stub bypasses the JSON round-trip the real path uses.
## Review Comment

`DispatchMutableHook` returns the same Go value back to the test, so the pre-hook tests won't catch serialization regressions in `SessionRequest` / `ResumeSessionRequest`—especially custom `MarshalJSON` / `UnmarshalJSON` behavior and fields excluded with `json:"-"`. Consider round-tripping through JSON here, or add one integration-style test against the real runtime manager.

## Triage

- Decision: `valid`
- Notes:
  - The `agentHookManager` test double returns mutated payloads as in-memory Go values, which skips the JSON marshal/unmarshal path used by the real extension hook runtime.
  - That means the pre-create / pre-resume tests would not catch regressions in `SessionRequest` or `ResumeSessionRequest` JSON behavior. I will make the test double round-trip mutated payloads through JSON before returning them.
  - Resolved by JSON round-tripping hook payloads inside `agentHookManager.DispatchMutableHook`.
