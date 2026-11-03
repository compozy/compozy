# BUG-20261103-resume-all-rate-limited: Resume all resumes only one paused agent

- Status: verified (fixed on feat/stays-on-hardening 4056dbcc2)
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

## Re-walk 2026-11-03 on 4056dbcc2

Fixed. Resume all now resumes one agent at a time (`docs/qa/reports/artifacts/2026-11-03-stays-on-hardening-qa/rewalk-s4-resume-requests.txt`).
- At `max_active_instances = 7` with 6 active and 3 paused for `unattended`, it resumed Alpha Watch (200, 251 ms). The next two requests returned 429 only after that one finished. The panel then read "Beta Watch and Ops Lead didn't resume. You have 7 active stays-on agents. Pause one or raise `agents.stays_on.max_active_instances`." (`https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/stays-on-hardening/4056dbcc2/04-s4-resume-all.png`).
- At `max_active_instances = 20`, all three resumed (200 each, 180–214 ms, no overlap; `https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/stays-on-hardening/4056dbcc2/04b-s4-resume-all-room-for-all.png`).
- Residual paper cuts: the open panel keeps a resumed agent under Needs you and its count until it refetches, and the config key shows literal backticks.
