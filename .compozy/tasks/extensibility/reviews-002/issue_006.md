---
status: resolved
file: internal/core/extension/dispatcher_test.go
line: 92
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56Rl8r,comment:PRRC_kwDORy7nkc621sgq
---

# Issue 006: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Add timeouts around these channel receives.**

If `DispatchMutable()` regresses before invoking one of the handlers, Lines 87-92 block forever and hang the package instead of failing fast.

<details>
<summary>🧪 Suggested change</summary>

```diff
-	if got := <-firstSeen; got != "base" {
-		t.Fatalf("first extension saw %q, want %q", got, "base")
-	}
-	if got := <-secondSeen; got != "base-one" {
-		t.Fatalf("second extension saw %q, want %q", got, "base-one")
-	}
+	select {
+	case got := <-firstSeen:
+		if got != "base" {
+			t.Fatalf("first extension saw %q, want %q", got, "base")
+		}
+	case <-time.After(time.Second):
+		t.Fatal("first extension was not called")
+	}
+	select {
+	case got := <-secondSeen:
+		if got != "base-one" {
+			t.Fatalf("second extension saw %q, want %q", got, "base-one")
+		}
+	case <-time.After(time.Second):
+		t.Fatal("second extension was not called")
+	}
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
	select {
	case got := <-firstSeen:
		if got != "base" {
			t.Fatalf("first extension saw %q, want %q", got, "base")
		}
	case <-time.After(time.Second):
		t.Fatal("first extension was not called")
	}
	select {
	case got := <-secondSeen:
		if got != "base-one" {
			t.Fatalf("second extension saw %q, want %q", got, "base-one")
		}
	case <-time.After(time.Second):
		t.Fatal("second extension was not called")
	}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/extension/dispatcher_test.go` around lines 87 - 92, The test
blocks indefinitely if DispatchMutable() fails to call handlers; change the two
blind channel receives on firstSeen and secondSeen to receive with a timeout
(e.g., using select and time.After or a context deadline) so the test fails
fast; specifically modify the receives that currently read from firstSeen and
secondSeen (after DispatchMutable and handler setup) to select between the
channel and a timeout case that calls t.Fatalf with a clear message indicating
which handler did not run.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:c1d29a9a-29d8-4bec-b3df-52129c8adbe5 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - The two direct receives after `DispatchMutable()` can block the package indefinitely if a regression prevents either hook handler from running.
  - I will replace those blind receives with bounded waits so the test fails quickly with a precise missing-handler message instead of hanging.
  - Resolved by routing both receives through a bounded helper that fails after one second with a handler-specific message.
