# BUG-20261009-subagent-show-needs-workspace: `session subagents show` fails outside a registered workspace directory

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** Low · **Priority:** P3
- **Persona Affected:** Ada
- **Journey Step:** see scenario
- **Scenarios:** RT-subagent-delegate
- **Found:** 2026-10-09 · **Report:** docs/qa/reports/2026-10-09-subagents.md

## Summary

Running `compozy session subagents show sub-…` from a directory that is not a registered workspace fails workspace resolution, while the list form works from anywhere.

## Reproduction

- **Environment:** isolated lab (daemon from branch `sa-qa` @ c17af94ec), real Claude (claude-agent-acp, Sonnet 5.5) parent and Codex (gpt-5.6-sol) children, headless Chromium 1440×900 @2x, light theme.

1. `cd` to an unregistered directory.
2. `compozy session subagents show sub-e94cc5c5e257fd21`.

**Expected:** The `_dx.md` transcript: the record prints with no workspace flag.
**Actual:** `resolve workspace override: … is not registered`; works with cwd inside the workspace.

## Evidence

- Subagent ids are global; the show/cancel verbs could resolve the workspace from the record.
- Report: `docs/qa/reports/2026-10-09-subagents.md`

## Retest 2026-10-09

Fixed in 8130f71fc. Re-walked on the stock build at 980d51fbe: `session subagents show <id>` works from `/tmp`. Report: `docs/qa/reports/2026-10-09-subagents-r2.md`.
