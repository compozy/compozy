# BUG-20261103-reviewer-mode-omitted: "Auto unavailable" can never show on the Permissions page

- Status: fixed (pending re-walk on the final head); fix commit 2f5647d5b always sends `reviewer_mode` (empty = no reviewer) and the Web reads it
- Impact (user-side): Trust-Damage
- Severity: Medium; Priority: P2
- Persona Affected: Dora
- Journey Step: Choose Let a reviewer decide for an agent whose provider has no reviewer (S5)
- Scenarios: MS-web-agent-permissions
- Origin: stays-on hardening QA walk, soh/qa 583415ef5
- Report: docs/qa/reports/2026-11-03-stays-on-hardening-qa.md

## Reproduction

`GET /api/providers` returns `reviewer_mode` only for claude (`auto`) and codex (`agent`); every other provider omits the field. The Web treats an absent field as unknown and only warns on an empty string, so no provider ever shows Auto unavailable. Evidence: `docs/qa/reports/artifacts/2026-11-03-stays-on-hardening-qa/providers-reviewer-mode.json`, screenshot `05-s5-permissions-levels.png`. Backend and Web need one contract (empty string or explicit absence meaning "no reviewer").
