# BUG-20261009-subagent-capabilities-oversized: Capabilities answer is too large for Claude to read

- **Status:** open
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** see scenario
- **Scenarios:** RT-subagent-delegate
- **Found:** 2026-10-09 · **Report:** docs/qa/reports/2026-10-09-subagents-r2.md

## Summary

Every `compozy__subagent_capabilities` call from a Claude parent returned 99,133 characters, over Claude Code's MCP result limit. Claude had to save it to a file and parse it with `jq` before delegating (seen in both rounds).

## Reproduction

- **Environment:** isolated lab, stock build `sa-qa` @ 980d51fbe, real Claude (claude-agent-acp) parent and Codex children.

1. In a Claude session, have the agent call `compozy__subagent_capabilities`.

**Expected:** A compact answer an agent can read inline.
**Actual:** `Error: result (99,133 characters) exceeds maximum allowed tokens. Output has been saved to …`.

## Evidence

- `providers` is ~106 KB: `opencode` lists 611 models (82 KB) and `hermes` 89 (13 KB), and every non-delegatable provider is included. Bound or summarize the model lists (for example, models only for `can_delegate` providers, with a cap) or let the agent ask per provider.

## Remediation

Capabilities now includes at most 40 models for each delegable provider, current/default first,
with additive `models_total` and `models_truncated`. Unavailable providers expose an empty model
list. Delegate validation retains the complete catalog. UT-017 covers a 611-model catalog and an
omitted valid delegate target; native binding coverage checks the additive wire fields. Real-provider
verification remains pending; this entry's status is unchanged until the QA re-walk.
