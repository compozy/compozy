---
status: resolved
file: internal/setup/reusable_agents.go
line: 141
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56P5uA,comment:PRRC_kwDORy7nkc62zc8x
---

# Issue 019: _⚠️ Potential issue_ | _🟠 Major_

## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Make agent installation atomic.**

This deletes the existing agent directory before the new contents are known to be copyable. If `copyBundleDirectory` fails midway, the user loses the last good install and is left with a partial replacement. Stage into a sibling temp directory and rename into place only after the copy succeeds.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/setup/reusable_agents.go` around lines 121 - 141, The current
install flow deletes/cleans the final target (cleanAndCreateDirectory on
targetPath) before copying, which can leave a partial or missing agent if
copyBundleDirectory fails; change the flow to create a sibling temp directory
(e.g., tempTarget := targetPath + ".tmp" or use ioutil.TempDir in the same
parent), call cleanAndCreateDirectory on that tempTarget, then call
copyBundleDirectory into tempTarget; if copy succeeds, atomically replace the
old installation by renaming tempTarget to targetPath (os.Rename) and if it
fails remove tempTarget and append the ReusableAgentFailureItem with Error set
to the copy error; ensure you still use reusableAgent.Directory and
agents.FS for the copy and preserve existing failure handling for cleanup
and continue.
```

</details>

<!-- fingerprinting:phantom:medusa:ocelot:ee6f376d-2c51-442f-8f6e-f006907140c7 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `InstallBundledReusableAgents()` removed the live target directory before the new bundle copy succeeded, so a copy failure could destroy the last known-good install and leave partial content behind.
- Fix: Switched reusable-agent installation to stage in a sibling temp directory and replace the target only after a successful copy, with rollback/preservation coverage for copy failure.
- Evidence: `go test ./internal/setup`
