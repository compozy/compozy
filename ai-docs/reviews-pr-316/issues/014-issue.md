# Issue 14 - Review Thread Comment

**File:** `sdk/compozy/registration_errors_test.go:455`
**Date:** 2025-11-01 12:25:24 America/Sao_Paulo
**Status:** - [x] RESOLVED ✓

## Resolution

- Wrapped duplicate registration assertions in a `t.Run` subtest to conform with suite conventions and enable parallel execution.

## Body

_⚠️ Potential issue_ | _🟠 Major_

**Adopt Should-style subtest for duplicate detection**

Per the test conventions, even single-scenario tests must run inside a `t.Run("Should ...")` block. Please wrap this logic accordingly so the suite honours the enforced structure. As per coding guidelines.

```diff
 func TestRegisterResourceDuplicateDetection(t *testing.T) {
-	ctx := lifecycleTestContext(t)
-	engine := &Engine{ctx: ctx, resourceStore: newResourceStoreStub()}
-	require.NoError(t, engine.registerProject(&engineproject.Config{Name: "dup"}, registrationSourceProgrammatic))
-	// ... snip ...
-	assert.Error(t, engine.registerWebhook(&enginewebhook.Config{Slug: "hook"}, registrationSourceProgrammatic))
+	t.Run("Should detect duplicate registrations across resources", func(t *testing.T) {
+		ctx := lifecycleTestContext(t)
+		engine := &Engine{ctx: ctx, resourceStore: newResourceStoreStub()}
+		require.NoError(t, engine.registerProject(&engineproject.Config{Name: "dup"}, registrationSourceProgrammatic))
+		require.NoError(t, engine.registerWorkflow(&engineworkflow.Config{ID: "wf"}, registrationSourceProgrammatic))
+		require.NoError(t, engine.registerTool(&enginetool.Config{ID: "tool"}, registrationSourceProgrammatic))
+		require.NoError(t, engine.registerKnowledge(&engineknowledge.BaseConfig{ID: "kb"}, registrationSourceProgrammatic))
+		require.NoError(t, engine.registerMemory(&enginememory.Config{ID: "mem"}, registrationSourceProgrammatic))
+		require.NoError(t, engine.registerMCP(&enginemcp.Config{ID: "mcp"}, registrationSourceProgrammatic))
+		schema := engineschema.Schema{"id": "schema-1", "type": "object"}
+		require.NoError(t, engine.registerSchema(&schema, registrationSourceProgrammatic))
+		require.NoError(
+			t,
+			engine.registerModel(
+				&enginecore.ProviderConfig{Provider: enginecore.ProviderName("openai"), Model: "gpt"},
+				registrationSourceProgrammatic,
+			),
+		)
+		require.NoError(t, engine.registerSchedule(&projectschedule.Config{ID: "schedule"}, registrationSourceProgrammatic))
+		require.NoError(t, engine.registerWebhook(&enginewebhook.Config{Slug: "hook"}, registrationSourceProgrammatic))
+		assert.Error(t, engine.registerProject(&engineproject.Config{Name: "dup"}, registrationSourceProgrammatic))
+		assert.Error(t, engine.registerWorkflow(&engineworkflow.Config{ID: "wf"}, registrationSourceProgrammatic))
+		assert.Error(t, engine.registerTool(&enginetool.Config{ID: "tool"}, registrationSourceProgrammatic))
+		assert.Error(t, engine.registerKnowledge(&engineknowledge.BaseConfig{ID: "kb"}, registrationSourceProgrammatic))
+		assert.Error(t, engine.registerMemory(&enginememory.Config{ID: "mem"}, registrationSourceProgrammatic))
+		assert.Error(t, engine.registerMCP(&enginemcp.Config{ID: "mcp"}, registrationSourceProgrammatic))
+		assert.Error(t, engine.registerSchema(&schema, registrationSourceProgrammatic))
+		assert.Error(
+			t,
+			engine.registerModel(
+				&enginecore.ProviderConfig{Provider: enginecore.ProviderName("openai"), Model: "gpt"},
+				registrationSourceProgrammatic,
+			),
+		)
+		assert.Error(t, engine.registerSchedule(&projectschedule.Config{ID: "schedule"}, registrationSourceProgrammatic))
+		assert.Error(t, engine.registerWebhook(&enginewebhook.Config{Slug: "hook"}, registrationSourceProgrammatic))
+	})
 }
```

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
In sdk/compozy/registration_errors_test.go around lines 420-455, the test body
must be wrapped in a Should-style subtest; wrap the entire setup and assertions
in a t.Run("Should detect duplicate registrations", func(t *testing.T) { ... })
block, moving the existing code into that closure (ensuring you use the inner t
for assertions to avoid shadowing) so the test follows the suite convention; no
other logic changes are needed.
```

</details>

<!-- fingerprinting:phantom:medusa:sabertoothed -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5gNDTH`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5gNDTH
```

---
*Generated from PR review - CodeRabbit AI*
