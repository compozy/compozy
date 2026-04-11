---
status: resolved
file: internal/core/model/content_test.go
line: 248
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc55LZWi,comment:PRRC_kwDORy7nkc61XmRJ
---

# Issue 015: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
#!/bin/bash
# Verify whether this test follows the required subtest naming pattern.
rg -n -C2 'func TestContentBlockMarshalUsesCamelCaseJSONTags|t\.Run\("Should' internal/core/model/content_test.go
```

Repository: compozy/compozy

Length of output: 162

---



</details>

**Wrap test logic in `t.Run("Should...")` subtest.**

This test violates the repository guideline requiring all test cases to use the `t.Run()` pattern with "Should..." naming. Move the current test body into a subtest.

<details>
<summary>Suggested refactor</summary>

```diff
 func TestContentBlockMarshalUsesCamelCaseJSONTags(t *testing.T) {
 	t.Parallel()
 
-	block, err := model.NewContentBlock(model.ToolResultBlock{
-		ToolUseID: "tool-7",
-		Content:   "ok",
-		IsError:   true,
-	})
-	if err != nil {
-		t.Fatalf("new content block: %v", err)
-	}
-
-	data, err := json.Marshal(block)
-	if err != nil {
-		t.Fatalf("marshal content block: %v", err)
-	}
-
-	encoded := string(data)
-	required := []string{`"toolUseId":"tool-7"`, `"isError":true`}
-	for _, field := range required {
-		if !strings.Contains(encoded, field) {
-			t.Fatalf("expected camelCase field %q in %s", field, encoded)
-		}
-	}
-
-	forbidden := []string{`"tool_use_id"`, `"is_error"`}
-	for _, field := range forbidden {
-		if strings.Contains(encoded, field) {
-			t.Fatalf("did not expect snake_case field %q in %s", field, encoded)
-		}
-	}
+	t.Run("Should marshal tool result using camelCase JSON tags", func(t *testing.T) {
+		t.Parallel()
+
+		block, err := model.NewContentBlock(model.ToolResultBlock{
+			ToolUseID: "tool-7",
+			Content:   "ok",
+			IsError:   true,
+		})
+		if err != nil {
+			t.Fatalf("new content block: %v", err)
+		}
+
+		data, err := json.Marshal(block)
+		if err != nil {
+			t.Fatalf("marshal content block: %v", err)
+		}
+
+		encoded := string(data)
+		required := []string{`"toolUseId":"tool-7"`, `"isError":true`}
+		for _, field := range required {
+			if !strings.Contains(encoded, field) {
+				t.Fatalf("expected camelCase field %q in %s", field, encoded)
+			}
+		}
+
+		forbidden := []string{`"tool_use_id"`, `"is_error"`}
+		for _, field := range forbidden {
+			if strings.Contains(encoded, field) {
+				t.Fatalf("did not expect snake_case field %q in %s", field, encoded)
+			}
+		}
+	})
 }
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/model/content_test.go` around lines 217 - 248, Wrap the
existing test body of TestContentBlockMarshalUsesCamelCaseJSONTags into a t.Run
subtest with a "Should..." name (e.g., t.Run("Should use camelCase JSON tags",
func(t *testing.T) { ... })), moving t.Parallel() into the subtest if
parallelism is desired, and keep all existing setup and assertions
(NewContentBlock, json.Marshal, checks against encoded for required/forbidden
fields) unchanged inside that subtest so the test follows the repository's t.Run
naming pattern.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:31e81b7d-c3ac-4ec7-978e-4a2e76adf09c -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  `TestContentBlockMarshalUsesCamelCaseJSONTags` is a single broad test body without the repo’s expected subtest shape. This is a style/test-structure issue rather than a production bug, but it is valid within the scoped test file. I will wrap the behavior in a `t.Run("Should ...")` subtest without weakening the assertions.
  Resolution: implemented and verified with the focused Go test run plus `make verify`.
