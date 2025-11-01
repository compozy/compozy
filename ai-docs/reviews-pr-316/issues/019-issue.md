# Issue 19 - Review Thread Comment

**File:** `sdk/compozy/lifecycle.go:472`
**Date:** 2025-11-01 01:57:03 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_🛠️ Refactor suggestion_ | _🟠 Major_

**Derive the logger inside `launchServer`**

`launchServer` also receives a pre-fetched logger, which violates the “no logger parameters” rule. Please pass the context instead, call `logger.FromContext(ctx)` within the helper, and update the `startHTTPComponents` caller. As per coding guidelines.

```diff
-func (e *Engine) launchServer(log logger.Logger, srv *http.Server, ln net.Listener) {
+func (e *Engine) launchServer(ctx context.Context, srv *http.Server, ln net.Listener) {
+	log := logger.FromContext(ctx)
```
And in `startHTTPComponents`:
```diff
-	e.launchServer(logger.FromContext(ctx), server, listener)
+	e.launchServer(ctx, server, listener)
```


> Committable suggestion skipped: line range outside the PR's diff.

<!-- fingerprinting:phantom:medusa:sabertoothed -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5gLa2w`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5gLa2w
```

---
*Generated from PR review - CodeRabbit AI*
