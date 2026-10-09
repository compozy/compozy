# BUG-20261009-subagent-idempotency-default: Delegating without an idempotency_key always fails

- **Status:** open
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** see scenario
- **Scenarios:** RT-subagent-delegate
- **Found:** 2026-10-09 · **Report:** docs/qa/reports/2026-10-09-subagents.md

## Summary

When the agent omits `idempotency_key` (the documented default is the invoking tool call id), the delegate tool returns `invalid_request: idempotency_key must contain 1 to 256 characters.` Claude had to retry with an explicit key; with an operator instruction not to pass one, all three delegations in a turn failed.

## Reproduction

- **Environment:** isolated lab (daemon from branch `sa-qa` @ c17af94ec), real Claude (claude-agent-acp, Sonnet 5.5) parent and Codex (gpt-5.6-sol) children, headless Chromium 1440×900 @2x, light theme.

1. Claude parent calls `compozy__subagent_delegate` with `task`, `title`, `target`, no `idempotency_key` (hosted MCP).

**Expected:** Row created with the tool call id as the key.
**Actual:** `{"error":{"code":"invalid_request","message":"idempotency_key must contain 1 to 256 characters."}}` (reproduced 4/4).

## Evidence

- The hosted-MCP invocation path does not pass the provider tool call id into the native tool context, so the default resolves to empty. Evidence: parent sess-b2170d1fc314631a tool call toolu_019Jy1rsdPLYKrbjxbq3eVD9; parent sess-f21ee28c8813d948 first turn.
- Report: `docs/qa/reports/2026-10-09-subagents.md`
