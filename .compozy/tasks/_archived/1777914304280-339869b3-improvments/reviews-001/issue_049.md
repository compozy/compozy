---
status: resolved
file: web/src/systems/runs/components/run-transcript-panel.tsx
line: 542
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-QUmI,comment:PRRC_kwDORy7nkc68K-RR
---

# Issue 049: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Later live tool updates are dropped instead of replacing earlier ones.**

`tool_call_started` and `tool_call_updated` share the same synthetic id, but this merge only skips duplicates. The first event wins, so later state/output updates for the same tool call never reach the transcript.

<details>
<summary>Suggested fix</summary>

```diff
 function mergeTranscriptMessages(
   baseMessages: readonly RunUIMessage[],
   liveEvents: readonly RunFeedEvent[]
 ): RunUIMessage[] {
-  const messages = [...baseMessages];
-  const seen = new Set(messages.map(message => message.id));
+  const messages = new Map(baseMessages.map(message => [message.id, message] as const));
   for (const event of liveEvents) {
     const message = runUIMessageFromLiveEvent(event);
-    if (!message || seen.has(message.id)) {
+    if (!message) {
       continue;
     }
-    messages.push(message);
-    seen.add(message.id);
+    messages.set(message.id, message);
   }
-  return messages;
+  return [...messages.values()];
 }
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@web/src/systems/runs/components/run-transcript-panel.tsx` around lines 528 -
542, mergeTranscriptMessages currently skips live events whose synthetic id
already exists (so tool_call_updated never replaces the earlier
tool_call_started); update the loop so when runUIMessageFromLiveEvent(event)
returns a message with an id that is already seen you locate the existing
message in the messages array (by comparing message.id) and replace or merge its
contents with the new message instead of continuing; keep the existing seen set
logic but ensure you update the messages array entry (preserving ordering) when
handling tool_call_updated/tool_call_started pairs so later updates are
reflected in the transcript.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:43567ed8-392f-4a75-9e7a-1958060562fd -->

<!-- 4e71b3a2 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Notes: `mergeTranscriptMessages` skipped duplicate live message ids, so a later `tool_call_updated` message could not replace the earlier `tool_call_started` message. The fix stores messages by id and lets later live events replace earlier entries while preserving insertion order.
