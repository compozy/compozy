---
status: resolved
file: internal/cli/daemon.go
line: 151
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc57_RAI,comment:PRRC_kwDORy7nkc65JPyT
---

# Issue 001: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Validate `--format` before the foreground early return.**

Right now `daemon start --foreground --format garbage` succeeds because Line 140 returns before `normalizeOperatorOutputFormat` runs. The detached path rejects the same bad value, so the command behaves inconsistently depending on mode.

<details>
<summary>🛠️ Proposed fix</summary>

```diff
 func (s *daemonStartState) run(cmd *cobra.Command, _ []string) error {
 	ctx, stop := signalCommandContext(cmd)
 	defer stop()

+	format, err := normalizeOperatorOutputFormat(s.outputFormat)
+	if err != nil {
+		return withExitCode(1, err)
+	}
+
 	if s.foreground || s.internalChild {
 		runOptions, err := cliDaemonRunOptionsFromEnv()
 		if err != nil {
 			return err
 		}
 		return runCLIDaemonForeground(ctx, runOptions)
 	}
-
-	format, err := normalizeOperatorOutputFormat(s.outputFormat)
-	if err != nil {
-		return withExitCode(1, err)
-	}
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
func (s *daemonStartState) run(cmd *cobra.Command, _ []string) error {
	ctx, stop := signalCommandContext(cmd)
	defer stop()

	format, err := normalizeOperatorOutputFormat(s.outputFormat)
	if err != nil {
		return withExitCode(1, err)
	}

	if s.foreground || s.internalChild {
		runOptions, err := cliDaemonRunOptionsFromEnv()
		if err != nil {
			return err
		}
		return runCLIDaemonForeground(ctx, runOptions)
	}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/daemon.go` around lines 136 - 151, The run method in
daemonStartState currently returns early for foreground/internalChild before
validating the output format, causing inconsistent behavior; call
normalizeOperatorOutputFormat(s.outputFormat) and handle/return any error before
the early-return branch (i.e., before checking s.foreground || s.internalChild)
so both foreground and detached paths validate the --format; keep subsequent
logic to obtain runOptions via cliDaemonRunOptionsFromEnv() and call
runCLIDaemonForeground(ctx, runOptions) unchanged after the validation.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:13e5f7e2-231d-4efa-b0c2-160e6a6e0bff -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: at the start of this batch, `daemonStartState.run` validated `s.outputFormat` only after the detached-start branch. When `--foreground` or the hidden `--internal-child` path was used, the function returned early through `runCLIDaemonForeground` and skipped `normalizeOperatorOutputFormat`, so invalid `--format` values were accepted in those modes.
- Resolution: moved the shared `normalizeOperatorOutputFormat(s.outputFormat)` call ahead of the early return in `internal/cli/daemon.go`, so detached, foreground, and internal-child execution all share the same validation path.
- Regression coverage: added `TestDaemonStartCommandRejectsInvalidFormatBeforeEarlyReturn` in `internal/cli/daemon_commands_test.go`, covering both `--foreground` and `--internal-child` with `--format garbage` and asserting exit code `1` before the foreground runner is invoked.
- Verification: `go test ./internal/cli -run 'TestDaemonStartCommand(ForegroundUsesDaemonRunner|RejectsInvalidFormatBeforeEarlyReturn)$' -count=1`; `make verify`.
