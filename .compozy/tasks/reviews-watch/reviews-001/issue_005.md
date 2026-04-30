---
provider: coderabbit
pr: "133"
round: 1
round_created_at: 2026-04-30T20:37:59.817595Z
status: resolved
file: internal/core/provider/provider_test.go
line: 36
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-22D1,comment:PRRC_kwDORy7nkc68_V6H
---

# Issue 005: _🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_
## Review Comment

_🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_

**Wrap this case in `t.Run("Should ...")` to match the enforced test pattern.**

<details>
<summary>Suggested diff</summary>

```diff
 func TestFetchWatchStatusReturnsUnsupportedError(t *testing.T) {
 	t.Parallel()
-
-	status, err := FetchWatchStatus(context.Background(), unsupportedWatchProvider{}, WatchStatusRequest{PR: "259"})
-	if err == nil {
-		t.Fatal("expected unsupported watch-status error")
-	}
-	if !errors.Is(err, ErrWatchStatusUnsupported) {
-		t.Fatalf("expected ErrWatchStatusUnsupported, got %v", err)
-	}
-	if status.State != WatchStatusUnsupported {
-		t.Fatalf("status state = %q, want %q", status.State, WatchStatusUnsupported)
-	}
+	t.Run("Should return unsupported status when provider lacks watch capability", func(t *testing.T) {
+		status, err := FetchWatchStatus(context.Background(), unsupportedWatchProvider{}, WatchStatusRequest{PR: "259"})
+		if err == nil {
+			t.Fatal("expected unsupported watch-status error")
+		}
+		if !errors.Is(err, ErrWatchStatusUnsupported) {
+			t.Fatalf("expected ErrWatchStatusUnsupported, got %v", err)
+		}
+		if status.State != WatchStatusUnsupported {
+			t.Fatalf("status state = %q, want %q", status.State, WatchStatusUnsupported)
+		}
+	})
 }
```
</details>

As per coding guidelines, `MUST use t.Run("Should...") pattern for ALL test cases`.

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
func TestFetchWatchStatusReturnsUnsupportedError(t *testing.T) {
	t.Parallel()
	t.Run("Should return unsupported status when provider lacks watch capability", func(t *testing.T) {
		status, err := FetchWatchStatus(context.Background(), unsupportedWatchProvider{}, WatchStatusRequest{PR: "259"})
		if err == nil {
			t.Fatal("expected unsupported watch-status error")
		}
		if !errors.Is(err, ErrWatchStatusUnsupported) {
			t.Fatalf("expected ErrWatchStatusUnsupported, got %v", err)
		}
		if status.State != WatchStatusUnsupported {
			t.Fatalf("status state = %q, want %q", status.State, WatchStatusUnsupported)
		}
	})
}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/provider/provider_test.go` around lines 23 - 36, Wrap the
existing test body of TestFetchWatchStatusReturnsUnsupportedError in a t.Run
subtest with a descriptive name (e.g., "Should return unsupported error") and
move t.Parallel() into that subtest; specifically, inside
TestFetchWatchStatusReturnsUnsupportedError call t.Run("Should ...", func(t
*testing.T) { t.Parallel(); /* existing assertions using FetchWatchStatus,
unsupportedWatchProvider{}, WatchStatusRequest{PR: "259"} and checks against
ErrWatchStatusUnsupported and WatchStatusUnsupported */ }) so the test follows
the enforced t.Run("Should...") pattern while still running in parallel.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:ed821098-705a-4bc3-acaf-ab448a3674f2 -->

<!-- d98c2f50 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: The unsupported watch-status test does not use the required `t.Run("Should...")` structure for the scenario it verifies.
- Fix plan: Move the existing assertions into a `Should ...` subtest and keep parallel execution on the scenario itself.
