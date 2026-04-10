---
status: resolved
file: internal/core/agents/execution.go
line: 145
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56P5tu,comment:PRRC_kwDORy7nkc62zc8c
---

# Issue 009: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Escape agent metadata before embedding it into prompt tags.**

`agent.Name`, `agent.Metadata.Title`, and `agent.Metadata.Description` are inserted raw into XML-like blocks. An agent file containing newlines or strings like `</agent_metadata>` / `</available_agents>` can break the framing and inject arbitrary instructions into what should be host-owned prompt structure. Serialize these fields with escaping (for example JSON) or sanitize control/tag characters first. Based on learnings: "Assess and document attack surface changes for every architectural decision".

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/agents/execution.go` around lines 109 - 145, The agent metadata
is injected raw into the prompt tags and can break framing or allow prompt
injection; update buildAgentMetadataBlock, buildAvailableAgentsBlock and
formatDiscoveryCatalogEntry to escape/serialize agent.Name, agent.Metadata.Title
and agent.Metadata.Description before concatenating into the XML-like blocks
(e.g., JSON-serialize the strings or escape control characters and
angle-brackets/newlines), and use the escaped values in the returned strings so
tags like </agent_metadata> or newlines in fields cannot break the host-owned
prompt structure.
```

</details>

<!-- fingerprinting:phantom:medusa:ocelot:ee6f376d-2c51-442f-8f6e-f006907140c7 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: Agent name/title/description were concatenated raw into host-owned XML-like prompt blocks, so metadata containing angle brackets or newlines could break framing and inject content into reserved sections.
- Fix: Escaped prompt metadata values before embedding them into `<agent_metadata>` and `<available_agents>` blocks; added regression coverage for malicious metadata values.
- Evidence: `go test ./internal/core/agents/...`
