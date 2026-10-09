# BUG-20261009-subagent-preview-raw-markdown: Subagent previews show raw markdown symbols

- **Status:** verified
- **Impact (user-side):** Cosmetic
- **Severity:** Low · **Priority:** P3
- **Persona Affected:** Bruno
- **Journey Step:** see scenario
- **Scenarios:** ET-web-subagent-card; ET-web-native-subagent
- **Found:** 2026-10-09 · **Report:** docs/qa/reports/2026-10-09-subagents.md

## Summary

Card line 2, group rows, and progress show `**Yes—only for…**`, `# API Versioning…`, and `**Planning…**` literally.

## Reproduction

- **Environment:** isolated lab (daemon from branch `sa-qa` @ c17af94ec), real Claude (claude-agent-acp, Sonnet 5.5) parent and Codex (gpt-5.6-sol) children, headless Chromium 1440×900 @2x, light theme.

1. Let a Codex subagent finish with a markdown answer; look at the card.

**Expected:** Plain first line (markdown stripped).
**Actual:** Raw `**`/`#` characters.

## Evidence

- Evidence: `/Users/pedronauck/Dev/compozy/_worktrees/subagents/.compozy/tasks/subagents/orchestration/screens/pr/02-card-completed.png`, `03b-group-expanded.png`.
- Report: `docs/qa/reports/2026-10-09-subagents.md`

## Retest 2026-10-09

Fixed in 7573fab3a. Re-walked on the stock build at 980d51fbe: Card line 2 reads plain text (`Yes—retry automatically, but only for transient failures.`). Report: `docs/qa/reports/2026-10-09-subagents-r2.md`.
