# Issues for `engine/infra/server/middleware/ratelimit/config.go`

## Issue 1 - Review Thread Comment

**File:** `engine/infra/server/middleware/ratelimit/config.go:135`
**Date:** 2025-10-20 03:07:20 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_🛠️ Refactor suggestion_ | _🟠 Major_

**Replace all magic numbers with named constants.**

All numeric limits (200, 60, 100, 20, 30) should use the named constants suggested earlier:

```diff
 // defaultRouteRates returns default per-route rate limit settings.
 func defaultRouteRates() map[string]RateConfig {
 	return map[string]RateConfig{
 		routes.Base() + "/memory": {
-			Limit:    200,
-			Period:   time.Minute,
+			Limit:    defaultMemoryRateLimit,
+			Period:   defaultRatePeriod,
 			Disabled: false,
 		},
 		routes.Hooks(): {
-			Limit:    60,
-			Period:   time.Minute,
+			Limit:    defaultHooksRateLimit,
+			Period:   defaultRatePeriod,
 			Disabled: false,
 		},
 		routes.Base() + "/workflow": {
-			Limit:    100,
-			Period:   time.Minute,
+			Limit:    defaultWorkflowRateLimit,
+			Period:   defaultRatePeriod,
 			Disabled: false,
 		},
 		routes.Base() + "/task": {
-			Limit:    100,
-			Period:   time.Minute,
+			Limit:    defaultTaskRateLimit,
+			Period:   defaultRatePeriod,
 			Disabled: false,
 		},
 		routes.Base() + "/auth": {
-			Limit:    20,
-			Period:   time.Minute,
+			Limit:    defaultAuthRateLimit,
+			Period:   defaultRatePeriod,
 			Disabled: false,
 		},
 		routes.Base() + "/users": {
-			Limit:    30,
-			Period:   time.Minute,
+			Limit:    defaultUsersRateLimit,
+			Period:   defaultRatePeriod,
 			Disabled: false,
 		},
 	}
 }
```



> Committable suggestion skipped: line range outside the PR's diff.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
In engine/infra/server/middleware/ratelimit/config.go around lines 89 to 123,
replace the hard-coded numeric rate limits (200, 60, 100, 20, 30) with the
previously suggested named constants (e.g., MemoryLimit, HooksLimit,
WorkflowLimit, TaskLimit, AuthLimit, UsersLimit); update the map entries to use
those constant identifiers instead of magic numbers and ensure the constants are
defined/visible in this package (or imported) and typed appropriately (int) so
the RateConfig fields compile.
```

</details>

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5eiyPz`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5eiyPz
```

---
*Generated from PR review - CodeRabbit AI*


---

## Issue 64 - Review Thread Comment

**File:** `engine/infra/server/middleware/ratelimit/config.go:81`
**Date:** 2025-10-20 10:04:02 America/Sao_Paulo
**Status:** - [x] RESOLVED ✓

## Body

_🧹 Nitpick_ | _🔵 Trivial_

**Consider extracting remaining magic numbers to constants.**

While the rate limit values now use named constants, lines 74-75 still contain magic numbers (`3` for MaxRetry and `30` for HealthCheckInterval). For consistency with the coding guidelines, consider adding:

```diff
const (
	defaultGlobalRateLimit   = 100
	defaultAPIKeyRateLimit   = 100
	defaultMemoryRateLimit   = 200
	defaultHooksRateLimit    = 60
	defaultWorkflowRateLimit = 100
	defaultTaskRateLimit     = 100
	defaultAuthRateLimit     = 20
	defaultUsersRateLimit    = 30
	defaultRatePeriod        = time.Minute
+	defaultMaxRetry          = 3
+	defaultHealthCheckInterval = 30 * time.Second
)
```

Then update DefaultConfig:
```diff
-		MaxRetry:            3,
-		HealthCheckInterval: 30 * time.Second,
+		MaxRetry:            defaultMaxRetry,
+		HealthCheckInterval: defaultHealthCheckInterval,
```

**Minor: Consider using `nil` instead of `[]string{}` for ExcludedIPs** (line 78), as it's more idiomatic in Go for empty slices.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
In engine/infra/server/middleware/ratelimit/config.go around lines 65 to 81,
replace the remaining magic numbers by introducing named constants (e.g., const
defaultMaxRetry = 3 and const defaultHealthCheckInterval = 30 * time.Second) and
use those constants in DefaultConfig for MaxRetry and HealthCheckInterval; also
change ExcludedIPs from an explicit empty slice ([]string{}) to nil to follow Go
idioms for empty slices. Ensure the new constants are declared at the top of the
file (or near other defaults) and update DefaultConfig to reference them.
```

</details>

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5elp1c`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5elp1c
```

---
*Generated from PR review - CodeRabbit AI*
