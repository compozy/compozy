# Issue 14 - Review Thread Comment

**File:** `sdk/compozy/constructor_clone_test.go:75`
**Date:** 2025-11-01 01:57:02 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_⚠️ Potential issue_ | _🟠 Major_

**Rename table-driven subtests to follow the “Should …” convention**

Lines 26-75 name the table-driven subtests “Workflow”, “Agent”, etc., but project guidelines require every `t.Run` label to start with “Should …”. These names will fail the convention check. Please rename each entry to the mandated format (e.g., “Should clone workflow configs when input is nil”).


As per coding guidelines.

```diff
-       {"Workflow", func(t *testing.T) {
+       {"Should clone workflow configs when input is nil", func(t *testing.T) {
…
-       {"Agent", func(t *testing.T) {
+       {"Should clone agent configs when input is nil", func(t *testing.T) {
…
-       {"Tool", func(t *testing.T) {
+       {"Should clone tool configs when input is nil", func(t *testing.T) {
```

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
		{"Should clone workflow configs when input is nil", func(t *testing.T) {
			clones, err := cloneWorkflowConfigs(nil)
			require.NoError(t, err)
			assert.Empty(t, clones)
		}},
		{"Should clone agent configs when input is nil", func(t *testing.T) {
			clones, err := cloneAgentConfigs(nil)
			require.NoError(t, err)
			assert.Empty(t, clones)
		}},
		{"Should clone tool configs when input is nil", func(t *testing.T) {
			clones, err := cloneToolConfigs(nil)
			require.NoError(t, err)
			assert.Empty(t, clones)
		}},
		{"Should clone knowledge configs when input is nil", func(t *testing.T) {
			clones, err := cloneKnowledgeConfigs(nil)
			require.NoError(t, err)
			assert.Empty(t, clones)
		}},
		{"Should clone memory configs when input is nil", func(t *testing.T) {
			clones, err := cloneMemoryConfigs(nil)
			require.NoError(t, err)
			assert.Empty(t, clones)
		}},
		{"Should clone MCP configs when input is nil", func(t *testing.T) {
			clones, err := cloneMCPConfigs(nil)
			require.NoError(t, err)
			assert.Empty(t, clones)
		}},
		{"Should clone schema configs when input is nil", func(t *testing.T) {
			clones, err := cloneSchemaConfigs(nil)
			require.NoError(t, err)
			assert.Empty(t, clones)
		}},
		{"Should clone model configs when input is nil", func(t *testing.T) {
			clones, err := cloneModelConfigs(nil)
			require.NoError(t, err)
			assert.Empty(t, clones)
		}},
		{"Should clone schedule configs when input is nil", func(t *testing.T) {
			clones, err := cloneScheduleConfigs(nil)
			require.NoError(t, err)
			assert.Empty(t, clones)
		}},
		{"Should clone webhook configs when input is nil", func(t *testing.T) {
			clones, err := cloneWebhookConfigs(nil)
			require.NoError(t, err)
			assert.Empty(t, clones)
		}},
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
In sdk/compozy/constructor_clone_test.go around lines 26 to 75 the table-driven
subtest names ("Workflow", "Agent", "Tool", etc.) do not follow the project
convention requiring labels to start with "Should ..."; update each t.Run entry
label to a descriptive "Should ..." sentence (for example "Should clone workflow
configs when input is nil") that begins with "Should" and describes the behavior
being tested, leaving the test bodies unchanged.
```

</details>

<!-- fingerprinting:phantom:medusa:sabertoothed -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5gLa2k`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5gLa2k
```

---
*Generated from PR review - CodeRabbit AI*
