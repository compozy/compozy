# Issue 4 - Review Thread Comment

**File:** `internal/cli/state.go:252`
**Date:** 2026-04-07 15:52:02 UTC
**Status:** - [ ] UNRESOLVED

## Technical Disposition

`INVALID` - the current exec flow no longer routes through `maybeCollectInteractiveParams`. `internal/cli/run.go` calls `resolveExecPromptSource(cmd, args)` as the exec setup step, and `prepareAndRun` does not invoke the interactive gate afterward for exec commands. A positional exec prompt therefore remains valid on non-interactive terminals, so the reported failure mode is obsolete in the current tree.

## Body

_⚠️ Potential issue_ | _🔴 Critical_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
#!/bin/bash
set -euo pipefail

echo "==> internal/cli/run.go"
rg -n -C4 'func \(s \*commandState\) exec|maybeCollectInteractiveParams|resolveExecPromptSource' internal/cli/run.go

echo
echo "==> internal/cli/commands.go"
rg -n -C4 'RunE|maybeCollectInteractiveParams|resolveExecPromptSource' internal/cli/commands.go
```

Repository: compozy/compozy

Length of output: 2698

---



</details>

**Fix the order of interactivity check and prompt source resolution for the `exec` command.**

The `maybeCollectInteractiveParams()` check at line 73 in `prepareAndRun` evaluates `cmd.Flags().NFlag()`, which counts only flag arguments and ignores positional arguments. For the `exec` command, this means `exec "my prompt"` will fail the interactivity requirement on non-interactive terminals even though the positional argument provides a valid prompt source. The `resolveExecPromptSource()` callback (which would validate the positional prompt) is invoked after this interactivity gate, making it unreachable.

Move the prompt source resolution before the interactivity check for the exec path, or gate the interactivity requirement to skip it when a positional prompt is already provided.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/state.go` around lines 226 - 252, The interactivity gate in
maybeCollectInteractiveParams blocks the exec command when a positional prompt
exists because cmd.Flags().NFlag() only counts flags; update prepareAndRun/exec
flow so resolveExecPromptSource is called (or positional prompt presence is
checked) before calling maybeCollectInteractiveParams, or alter
maybeCollectInteractiveParams to skip the interactive-terminal check when
resolveExecPromptSource (or a positional prompt) indicates a provided prompt;
specifically locate prepareAndRun and the exec path that calls
resolveExecPromptSource and reorder or add a conditional that if
resolveExecPromptSource returns a valid prompt (or if positional args exist)
then do not enforce the isInteractive() check that uses cmd.Flags().NFlag().
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:f9f66184-5b4a-4f5a-94d3-2e0f7df9fe75 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDORy7nkc55VFa9`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc55VFa9
```

---
*Generated from PR review - CodeRabbit AI*
