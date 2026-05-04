---
status: resolved
file: internal/api/httpapi/browser_middleware_test.go
line: 49
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58gqW_,comment:PRRC_kwDORy7nkc651WGn
---

# Issue 009: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
cd /tmp && find . -name "browser_middleware_test.go" -o -name "static_test.go" 2>/dev/null | head -20
```

Repository: compozy/compozy

Length of output: 41

---

🏁 Script executed:

```shell
git ls-files | grep -E "(browser_middleware_test|static_test)" | head -20
```

Repository: compozy/compozy

Length of output: 142

---

🏁 Script executed:

```shell
cat -n internal/api/httpapi/browser_middleware_test.go | head -60
```

Repository: compozy/compozy

Length of output: 2119

---

🏁 Script executed:

```shell
cat -n internal/api/httpapi/static_test.go | head -60
```

Repository: compozy/compozy

Length of output: 2005

---

🏁 Script executed:

```shell
rg "gin\.SetMode" --type go
```

Repository: compozy/compozy

Length of output: 2641

---

🏁 Script executed:

```shell
cat -n internal/api/httpapi/browser_middleware_test.go | sed -n '46,120p'
```

Repository: compozy/compozy

Length of output: 2690

---

🏁 Script executed:

```shell
cat -n internal/api/core/handlers_error_paths_test.go | grep -A 15 "gin.SetMode"
```

Repository: compozy/compozy

Length of output: 633

---



</details>

**Move `gin.SetMode()` before `t.Parallel()` and restore in `t.Cleanup()`.**

`gin.SetMode()` is process-global state. Calling it after `t.Parallel()` (line 47) creates a race condition with other parallel tests in the same package, causing the race detector to fail. This violates the coding guideline: "Run tests with `-race` flag; the race detector must pass before committing."

Refactor by either:
1. Removing `t.Parallel()` from this test if gin mode setup is required per-test
2. Saving the previous mode, setting the new mode before `t.Parallel()`, and using `t.Cleanup(func() { gin.SetMode(previousMode) })` to restore (see `internal/api/core/handlers_error_paths_test.go` for this pattern)

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/api/httpapi/browser_middleware_test.go` around lines 46 - 49, In
TestBrowserMiddlewareRequestDetectionAndCSRFCookies save the current gin mode
into a variable, call gin.SetMode(...) before invoking t.Parallel(), and
register a t.Cleanup(func() { gin.SetMode(previousMode) }) to restore the
original mode; this removes the race on the process-global gin.SetMode while
keeping the test parallelizable and references the
TestBrowserMiddlewareRequestDetectionAndCSRFCookies function, gin.SetMode,
t.Parallel, and t.Cleanup to locate where to apply the change.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:5b81ebf2-33d3-49d0-b9c4-2c97e797915b -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - The test mutates Gin’s global mode after declaring the parent test parallel, which creates a real race risk with other parallel package tests.
  - Root cause: process-global Gin mode setup is not isolated/restored around the test.
  - Intended fix: set the mode before any parallel execution that depends on it, restore it with `t.Cleanup`, and keep the subtests parallel-safe.

## Resolution

- Moved the Gin global mode setup ahead of parallel execution in `browser_middleware_test.go` and restored the prior mode via `t.Cleanup`.
- Verified with `make verify`.
