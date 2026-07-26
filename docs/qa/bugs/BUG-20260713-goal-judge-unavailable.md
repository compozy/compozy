# BUG-20260713-goal-judge-unavailable: A plain Goal cannot start from a live Cursor session

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Lea
- **Journey Step:** J-26 start a conversational Goal, step 1
- **Scenarios:** GL-001; GL-003
- **Found:** 2026-07-13 · **Report:** docs/qa/reports/2026-07-13-automation-features.md
- **Origin:** n/a

## Summary

Lea submitted a valid plain `/goal` objective from a live Cursor/Grok session. No Goal started and the only visible response was the opaque code `goal_judge_unavailable`. The UI did not identify the missing/unusable judge, link to configuration, suggest a compatible runtime, or provide a retry path, so the canonical Goal journey is blocked before its first turn.

## Reproduction

- **Charter:** CH-046 · **Tour:** Feature Tour
- **Environment:** laptop / wifi-fast / en-US; live Cursor ACP session `sess-b1c980b86709053d` in the isolated workspace.

1. Open a live, idle session with a completed real-provider turn.
2. Submit `/goal Produce a complete launch go/no-go decision with evidence ...`.
3. Observe the inline result and inspect the session for an active Goal chip/Run.

**Expected:** A valid objective starts one session-scoped Goal with the configured canonical judge, or a typed failure identifies the unavailable judge/configuration and gives a concrete recovery with no side effect.
**Actual:** The composer renders only `goal_judge_unavailable`; no Goal begins and the operator cannot determine how to recover.

## Evidence

- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-goal-judge-unavailable.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/journey-log.jsonl`
- Session `sess-b1c980b86709053d`; no Goal chip or turn appeared after the command.

## Fix

- **Root cause:** The immutable agent profile could carry an empty model even though the active Cursor session had already reconciled the canonical provider model. Goal judge definition construction stopped at the empty profile value instead of falling back to the active session's reconciled model, so judge availability failed before Goal creation.
- **Fix commit:** pending final whole-diff commit.
- **Regression test:** Canonical daemon Goal command tests distinguish profile-first precedence when both profile and session models are non-empty, session-model fallback when the profile is empty, and higher explicit config/run precedence. The selected model is pinned into the Goal definition so later session changes cannot mutate an active Goal.

## Verification

- Same-persona browser replay passed on live Cursor/Grok session `sess-7842125cce618d86`. The valid command created exactly one durable `__session_goal__` Run, `looprun-5a1acf5934fef596`, in 474 ms, exposed the active Goal status, executed three correlated provider turns, and preserved the Run after settled clear. Bare and oversized objectives independently produced actionable no-side-effect validation guidance. Constrained judge cleanup and active-clear projection are tracked separately.
