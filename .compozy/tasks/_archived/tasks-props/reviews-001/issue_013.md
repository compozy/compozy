---
status: resolved
file: sdk/extension/compat_test.go
line: 38
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc57ypzi,comment:PRRC_kwDORy7nkc644MtP
---

# Issue 013: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Use `Should...` naming for the new subtest cases.**

These added case names feed directly into `t.Run(tc.name, ...)`, so they should follow the mandated `Should...` pattern.

<details>
<summary>Suggested rename</summary>

```diff
-		{name: "TaskRuntime", public: extension.TaskRuntime{}, runtime: model.TaskRuntime{}},
-		{name: "TaskRuntimeTask", public: extension.TaskRuntimeTask{}, runtime: model.TaskRuntimeTask{}},
+		{name: "ShouldAlignTaskRuntimeFields", public: extension.TaskRuntime{}, runtime: model.TaskRuntime{}},
+		{name: "ShouldAlignTaskRuntimeTaskFields", public: extension.TaskRuntimeTask{}, runtime: model.TaskRuntimeTask{}},
```
</details>

  
As per coding guidelines, "MUST use t.Run("Should...") pattern for ALL test cases".

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
		{name: "ShouldAlignTaskRuntimeFields", public: extension.TaskRuntime{}, runtime: model.TaskRuntime{}},
		{name: "ShouldAlignTaskRuntimeTaskFields", public: extension.TaskRuntimeTask{}, runtime: model.TaskRuntimeTask{}},
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@sdk/extension/compat_test.go` around lines 37 - 38, Rename the test case name
fields that feed into t.Run to follow the mandated "Should..." pattern: update
the case with public: extension.TaskRuntime{} (currently name "TaskRuntime") to
a "Should..." description (e.g., "Should retain TaskRuntime compatibility") and
update the case with public: extension.TaskRuntimeTask{} (currently name
"TaskRuntimeTask") to a "Should..." description (e.g., "Should retain
TaskRuntimeTask compatibility"); these name values are the tc.name passed into
t.Run(tc.name, ...) so ensure the name fields in the test cases slice are
changed accordingly.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:975d3017-616b-4f3b-8110-41c2ebb5f770 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - Confirmed by inspection. The new `TaskRuntime` and `TaskRuntimeTask` case names are passed directly to `t.Run(tc.name, ...)` and do not follow the repository's `Should...` naming convention.
  - Root cause: the new compatibility cases used bare type names instead of behavior-oriented subtest labels.
  - Intended fix: rename the two new case labels to `Should...` descriptions without changing the compatibility assertions themselves.
  - Resolution: the new compatibility cases now use `Should...` subtest labels.
