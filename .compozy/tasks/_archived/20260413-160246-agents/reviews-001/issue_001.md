---
status: resolved
file: internal/cli/agents_commands.go
line: 65
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56P5tb,comment:PRRC_kwDORy7nkc62zc8D
---

# Issue 001: _⚠️ Potential issue_ | _🔴 Critical_
## Review Comment

_⚠️ Potential issue_ | _🔴 Critical_

**Register the hidden `mcp-serve` subcommand.**

`newMCPServeCommand()` is defined below but never added to `agents`. That makes the reserved MCP server entrypoint unreachable, so any flow that tries to spawn `compozy agents mcp-serve` will fail at runtime.

<details>
<summary>Suggested fix</summary>

```diff
 	cmd.AddCommand(
 		newAgentsListCommand(),
 		newAgentsInspectCommand(),
+		newMCPServeCommand(),
 	)
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
	cmd.AddCommand(
		newAgentsListCommand(),
		newAgentsInspectCommand(),
		newMCPServeCommand(),
	)
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/agents_commands.go` around lines 62 - 65, The agents command
group never registers the hidden mcp-serve subcommand, so newMCPServeCommand()
is defined but unreachable; add newMCPServeCommand() to the cmd.AddCommand call
in the same scope where newAgentsListCommand() and newAgentsInspectCommand() are
registered (i.e., include newMCPServeCommand() alongside the existing commands),
ensuring any required hidden flag/state set by newMCPServeCommand() remains
unchanged.
```

</details>

<!-- fingerprinting:phantom:medusa:ocelot:b2073ba0-ef35-479c-9162-cd2efca1d22d -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `newMCPServeCommand()` existed but `newAgentsCommand()` only registered `list` and `inspect`, so the reserved MCP stdio entrypoint was unreachable from the CLI.
- Fix: Registered the hidden `mcp-serve` subcommand under `compozy agents` and expanded the CLI helper test to assert the command remains registered and hidden.
- Evidence: `go test ./internal/cli`
