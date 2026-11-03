# BUG-20261103-resume-all-rate-limited: Resume all resumes only one paused agent

- Status: open (fix in progress on F1)
- Impact (user-side): Blocks-Completion
- Severity: High; Priority: P1
- Persona Affected: Dora
- Journey Step: Resume every agent the spend brake paused from the +N crew panel (S4)
- Scenarios: MS-web-stays-on-placement
- Origin: stays-on hardening QA walk, soh/qa 583415ef5
- Report: docs/qa/reports/2026-11-03-stays-on-hardening-qa.md

## Reproduction

Pause two or more stays-on agents in one project for `unattended` (low `agents.stays_on.unattended_turn_limit`, timer chain). Open the +N crew panel and press Resume all.

Expected: every `unattended` agent resumes. Observed: the Web sends parallel `POST /api/agent-instances/:id/resume` requests; all but one return 429 within 1 ms (daemon log 03:06:04 local, client `web-d33a912c…`), the UI shows no error, and the other agents stay paused. Ops Lead's `unattended_turns` also read 0 afterwards although its resume was refused. Screenshot: `04-s4-resume-all.png` (R2 `stays-on-hardening/583415ef5/`).
