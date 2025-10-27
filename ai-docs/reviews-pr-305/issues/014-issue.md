# Issue 14 - Review Thread Comment

**File:** `test/integration/temporal/mode_switching_test.go:45`
**Date:** 2025-10-27 13:58:52 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_⚠️ Potential issue_ | _🟠 Major_

**Ensure cleanup of the standalone server.**

Register t.Cleanup right after start to avoid leaks on failures.

Apply:

```diff
- server := startStandaloneServer(ctx, t, embeddedCfg)
+ server := startStandaloneServer(ctx, t, embeddedCfg)
+ t.Cleanup(func() {
+     stopTemporalServer(ctx, t, server)
+ })
```

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
In test/integration/temporal/mode_switching_test.go around lines 39 to 45, you
start a standalone server but don’t register cleanup immediately; add t.Cleanup
right after server := startStandaloneServer(...) to ensure the server is stopped
on test exit (e.g. t.Cleanup(func() { server.Stop() }) or server.Close()
depending on the server API), so the server is always torn down even if the test
fails before later cleanup.
```

</details>

<!-- fingerprinting:phantom:medusa:chinchilla -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5fez8L`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5fez8L
```

---
*Generated from PR review - CodeRabbit AI*
