---
status: resolved
file: web/src/routes/_app/workflows_.$slug.tasks_.$taskId.tsx
line: 95
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-QUl9,comment:PRRC_kwDORy7nkc68K-RE
---

# Issue 046: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Don't prefer older successes over newer failures.**

`statusRank()` puts `completed` / `succeeded` ahead of every other terminal state. If a task has an older successful run and a newer failed run, `selectTranscriptRunId()` will keep showing the stale success transcript instead of the latest failure log. Keep active states highest, then order the remaining runs by recency.

<details>
<summary>Suggested fix</summary>

```diff
 function statusRank(status: string): number {
   const normalized = status.toLowerCase();
-  if (normalized === "running" || normalized === "queued" || normalized === "starting") {
-    return 2;
-  }
-  if (normalized === "completed" || normalized === "succeeded") {
-    return 1;
-  }
-  return 0;
+  return normalized === "running" || normalized === "queued" || normalized === "starting" ? 1 : 0;
 }
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@web/src/routes/_app/workflows_`.$slug.tasks_.$taskId.tsx around lines 77 -
95, The current statusRank gives completed/succeeded a higher rank than
failed/other terminals, causing older successes to be chosen over newer
failures; update statusRank (used by selectTranscriptRunId and the sorted
comparator) so only active states ("running", "queued", "starting") return the
highest rank (e.g., 2) and all other statuses (including "completed",
"succeeded", "failed", etc.) return the same lower rank (e.g., 0), leaving the
comparator to break ties by timestampValue(right.started_at) so the most recent
terminal run is selected.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:b85fd798-8857-42d7-85f5-e2380613dd23 -->

<!-- 4e71b3a2 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Notes: `statusRank` gave completed/succeeded runs priority over other terminal states, so an older success could be selected before a newer failure. The fix ranks only active states above terminal states and lets timestamp ordering choose among terminal runs.
