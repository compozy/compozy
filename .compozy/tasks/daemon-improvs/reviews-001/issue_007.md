---
status: resolved
file: internal/api/contract/types.go
line: 273
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58go7y,comment:PRRC_kwDORy7nkc651UMM
---

# Issue 007: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
# First, verify the file exists and check its structure
find . -name "types.go" -path "*/api/contract/*" | head -20
```

Repository: compozy/compozy

Length of output: 91

---

🏁 Script executed:

```shell
# Check the specific lines mentioned (245-273)
if [ -f "internal/api/contract/types.go" ]; then
  echo "=== Lines 245-273 ===" && sed -n '245,273p' internal/api/contract/types.go
  echo ""
  echo "=== Full file line count ===" && wc -l internal/api/contract/types.go
fi
```

Repository: compozy/compozy

Length of output: 742

---

🏁 Script executed:

```shell
# Look for the structs mentioned in the review
rg "type (SessionEntry|SessionPlanState|SessionMetaState|SessionViewSnapshot)" internal/api/contract/types.go -A 10
```

Repository: compozy/compozy

Length of output: 1255

---

🏁 Script executed:

```shell
# Check if RunJobSummary exists and how Session is used
rg "RunJobSummary" internal/api/contract/types.go -B 2 -A 10
```

Repository: compozy/compozy

Length of output: 1255

---

🏁 Script executed:

```shell
# Look for any custom JSON marshaling logic in the file
rg "MarshalJSON|UnmarshalJSON" internal/api/contract/types.go -B 2 -A 5
```

Repository: compozy/compozy

Length of output: 378

---

🏁 Script executed:

```shell
# Check for any JSON tags in the file to understand the convention being used
rg 'json:"' internal/api/contract/types.go | head -20
```

Repository: compozy/compozy

Length of output: 1017

---

🏁 Script executed:

```shell
# Check more of RunJobSummary to see if Session field exists
rg "RunJobSummary" internal/api/contract/types.go -A 20 | head -40
```

Repository: compozy/compozy

Length of output: 2051

---

🏁 Script executed:

```shell
# Look for Session field specifically in RunJobSummary or related structs
rg "Session.*SessionViewSnapshot" internal/api/contract/types.go
```

Repository: compozy/compozy

Length of output: 122

---

🏁 Script executed:

```shell
# Check if these session structs are used anywhere in the codebase
rg "SessionViewSnapshot|SessionEntry|SessionPlanState|SessionMetaState" --type go | grep -v "^internal/api/contract/types.go" | head -20
```

Repository: compozy/compozy

Length of output: 1709

---



</details>

**Add JSON tags to the nested session snapshot structs.**

`RunJobSummary.Session` is part of the JSON transport payload, but these nested structs marshal without tags, causing Go's default encoding to use PascalCase field names (`ID`, `Entries`, `Plan`, etc.) instead of snake_case. This breaks the wire contract for all clients expecting `id`, `entries`, `plan`, etc.

<details>
<summary>Suggested change</summary>

```diff
 type SessionEntry struct {
-	ID            string
-	Kind          SessionEntryKind
-	Title         string
-	Preview       string
-	ToolCallID    string
-	ToolCallState ToolCallState
-	Blocks        []ContentBlock
+	ID            string           `json:"id"`
+	Kind          SessionEntryKind `json:"kind"`
+	Title         string           `json:"title,omitempty"`
+	Preview       string           `json:"preview,omitempty"`
+	ToolCallID    string           `json:"tool_call_id,omitempty"`
+	ToolCallState ToolCallState    `json:"tool_call_state,omitempty"`
+	Blocks        []ContentBlock   `json:"blocks,omitempty"`
 }
 
 type SessionPlanState struct {
-	Entries      []SessionPlanEntry
-	PendingCount int
-	RunningCount int
-	DoneCount    int
+	Entries      []SessionPlanEntry `json:"entries,omitempty"`
+	PendingCount int                `json:"pending_count,omitempty"`
+	RunningCount int                `json:"running_count,omitempty"`
+	DoneCount    int                `json:"done_count,omitempty"`
 }
 
 type SessionMetaState struct {
-	CurrentModeID     string
-	AvailableCommands []SessionAvailableCommand
-	Status            SessionStatus
+	CurrentModeID     string                    `json:"current_mode_id,omitempty"`
+	AvailableCommands []SessionAvailableCommand `json:"available_commands,omitempty"`
+	Status            SessionStatus             `json:"status,omitempty"`
 }
 
 type SessionViewSnapshot struct {
-	Revision int
-	Entries  []SessionEntry
-	Plan     SessionPlanState
-	Session  SessionMetaState
+	Revision int              `json:"revision"`
+	Entries  []SessionEntry   `json:"entries,omitempty"`
+	Plan     SessionPlanState `json:"plan,omitempty"`
+	Session  SessionMetaState `json:"session,omitempty"`
 }
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/api/contract/types.go` around lines 245 - 273, The nested session
snapshot structs (SessionEntry, SessionPlanState, SessionMetaState,
SessionViewSnapshot) are missing JSON tags so the marshalled fields use
PascalCase instead of the expected snake_case; update each struct's fields
(e.g., SessionEntry.ID, SessionEntry.Kind, SessionPlanState.Entries,
SessionPlanState.PendingCount, SessionMetaState.CurrentModeID,
SessionViewSnapshot.Revision/Entries/Plan/Session, etc.) to include explicit
`json:"..."` tags using the expected snake_case names (e.g., `id`, `kind`,
`title`, `preview`, `tool_call_id`, `tool_call_state`, `blocks`, `entries`,
`pending_count`, `running_count`, `done_count`, `current_mode_id`,
`available_commands`, `status`, `revision`, `plan`, `session`) so JSON
encoding/decoding matches the wire contract.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:0a63c130-a1da-4180-ae3d-657764834efe -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: the nested session snapshot structs used by `RunJobSummary.Session` lack explicit JSON tags, so the transport JSON falls back to Go field names like `ID`, `Entries`, and `Plan` instead of the canonical snake_case contract.
- Fix approach: add explicit JSON tags to the nested session snapshot structs and strengthen the contract tests so they assert the emitted wire keys rather than relying on a marshal/unmarshal round trip that would mask the mismatch.
- Resolution: added explicit JSON tags to the nested session snapshot structs and expanded the contract test to assert snake_case wire keys like `revision`, `entries`, `tool_call_state`, and `current_mode_id`.
- Regression coverage: `go test ./internal/api/client ./internal/api/contract ./internal/api/core ./internal/api/httpapi` passed after the new wire-shape assertions landed.
- Verification: `make verify` passed after the final edit with `2534` tests, `2` skipped daemon helper-process tests, and a successful `go build ./cmd/compozy`.
