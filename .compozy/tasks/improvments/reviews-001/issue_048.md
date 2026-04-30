---
status: resolved
file: web/src/systems/runs/components/run-transcript-panel.tsx
line: 525
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-QUmE,comment:PRRC_kwDORy7nkc68K-RN
---

# Issue 048: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Failed tool parts lose the backend `errorText`.**

If a failed tool message has `errorText` but no structured `output`, this conversion sets `isError` but still passes `result: undefined`. `RunToolCard` then falls back to the generic message and hides the actual failure reason from the transcript.

<details>
<summary>Suggested fix</summary>

```diff
 function convertToolPart(part: RunUIMessagePart): ThreadPart {
   const input = toJSONObject(part.input);
-  const output = part.output ?? null;
+  const output = part.output ?? (part.errorText ? { summary: part.errorText } : null);
   return {
     type: "tool-call",
     toolCallId: part.toolCallId || part.id || `${part.toolName || "tool"}-call`,
     toolName: part.toolName || "tool",
     args: input,
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
function convertToolPart(part: RunUIMessagePart): ThreadPart {
  const input = toJSONObject(part.input);
  const output = part.output ?? (part.errorText ? { summary: part.errorText } : null);
  return {
    type: "tool-call",
    toolCallId: part.toolCallId || part.id || `${part.toolName || "tool"}-call`,
    toolName: part.toolName || "tool",
    args: input,
    argsText: JSON.stringify(input),
    result: part.state === "output-available" || part.state === "output-error" ? output : undefined,
    isError: part.state === "output-error" || Boolean(part.errorText),
  };
}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@web/src/systems/runs/components/run-transcript-panel.tsx` around lines 514 -
525, convertToolPart currently drops backend errorText when a tool failed but
has no structured output, causing RunToolCard to hide the real failure; update
convertToolPart so the returned ThreadPart.result uses the structured output if
present, otherwise falls back to include part.errorText (e.g., a string or a
small object like { error: part.errorText }) when part.state indicates an error
(state === "output-error" or Boolean(part.errorText)), and keep isError true;
modify the result assignment in convertToolPart so RunToolCard can render the
backend errorText as the result when output is missing.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:43567ed8-392f-4a75-9e7a-1958060562fd -->

<!-- 4e71b3a2 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Notes: `convertToolPart` marked failed tool calls as errors but discarded `errorText` when no structured output existed. The fix carries `errorText` into the result payload so the transcript can render the backend failure reason.
