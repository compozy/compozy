---
status: resolved
file: internal/core/run/internal/acpshared/session_handler.go
line: 112
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56P5t6,comment:PRRC_kwDORy7nkc62zc8q
---

# Issue 015: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Avoid failing the parent session on nested lifecycle emission errors.**

Lines 110-112 return immediately if reusable-agent lifecycle decoding/submission fails. That lets a malformed `run_agent` tool result or journal hiccup kill an otherwise healthy parent session after its output was already applied. This should be best-effort with a warning, not a hard stop. Based on learnings: Name concrete failure modes when identifying potential issues.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/run/internal/acpshared/session_handler.go` around lines 109 -
112, The call to h.emitReusableAgentLifecycleFromUpdate after
h.applySessionUpdate must not abort the parent session on failure; instead,
catch the error from emitReusableAgentLifecycleFromUpdate, log a warning that
includes concrete failure modes (e.g., "decoding error", "submission error",
"journal hiccup") and relevant context (session id, update id), and continue
normal execution so the parent session stays healthy; update the code around
applySessionUpdate/emitReusableAgentLifecycleFromUpdate to handle the error
non-fatally (log + continue) rather than returning it.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:5b83f2d8-737c-414c-9d4a-933187b6f725 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Analysis: The current code already ignores undecodable `run_agent` blocks, but lifecycle submit failures from `emitReusableAgentLifecycleFromUpdate()` still aborted the parent session after `applySessionUpdate()` had already mutated the transcript state.
- Fix: Converted lifecycle emission failures into warnings with session/update context and continued normal session-update processing; added regression coverage proving the parent session keeps processing updates.
- Evidence: `go test ./internal/core/run/internal/acpshared`
