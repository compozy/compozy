---
status: resolved
file: internal/core/agent/registry_overlay.go
line: 277
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56Rl8q,comment:PRRC_kwDORy7nkc621sgp
---

# Issue 005: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Backslashes in double-quoted args are being dropped.**

Line 268/276 marks every `\` (outside single quotes) as an escape starter. In double-quoted strings this strips path separators, e.g. `"C:\Program Files\Tool\tool.exe"` becomes `C:Program FilesTooltool.exe`, breaking executable resolution.



<details>
<summary>💡 Suggested fix</summary>

```diff
 type overlayWordParser struct {
 	parts         []string
 	current       strings.Builder
 	inSingleQuote bool
 	inDoubleQuote bool
 	escaped       bool
+	escapedInDq   bool
 }

 func (p *overlayWordParser) handleEscapedRune(r rune) bool {
 	if !p.escaped {
 		return false
 	}
+	if p.escapedInDq && !isDoubleQuoteEscapable(r) {
+		p.current.WriteRune('\\')
+	}
 	p.current.WriteRune(r)
 	p.escaped = false
+	p.escapedInDq = false
 	return true
 }

 func (p *overlayWordParser) handleEscapeStart(r rune) bool {
 	if r != '\\' {
 		return false
 	}
 	if p.inSingleQuote {
 		p.current.WriteRune(r)
 		return true
 	}
 	p.escaped = true
+	p.escapedInDq = p.inDoubleQuote
 	return true
 }
+
+func isDoubleQuoteEscapable(r rune) bool {
+	switch r {
+	case '"', '\\', '$', '`':
+		return true
+	default:
+		return false
+	}
+}
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/agent/registry_overlay.go` around lines 259 - 277, The bug is
that handleEscapeStart marks every '\' (unless inSingleQuote) as starting an
escape, which causes backslashes inside double-quoted strings to be consumed;
update handleEscapeStart so that if p.inSingleQuote you WriteRune('\\') as
before, but if p.inDoubleQuote you also WriteRune('\\') and return true (do not
set p.escaped); only set p.escaped = true for unquoted input (or other contexts
where escaping is intended). Keep handleEscapedRune unchanged (it should still
consume the next rune when p.escaped is true). Reference: function
handleEscapeStart, function handleEscapedRune, fields p.inSingleQuote,
p.inDoubleQuote, p.escaped, and p.current.WriteRune.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:39ae80a8-45eb-4fc5-92f0-3da3d1575570 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - `handleEscapeStart()` currently treats every backslash outside single quotes as an escape prefix. Inside double-quoted arguments that drops literal path separators and breaks Windows-style paths or other quoted backslash content.
  - I will preserve literal backslashes inside double quotes unless they introduce one of the shell-escapable characters. This fix also needs a narrow regression test in `internal/core/agent/registry_overlay_test.go`, which is outside the listed code-file scope but is the minimal test touch needed to prove the parser behavior.
  - Resolved by preserving non-escapable backslashes in double-quoted words and adding Windows-path regression coverage in `internal/core/agent/registry_overlay_test.go`.
