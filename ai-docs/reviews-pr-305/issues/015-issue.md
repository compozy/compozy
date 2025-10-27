# Issue 15 - Review Thread Comment

**File:** `test/integration/temporal/persistence_test.go:50`
**Date:** 2025-10-27 13:58:52 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_⚠️ Potential issue_ | _🟠 Major_

**Add cleanup for restarted server to prevent leaks.**

The restarted server isn’t stopped; add a cleanup to avoid dangling listeners/locks during CI. 


```diff
-	restarted := startStandaloneServer(restartCtx, t, restartCfg)
+	restarted := startStandaloneServer(restartCtx, t, restartCfg)
+	t.Cleanup(func() {
+		stopTemporalServer(restartCtx, t, restarted)
+	})
```

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
In test/integration/temporal/persistence_test.go around lines 39 to 50, the
restarted server started with startStandaloneServer is not being stopped; add a
cleanup to avoid leaking resources by registering t.Cleanup(func() {
restarted.Stop() }) immediately after creating restarted (or call the
appropriate shutdown method if the server type uses a different name, e.g.,
Close or Shutdown) so the server is stopped when the test finishes.
```

</details>

<!-- fingerprinting:phantom:medusa:chinchilla -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5fez8N`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5fez8N
```

---
*Generated from PR review - CodeRabbit AI*
