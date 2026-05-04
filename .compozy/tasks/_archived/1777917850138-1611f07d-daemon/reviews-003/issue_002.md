---
status: resolved
file: internal/cli/daemon_commands.go
line: 319
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc57_RAH,comment:PRRC_kwDORy7nkc65JPyR
---

# Issue 002: _⚠️ Potential issue_ | _🔴 Critical_
## Review Comment

_⚠️ Potential issue_ | _🔴 Critical_

**Interactive collection now blocks valid non-interactive `tasks run <slug>` calls.**

At Line 316, interactive collection runs before positional slug resolution (Line 319). With zero flags, `maybeCollectInteractiveParams` can fail in non-TTY even when a positional slug is already provided, so `compozy tasks run my-feature` may fail unexpectedly.


<details>
<summary>🐛 Proposed fix</summary>

```diff
- if err := s.maybeCollectInteractiveParams(cmd); err != nil {
- 	return err
- }
+ if len(args) == 0 && strings.TrimSpace(s.name) == "" {
+ 	if err := s.maybeCollectInteractiveParams(cmd); err != nil {
+ 		return err
+ 	}
+ }
  if err := s.resolveTaskWorkflowName(args); err != nil {
  	return withExitCode(1, err)
  }
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/daemon_commands.go` around lines 316 - 319, The interactive
prompt is being invoked before resolving a positional task slug, causing
non-interactive "tasks run <slug>" to fail; modify the call order so
s.resolveTaskWorkflowName(args) runs before s.maybeCollectInteractiveParams(cmd)
(or alternatively have maybeCollectInteractiveParams early-return when a
positional slug is present), ensuring resolveTaskWorkflowName is used to detect
an explicit slug and only then fall back to interactive collection via
maybeCollectInteractiveParams.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:ea86bdc4-4f01-49d9-95c4-0503e731ccf2 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `internal/cli/daemon_commands.go` called `maybeCollectInteractiveParams(cmd)` before `resolveTaskWorkflowName(args)` inside `runTaskWorkflow`. `maybeCollectInteractiveParams` only checked whether any flags were present, so non-interactive `compozy tasks run demo` was treated like a zero-input form flow and failed before the explicit positional slug was applied.
- Fix approach: keep the generic interactive helper untouched and fix the daemon task-run path at the call site by only invoking interactive collection when neither a positional slug nor `--name` has already selected the workflow.
- Resolution: `runTaskWorkflow` now falls back to `maybeCollectInteractiveParams` only when both `args` and `s.name` are empty, then resolves the workflow slug before dispatching the daemon task-run request.
- Regression coverage: `TestTasksRunCommandPositionalSlugSkipsInteractiveFormWithoutTTY` in `internal/cli/root_command_execution_test.go` proves non-interactive `tasks run demo` succeeds without invoking the interactive form path.
- Verification: `go test ./internal/cli -run 'TestTasksRunCommand(PositionalSlugSkipsInteractiveFormWithoutTTY|NoFlagsUsesInteractiveForm|AutoModeResolvesToStreamInNonInteractiveExecution)$' -count=1` passed after the final artifact updates. `make verify` also passed immediately after the in-scope code/test fix landed with `2394` tests, `1` skipped helper-process test, and a successful `go build ./cmd/compozy`.
- Verification note: a later `make verify` rerun failed outside this batch in `internal/core/run/ui TestAttachRemoteWithRealSetupHydratesCompletedSnapshotIntoController`; `git diff -- internal/core/run/ui/remote_test.go` shows that failure comes from a concurrent, unrelated addition in `internal/core/run/ui/remote_test.go`, so no further in-scope code change was required for `issue_002`.
