# BUG-20261009-subagent-deny-message-prefixed: Hook denial message is wrapped in internal text

- **Status:** open
- **Impact (user-side):** Cosmetic
- **Severity:** Low · **Priority:** P3
- **Persona Affected:** Ada
- **Journey Step:** see scenario
- **Scenarios:** RT-subagent-delegate
- **Found:** 2026-10-09 · **Report:** docs/qa/reports/2026-10-09-subagents-r2.md

## Summary

A `spawn.pre_create` deny returns `capability_denied` with `hooks: event "spawn.pre_create" denied: QA policy: DENYME delegations are blocked.`; `_dx.md` says the message is the hook reason.

## Reproduction

- **Environment:** isolated lab, stock build `sa-qa` @ 980d51fbe, real Claude (claude-agent-acp) parent and Codex children.

1. Sync `spawn.pre_create` hook that denies subagent delegations with a reason.
2. Delegate.

**Expected:** `message` = the hook's `deny_reason`.
**Actual:** The reason prefixed with `hooks: event "spawn.pre_create" denied: `.

## Evidence

- Reproduced in the round-2 lab; see the report.
