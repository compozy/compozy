# Issue 6 - Review Thread Comment

**File:** `internal/cli/state.go:423`
**Date:** 2026-04-07 15:52:02 UTC
**Status:** - [x] RESOLVED

## Technical Disposition

`VALID` - the current tree already clears stale prompt-file state before selecting a source. `resolveExecPromptSource` snapshots the current prompt-file flag into a local variable, resets `s.promptFile = ""`, and only restores it when the prompt-file branch wins, which removes the stale-state path described in the review.

## Body

_⚠️ Potential issue_ | _🟡 Minor_

**Clear stale `promptFile` state before selecting the next source.**

This method resets `promptText`, `readPromptStdin`, and `resolvedPromptText`, but it leaves the previous `s.promptFile` value live unless the file branch wins again. Reusing the same `commandState` can therefore false-positive the exclusivity check or reopen the old file.

<details>
<summary>Suggested fix</summary>

```diff
 func (s *commandState) resolveExecPromptSource(cmd *cobra.Command, args []string) error {
+	promptFile := strings.TrimSpace(s.promptFile)
+
 	s.promptText = ""
+	s.promptFile = ""
 	s.readPromptStdin = false
 	s.resolvedPromptText = ""
-
-	promptFile := strings.TrimSpace(s.promptFile)
 	stdinPrompt, hasStdinPrompt, err := readPromptFromCommandInput(cmd.InOrStdin())
 	if err != nil {
 		return err
 	}
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
func (s *commandState) resolveExecPromptSource(cmd *cobra.Command, args []string) error {
	promptFile := strings.TrimSpace(s.promptFile)

	s.promptText = ""
	s.promptFile = ""
	s.readPromptStdin = false
	s.resolvedPromptText = ""

	positionalPrompt := ""
	if len(args) == 1 && strings.TrimSpace(args[0]) != "" {
		positionalPrompt = args[0]
	}
	stdinPrompt, hasStdinPrompt, err := readPromptFromCommandInput(cmd.InOrStdin())
	if err != nil {
		return err
	}

	sourceCount := 0
	if positionalPrompt != "" {
		sourceCount++
	}
	if promptFile != "" {
		sourceCount++
	}
	if hasStdinPrompt {
		sourceCount++
	}

	if sourceCount > 1 {
		return fmt.Errorf(
			"%s accepts only one prompt source at a time: positional prompt, --prompt-file, or stdin",
			cmd.CommandPath(),
		)
	}

	switch {
	case positionalPrompt != "":
		s.promptText = positionalPrompt
		s.resolvedPromptText = positionalPrompt
		return nil
	case promptFile != "":
		content, err := os.ReadFile(promptFile)
		if err != nil {
			return fmt.Errorf("read prompt file %s: %w", promptFile, err)
		}
		if strings.TrimSpace(string(content)) == "" {
			return fmt.Errorf("prompt file %s is empty", promptFile)
		}
		s.promptFile = promptFile
		s.resolvedPromptText = string(content)
		return nil
	case hasStdinPrompt:
		s.readPromptStdin = true
		s.resolvedPromptText = stdinPrompt
		return nil
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/state.go` around lines 371 - 423, The method
resolveExecPromptSource leaves s.promptFile from a prior run which can make the
exclusivity check or behavior incorrect; ensure s.promptFile is cleared unless
the current chosen source is the prompt-file. Specifically, after computing
local promptFile := strings.TrimSpace(s.promptFile) (and before the sourceCount
logic) or immediately when taking the positionalPrompt or stdin branch, set
s.promptFile = "" so stale state is removed; when the prompt-file branch wins
keep the existing assignment s.promptFile = promptFile as currently done. This
guarantees s.promptFile only reflects the active source and prevents false
positives in the exclusivity check.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:f9f66184-5b4a-4f5a-94d3-2e0f7df9fe75 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDORy7nkc55VFbL`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc55VFbL
```

---
*Generated from PR review - CodeRabbit AI*
