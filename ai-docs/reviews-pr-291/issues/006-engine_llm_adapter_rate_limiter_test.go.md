# Issues for `engine/llm/adapter/rate_limiter_test.go`

## Issue 8 - Review Thread Comment

**File:** `engine/llm/adapter/rate_limiter_test.go:271`
**Date:** 2025-10-20 03:07:20 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_🧹 Nitpick_ | _🔵 Trivial_

**Consider refactoring to use the public API or document the rationale for internal testing.**

This test directly instantiates `providerRateLimiter` and accesses unexported fields (`sem`, `tokenLimiter`, `metrics.activeRequests`) and calls the unexported `release()` method. This couples the test tightly to implementation details and makes it brittle.

All other tests in this file use the public `RateLimiterRegistry` API (`Acquire`/`Release`), which provides better encapsulation and maintainability. If testing the internal `releaseSlotBeforeTokenWait` behavior requires direct access, consider:

1. Adding a public method to the registry that exercises this path, or
2. Adding a comment explaining why internal testing is necessary here (e.g., to verify precise sequencing that cannot be observed through the public API).



Example refactor using the public API:

```diff
-	t.Run("Should release slot before token wait when enabled", func(t *testing.T) {
-		limiter := &providerRateLimiter{
-			provider:                   core.ProviderOpenAI,
-			enabled:                    true,
-			concurrency:                1,
-			sem:                        semaphore.NewWeighted(1),
-			tokenLimiter:               rate.NewLimiter(rate.Every(100*time.Millisecond), 1),
-			releaseSlotBeforeTokenWait: true,
-		}
-		require.True(t, limiter.sem.TryAcquire(1))
-		limiter.metrics.activeRequests.Add(1)
-		ctx, cancel := context.WithCancel(t.Context())
-		done := make(chan struct{})
-		go func() {
-			limiter.release(ctx, 2)
-			close(done)
-		}()
-		require.Eventually(t, func() bool {
-			if limiter.metrics.activeRequests.Load() != 0 {
-				return false
-			}
-			if !limiter.sem.TryAcquire(1) {
-				return false
-			}
-			limiter.sem.Release(1)
-			return true
-		}, 50*time.Millisecond, 5*time.Millisecond)
-		cancel()
-		require.Eventually(t, func() bool {
-			select {
-			case <-done:
-				return true
-			default:
-				return false
-			}
-		}, time.Second, 10*time.Millisecond)
-	})
+	t.Run("Should release slot before token wait when enabled", func(t *testing.T) {
+		registry := NewRateLimiterRegistry(appconfig.LLMRateLimitConfig{
+			Enabled:                           true,
+			DefaultConcurrency:                1,
+			DefaultQueueSize:                  0,
+			DefaultReleaseSlotBeforeTokenWait: true,
+		}, providermetrics.Nop())
+
+		override := &core.ProviderRateLimitConfig{
+			Concurrency:                1,
+			TokensPerMinute:            600,
+			ReleaseSlotBeforeTokenWait: true,
+		}
+
+		ctx := t.Context()
+		require.NoError(t, registry.Acquire(ctx, core.ProviderOpenAI, override))
+		
+		// Release with tokens; slot should be freed before token wait completes
+		start := time.Now()
+		go func() {
+			time.Sleep(10 * time.Millisecond)
+			registry.Release(ctx, core.ProviderOpenAI, 100)
+		}()
+
+		// Second acquire should succeed quickly because slot is released before token wait
+		require.NoError(t, registry.Acquire(ctx, core.ProviderOpenAI, override))
+		elapsed := time.Since(start)
+		
+		// Should acquire quickly (not blocked by token rate limit)
+		require.Less(t, elapsed, 50*time.Millisecond, "slot should be released before token wait")
+		registry.Release(ctx, core.ProviderOpenAI, 0)
+	})
```

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
	t.Run("Should release slot before token wait when enabled", func(t *testing.T) {
		registry := NewRateLimiterRegistry(appconfig.LLMRateLimitConfig{
			Enabled:                           true,
			DefaultConcurrency:                1,
			DefaultQueueSize:                  0,
			DefaultReleaseSlotBeforeTokenWait: true,
		}, providermetrics.Nop())

		override := &core.ProviderRateLimitConfig{
			Concurrency:                1,
			TokensPerMinute:            600,
			ReleaseSlotBeforeTokenWait: true,
		}

		ctx := t.Context()
		require.NoError(t, registry.Acquire(ctx, core.ProviderOpenAI, override))
		
		// Release with tokens; slot should be freed before token wait completes
		start := time.Now()
		go func() {
			time.Sleep(10 * time.Millisecond)
			registry.Release(ctx, core.ProviderOpenAI, 100)
		}()

		// Second acquire should succeed quickly because slot is released before token wait
		require.NoError(t, registry.Acquire(ctx, core.ProviderOpenAI, override))
		elapsed := time.Since(start)
		
		// Should acquire quickly (not blocked by token rate limit)
		require.Less(t, elapsed, 50*time.Millisecond, "slot should be released before token wait")
		registry.Release(ctx, core.ProviderOpenAI, 0)
	})
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
In engine/llm/adapter/rate_limiter_test.go around lines 235 to 271, the test
instantiates providerRateLimiter and touches unexported fields/methods (sem,
tokenLimiter, metrics.activeRequests, release) which couples the test to
internal implementation; either refactor the test to exercise this behavior via
the public RateLimiterRegistry API (Acquire/Release) by adding a registry-level
test path that reproduces the releaseSlotBeforeTokenWait timing, or if that is
impractical, add a brief comment above the test explaining why direct access to
internals is required and consider adding a small exported test helper on the
package (or an exported registry method) that exposes the needed behavior for
testing instead of accessing unexported fields directly.
```

</details>

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5eiyP7`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5eiyP7
```

---
*Generated from PR review - CodeRabbit AI*


---

## Issue 73 - Review Thread Comment

**File:** `engine/llm/adapter/rate_limiter_test.go:203`
**Date:** 2025-10-20 10:04:03 America/Sao_Paulo
**Status:** - [x] RESOLVED ✓

## Body

_🧹 Nitpick_ | _🔵 Trivial_

**Prefer testify assertions over manual if/fatal for timing checks.**

Replace the manual elapsed check with require.GreaterOrEqual for consistency and clearer failure messages. As per coding guidelines.

```diff
-        if elapsed < 800*time.Millisecond {
-            t.Fatalf("expected rate limiter to enforce ~1s spacing, elapsed=%v", elapsed)
-        }
+        require.GreaterOrEqual(t, elapsed, 800*time.Millisecond, "should enforce ~1s spacing")
```

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
		require.GreaterOrEqual(t, elapsed, 800*time.Millisecond, "should enforce ~1s spacing")
	})
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
In engine/llm/adapter/rate_limiter_test.go around lines 200 to 203, replace the
manual timing check that uses if elapsed < 800*time.Millisecond { t.Fatalf(...)
} with a testify assertion: call require.GreaterOrEqual(t, elapsed,
800*time.Millisecond, "expected rate limiter to enforce ~1s spacing") to provide
consistent, clearer failure messages; also ensure the package imports
"github.com/stretchr/testify/require" if not already imported.
```

