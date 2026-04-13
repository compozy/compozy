---
status: resolved
file: internal/core/run/internal/acpshared/reusable_agent_lifecycle.go
line: 177
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56P5t5,comment:PRRC_kwDORy7nkc62zc8p
---

# Issue 014: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Keep `nestedToolCalls` in sync with journal write success.**

`handleNestedReusableAgentToolUse` records the call before emitting `NestedStarted`, and `handleNestedReusableAgentToolResult` deletes it before emitting completion. If `runJournal.Submit` fails, the retry path has already lost the original state, so the lifecycle event becomes permanently non-retriable. Move the insert/delete to after a successful submit, or roll the map change back on error.  



Also applies to: 219-223

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/run/internal/acpshared/reusable_agent_lifecycle.go` around
lines 159 - 177, The code is mutating h.nestedToolCalls before calling
submitReusableAgentLifecycle, which can leave the map out-of-sync if
runJournal.Submit fails; update handleNestedReusableAgentToolUse so it calls
submitReusableAgentLifecycle first and only inserts the nestedToolCalls entry
after the submit succeeds (or, alternatively, perform the insert then roll it
back if submit returns an error), and apply the same change to the analogous
block referenced at lines 219-223 (handleNestedReusableAgentToolResult deletion)
so deletions are performed only after a successful submit or are reverted on
submit failure; use the nestedToolCalls map, submitReusableAgentLifecycle,
handleNestedReusableAgentToolUse and handleNestedReusableAgentToolResult
identifiers to locate and update the logic.
```

</details>

<!-- fingerprinting:phantom:medusa:ocelot:ee6f376d-2c51-442f-8f6e-f006907140c7 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `nestedToolCalls` was updated before lifecycle submission succeeded on nested start and deleted before submission succeeded on nested completion, so submit failures permanently desynchronized retry state.
- Fix: Nested-start tracking now happens only after a successful lifecycle submit, and nested completion deletes tracked state only after a successful completion submit; added regression tests for both failure paths.
- Evidence: `go test ./internal/core/run/internal/acpshared`
