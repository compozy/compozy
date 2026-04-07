---
status: resolved
file: internal/core/migration/migrate.go
line: 88
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc55LZWQ,comment:PRRC_kwDORy7nkc61XmQ3
---

# Issue 011: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Honor cancellation during the write phase.**

`ctx` is checked while walking, but once pending writes begin this loop will rewrite every file even after cancellation. A long migration can still mutate the workspace after the user has aborted the command.

<details>
<summary>💡 Suggested fix</summary>

```diff
 	for _, file := range pending {
+		if err := ctx.Err(); err != nil {
+			return result, fmt.Errorf("migration canceled during write: %w", err)
+		}
 		if err := os.WriteFile(file.path, []byte(file.content), 0o600); err != nil {
 			return result, fmt.Errorf("write migrated artifact %s: %w", file.path, err)
 		}
 	}
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
	sort.Slice(pending, func(i, j int) bool {
		return pending[i].path < pending[j].path
	})
	for _, file := range pending {
		if err := ctx.Err(); err != nil {
			return result, fmt.Errorf("migration canceled during write: %w", err)
		}
		if err := os.WriteFile(file.path, []byte(file.content), 0o600); err != nil {
			return result, fmt.Errorf("write migrated artifact %s: %w", file.path, err)
		}
	}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/migration/migrate.go` around lines 81 - 88, The write loop over
pending artifacts currently always writes every file; change it to honor the
migration context by checking ctx.Err() or using a select on ctx.Done() before
each os.WriteFile call and aborting early (returning ctx.Err() or a wrapped
context.Canceled error) so no further files are mutated after cancellation;
update the loop that iterates over pending (the slice sorted with sort.Slice) to
perform this context check before attempting each write and avoid proceeding to
os.WriteFile when the context is cancelled.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:6c29911c-ba13-4d74-ad6e-790b2357b234 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  The migration walk honors `ctx` while scanning, but the write loop does not re-check cancellation once writes start. That means a canceled migration can still mutate additional files during the write phase. The fix is to abort before each write when `ctx.Err()` is set and add coverage for cancellation during the write loop.
  Resolution: implemented and verified with the focused Go test run plus `make verify`.
