# Issue 5 - Review Thread Comment

**File:** `internal/cli/state.go:261`
**Date:** 2026-04-07 15:52:02 UTC
**Status:** - [ ] UNRESOLVED

## Technical Disposition

`VALID` - the current tree already rejects non-positive timeouts. `internal/cli/state.go` checks `parsed <= 0` after `time.ParseDuration` and returns `invalid timeout %q: must be > 0`, so no further code change is needed for this review item.

## Body

_⚠️ Potential issue_ | _🟡 Minor_

**Reject non-positive timeouts.**

`time.ParseDuration` accepts `0` and negative values, so `--timeout=0` or `--timeout=-1m` currently builds a config that can fail jobs immediately instead of surfacing a bad input error.

<details>
<summary>Suggested fix</summary>

```diff
 	if s.timeout != "" {
 		parsed, err := time.ParseDuration(s.timeout)
 		if err != nil {
 			return core.Config{}, fmt.Errorf("parse timeout: %w", err)
 		}
+		if parsed <= 0 {
+			return core.Config{}, fmt.Errorf("parse timeout: must be > 0")
+		}
 		timeoutDuration = parsed
 	}
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
	timeoutDuration := time.Duration(0)
	if s.timeout != "" {
		parsed, err := time.ParseDuration(s.timeout)
		if err != nil {
			return core.Config{}, fmt.Errorf("parse timeout: %w", err)
		}
		if parsed <= 0 {
			return core.Config{}, fmt.Errorf("parse timeout: must be > 0")
		}
		timeoutDuration = parsed
	}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/state.go` around lines 255 - 261, The code currently accepts
zero or negative durations from s.timeout because time.ParseDuration allows
them; update the parsing logic in the function that builds the core.Config
(where s.timeout is parsed into timeoutDuration) to validate parsed > 0 and
return an error when parsed <= 0, e.g., after calling
time.ParseDuration(s.timeout) check the parsed value and return
fmt.Errorf("invalid timeout: must be > 0") (or similar) rather than using a
non-positive timeoutDuration; keep the variable names timeoutDuration, s.timeout
and the time.ParseDuration call so the change is localized.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:f9f66184-5b4a-4f5a-94d3-2e0f7df9fe75 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDORy7nkc55VFbD`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc55VFbD
```

---
*Generated from PR review - CodeRabbit AI*
