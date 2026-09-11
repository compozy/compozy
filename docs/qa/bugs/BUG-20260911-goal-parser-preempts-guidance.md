# BUG-20260911-goal-parser-preempts-guidance: Empty Goal submission appears as an internal server error

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Lea
- **Journey Step:** J-26, correct an invalid Goal command
- **Scenarios:** GL-003
- **Found:** 2026-09-11
- **Report:** docs/qa/reports/2026-09-10-qa-execution-unblock.md

## Reproduction and impact

In the real Web session sess-dfea19294366ffd4, submit bare /goal. Web displays Internal Server Error and retains unconfirmed input with Retry. CLI repeats a missing-objective parser error rather than the structured Goal response. Independent Goal status remains null. A subsequent valid Goal completes with real Cursor/Grok worker and judge, so the failure is specific to invalid command admission. Evidence: goal-bare-submitted-web.png/.txt, goal-bare-cli.json, goal-after-invalid-status.json, goal-first-run-proof.json.

## Cause and bounded fix

preparePromptSkillInvocations calls ParseGoalCommand and returns a recognized Goal's parse error before the Goal dispatcher can convert its typed reason into the existing structured error decision. Reserved Goal text must bypass skill invocation parsing whether valid or invalid; dispatch remains the single owner of Goal validation. The existing manager Goal admission suite covers bare and oversized input with no executor calls or Goal snapshot. This differs from the older frontend reason-code mapping bug: its established guidance cannot run without the structured payload.

## Cross-surface impact

Session prompt ingress over CLI/HTTP/UDS now reaches the existing Goal rejection envelope and reason-code mapping; Web can display the documented guidance. Goal grammar, runtime selection, authorization, hooks/config, SDK schema, and workspace data are unchanged. Invalid inputs never reach Goal execution. Internal literal Goal text remains an ordinary prompt. Official skill already documents the grammar and textual clauses; no new instruction/API surface or migration is introduced. Real Web/CLI/HTTP/UDS retest passed; see the verification below.

## Verification

Fresh Web session sess-a6f30575423e1660 displays human guidance for bare and oversized input. CLI returns the existing structured Goal reason; HTTP and UDS return 422. Independent Run lists remain empty and the input queue has zero entries. A subsequent valid command returns HTTP202 and creates one session-origin Run; real Cursor Grok 4.6 High Fast worker and judge complete it with one approved turn. Web reload and independent kickoff.md read confirm the result. goal-parser-retest-proof.json indexes the evidence. Canonical admission regression failed before the production fix and passes with race detection; test-conventions check, Go build and make gate passed. Fix commit is recorded in progress.json and the subsequent report checkpoint.
