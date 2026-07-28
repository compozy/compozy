# BUG-20260713-goal-errors-expose-reason-code: Invalid Goal input exposes internal reason codes

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Lea
- **Journey Step:** J-26, correct an invalid Goal command
- **Scenarios:** GL-003
- **Found:** 2026-07-13 · **Report:** docs/qa/reports/2026-07-13-automation-features.md
- **Origin:** CH-046 invalid-input controls

## Summary

Bare `/goal` and an oversized objective are rejected without side effects, but the chat shows only raw internal codes: `goal_objective_required` and `goal_objective_too_large`. The user receives no plain-language correction or limit guidance despite the transport already distinguishing the two cases.

## Reproduction

1. In `CH-046` / Feature Tour, submit bare `/goal` from a live session.
2. Submit a `/goal` objective longer than the supported maximum.
3. Inspect each chat error and confirm no Goal Run or snapshot is created.

**Expected:** The UI explains what is wrong, how to recover, and the relevant limit while preserving a stable machine reason code outside the primary message.
**Actual:** The entire user-facing message is the raw underscore-delimited reason code.

## Evidence

- Live session `sess-7842125cce618d86`; raw messages `goal_objective_required` and `goal_objective_too_large`.
- `docs/qa/reports/2026-07-13-automation-features.md`, CH-046 continuation.

## Fix

- **Root cause:** The Goal-aware chat transport converted an error response into plain text by falling back directly to `reason_code`. The ordinary assistant-ui error notice then rendered that machine identifier as the complete user message.
- **Fix commit:** uncommitted QA remediation batch
- **Regression test:** The canonical Goal transport and Session runtime integration suites cover both typed reason codes, exactly one request, actionable mapped copy, hidden raw identifiers, preserved retry input, and an enabled composer after the response.

## Verification

- Live session `sess-7842125cce618d86` submitted bare `/goal` and a 5,000-character objective through the production composer. Each request produced one user turn, no Goal state, and one dedicated alert.
- The UI rendered `Add an objective after /goal, then try again.` and `Shorten the Goal objective, then try again.`; neither DOM contained `goal_objective_required` nor `goal_objective_too_large`.
- The composer remained enabled and accepted a retry draft immediately after the response.
- Evidence: `goal-objective-required-human-guidance.dom.txt` and `goal-objective-too-large-human-guidance.dom.txt` in the isolated QA screenshot directory.
