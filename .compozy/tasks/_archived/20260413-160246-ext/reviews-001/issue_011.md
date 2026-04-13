---
status: resolved
file: internal/core/agent/client.go
line: 57
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56RVO4,comment:PRRC_kwDORy7nkc621Vap
---

# Issue 011: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🌐 Web query:

`In Go's encoding/json package, how are []byte struct fields marshaled, and how should a text prompt field be represented if the JSON payload must stay human-readable?`

💡 Result:

- In `encoding/json`, a struct field of type `[]byte` is **marshaled as a JSON string containing the bytes base64-encoded** (and a **nil `[]byte` marshals as JSON `null`**). [1]  
- If your payload must stay **human-readable** and the field is **text** (e.g., a “prompt”), represent it as a **Go `string`** so it marshals as a normal JSON string (UTF-8), not base64. If you must keep a `[]byte` in Go for other reasons, wrap it in a custom type and implement `MarshalJSON`/`UnmarshalJSON` (or `encoding.TextMarshaler`) to encode it as plain text instead of base64. [1][2]

**Sources**  
[1] Go `encoding/json` docs (`Marshal` mapping rules for `[]byte`) ([pkg.go.dev](https://pkg.go.dev/encoding/json%40go1.26.1?utm_source=openai))  
[2] golang-nuts discussion noting the default base64 behavior and suggesting custom marshalers ([groups.google.com](https://groups.google.com/g/golang-nuts/c/kXMWIfSTix0?utm_source=openai))

Citations:

- 1: https://pkg.go.dev/encoding/json%40go1.26.1?utm_source=openai
- 2: https://groups.google.com/g/golang-nuts/c/kXMWIfSTix0?utm_source=openai

---

🏁 Script executed:

```shell
# First, check the file exists and examine the flagged lines
cat -n internal/core/agent/client.go | head -100
```

Repository: compozy/compozy

Length of output: 4520

---

🏁 Script executed:

```shell
# Search for where SessionRequest or ResumeSessionRequest are marshaled or used in JSON context
rg -n "SessionRequest|ResumeSessionRequest" internal/core/agent/ -A 3 -B 1 | head -80
```

Repository: compozy/compozy

Length of output: 4385

---

🏁 Script executed:

```shell
# Check for custom MarshalJSON implementations for SessionRequest or ResumeSessionRequest
rg -n "MarshalJSON|UnmarshalJSON" internal/core/agent/

# Also check the hook dispatch code to confirm json.Marshal is used
cat -n internal/core/agent/hooks.go | head -110
```

Repository: compozy/compozy

Length of output: 3382

---

🏁 Script executed:

```shell
# Find DispatchMutableHook implementation
rg -n "DispatchMutableHook" internal/core/model/ -A 10 | head -50
```

Repository: compozy/compozy

Length of output: 1869

---

🏁 Script executed:

```shell
# Find RuntimeManager implementation that actually handles the JSON serialization
rg -n "type.*RuntimeManager" internal/core/model/ -A 3

# Also search for JSON marshaling in hook dispatch
rg -n "json.Marshal" internal/core/ | grep -i hook
```

Repository: compozy/compozy

Length of output: 574

---

🏁 Script executed:

```shell
# Find the actual implementation of RuntimeManager.DispatchMutableHook
rg -n "func.*DispatchMutableHook" internal/core/ -A 15 | head -60
```

Repository: compozy/compozy

Length of output: 4143

---

🏁 Script executed:

```shell
# Search for actual RuntimeManager implementation (not test mocks)
fd "\.go$" internal/core/extension/ | xargs rg -l "DispatchMutableHook" | head -5

# Look for extension handler that serializes to JSON
rg -n "json.Marshal.*payload\|Unmarshal" internal/core/extension/ | head -20
```

Repository: compozy/compozy

Length of output: 176

---

🏁 Script executed:

```shell
# Examine the extension manager implementation
rg -n "DispatchMutableHook" internal/core/extension/manager.go -A 20 | head -80
```

Repository: compozy/compozy

Length of output: 1107

---

🏁 Script executed:

```shell
# Find DispatchMutable implementation
rg -n "func.*DispatchMutable" internal/core/extension/ -A 15 | head -100
```

Repository: compozy/compozy

Length of output: 7575

---

🏁 Script executed:

```shell
# Find the actual dispatcher.go implementation
rg -n "func.*DispatchMutable\(" internal/core/extension/dispatcher.go -A 30 | head -100
```

Repository: compozy/compozy

Length of output: 1134

---

🏁 Script executed:

```shell
# Find invokeHook to see how payload is serialized
rg -n "func.*invokeHook" internal/core/extension/dispatcher.go -A 30 | head -120
```

Repository: compozy/compozy

Length of output: 989

---

🏁 Script executed:

```shell
# Continue finding the marshaling in invokeHook
rg -n "func.*invokeHook" internal/core/extension/dispatcher.go -A 50 | tail -40
```

Repository: compozy/compozy

Length of output: 1250

---

🏁 Script executed:

```shell
# Get the rest of invokeHook to see the Caller.Call invocation
rg -n "func.*invokeHook" internal/core/extension/dispatcher.go -A 60 | tail -30
```

Repository: compozy/compozy

Length of output: 896

---

🏁 Script executed:

```shell
# Find what Caller is and its Call method
rg -n "type.*Caller|func.*Call\(" internal/core/extension/ | grep -E "Caller|\.Call" | head -20
```

Repository: compozy/compozy

Length of output: 358

---

🏁 Script executed:

```shell
# Find ExtensionCaller interface definition and understand how it serializes
cat -n internal/core/extension/chain.go | head -50
```

Repository: compozy/compozy

Length of output: 2064

---

🏁 Script executed:

```shell
# Find actual implementations of ExtensionCaller.Call to see JSON serialization
rg -n "func.*Call.*context.Context.*method.*params" internal/core/extension/ -A 20 | head -100
```

Repository: compozy/compozy

Length of output: 3009

---



</details>

**Change `Prompt` from `[]byte` to `string` in the hook JSON contract.**

`encoding/json` marshals `[]byte` as base64-encoded strings. This makes pre-create/pre-resume hook payloads expose the prompt as base64, not plain text. Extensions expecting to inspect or rewrite the prompt will receive an unexpectedly encoded value. Use a `string` field (or implement `MarshalJSON`/`UnmarshalJSON` for text representation) to maintain a human-readable JSON contract.

Also applies to: 62-71

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/agent/client.go` around lines 49 - 57, Change the Prompt field
from []byte to string in the hook JSON contract structs in
internal/core/agent/client.go (replace Prompt []byte `json:"prompt,omitempty"`
with Prompt string `json:"prompt,omitempty"` for the struct containing
WorkingDir/Model/MCPServers/ExtraEnv/Context/RunID/JobID/RuntimeMgr and any
other similar hook/request structs in the same file), and update all call sites
that set or read Prompt (convert between []byte and string using string(b) or
[]byte(s) as needed) so the JSON marshals to plain text; alternatively implement
MarshalJSON/UnmarshalJSON on those structs if you need custom byte handling, but
prefer the simple type change to preserve human-readable prompt payloads.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:ce4005c2-f225-49f9-bac5-6ac8c129da42 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Notes:
  - The pre-session hook payloads serialize `SessionRequest` / `ResumeSessionRequest` through `encoding/json`, and the `Prompt []byte` field is therefore emitted as base64 instead of readable text.
  - Root cause: the internal binary-oriented field type is leaking into the external hook JSON contract even though prompts are treated as text throughout the ACP session flow.
  - Implemented fix: kept the internal prompt representation compatible with the existing session code while changing the JSON contract to encode prompts as plain text for hook dispatches; this required updating the affected hook tests and mock-extension harness expectations.
  - Additional scope needed: minimal test/harness edits outside the listed code files are required because the current integration tests explicitly assert the base64 hook payload format.
