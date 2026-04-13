---
status: resolved
file: internal/core/extension/dispatcher_test.go
line: 302
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56RVPB,comment:PRRC_kwDORy7nkc621Va2
---

# Issue 022: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Replace the 50ms wall-clock check with synchronization.**

This assertion is scheduler-dependent and will intermittently fail under `go test -race` or slower CI machines. A done channel plus `select` proves `DispatchObserver()` returns before `releaseSlow` is closed without relying on a timing threshold.



<details>
<summary>Suggested fix</summary>

```diff
-	startedAt := time.Now()
-	dispatcher.DispatchObserver(context.Background(), HookAgentPostSessionCreate, map[string]any{
-		"session_id": "sess-1",
-	})
-	elapsed := time.Since(startedAt)
-	if elapsed > 50*time.Millisecond {
-		t.Fatalf("DispatchObserver() blocked for %v, want fast return", elapsed)
-	}
+	returned := make(chan struct{})
+	go func() {
+		dispatcher.DispatchObserver(context.Background(), HookAgentPostSessionCreate, map[string]any{
+			"session_id": "sess-1",
+		})
+		close(returned)
+	}()
+	select {
+	case <-returned:
+	case <-time.After(time.Second):
+		t.Fatal("DispatchObserver() blocked waiting for observers")
+	}
```
</details>


As per coding guidelines: `**/*_test.go`: Run tests with `-race` flag; the race detector must pass before committing.

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
	returned := make(chan struct{})
	go func() {
		dispatcher.DispatchObserver(context.Background(), HookAgentPostSessionCreate, map[string]any{
			"session_id": "sess-1",
		})
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("DispatchObserver() blocked waiting for observers")
	}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/extension/dispatcher_test.go` around lines 295 - 302, Replace
the wall-clock assertion with channel synchronization: launch
DispatchObserver(context.Background(), HookAgentPostSessionCreate, ...) in a
goroutine that closes a done channel when it returns, then use select to assert
done is received before you close/release the test's releaseSlow channel (or
before a reasonable global test timeout); this verifies DispatchObserver
returned without relying on a 50ms sleep. Reference DispatchObserver and
HookAgentPostSessionCreate to locate where to add the done channel and select
logic.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:388c7721-ae35-4ddb-a0e7-78fbd9aa7a58 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes: The current assertion depends on `DispatchObserver` returning within an arbitrary 50ms wall-clock budget, which is scheduler-sensitive and can flake under `-race` or slower CI machines. A synchronization-based assertion is the correct way to prove the method returns without waiting for the slow observer to finish.
