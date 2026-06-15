---
provider: coderabbit
pr: "201"
round: 1
round_created_at: 2026-06-15T18:05:13.931425Z
status: resolved
file: internal/core/run/transcript/model.go
line: 356
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc6Jqhqa,comment:PRRC_kwDORy7nkc7LlP3j
---

# Issue 011: _⚠️ Potential issue_ | _🟠 Major_ | _⚡ Quick win_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_ | _⚡ Quick win_

**Restrict message ID lookup to user-message entries to prevent cross-kind merges.**

At Line 351, lookup matches any entry ID. If `MessageID` collides with an existing non-user ID, `applyUserMessage` (Line 147) can merge user content into the wrong transcript entry.






<details>
<summary>Suggested fix</summary>

```diff
-func (m *ViewModel) findEntryID(id string) int {
+func (m *ViewModel) findEntryID(kind EntryKind, id string) int {
 	id = strings.TrimSpace(id)
 	if id == "" {
 		return -1
 	}
 	for i := range m.entries {
-		if m.entries[i].ID == id {
+		if m.entries[i].Kind == kind && m.entries[i].ID == id {
 			return i
 		}
 	}
 	return -1
 }
```

```diff
-	if idx := m.findEntryID(messageID); idx >= 0 {
+	if idx := m.findEntryID(EntryKindUserMessage, messageID); idx >= 0 {
 		return m.mergeIntoEntry(idx, update.Blocks)
 	}
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
func (m *ViewModel) findEntryID(kind EntryKind, id string) int {
	id = strings.TrimSpace(id)
	if id == "" {
		return -1
	}
	for i := range m.entries {
		if m.entries[i].Kind == kind && m.entries[i].ID == id {
			return i
		}
	}
	return -1
}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against current code. Fix only still-valid issues, skip the
rest with a brief reason, keep changes minimal, and validate.

In `@internal/core/run/transcript/model.go` around lines 345 - 356, The
findEntryID method searches for entries by ID without restricting to
user-message entries only. This allows user message IDs to collide with non-user
entry IDs, causing applyUserMessage to incorrectly merge user content into wrong
entries. Modify the findEntryID method to add an additional condition in the
entry lookup loop that checks whether each entry is a user-message type before
matching on ID, ensuring only user-message entries are returned from the search.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk -->

<!-- cr-comment:v1:af15e19a1c3bd53348685376 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: transcript user-message updates look up entries by ID only, so a user `MessageID` can collide with a non-user entry ID and merge user content into the wrong transcript entry.
- Fix approach: make the entry lookup kind-aware for user-message updates and add a regression where a user message ID collides with an assistant entry.

## Resolution

- Resolved by scoping user-message updates to user entries.
- Verification: `rtk make verify` exited 0 after the code changes.
