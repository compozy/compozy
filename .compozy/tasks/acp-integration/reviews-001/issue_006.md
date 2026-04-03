---
status: resolved
file: internal/core/agent/session.go
line: 454
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc54vHB0,comment:PRRC_kwDORy7nkc60y0fx
---

# Issue 006: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Silently returning nil on marshal error loses error context.**

`marshalRawJSON` returns `nil` when `json.Marshal` fails. This could mask serialization issues. Consider propagating the error or logging it.


<details>
<summary>🔧 Suggested improvement</summary>

```diff
 func marshalRawJSON(value any) json.RawMessage {
 	if value == nil {
 		return nil
 	}
 	if raw, ok := value.(json.RawMessage); ok {
 		return append(json.RawMessage(nil), raw...)
 	}

 	payload, err := json.Marshal(value)
 	if err != nil {
+		// Log or handle the error for observability
 		return nil
 	}
 	return payload
 }
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/agent/session.go` around lines 441 - 454, The function
marshalRawJSON currently swallows json.Marshal errors by returning nil; change
marshalRawJSON to return (json.RawMessage, error) so callers can handle
serialization errors instead of silently losing context, update all call sites
of marshalRawJSON to handle the returned error, and ensure any special-case
behavior for json.RawMessage is preserved while propagating the underlying
json.Marshal error (or logging it) rather than returning nil.
```

</details>

<!-- fingerprinting:phantom:medusa:ocelot:e9a5cb25-70ed-4597-839e-cc9e1e67363a -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `invalid`
- Rationale: `marshalRawJSON` is only used for optional diagnostic payloads (`ToolUseBlock.Input` and `SessionError.Data`). Converting marshal failures into hard errors would turn non-critical display metadata into session/update failures and could hide the primary ACP event or error. The existing `nil` fallback is deliberate best-effort behavior for auxiliary data.
