---
status: resolved
file: internal/core/run/internal/acpshared/reusable_agent_lifecycle.go
line: 184
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56QRlC,comment:PRRC_kwDORy7nkc62z8Sf
---

# Issue 007: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**TOCTOU race may emit duplicate `nested-started` events.**

The lock is released after the existence check (line 163) and re-acquired after submission (line 179). Two concurrent goroutines processing the same `toolCallID` can both pass the initial check and both call `submitReusableAgentLifecycle` before either inserts into the map. The second re-check at line 180 prevents duplicate map entries but not duplicate event submissions.

Consider holding the lock across the entire operation, or accepting the (rare) duplicate events if they're idempotent on the consumer side.


<details>
<summary>🔒 Suggested fix: hold lock across submission</summary>

```diff
 func (h *SessionUpdateHandler) handleNestedReusableAgentToolUse(
 	update model.SessionUpdate,
 	block model.ToolUseBlock,
 ) error {
 	toolCallID := firstNonEmpty(strings.TrimSpace(update.ToolCallID), strings.TrimSpace(block.ID))
 	if toolCallID == "" {
 		return nil
 	}

 	h.mu.Lock()
-	if _, exists := h.nestedToolCalls[toolCallID]; exists {
-		h.mu.Unlock()
-		return nil
-	}
-	h.mu.Unlock()
-
-	call := nestedReusableAgentCall{}
-	if input, ok := decodeRunAgentToolInput(block); ok {
-		call.Name = input.Name
-	}
-
-	if err := submitReusableAgentLifecycle(h.ctx, h.journal, h.runID, kinds.ReusableAgentLifecyclePayload{
-		Stage:           kinds.ReusableAgentLifecycleStageNestedStarted,
-		AgentName:       call.Name,
-		ParentAgentName: h.currentReusableAgentName(),
-		ToolCallID:      toolCallID,
-	}); err != nil {
-		return err
-	}
-
-	h.mu.Lock()
-	if _, exists := h.nestedToolCalls[toolCallID]; !exists {
-		h.nestedToolCalls[toolCallID] = call
+	if _, exists := h.nestedToolCalls[toolCallID]; exists {
+		h.mu.Unlock()
+		return nil
 	}
+	call := nestedReusableAgentCall{}
+	if input, ok := decodeRunAgentToolInput(block); ok {
+		call.Name = input.Name
+	}
+	h.nestedToolCalls[toolCallID] = call
 	h.mu.Unlock()
+
+	if err := submitReusableAgentLifecycle(h.ctx, h.journal, h.runID, kinds.ReusableAgentLifecyclePayload{
+		Stage:           kinds.ReusableAgentLifecycleStageNestedStarted,
+		AgentName:       call.Name,
+		ParentAgentName: h.currentReusableAgentName(),
+		ToolCallID:      toolCallID,
+	}); err != nil {
+		// Rollback on failure
+		h.mu.Lock()
+		delete(h.nestedToolCalls, toolCallID)
+		h.mu.Unlock()
+		return err
+	}
 	return nil
 }
```
</details>

<!-- fingerprinting:phantom:medusa:ocelot:31ed1798-56d0-4921-97de-29473453283d -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `invalid`
- Notes:
  - `SessionUpdateHandler` is consumed serially by `StreamSessionUpdates`, which reads one ACP session update at a time and invokes `HandleUpdate` synchronously. Under that contract, two goroutines cannot legitimately race through `handleNestedReusableAgentToolUse` for the same `toolCallID`.
  - The existing lock protects shared handler state, but the duplicate-event scenario described here requires violating the handler's current single-consumer usage model.
  - I did not find a real concurrent call site in this code path, so I am not widening the critical section around journal submission for a non-reproducible race.
  - Resolution: analysis complete; no code change required.
