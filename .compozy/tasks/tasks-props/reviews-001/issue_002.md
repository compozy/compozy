---
status: resolved
file: internal/cli/root_test.go
line: 673
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc57ypy0,comment:PRRC_kwDORy7nkc644MsS
---

# Issue 002: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Wrap this new test case in a `t.Run("Should...")` subtest to meet repo test policy.**

The assertions are good, but the newly added case should follow the required subtest pattern.

<details>
<summary>Proposed refactor</summary>

```diff
 func TestAddCommonFlagsUseResilientRetryDefaults(t *testing.T) {
 	t.Parallel()
-
-	state := newCommandState(commandKindStart, core.ModePRDTasks)
-	cmd := newTestCommand(state)
-
-	if got := state.maxRetries; got != defaultMaxRetries {
-		t.Fatalf("unexpected max-retries default on command state: got %d want %d", got, defaultMaxRetries)
-	}
-	flag := cmd.Flags().Lookup("max-retries")
-	if flag == nil {
-		t.Fatal("expected max-retries flag to be registered")
-	}
-	if got, want := flag.DefValue, strconv.Itoa(defaultMaxRetries); got != want {
-		t.Fatalf("unexpected max-retries flag default: got %q want %q", got, want)
-	}
+	t.Run("Should use resilient max-retries defaults", func(t *testing.T) {
+		t.Parallel()
+
+		state := newCommandState(commandKindStart, core.ModePRDTasks)
+		cmd := newTestCommand(state)
+
+		if got := state.maxRetries; got != defaultMaxRetries {
+			t.Fatalf("unexpected max-retries default on command state: got %d want %d", got, defaultMaxRetries)
+		}
+		flag := cmd.Flags().Lookup("max-retries")
+		if flag == nil {
+			t.Fatal("expected max-retries flag to be registered")
+		}
+		if got, want := flag.DefValue, strconv.Itoa(defaultMaxRetries); got != want {
+			t.Fatalf("unexpected max-retries flag default: got %q want %q", got, want)
+		}
+	})
 }
```
</details>


As per coding guidelines, `**/*_test.go`: `MUST use t.Run("Should...") pattern for ALL test cases`.

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
func TestAddCommonFlagsUseResilientRetryDefaults(t *testing.T) {
	t.Parallel()
	t.Run("Should use resilient max-retries defaults", func(t *testing.T) {
		t.Parallel()

		state := newCommandState(commandKindStart, core.ModePRDTasks)
		cmd := newTestCommand(state)

		if got := state.maxRetries; got != defaultMaxRetries {
			t.Fatalf("unexpected max-retries default on command state: got %d want %d", got, defaultMaxRetries)
		}
		flag := cmd.Flags().Lookup("max-retries")
		if flag == nil {
			t.Fatal("expected max-retries flag to be registered")
		}
		if got, want := flag.DefValue, strconv.Itoa(defaultMaxRetries); got != want {
			t.Fatalf("unexpected max-retries flag default: got %q want %q", got, want)
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

In `@internal/cli/root_test.go` around lines 657 - 673, Wrap the existing
TestAddCommonFlagsUseResilientRetryDefaults body in a t.Run subtest with a
"Should ..." name (e.g., t.Run("Should use resilient retry defaults", func(t
*testing.T) { ... })), move the t.Parallel() call into that subtest, and keep
all existing assertions and lookups (state := newCommandState, cmd :=
newTestCommand, checks for state.maxRetries and flag.DefValue) unchanged inside
the subtest so the test complies with the t.Run("Should...") pattern.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:ca55a74b-5b44-4aac-929e-af575e1e15e0 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - Confirmed by inspection. `TestAddCommonFlagsUseResilientRetryDefaults` is a single top-level body without the repository's required `t.Run("Should...")` case wrapper.
  - Root cause: the newly added retry-default regression test skipped the local test naming/subtest convention used elsewhere in the package.
  - Intended fix: wrap the assertions in a `Should...` subtest while preserving the current coverage of the default flag value and command state.
  - Resolution: the retry-default test now uses a `Should...` subtest and continues to validate the registered `--max-retries` default.
