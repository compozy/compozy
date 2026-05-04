---
status: resolved
file: internal/api/core/handlers.go
line: 775
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc579mm4,comment:PRRC_kwDORy7nkc65HKYD
---

# Issue 006: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Cap `limit` before handing it to the run service.**

Both endpoints accept any positive integer. A very large `limit` can force oversized queries and responses from a single request, which is an avoidable latency/memory footgun. Clamp or reject values above a fixed max.


<details>
<summary>Suggested fix</summary>

```diff
+const maxPageLimit = 500
+
  limit, err := parsePositiveInt(c.Query("limit"), "limit")
  if err != nil {
  	h.respondError(c, err)
  	return
  }
  if limit == 0 {
  	limit = 100
  }
+ if limit > maxPageLimit {
+ 	h.respondError(c, validationProblem(
+ 		"limit_invalid",
+ 		fmt.Sprintf("limit must be less than or equal to %d", maxPageLimit),
+ 		map[string]any{"field": "limit"},
+ 	))
+ 	return
+ }
```

Apply the same bound in both `ListRuns` and `ListRunEvents`.
</details>


Also applies to: 840-847

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/api/core/handlers.go` around lines 768 - 775, The handlers currently
accept any positive integer from parsePositiveInt into the local variable limit;
clamp limit to a safe maximum before calling the run service (e.g., define a
MAX_LIMIT constant and set limit = min(limit, MAX_LIMIT) or return an error if
limit > MAX_LIMIT), and apply the same change in both ListRuns and ListRunEvents
so oversized requests are prevented; update the code paths that call the run
service with this clamped limit (reference symbols: parsePositiveInt, limit
variable, ListRuns, ListRunEvents, and the run service call).
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:c1d7e4c5-68cf-4aef-a285-1de9756bb650 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Root cause: `ListRuns` and `ListRunEvents` accept any positive `limit`, which leaves the API open to oversized queries and large response payloads from a single request.
- Fix plan: define a shared maximum page limit, reject values above it with a validation error, and add coverage for both handlers.
- Scope note: the limit cap required one downstream compatibility adjustment in `pkg/compozy/runs/run.go` so the public run reader stopped requesting pages larger than the new API ceiling.
- Resolution: Implemented and verified with `make verify`.
