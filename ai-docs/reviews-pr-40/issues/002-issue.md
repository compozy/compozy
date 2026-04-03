# Issue 2 - Review Thread Comment

**File:** `internal/cli/form.go:85`
**Date:** 2026-04-03 18:11:21 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_⚠️ Potential issue_ | _🟡 Minor_

**`addDirs` round-tripping breaks on valid paths containing commas.**

This serializes the slice with `", "`, but `parseAddDirInput` later splits on every comma. Accepting a prefilled value like `["docs,archive"]` unchanged rewrites it into two directories.

<details>
<summary>🛠️ Minimal guard against corrupting existing values</summary>

```diff
 	if len(state.addDirs) > 0 {
-		inputs.addDirs = strings.Join(state.addDirs, ", ")
+		safe := true
+		for _, dir := range state.addDirs {
+			if strings.ContainsAny(dir, ",\n") {
+				safe = false
+				break
+			}
+		}
+		if safe {
+			inputs.addDirs = strings.Join(state.addDirs, ", ")
+		}
 	}
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
	if len(state.addDirs) > 0 {
		safe := true
		for _, dir := range state.addDirs {
			if strings.ContainsAny(dir, ",\n") {
				safe = false
				break
			}
		}
		if safe {
			inputs.addDirs = strings.Join(state.addDirs, ", ")
		}
	}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/form.go` around lines 83 - 85, The current prefill of
inputs.addDirs uses strings.Join(state.addDirs, ", ") which will corrupt paths
that contain commas because parseAddDirInput splits on commas; to fix, before
setting inputs.addDirs check each entry in state.addDirs for commas (using
strings.Contains(dir, ",") or similar) and only assign inputs.addDirs =
strings.Join(state.addDirs, ", ") when none contain commas—otherwise leave
inputs.addDirs unset (or empty) so parseAddDirInput isn’t forced to split an
already-valid comma-containing path. Ensure you reference state.addDirs and
parseAddDirInput when locating the code to change.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:5772648d-4177-42ad-99f4-1954cf786608 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Disposition: `VALID`
- Restated change: avoid serializing `state.addDirs` into an ambiguous comma-delimited string when any existing directory already contains a comma.
- Evidence: `newFormInputsFromState` joins `state.addDirs` with `", "`, but `parseAddDirInput` splits on every comma, so a preserved value like `docs,archive` round-trips as two paths.
- Implementation note: fix together with Issue 1 so the form preserves existing ambiguous values unless the user explicitly replaces or clears them.

## Resolve

Thread ID: `PRRT_kwDORy7nkc54yeIb`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc54yeIb
```

---
*Generated from PR review - CodeRabbit AI*
