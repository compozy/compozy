---
status: resolved
file: internal/contentblock/engine.go
line: 60
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc55LZVg,comment:PRRC_kwDORy7nkc61XmQB
---

# Issue 003: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Validate the raw payload before re-emitting it.**

`MarshalEnvelopeJSON` currently returns any non-empty `data` blob verbatim. That means malformed JSON, or JSON whose embedded `"type"` disagrees with `blockType`, still serializes successfully and makes `blockType` effectively unused on marshal.


<details>
<summary>💡 Suggested fix</summary>

```diff
 func MarshalEnvelopeJSON[T ~string](blockType T, data json.RawMessage) ([]byte, error) {
 	if blockType == "" {
 		return nil, fmt.Errorf("marshal content block: missing type")
 	}
 	if len(data) == 0 {
 		return nil, fmt.Errorf("marshal %s block: missing data", blockType)
 	}
-	return data, nil
+	var envelope struct {
+		Type T `json:"type"`
+	}
+	if err := json.Unmarshal(data, &envelope); err != nil {
+		return nil, fmt.Errorf("marshal %s block: invalid data: %w", blockType, err)
+	}
+	if envelope.Type != blockType {
+		return nil, fmt.Errorf("marshal %s block: unexpected type %q", blockType, envelope.Type)
+	}
+	return append(json.RawMessage(nil), data...), nil
 }
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
// MarshalEnvelopeJSON preserves the canonical JSON payload stored in an envelope.
func MarshalEnvelopeJSON[T ~string](blockType T, data json.RawMessage) ([]byte, error) {
	if blockType == "" {
		return nil, fmt.Errorf("marshal content block: missing type")
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("marshal %s block: missing data", blockType)
	}
	var envelope struct {
		Type T `json:"type"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("marshal %s block: invalid data: %w", blockType, err)
	}
	if envelope.Type != blockType {
		return nil, fmt.Errorf("marshal %s block: unexpected type %q", blockType, envelope.Type)
	}
	return append(json.RawMessage(nil), data...), nil
}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/contentblock/engine.go` around lines 51 - 60, MarshalEnvelopeJSON
currently returns any non-empty data verbatim without validating it's
well-formed JSON or that an embedded "type" matches blockType; update
MarshalEnvelopeJSON(blockType, data) to first ensure json.Valid(data) (return an
error if not), then unmarshal data into a small container (e.g.
map[string]json.RawMessage or struct with a Type field) and, if a "type" field
is present, verify it equals blockType (return an error if it disagrees); if
validation passes, return the original data unchanged and include clear
contextual error messages mentioning blockType.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:51576401-e056-421f-904c-06271e28cefe -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  `MarshalEnvelopeJSON` currently returns any non-empty payload verbatim, so malformed JSON or a mismatched embedded `type` can bypass the typed envelope invariant. That makes `blockType` enforcement incomplete during marshaling. The fix is to validate the raw JSON, decode the embedded type, reject mismatches, and return a cloned payload only after validation succeeds.
  Resolution: implemented and verified with the focused Go test run plus `make verify`.
