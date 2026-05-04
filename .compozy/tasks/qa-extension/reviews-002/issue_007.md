---
provider: coderabbit
pr: "138"
round: 2
round_created_at: 2026-05-02T04:56:54.019903Z
status: pending
file: sdk/extension/types_test.go
line: 68
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5_GD6d,comment:PRRC_kwDORy7nkc69UBlZ
---

# Issue 007: _🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_
## Review Comment

_🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_

**Use `t.Run("Should...")` subtests (table-driven)**

The coverage is good, but this file should follow the repo’s required test-case structure with `t.Run("Should...")` subtests (preferably table-driven).




<details>
<summary>Refactor sketch</summary>

```diff
-func TestSessionRequestJSONUsesReadablePromptText(t *testing.T) {
-	t.Parallel()
-	...
-}
-
-func TestResumeSessionRequestJSONUsesReadablePromptText(t *testing.T) {
+func TestRequestJSONUsesReadablePromptText(t *testing.T) {
 	t.Parallel()
-
-	request := extension.ResumeSessionRequest{
-		SessionID:  "sess-123",
-		Prompt:     []byte("resume prompt"),
-		WorkingDir: "/tmp/work",
-		Model:      "gpt-5.5",
+	tests := []struct {
+		name    string
+		request any
+		want    string
+		notB64  string
+	}{
+		{
+			name: "Should marshal and unmarshal SessionRequest prompt as readable text",
+			request: extension.SessionRequest{
+				Prompt: []byte("plain prompt"), WorkingDir: "/tmp/work", Model: "gpt-5.5",
+			},
+			want:   "plain prompt",
+			notB64: "cGxhaW4gcHJvbXB0",
+		},
+		{
+			name: "Should marshal and unmarshal ResumeSessionRequest prompt as readable text",
+			request: extension.ResumeSessionRequest{
+				SessionID: "sess-123", Prompt: []byte("resume prompt"), WorkingDir: "/tmp/work", Model: "gpt-5.5",
+			},
+			want:   "resume prompt",
+			notB64: "cmVzdW1lIHByb21wdA==",
+		},
 	}
-
-	raw, err := json.Marshal(request)
-	...
+	for _, tc := range tests {
+		tc := tc
+		t.Run(tc.name, func(t *testing.T) {
+			t.Parallel()
+			raw, err := json.Marshal(tc.request)
+			if err != nil { t.Fatalf("marshal: %v", err) }
+			if strings.Contains(string(raw), tc.notB64) { t.Fatalf("expected readable prompt JSON, got %s", string(raw)) }
+			if !strings.Contains(string(raw), `"prompt":"`+tc.want+`"`) { t.Fatalf("expected readable prompt JSON, got %s", string(raw)) }
+		})
+	}
 }
```
</details>

As per coding guidelines, `MUST use t.Run("Should...") pattern for ALL test cases` and `Use table-driven tests with subtests (t.Run) as the default pattern`.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@sdk/extension/types_test.go` around lines 11 - 68, Refactor the two top-level
tests TestSessionRequestJSONUsesReadablePromptText and
TestResumeSessionRequestJSONUsesReadablePromptText into table-driven subtests
using t.Run("Should ..."): create a slice of cases (including name, request
value, expected JSON snippets and expected round-trip prompt) and iterate with
for _, tc := range cases { tc := tc; t.Run(tc.name, func(t *testing.T){
t.Parallel(); ... }) }, moving the existing marshal/unmarshal/assert logic into
the subtest body; reference the existing types extension.SessionRequest and
extension.ResumeSessionRequest and preserve all current assertions (checking no
base64, checking readable `"prompt":"..."`, and round-trip prompt equality).
Ensure each subtest name starts with "Should" per guidelines.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:0ad89fc6-58e3-486d-a0d8-db7a327c49c6 -->

<!-- d98c2f50 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `UNREVIEWED`
- Notes:
