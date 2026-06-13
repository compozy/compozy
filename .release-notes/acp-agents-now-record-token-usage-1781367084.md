---
title: ACP agents now record token usage
type: fix
---

Token usage from ACP-backed agents (Claude Code, Codex, and other ACP runtimes) is now recorded and surfaced. Previously the engine discarded the usage payload returned with each prompt response, so run journals and the Compozy UI always reported zero tokens for ACP agents. Usage is now converted after every prompt response and published as a session update, so it reaches the run journal and the live event stream.

### What gets reported

After each ACP prompt response, Compozy maps the runtime's usage payload into the run's usage totals:

| Reported field       | Source                                                                     |
| -------------------- | -------------------------------------------------------------------------- |
| Input tokens         | ACP `inputTokens`                                                          |
| Output tokens        | ACP `outputTokens` + `thoughtTokens` (reasoning tokens folded into output) |
| Total tokens         | ACP `totalTokens` (the session-wide sum of all token types)                |
| Cache reads / writes | ACP `cachedReadTokens` / `cachedWriteTokens`                               |

Reasoning/thought tokens are summed into output tokens for this release; a dedicated reasoning-token field is planned for a later milestone. Totals stay self-consistent because ACP's `totalTokens` already includes reasoning tokens, so `input + output` continues to match `total` after folding.

### Behavior

- **Accumulates across turns.** Each prompt response adds to both the per-job and the aggregate run totals, so long-running sessions report cumulative usage rather than only the last turn.
- **Zero-usage updates are skipped.** Empty payloads never perturb the totals or emit spurious usage events.
- **Streamed to the UI and journal.** Every non-empty update emits a usage event and is appended to the durable run journal, so token counts show up live and on replay.

### Under the hood

This change upgrades the ACP SDK (`github.com/coder/acp-go-sdk`) to v0.13.5. The SDK dropped the `session/set_model` RPC, so model selection for ACP agents is resolved at session creation rather than switched at runtime — pass the model up front (Compozy already pins the Claude agent's model via the environment).
