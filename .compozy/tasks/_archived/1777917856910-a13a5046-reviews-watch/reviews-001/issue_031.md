---
provider: coderabbit
pr: "133"
round: 1
round_created_at: 2026-04-30T20:37:59.817595Z
status: resolved
file: web/src/systems/reviews/hooks/use-reviews.ts
line: 52
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-22Ey,comment:PRRC_kwDORy7nkc68_V7U
---

# Issue 031: _⚠️ Potential issue_ | _🟡 Minor_ | _⚡ Quick win_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_ | _⚡ Quick win_

**Block invalid round `0` before querying.**

Line 51 currently enables fetch for `round >= 0`, which allows `0` and triggers avoidable invalid API requests for round endpoints.

 

<details>
<summary>Suggested fix</summary>

```diff
-      if (round == null) {
+      if (round == null || round <= 0) {
         throw new Error("review round is required to load a review round");
       }
       return getReviewRound({ workspaceId, slug, round });
     },
-    enabled: Boolean(workspaceId) && Boolean(slug) && round != null && round >= 0,
+    enabled: Boolean(workspaceId) && Boolean(slug) && round != null && round > 0,
   });
 }
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
      if (round == null || round <= 0) {
        throw new Error("review round is required to load a review round");
      }
      return getReviewRound({ workspaceId, slug, round });
    },
    enabled: Boolean(workspaceId) && Boolean(slug) && round != null && round > 0,
  });
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@web/src/systems/reviews/hooks/use-reviews.ts` around lines 46 - 52, The
enabled check currently allows round == 0 which causes invalid API calls; update
the guard to require round > 0 and add a runtime validation before calling
getReviewRound to throw on non-positive rounds. Specifically, change the
react-query enabled expression to use round != null && round > 0 (instead of
round >= 0) and adjust the early validation (the if (round == null) block) to
also reject round <= 0 (e.g., if (round == null || round <= 0) throw ...) so
getReviewRound is never invoked with round 0.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:9d503b4c-1a51-4ef5-a14d-2e16d6ffd95a -->

<!-- 4e71b3a2 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes: `useReviewRound` currently enables the query for `round === 0`, which is not a valid review round and can produce an avoidable request. I will block non-positive rounds in both the runtime guard and the query `enabled` condition, and add a hook-level test to prove the request is skipped.
- Resolution: Blocked non-positive rounds across the review hooks, added hook-level regression coverage for `round = 0`, and confirmed the behavior through the full verification gate.
