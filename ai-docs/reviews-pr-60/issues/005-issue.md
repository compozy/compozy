# Issue 5 - Review Thread Comment

**File:** `internal/cli/root.go:814`
**Date:** 2026-04-05 19:22:18 America/Sao_Paulo
**Status:** - [x] RESOLVED

**Disposition:** VALID

**Rationale:** `resolveExecPromptSource()` eagerly reads stdin before it knows stdin is the chosen prompt source. That can block `exec` even when a positional prompt or `--prompt-file` already fully determines the input, so the read must move behind explicit-source resolution.

## Body

_⚠️ Potential issue_ | _🟠 Major_

**Don't consume stdin until you've actually chosen it as the prompt source.**

`readPromptFromCommandInput(cmd.InOrStdin())` runs even when `[prompt]` or `--prompt-file` is already set. On a live pipe/FIFO, `io.ReadAll` will wait for EOF, so `compozy exec "hi"` can hang instead of starting or quickly rejecting the extra input. Probe stdin only after explicit sources are ruled out, or use a non-blocking readiness check.



Also applies to: 863-885

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/root.go` around lines 811 - 814, Avoid calling
readPromptFromCommandInput(cmd.InOrStdin()) eagerly; only probe stdin after
explicit prompt sources (the [prompt] value and --prompt-file) have been ruled
out. Change the logic in root command setup so you first check whether a prompt
value or prompt-file flag is set, and if neither is present then detect whether
stdin is a non-interactive pipe/tty (use os.Stdin.Stat to check for a pipe) and
perform a non-blocking readiness check before calling
readPromptFromCommandInput; if stdin is not ready or is a terminal, skip calling
readPromptFromCommandInput to prevent blocking. Ensure the same change is
applied to the other block that currently calls readPromptFromCommandInput (the
code around the 863–885 region) so both places only read stdin when explicitly
needed.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:5606214d-8719-43d5-98a2-064a6cd78ca1 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDORy7nkc547tHg`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc547tHg
```

---
*Generated from PR review - CodeRabbit AI*
