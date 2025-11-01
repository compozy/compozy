# Issue 18 - Review Thread Comment

**File:** `sdk/compozy/lifecycle.go:322`
**Date:** 2025-11-01 01:57:03 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_🛠️ Refactor suggestion_ | _🟠 Major_

**Stop passing loggers as parameters**

`finalizeStop` still accepts a `logger.Logger` parameter, which conflicts with the guideline to always retrieve loggers from context rather than passing them around. Please refactor this helper to take a `context.Context`, call `logger.FromContext(ctx)` inside, and adjust the call site accordingly. As per coding guidelines.

```diff
-func (e *Engine) finalizeStop(errs []error, log logger.Logger) error {
+func (e *Engine) finalizeStop(ctx context.Context, errs []error) error {
+	log := logger.FromContext(ctx)
```
And in `Stop`:
```diff
-	return e.finalizeStop(errs, log)
+	return e.finalizeStop(ctx, errs)
```

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
In sdk/compozy/lifecycle.go around lines 304 to 322, finalizeStop currently
accepts a logger.Logger parameter which violates the guideline to obtain loggers
from context; change the signature to accept context.Context instead, call
logger.FromContext(ctx) inside finalizeStop to get the logger, replace all uses
of the passed-in log variable with that logger, and update every caller (e.g.,
Stop) to pass the current context rather than a logger; ensure nil-safe handling
(if logger.FromContext returns nil or a no-op logger, behave as before) and
update imports/signatures accordingly.
```

</details>

<!-- fingerprinting:phantom:medusa:sabertoothed -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5gLa2u`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5gLa2u
```

---
*Generated from PR review - CodeRabbit AI*
