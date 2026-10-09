# BUG-20261009-subagent-card-hosted-tool-name: Real Claude delegations never get a subagent card in the parent transcript

- **Status:** open
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P0
- **Persona Affected:** Bruno
- **Journey Step:** see scenario
- **Scenarios:** ET-web-subagent-card
- **Found:** 2026-10-09 · **Report:** docs/qa/reports/2026-10-09-subagents.md

## Summary

Bruno watches a Claude session delegate to Codex; the transcript shows `Delegated a subagent` tool rows and never a card, even after reload.

## Reproduction

- **Environment:** isolated lab (daemon from branch `sa-qa` @ c17af94ec), real Claude (claude-agent-acp, Sonnet 5.5) parent and Codex (gpt-5.6-sol) children, headless Chromium 1440×900 @2x, light theme.

1. Claude parent (claude-agent-acp, hosted MCP) delegates with `compozy__subagent_delegate`.
2. `GET …/sessions/{id}/transcript` and look for `data-compozy-subagent`.

**Expected:** One `data-compozy-subagent` part at the delegation point; the Web draws a card.
**Actual:** Zero parts. The persisted tool name is `mcp__compozy-hosted-tools__compozy__subagent_delegate`.

## Evidence

- `internal/transcript/ui_messages_subagent.go:44-45` compares the tool name to the bare `compozy__subagent_delegate`. Stripping the hosted prefixes in a QA-local build produced the part and the card for new delegations. The projection is persisted at ingest, so sessions recorded before a fix keep no card unless re-projected. The Codex-parent tool-name shape was not walked.
- Report: `docs/qa/reports/2026-10-09-subagents.md`
