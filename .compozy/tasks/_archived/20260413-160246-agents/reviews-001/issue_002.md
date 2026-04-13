---
status: resolved
file: internal/cli/agents_commands.go
line: 449
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56P5tf,comment:PRRC_kwDORy7nkc62zc8J
---

# Issue 002: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Don't hide non-`ENOENT` MCP config path failures.**

`optionalExistingPath` collapses every `os.Stat` error to `""`. On permission errors or broken symlinks, `agents inspect` will print `MCP config: -` instead of the actual path, which removes the clue users need to debug the definition.

<details>
<summary>Suggested fix</summary>

```diff
 func optionalExistingPath(path string) string {
 	if strings.TrimSpace(path) == "" {
 		return ""
 	}
 	if _, err := os.Stat(path); err != nil {
-		return ""
+		if os.IsNotExist(err) {
+			return ""
+		}
+		return path
 	}
 	return path
 }
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
func optionalExistingPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		return path
	}
	return path
}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/agents_commands.go` around lines 442 - 449, The helper
optionalExistingPath currently returns "" for any os.Stat error, hiding useful
paths on permission errors or broken symlinks; change it so it only returns ""
when the path is blank or when os.Stat returns an IsNotExist error
(os.IsNotExist(err)); for any other non-nil error return the original path so
callers (e.g., agents inspect) can show the actual path and surface the
underlying failure; update function optionalExistingPath to use
os.IsNotExist(err) to distinguish the ENOENT case.
```

</details>

<!-- fingerprinting:phantom:medusa:ocelot:b2073ba0-ef35-479c-9162-cd2efca1d22d -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `optionalExistingPath()` collapsed every `os.Stat` failure to blank output, which hid useful MCP config paths for non-ENOENT cases such as `ENOTDIR` or permission failures.
- Fix: Changed the helper to blank the path only for `os.IsNotExist(err)` and preserve the original path for other stat failures; added regression coverage for an `ENOTDIR` path.
- Evidence: `go test ./internal/cli`
