---
status: resolved
file: internal/core/run/exec/exec_test.go
line: 598
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56T0ib,comment:PRRC_kwDORy7nkc624f7v
---

# Issue 020: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Use `t.Run("Should...")` for these new test cases.**

These new tests are single-case top-level tests; this file’s test policy requires explicit `t.Run("Should...")` case blocks for each scenario.

<details>
<summary>♻️ Suggested structure</summary>

```diff
 func TestApplyExecPromptPostBuildHookMutatesPrompt(t *testing.T) {
 	t.Parallel()
-
-	manager := &execHookManager{
-		dispatchMutable: func(_ context.Context, hook string, payload any) (any, error) {
-			...
-		},
-	}
-	state := &execRunState{ ... }
-
-	got, err := applyExecPromptPostBuildHook(context.Background(), state, "decorate me")
-	if err != nil {
-		t.Fatalf("applyExecPromptPostBuildHook: %v", err)
-	}
-	if got != "decorate me\n\nDecorated by exec hook." {
-		t.Fatalf("unexpected prompt mutation: %q", got)
-	}
+	t.Run("Should mutate prompt via prompt.post_build hook", func(t *testing.T) {
+		manager := &execHookManager{
+			dispatchMutable: func(_ context.Context, hook string, payload any) (any, error) {
+				...
+			},
+		}
+		state := &execRunState{ ... }
+		got, err := applyExecPromptPostBuildHook(context.Background(), state, "decorate me")
+		if err != nil {
+			t.Fatalf("applyExecPromptPostBuildHook: %v", err)
+		}
+		if got != "decorate me\n\nDecorated by exec hook." {
+			t.Fatalf("unexpected prompt mutation: %q", got)
+		}
+	})
 }
```
</details>
As per coding guidelines: `**/*_test.go`: MUST use t.Run("Should...") pattern for ALL test cases.

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
func TestApplyExecPromptPostBuildHookMutatesPrompt(t *testing.T) {
	t.Parallel()

	t.Run("Should mutate prompt via prompt.post_build hook", func(t *testing.T) {
		manager := &execHookManager{
			dispatchMutable: func(_ context.Context, hook string, payload any) (any, error) {
				if hook != "prompt.post_build" {
					t.Fatalf("unexpected mutable hook %q", hook)
				}

				current, ok := payload.(execPromptPostBuildPayload)
				if !ok {
					t.Fatalf("payload type = %T, want execPromptPostBuildPayload", payload)
				}
				current.PromptText += "\n\nDecorated by exec hook."
				return current, nil
			},
		}
		state := &execRunState{
			ctx:            context.Background(),
			runArtifacts:   model.NewRunArtifacts(t.TempDir(), "exec-hook-run"),
			runtimeManager: manager,
		}

		got, err := applyExecPromptPostBuildHook(context.Background(), state, "decorate me")
		if err != nil {
			t.Fatalf("applyExecPromptPostBuildHook: %v", err)
		}
		if got != "decorate me\n\nDecorated by exec hook." {
			t.Fatalf("unexpected prompt mutation: %q", got)
		}
	})
}

func TestExecRunStateDispatchesRunHooks(t *testing.T) {
	t.Run("Should dispatch run hooks and mutate config", func(t *testing.T) {
		manager := &execHookManager{
			dispatchMutable: func(_ context.Context, hook string, payload any) (any, error) {
				if hook != "run.pre_start" {
					return payload, nil
				}

				current, ok := payload.(execRunPreStartPayload)
				if !ok {
					t.Fatalf("payload type = %T, want execRunPreStartPayload", payload)
				}
				current.Config.Model = "gpt-5.4-mini"
				return current, nil
			},
		}
		cfg := &model.RuntimeConfig{
			WorkspaceRoot: workspaceRootForExecTest(t),
			IDE:           model.IDECodex,
			Model:         "gpt-5.4",
			AccessMode:    model.AccessModeDefault,
		}
		state := &execRunState{
			ctx:            context.Background(),
			record:         PersistedExecRun{UpdatedAt: time.Now().UTC()},
			runArtifacts:   model.NewRunArtifacts(t.TempDir(), "exec-run-hooks"),
			runtimeManager: manager,
		}

		if err := applyExecRunPreStartHook(context.Background(), state, cfg); err != nil {
			t.Fatalf("applyExecRunPreStartHook: %v", err)
		}
		if cfg.Model != "gpt-5.4-mini" {
			t.Fatalf("expected run.pre_start to mutate model, got %q", cfg.Model)
		}

		if err := state.writeStarted(cfg); err != nil {
			t.Fatalf("writeStarted: %v", err)
		}
		if err := state.completeTurn(execExecutionResult{
			status: runStatusSucceeded,
			output: "done",
		}); err != nil {
			t.Fatalf("completeTurn: %v", err)
		}

		if got := len(manager.observerPayloads["run.post_start"]); got != 1 {
			t.Fatalf("expected one run.post_start payload, got %d", got)
		}
		if got := len(manager.observerPayloads["run.pre_shutdown"]); got != 1 {
			t.Fatalf("expected one run.pre_shutdown payload, got %d", got)
		}
		payloads := manager.observerPayloads["run.post_shutdown"]
		if len(payloads) != 1 {
			t.Fatalf("expected one run.post_shutdown payload, got %d", len(payloads))
		}
		payload, ok := payloads[0].(execRunPostShutdownPayload)
		if !ok {
			t.Fatalf("payload type = %T, want execRunPostShutdownPayload", payloads[0])
		}
		if payload.Summary.Status != runStatusSucceeded || payload.Summary.JobsSucceeded != 1 {
			t.Fatalf("unexpected run.post_shutdown summary: %#v", payload.Summary)
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

In `@internal/core/run/exec/exec_test.go` around lines 504 - 598, Wrap each
top-level test body in a descriptive t.Run subtest; specifically, in
TestApplyExecPromptPostBuildHookMutatesPrompt and
TestExecRunStateDispatchesRunHooks move the existing test logic into
t.Run("Should ...", func(t *testing.T) { ... }) blocks (call t.Parallel() inside
the subtest if parallelism is desired) so the file follows the required
t.Run("Should...") pattern while keeping the existing assertions and references
to execHookManager, applyExecPromptPostBuildHook, applyExecRunPreStartHook,
state.writeStarted, and state.completeTurn unchanged.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:03d7857f-d529-43ca-be71-43278e85f981 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - Confirmed by inspection. The two newly added top-level tests in `exec_test.go` do not follow the repository’s required `t.Run("Should...")` pattern used elsewhere in this codebase.
  - Root cause: the test bodies were added directly at top level instead of being wrapped in descriptive scenario subtests.
  - Intended fix: wrap each scenario in a `t.Run("Should ...")` block while keeping the current assertions intact.
  - Resolution: wrapped both exec hook tests in descriptive `t.Run("Should ...")` subtests.
