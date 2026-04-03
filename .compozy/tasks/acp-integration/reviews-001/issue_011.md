---
status: resolved
file: internal/core/run/execution_acp_integration_test.go
line: 526
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc54vHB9,comment:PRRC_kwDORy7nkc60y0f8
---

# Issue 011: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Potential race condition in counter file access across concurrent helper processes.**

The `loadRunACPHelperScenario` function reads, increments, and writes a counter file without file locking. If multiple ACP helper processes spawn concurrently (e.g., during parallel job execution), they could read the same counter value before either writes the increment, causing scenario index collision.

While the current tests appear to serialize helper invocations, this could become a subtle issue if test patterns change.



<details>
<summary>Consider using file locking or atomic counter mechanism</summary>

```diff
 func loadRunACPHelperScenario() (runACPHelperScenario, error) {
 	// ... scenario parsing ...
 
 	counterFile := os.Getenv("GO_RUN_ACP_HELPER_COUNTER_FILE")
 	if counterFile != "" {
+		// Consider using syscall.Flock or a lock file pattern
+		// to prevent concurrent read-modify-write races
 		content, err := os.ReadFile(counterFile)
 		// ...
 	}
 }
```

Alternatively, document that tests using multi-scenario sequences must not run helper processes concurrently.
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/run/execution_acp_integration_test.go` around lines 489 - 526,
The loadRunACPHelperScenario function reads and updates counterFile without
synchronization causing a race when multiple helper processes run; wrap the
read-modify-write sequence for counterFile in an exclusive file lock (create the
file if missing), e.g., open counterFile with os.OpenFile and obtain an advisory
flock (or use a small locking library) before reading, parsing (strconv.Atoi),
computing next, and writing the new value, release the lock after write, and
handle timeouts/errors so concurrent helpers serialize access to counterFile.
```

</details>

<!-- fingerprinting:phantom:medusa:ocelot:cbb17a11-3383-4bd3-8d87-acbc7d74f5bd -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `invalid`
- Rationale: The counter file is only used when multiple helper scenario sets are configured. In the current test suite that path is exercised by sequential retry scenarios, while the concurrent worker test uses a single scenario set and never touches the counter file. There is no current concurrent read-modify-write path to fix in this batch.
