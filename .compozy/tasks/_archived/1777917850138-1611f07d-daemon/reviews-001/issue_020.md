---
status: resolved
file: internal/cli/daemon.go
line: 54
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc579mnC,comment:PRRC_kwDORy7nkc65HKYO
---

# Issue 020: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
cat -n internal/cli/daemon.go | head -70
```

Repository: compozy/compozy

Length of output: 2164

---

🏁 Script executed:

```shell
rg "RunE:" internal/cli/ -A 3 | head -50
```

Repository: compozy/compozy

Length of output: 2780

---

🏁 Script executed:

```shell
rg "signalCommandContext" internal/cli/ -A 5 -B 2
```

Repository: compozy/compozy

Length of output: 16222

---

🏁 Script executed:

```shell
cat -n internal/cli/command_context.go
```

Repository: compozy/compozy

Length of output: 537

---

🏁 Script executed:

```shell
grep -r "signal.NotifyContext(context.Background()" internal/cli/
```

Repository: compozy/compozy

Length of output: 171

---



</details>

**Use the `signalCommandContext` helper to properly derive context from the command.**

The daemon start command is the only CLI command not using the established `signalCommandContext(cmd)` helper, which safely checks if `cmd.Context()` is available before falling back to `context.Background()`. Starting from `context.Background()` drops any cancellation or deadline already attached by Cobra, preventing embedded invocations and tests from cleanly stopping the daemon.

<details>
<summary>Suggested change</summary>

```diff
-		RunE: func(_ *cobra.Command, _ []string) error {
-			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
+		RunE: func(cmd *cobra.Command, _ []string) error {
+			ctx, stop := signalCommandContext(cmd)
 			defer stop()
 
 			return daemon.Run(ctx, daemon.RunOptions{
 				Version: version.String(),
 			})
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signalCommandContext(cmd)
			defer stop()

			return daemon.Run(ctx, daemon.RunOptions{
				Version: version.String(),
			})
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/daemon.go` around lines 48 - 54, The RunE handler currently
creates a context with signal.NotifyContext(context.Background(), ...), losing
any cancellation/deadline from Cobra; replace that logic by calling the existing
helper signalCommandContext(cmd) to derive the context (i.e. use ctx :=
signalCommandContext(cmd) and defer any stop returned by that helper if
applicable) and pass that ctx into daemon.Run in RunE so the command respects
injected contexts and tests; remove the manual signal.NotifyContext usage and
keep the daemon.Run invocation and RunOptions.Version as-is.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:c36ff8f6-d8b2-44c7-a1cd-4e6f4795be06 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Root cause: the daemon start command creates its own signal context from `context.Background()` and ignores any cancellation/deadline already attached to the Cobra command.
- Fix plan: switch it to `signalCommandContext(cmd)` so embedded invocations and tests can control daemon lifetime through the command context.
- Resolution: Implemented and verified with `make verify`.