</details>

<!-- fingerprinting:phantom:medusa:chinchilla -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5elp1-`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5elp1-
```

---
*Generated from PR review - CodeRabbit AI*


---

## Issue 74 - Review Thread Comment

**File:** `engine/llm/adapter/rate_limiter_test.go:233`
**Date:** 2025-10-20 10:04:03 America/Sao_Paulo
**Status:** - [x] RESOLVED ✓

## Body

_🧹 Nitpick_ | _🔵 Trivial_

**Apply consistent assertion style for token rate timing.**

Same as above—use require.GreaterOrEqual for readability and consistency. As per coding guidelines.

```diff
-        if elapsed < 800*time.Millisecond {
-            t.Fatalf("expected token limiter to enforce ~1s spacing, elapsed=%v", elapsed)
-        }
+        require.GreaterOrEqual(t, elapsed, 800*time.Millisecond, "should enforce ~1s spacing")
```

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
		if elapsed < 800*time.Millisecond {
			t.Fatalf("expected token limiter to enforce ~1s spacing, elapsed=%v", elapsed)
		}
	})
```

```suggestion
		require.GreaterOrEqual(t, elapsed, 800*time.Millisecond, "should enforce ~1s spacing")
	})
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
In engine/llm/adapter/rate_limiter_test.go around lines 230 to 233, the test
uses t.Fatalf to assert elapsed >= 800*time.Millisecond; replace that with
require.GreaterOrEqual(t, elapsed, 800*time.Millisecond, "expected token limiter
to enforce ~1s spacing, elapsed=%v", elapsed) to match the project's assertion
style and improve readability (ensure the testify/require import is present).
```

</details>

<!-- fingerprinting:phantom:medusa:chinchilla -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5elp2E`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5elp2E
```

---
*Generated from PR review - CodeRabbit AI*
