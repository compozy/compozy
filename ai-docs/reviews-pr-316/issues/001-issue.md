# Issue 1 - Review Thread Comment

**File:** `engine/llm/tool_registry_test.go:316`
**Date:** 2025-11-01 01:57:00 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_🧹 Nitpick_ | _🔵 Trivial_

**Consider splitting this 95-line test function.**

The function exceeds the 50-line guideline. Consider extracting each subtest into its own top-level test function for better isolation and readability.



Example refactor:

```diff
-func TestNativeToolAdapter(t *testing.T) {
-	buildConfig := func() *tool.Config {
-		return &tool.Config{
-			ID:             "native-tool",
-			Description:    "Native tool",
-			Implementation: tool.ImplementationNative,
-			InputSchema: &schema.Schema{
-				"type": "object",
-				"properties": map[string]any{
-					"name": map[string]any{"type": "string"},
-				},
-				"required": []string{"name"},
-			},
-			OutputSchema: &schema.Schema{
-				"type": "object",
-				"properties": map[string]any{
-					"result": map[string]any{"type": "string"},
-				},
-				"required": []string{"result"},
-			},
-			Config: &core.Input{"sample": true},
-		}
-	}
-
-	t.Run("Should execute native handler successfully", func(t *testing.T) {
-		...
-	})
-	...
-}
+func buildNativeToolConfig() *tool.Config {
+	return &tool.Config{
+		ID:             "native-tool",
+		Description:    "Native tool",
+		Implementation: tool.ImplementationNative,
+		InputSchema: &schema.Schema{
+			"type": "object",
+			"properties": map[string]any{
+				"name": map[string]any{"type": "string"},
+			},
+			"required": []string{"name"},
+		},
+		OutputSchema: &schema.Schema{
+			"type": "object",
+			"properties": map[string]any{
+				"result": map[string]any{"type": "string"},
+			},
+			"required": []string{"result"},
+		},
+		Config: &core.Input{"sample": true},
+	}
+}
+
+func TestNativeToolAdapter_ExecutesHandlerSuccessfully(t *testing.T) {
+	nativeuser.Reset()
+	handler := func(_ context.Context, input map[string]any, cfg map[string]any) (map[string]any, error) {
+		assert.Equal(t, map[string]any{"sample": true}, cfg)
+		assert.Equal(t, "alice", input["name"])
+		return map[string]any{"result": "ok"}, nil
+	}
+	require.NoError(t, nativeuser.Register("native-tool", handler))
+	adapter := NewNativeToolAdapter(buildNativeToolConfig())
+	output, err := adapter.Call(t.Context(), `{"name":"alice"}`)
+	require.NoError(t, err)
+	assert.Contains(t, output, "\"result\":\"ok\"")
+}
+
+func TestNativeToolAdapter_ValidatesInputSchema(t *testing.T) {
+	nativeuser.Reset()
+	require.NoError(
+		t,
+		nativeuser.Register(
+			"native-tool",
+			func(context.Context, map[string]any, map[string]any) (map[string]any, error) {
+				return map[string]any{"result": "ok"}, nil
+			},
+		),
+	)
+	adapter := NewNativeToolAdapter(buildNativeToolConfig())
+	_, err := adapter.Call(t.Context(), `{"unexpected":true}`)
+	require.Error(t, err)
+	coreErr, ok := err.(*core.Error)
+	require.True(t, ok)
+	assert.Equal(t, "INVALID_TOOL_INPUT", coreErr.Code)
+}
+// ... similar for panic recovery and output validation tests
```

As per coding guidelines.


> Committable suggestion skipped: line range outside the PR's diff.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
engine/llm/tool_registry_test.go lines 222-316: the TestNativeToolAdapter
function is ~95 lines and violates the 50-line guideline; split each t.Run
subtest into its own top-level test function. Create separate test functions
(e.g., TestNativeToolAdapter_Success, TestNativeToolAdapter_InvalidInput,
TestNativeToolAdapter_PanicRecovery, TestNativeToolAdapter_InvalidOutput) that
each set up their own buildConfig, nativeuser.Reset, register the handler,
instantiate NewNativeToolAdapter and perform the same assertions as the
corresponding subtest; keep shared helper code (like buildConfig) as a small
package-level helper function used by the new tests to avoid duplication.
```

</details>

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5gLa2K`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5gLa2K
```

---
*Generated from PR review - CodeRabbit AI*
