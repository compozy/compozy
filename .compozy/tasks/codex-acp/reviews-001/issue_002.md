---
provider: coderabbit
pr: "151"
round: 1
round_created_at: 2026-05-14T00:24:33.853673Z
status: resolved
file: internal/core/agent/terminal.go
line: 337
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc6B7N8O,comment:PRRC_kwDORy7nkc7BA5WE
---

# Issue 002: _⚠️ Potential issue_ | _🟠 Major_ | _⚡ Quick win_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_ | _⚡ Quick win_

**Add a server-side output cap when `OutputByteLimit` is unset.**

`nil`/`<= 0` currently means “buffer everything in RAM”. One noisy command can exhaust the agent process before the caller ever polls `TerminalOutput`.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against current code. Fix only still-valid issues, skip the
rest with a brief reason, keep changes minimal, and validate.

In `@internal/core/agent/terminal.go` around lines 332 - 337,
newTerminalOutputBuffer currently treats a nil or non-positive limit as
"unlimited", which can let a single command exhaust RAM; change
newTerminalOutputBuffer to apply a server-side default cap when limit is nil or
<= 0 (e.g., a constant like defaultOutputByteLimit such as 10*1024*1024 bytes),
so resolvedLimit is set to that default instead of 0; update/introduce the
default constant (name: defaultOutputByteLimit) and ensure
terminalOutputBuffer{limit: resolvedLimit} uses that capped value so
OutputByteLimit unset no longer means unlimited.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - `newTerminalOutputBuffer` currently resolves nil or non-positive `OutputByteLimit` to `0`, which disables truncation entirely.
  - That means ACP terminal output can grow without bound in RAM when clients omit the field, which is not safe for a long-lived agent process.
  - Fix approach: introduce a server-side default cap and apply it whenever the caller does not provide a positive limit; add buffer-level coverage for nil and non-positive inputs.
  - Resolution: unset or non-positive limits now fall back to a 10 MiB server-side cap via `defaultOutputByteLimit`; `TestNewTerminalOutputBufferAppliesServerDefaultWhenLimitUnset` covers nil/zero/negative inputs.
