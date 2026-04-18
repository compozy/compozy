---
status: resolved
file: internal/cli/state.go
line: 367
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56T0iP,comment:PRRC_kwDORy7nkc624f7i
---

# Issue 005: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Use `s.isInteractive` callback for consistency with the testability pattern.**

Line 358 calls the global `isInteractiveTerminal()` directly, bypassing the `s.isInteractive` callback that's wired for testability. This inconsistency could cause tests that mock `isInteractive` to behave unexpectedly.


<details>
<summary>🐛 Proposed fix</summary>

```diff
+	isInteractive := s.isInteractive
+	if isInteractive == nil {
+		isInteractive = isInteractiveTerminal
+	}
+
-	if !isInteractiveTerminal() {
+	if !isInteractive() {
 		if s.tui && tuiExplicit {
 			return fmt.Errorf(
 				"%s requires an interactive terminal for tui mode; rerun with --tui=false",
 				cmd.CommandPath(),
 			)
 		}
 		s.tui = false
 		return nil
 	}
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
	isInteractive := s.isInteractive
	if isInteractive == nil {
		isInteractive = isInteractiveTerminal
	}

	if !isInteractive() {
		if s.tui && tuiExplicit {
			return fmt.Errorf(
				"%s requires an interactive terminal for tui mode; rerun with --tui=false",
				cmd.CommandPath(),
			)
		}
		s.tui = false
		return nil
	}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/state.go` around lines 358 - 367, Replace the direct call to
isInteractiveTerminal() with the instance callback s.isInteractive() so tests
can mock terminal behavior; specifically, in the conditional that checks
terminal interactivity (the block referencing isInteractiveTerminal(), s.tui,
tuiExplicit, and cmd.CommandPath()), call s.isInteractive() instead and preserve
the existing logic and error message when s.tui && tuiExplicit.
```

</details>

<!-- fingerprinting:phantom:medusa:ocelot:ebe35915-8a14-4a1b-9542-96c1f5a418c5 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - Root cause: `normalizePresentationMode` bypasses the injected `s.isInteractive` callback and calls `isInteractiveTerminal()` directly, which breaks the command-state testability seam.
  - Fix plan: resolve the callback once inside `normalizePresentationMode`, fall back to `isInteractiveTerminal` only when it is nil, and add regression coverage around the injected behavior.
  - Resolved: `internal/cli/state.go` now uses the injected callback consistently, with regression coverage in `internal/cli/root_command_execution_test.go`; verified with `make verify`.
