---
status: resolved
file: web/src/systems/reviews/components/review-detail-view.tsx
line: 248
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc59RNfq,comment:PRRC_kwDORy7nkc662oQY
---

# Issue 011: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Use the runs status resolver for related run badges.**

Line 248 now feeds `run.status` through the review-issue resolver from `reviews-index-view`, which only understands issue states like `resolved`, `dispatched`, and `open`. Daemon run states will fall into the wrong tone here; `task-detail-view.tsx` already uses the runs resolver for the same kind of row.

<details>
<summary>Suggested fix</summary>

```diff
+import { resolveStatusTone as resolveRunStatusTone } from "@/systems/runs";
 import { resolveSeverityTone, resolveStatusTone } from "./reviews-index-view";
@@
-                <StatusBadge tone={resolveStatusTone(run.status)}>{run.status}</StatusBadge>
+                <StatusBadge tone={resolveRunStatusTone(run.status)}>{run.status}</StatusBadge>
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
import { resolveStatusTone as resolveRunStatusTone } from "@/systems/runs";
import { resolveSeverityTone, resolveStatusTone } from "./reviews-index-view";
```

```suggestion
                <StatusBadge tone={resolveRunStatusTone(run.status)}>{run.status}</StatusBadge>
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@web/src/systems/reviews/components/review-detail-view.tsx` at line 248, The
StatusBadge is using resolveStatusTone (which handles issue states) but must use
the runs status resolver used for daemon runs; replace the call to
resolveStatusTone(run.status) with the runs resolver used in
task-detail-view.tsx (import that resolver the same way task-detail-view.tsx
does, e.g., resolveRunsStatusTone or the exact function name exported there) so
StatusBadge receives the correct tone for run.status; update the import at the
top and swap the function used in the JSX (StatusBadge tone={...}) accordingly.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:8d35343f-4585-425c-9c2a-f51e383d6f5d -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - `review-detail-view` currently resolves `run.status` with the review-issue status helper, which only understands review issue states such as `open`, `resolved`, and `dispatched`.
  - Daemon run states like `running`, `queued`, and `completed` therefore receive the wrong badge tone in related run rows.
  - Fix: use the canonical runs resolver from `@/systems/runs` and extend the review detail test coverage to lock the related-run badge behavior.
