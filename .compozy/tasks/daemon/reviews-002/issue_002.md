---
status: resolved
file: cmd/compozy/main_test.go
line: 95
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc579yx9,comment:PRRC_kwDORy7nkc65HZWN
---

# Issue 002: _🛠️ Refactor suggestion_ | _🟠 Major_
## Review Comment

_🛠️ Refactor suggestion_ | _🟠 Major_

**Use `Should...` subtest names and add the missing `__completeNoDesc` case.**

This table is solid, but it misses one handled branch and doesn’t follow the required subtest naming convention.

<details>
<summary>Proposed test-case update</summary>

```diff
-		{name: "no args", args: nil, want: false},
-		{name: "help flag", args: []string{"--help"}, want: false},
-		{name: "nested help flag", args: []string{"tasks", "run", "--help"}, want: false},
-		{name: "nested help command", args: []string{"tasks", "help"}, want: false},
-		{name: "version flag", args: []string{"--version"}, want: false},
-		{name: "help command", args: []string{"help"}, want: false},
-		{name: "version command", args: []string{"version"}, want: false},
-		{name: "completion command", args: []string{"completion", "bash"}, want: false},
-		{name: "shell completion probe", args: []string{"__complete", "tasks"}, want: false},
-		{name: "workflow command", args: []string{"tasks", "run", "daemon"}, want: true},
+		{name: "Should skip update check when no args are provided", args: nil, want: false},
+		{name: "Should skip update check for help flag", args: []string{"--help"}, want: false},
+		{name: "Should skip update check for nested help flag", args: []string{"tasks", "run", "--help"}, want: false},
+		{name: "Should skip update check for nested help command", args: []string{"tasks", "help"}, want: false},
+		{name: "Should skip update check for version flag", args: []string{"--version"}, want: false},
+		{name: "Should skip update check for help command", args: []string{"help"}, want: false},
+		{name: "Should skip update check for version command", args: []string{"version"}, want: false},
+		{name: "Should skip update check for completion command", args: []string{"completion", "bash"}, want: false},
+		{name: "Should skip update check for shell completion probe", args: []string{"__complete", "tasks"}, want: false},
+		{name: "Should skip update check for shell completion probe without descriptions", args: []string{"__completeNoDesc", "tasks"}, want: false},
+		{name: "Should start update check for workflow command", args: []string{"tasks", "run", "daemon"}, want: true},
```
</details>


As per coding guidelines, `**/*_test.go`: `MUST use t.Run("Should...") pattern for ALL test cases`.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@cmd/compozy/main_test.go` around lines 85 - 95, Rename each test case name in
the table to follow the "Should..." subtest naming convention (these are the
entries in the tests slice used by t.Run(tc.name, ...)), and add the missing
completion probe case with args []string{"__completeNoDesc", "tasks"} (expecting
false) so the branch is covered; leave the rest of the table and the t.Run usage
unchanged.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:00ea1a22-8baf-4c76-9cdf-1f9bb20d4779 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `TestShouldStartUpdateCheck` is missing the `__completeNoDesc` branch that `shouldStartUpdateCheck()` handles, and its subtest names do not follow the repository `Should...` convention required for table-driven subtests.
- Fix plan: rename each table entry to `Should...` and add the missing `__completeNoDesc` case with `want: false`.
- Resolution: subtest names now follow the `Should...` convention and the missing `__completeNoDesc` completion-probe case is covered in `cmd/compozy/main_test.go`.
