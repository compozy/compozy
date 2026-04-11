---
status: resolved
file: internal/cli/state.go
line: 423
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc55LZVc,comment:PRRC_kwDORy7nkc61XmP8
---

# Issue 002: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Count stdin when enforcing prompt-source exclusivity.**

Non-empty stdin is never included in `sourceCount`, so `exec "prompt" < file` and `exec --prompt-file x < file` currently accept two sources and silently discard stdin instead of raising the advertised “exactly one prompt source” error.

<details>
<summary>💡 Suggested fix</summary>

```diff
-	sourceCount := 0
+	stdinPrompt, hasStdinPrompt, err := readPromptFromCommandInput(cmd.InOrStdin())
+	if err != nil {
+		return err
+	}
+
+	sourceCount := 0
 	if positionalPrompt != "" {
 		sourceCount++
 	}
 	if promptFile != "" {
 		sourceCount++
 	}
+	if hasStdinPrompt {
+		sourceCount++
+	}
@@
-	default:
-		stdinPrompt, hasStdinPrompt, err := readPromptFromCommandInput(cmd.InOrStdin())
-		if err != nil {
-			return err
-		}
-		if !hasStdinPrompt {
-			return fmt.Errorf(
-				"%s requires exactly one prompt source: positional prompt, --prompt-file, or non-empty stdin",
-				cmd.CommandPath(),
-			)
-		}
+	case hasStdinPrompt:
 		s.readPromptStdin = true
 		s.resolvedPromptText = stdinPrompt
 		return nil
+	default:
+		return fmt.Errorf(
+			"%s requires exactly one prompt source: positional prompt, --prompt-file, or non-empty stdin",
+			cmd.CommandPath(),
+		)
 	}
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/state.go` around lines 379 - 423, The logic currently omits
non-empty stdin from sourceCount, allowing two prompt sources; fix by calling
readPromptFromCommandInput(cmd.InOrStdin()) before computing/validating
sourceCount, store stdinPrompt and hasStdinPrompt, increment sourceCount if
hasStdinPrompt, then perform the existing exclusivity check and use the stored
stdinPrompt/hasStdinPrompt in the switch; ensure you don't re-read stdin later
(use the saved stdinPrompt and set s.readPromptStdin = true /
s.resolvedPromptText = stdinPrompt when selecting the stdin case).
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:6c29911c-ba13-4d74-ad6e-790b2357b234 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  `resolveExecPromptSource` only counts positional prompt text and `--prompt-file` before the exclusivity check, then reads stdin later in the default branch. That means a non-empty stdin prompt is ignored during source counting, so mixed prompt sources are accepted incorrectly. The fix is to read stdin once up front, include it in `sourceCount`, and reuse the buffered result instead of rereading stdin.
  Resolution: implemented and verified with the focused Go test run plus `make verify`.
