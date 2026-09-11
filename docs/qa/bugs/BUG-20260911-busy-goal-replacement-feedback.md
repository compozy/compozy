# BUG-20260911-busy-goal-replacement-feedback: Busy submission loses replacement guidance

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Lea
- **Journey Step:** J-26 require and prepare an expected-Run replacement
- **Scenarios:** GL-009; GL-010; GL-011
- **Found:** 2026-09-11
- **Report:** docs/qa/reports/2026-09-10-qa-execution-unblock.md

## Reproduction

Start a real Goal in sess-7e275639d4a31b55, open the Goal details, and send a new /goal through Steer while the worker is active. The existing Goal is preserved, but Draft replacement does not appear. After the same worker settles at its requested pause boundary, send the same command through the idle composer: the expanded Goal shows Draft replacement. Clearing the retained draft then clicking that action correctly stages the exact Run ID. The subsequent replacement completes, and a stale old ID is rejected without changing it.

Evidence: goal-replace-retake-busy-feedback.txt, goal-replace-retake-idle-feedback.txt, goal-replace-idle-affordance.png, goal-replace-prefilled-after-clear.txt, goal-replace-snapshot-watch.json and goal-replacement-stale-web.png. The earlier collapsed-strip observation was inconclusive and is not the reproduction. Both ordinary origin sessions were stopped before diagnosis.

## Cause and impact

The busy-send adapter throws a generic SessionApiError for non-2xx responses before preserving prompt.goal. Its mutation therefore cannot publish the typed Goal feedback used by the existing replacement action. Successful busy Goal results also need the same feedback owner. The idle chat transport already reports the complete result before exposing error guidance.

Preserve the structured failure in a typed session error, then publish result/command to the existing session feedback owner and scoped Goal cache. Keep errors rejected so the draft and failure state remain truthful. No server route, DTO, tool ID, hook, config, workspace isolation, migration, permission or official-skill grammar changes; Web alone loses this existing contract.

## Validation

Invariant at the HTTP adapter: a rejected Goal retains its exact typed result and HTTP status. Invariant at the action layer: busy Goal outcomes update only the submitting session/workspace feedback and Goal envelope, retaining failed submission semantics. Canonical suites: session adapters session-api.test.ts and session hooks use-session-actions.test.tsx. Four regressions failed before repair;101 adapter/action tests passed after repair. Root Turbo typecheck/build passed after correcting the new durable request fixture. Fresh real replay and independent reads passed; see busy-goal-feedback-proof.json. The busy failure retained active_prompt and queue0/current Run, exact prefill survived reload, successor completed approved, and stop was confirmed. Required make gate passed all affected lanes, including771 Web files/7209 tests and lint/typecheck. Local commit follows explicit no-stash precommit checks.

Fix commit: ed4593651. Explicit lint-staged --no-stash, commitlint and staged diff checks passed; unrelated deletion and handoff files preserved.
