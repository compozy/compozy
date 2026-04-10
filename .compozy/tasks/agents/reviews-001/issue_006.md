---
status: resolved
file: internal/core/agent/client_test.go
line: 846
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56P5tk,comment:PRRC_kwDORy7nkc62zc8O
---

# Issue 006: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Normalize MCP server env before asserting equality.**

These checks are order-sensitive, but the test inputs originate from `map[string]string` env values. If the production conversion preserves map iteration order, `req.McpServers` can differ only by `Env` slice ordering and still be correct, making this test flaky in CI.




Also applies to: 871-872

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/agent/client_test.go` around lines 845 - 846, The test is flaky
because req.McpServers vs a.scenario.ExpectedNewSessionMCPServers can differ
only by ordering of Env slices; before the DeepEqual checks (for req.McpServers
and the similar check at 871-872), normalize ordering deterministically by
sorting each server's Env slice and then sorting the server slice itself (e.g.,
by a stable key like Host/Port or Name) so comparisons are order-insensitive;
update the assertions to compare these normalized copies of req.McpServers and
a.scenario.ExpectedNewSessionMCPServers (and the other pair) instead of the raw
values.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:0e499b0b-a5c1-4d94-badc-b97a6231c088 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `invalid`
- Analysis: The compared ACP env payload is already deterministic because `toACPEnvVars()` sorts map keys before building `[]acp.EnvVariable`, and the MCP server slice order is preserved from the input slice.
- Why no change: The reported flake source is not present in the current production conversion path, so normalizing the test assertions would add noise without fixing a real defect.
- Evidence: inspected `internal/core/agent/client.go` and reran `go test ./internal/core/agent`
