# BUG-20261009-subagent-status-empty-input: Status call without subagent_id answers an internal error

- **Status:** open
- **Impact (user-side):** Friction
- **Severity:** Low · **Priority:** P3
- **Persona Affected:** Ada
- **Journey Step:** see scenario
- **Scenarios:** RT-subagent-delegate
- **Found:** 2026-10-09 · **Report:** docs/qa/reports/2026-10-09-subagents.md

## Summary

Claude's first `compozy__subagent_status` call in a wake turn arrived with empty input and got `schema_invalid: Internal Server Error`; its retry with the id worked.

## Reproduction

- **Environment:** isolated lab (daemon from branch `sa-qa` @ c17af94ec), real Claude (claude-agent-acp, Sonnet 5.5) parent and Codex (gpt-5.6-sol) children, headless Chromium 1440×900 @2x, light theme.

1. Call `compozy__subagent_status` with `{}` through hosted MCP.

**Expected:** `invalid_request` with a field message (for example `subagent_id is required.`).
**Actual:** `schema_invalid: Internal Server Error` (tool call toolu_016FXSyLWcnrKKaBopphowhF).

## Evidence

- Error mapping only; behavior otherwise correct.
- Report: `docs/qa/reports/2026-10-09-subagents.md`
