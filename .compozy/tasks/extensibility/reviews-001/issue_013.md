---
status: resolved
file: internal/core/agent/hooks.go
line: 37
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56RVO8,comment:PRRC_kwDORy7nkc621Vaw
---

# Issue 013: _⚠️ Potential issue_ | _🔴 Critical_
## Review Comment

_⚠️ Potential issue_ | _🔴 Critical_

**Preserve the original request context after hook mutation.**

Both helpers copy `RunID`, `JobID`, and `RuntimeMgr`, but they drop `Context`. After a mutable pre-hook returns, the session request can lose its original cancellation/deadline state and continue detached from the caller lifecycle. Copy `Context` back here as well.


As per coding guidelines, "`**/*.go`: Pass `context.Context` as the first argument to all functions crossing runtime boundaries; avoid `context.Background()` outside `main` and focused tests`."


Also applies to: 68-72

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/agent/hooks.go` around lines 33 - 37, The withHookContextFrom
function currently copies RunID, JobID and RuntimeMgr but omits Context, which
can detach the mutated SessionRequest from the caller's cancellation/deadline;
update withHookContextFrom to copy src.Context into r.Context (i.e., r.Context =
src.Context) so the original context is preserved after pre-hook mutation, and
apply the same change to the other helper noted in the review (the partner
helper that also copies RunID/JobID/RuntimeMgr).
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:d791b4d1-a09d-47ff-aca9-2faf6e21ecb7 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Notes:
  - Both `withHookContextFrom` helpers restore `RunID`, `JobID`, and `RuntimeMgr` after mutable hook dispatch but omit the original `Context`.
  - Root cause: hook-mutation round-trips rebuild the request structs and lose the caller context unless it is copied back explicitly.
  - Implemented fix: copied `Context` in both helper functions and added regression tests proving pre-hook mutation preserves the original context object.
