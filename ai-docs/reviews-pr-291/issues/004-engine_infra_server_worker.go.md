# Issues for `engine/infra/server/worker.go`

## Issue 4 - Review Thread Comment

**File:** `engine/infra/server/worker.go:186`
**Date:** 2025-10-20 03:07:20 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_🛠️ Refactor suggestion_ | _🟠 Major_

**Remove logger parameter; retrieve from context instead.**

The method accepts `log logger.Logger` as a parameter (line 217), violating the coding guideline: "Use logger.FromContext(ctx); never pass a logger as a parameter or via DI". Since the method has access to `s.ctx`, it should retrieve the logger internally.

As per coding guidelines.

Apply this diff:

```diff
-	s.handleScheduleReconciliationFailure(err, log, startTime, cfg)
+	s.handleScheduleReconciliationFailure(err, startTime, cfg)
```

```diff
 func (s *Server) handleScheduleReconciliationFailure(
 	err error,
-	log logger.Logger,
 	start time.Time,
 	cfg *config.Config,
 ) {
 	if err == nil {
 		return
 	}
 	if s.ctx.Err() == context.Canceled {
+		log := logger.FromContext(s.ctx)
 		log.Info("Schedule reconciliation canceled during server shutdown")
 		return
 	}
+	log := logger.FromContext(s.ctx)
 	finalErr := fmt.Errorf("schedule reconciliation failed after maximum retries: %w", err)
 	s.reconciliationState.setError(finalErr)
 	log.Error("Schedule reconciliation exhausted retries",
 		"error", err,
 		"duration", time.Since(start),
 		"max_duration", cfg.Server.Timeouts.ScheduleRetryMaxDuration)
 }
```


Also applies to: 214-234

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
In engine/infra/server/worker.go around line 186 (and applying to the method
implementation spanning ~214-234), remove the logger.Logger parameter from
s.handleScheduleReconciliationFailure and all its call sites: stop passing log
into the call at line 186 and any other calls in the indicated range; change the
method signature to no longer accept logger and, inside the method body, obtain
the logger via logger.FromContext(s.ctx). Update call sites to match the new
signature and remove any now-unused log variables/imports.
```

</details>

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5eiyP3`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5eiyP3
```

---
*Generated from PR review - CodeRabbit AI*


---

## Issue 5 - Review Thread Comment

**File:** `engine/infra/server/worker.go:212`
**Date:** 2025-10-20 03:07:20 America/Sao_Paulo
**Status:** - [x] RESOLVED ✓

## Body

_🧹 Nitpick_ | _🔵 Trivial_

**Replace magic number with a named constant.**

Line 209 returns `time.Second` directly. Per coding guidelines, fixed implementation values should use named constants with clear intent.

Define a constant at the package level:

```diff
+const defaultScheduleRetryBaseDelay = time.Second
+
 const maxScheduleRetryAttemptsCap = 1_000_000
```

Then use it in the function:

```diff
 func scheduleRetryBaseDelay(cfg *config.Config) time.Duration {
 	base := cfg.Server.Timeouts.ScheduleRetryBaseDelay
 	if secs := cfg.Server.Timeouts.ScheduleRetryBackoffSeconds; secs > 0 {
 		base = time.Duration(secs) * time.Second
 	}
 	if base <= 0 {
-		return time.Second
+		return defaultScheduleRetryBaseDelay
 	}
 	return base
 }
```

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
// scheduleRetryBaseDelay resolves the base delay for retrying schedule reconciliation.
func scheduleRetryBaseDelay(cfg *config.Config) time.Duration {
	base := cfg.Server.Timeouts.ScheduleRetryBaseDelay
	if secs := cfg.Server.Timeouts.ScheduleRetryBackoffSeconds; secs > 0 {
		base = time.Duration(secs) * time.Second
	}
	if base <= 0 {
		return defaultScheduleRetryBaseDelay
	}
	return base
}
```

```suggestion
const defaultScheduleRetryBaseDelay = time.Second
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
In engine/infra/server/worker.go around lines 202 to 212, the function
scheduleRetryBaseDelay returns the literal time.Second (line 209); replace this
magic number with a package-level named constant to express intent. Add a
constant like defaultScheduleRetryBaseDelay := time.Second (or const
defaultScheduleRetryBaseDelay time.Duration = time.Second) at the top of the
file (package level), then update the function to return
defaultScheduleRetryBaseDelay instead of time.Second; keep the existing behavior
and types unchanged.
```

</details>

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5eiyP4`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5eiyP4
```

---
*Generated from PR review - CodeRabbit AI*
