# BUG-20261009-subagent-runtime-speed-empty: Delegated runtime reports an empty speed

- **Status:** open
- **Impact (user-side):** Cosmetic
- **Severity:** Low · **Priority:** P3
- **Persona Affected:** Ada
- **Journey Step:** see scenario
- **Scenarios:** RT-subagent-delegate
- **Found:** 2026-10-09 · **Report:** docs/qa/reports/2026-10-09-subagents.md

## Summary

`runtime.speed` is `""` in delegate/status/list payloads when speed is omitted; `show` prints `codex · gpt-5.6-sol · medium · ` with a dangling separator, while the child's prompt runtime is `normal`.

## Reproduction

- **Environment:** isolated lab (daemon from branch `sa-qa` @ c17af94ec), real Claude (claude-agent-acp, Sonnet 5.5) parent and Codex (gpt-5.6-sol) children, headless Chromium 1440×900 @2x, light theme.

1. Delegate with no `target.speed`.
2. `compozy session subagents show <id>`.

**Expected:** `speed: "normal"` (or the resolved value).
**Actual:** Empty string.

## Evidence

- Reproduced in the lab; see the run report.
- Report: `docs/qa/reports/2026-10-09-subagents.md`
