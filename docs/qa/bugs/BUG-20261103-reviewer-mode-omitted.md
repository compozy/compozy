# BUG-20261103-reviewer-mode-omitted: "Auto unavailable" can never show on the Permissions page

- Status: verified (fixed on feat/stays-on-hardening 4056dbcc2)
- Impact (user-side): Trust-Damage
- Severity: Medium; Priority: P2
- Persona Affected: Dora
- Journey Step: Choose Let a reviewer decide for an agent whose provider has no reviewer (S5)
- Scenarios: MS-web-agent-permissions
- Origin: stays-on hardening QA walk, soh/qa 583415ef5
- Report: docs/qa/reports/2026-11-03-stays-on-hardening-qa.md

## Reproduction

`GET /api/providers` returns `reviewer_mode` only for claude (`auto`) and codex (`agent`); every other provider omits the field. The Web treats an absent field as unknown and only warns on an empty string, so no provider ever shows Auto unavailable. Evidence: `docs/qa/reports/artifacts/2026-11-03-stays-on-hardening-qa/providers-reviewer-mode.json`, screenshot `05-s5-permissions-levels.png`. Backend and Web need one contract (empty string or explicit absence meaning "no reviewer").

## Re-walk 2026-11-03 on 4056dbcc2

Fixed. `GET /api/providers` now sends `reviewer_mode: ""` for providers without a reviewer (`docs/qa/reports/artifacts/2026-11-03-stays-on-hardening-qa/rewalk-providers-reviewer-mode.json`). For a stays-on Gemini agent, the Permissions page marks Let a reviewer decide as **Not available** (`https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/stays-on-hardening/4056dbcc2/05-s5-permissions-levels.png`). Difference from the draft: a pill replaces the draft's alert that names the provider.
