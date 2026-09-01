# QA Run Report — 2026-09-01 — Issue 521/522 Session Cancel and Clear

- **Scope:** Request-scoped prompt cancellation, late-cancel idempotency, and atomic transcript reads during conversation clear.
- **Cadence tier:** targeted
- **Build:** `5ae7dbbfe` + working tree · **Environment:** isolated targeted lab, Codex provider, HTTP `127.0.0.1:53131`
- **Started:** 2026-09-01T18:14:16Z · **Status:** QA pass; delivery CI pending

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Agent / headless operator | desktop / wifi-fast / en-US | `CH-herdr-session-orchestration` |
| Théo | Returning session operator | desktop / wifi-fast / en-US | `CH-untested-valid-005-14-theo`, `CH-stopped-session-prompt-continuity` |

## Flows in Scope

- `J-15` — cancel one prompt through structured surfaces while preserving the live session and its next turn ([journey](../journeys/J-15-operate-session-via-cli-api.md)).
- `J-14` — clear a transcript and confirm the same session stays empty after a fresh read ([journey](../journeys/J-14-read-a-finished-transcript.md)).
- `J-13` — adjacent canary: submit a later prompt and preserve one durable session identity ([journey](../journeys/J-13-follow-a-live-run.md)).

Taxonomy coverage: the functional and journey contracts are the primary scope; interruption, repeat action, concurrent read, refresh, and error recovery cover edge and experiential dimensions; CLI/API/runtime parity covers cross-cutting consistency. Layout, locale, and mobile compatibility are deliberately out of scope because no UI layout or copy changed.

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | `CH-herdr-session-orchestration` | `J-15` / `RT-session-prompt-cancel` | Ada | Interrupt Tour | Pass | #522 | working tree |
| 2 | `CH-untested-valid-005-14-theo` | `J-14` / `RT-017` | Théo | Feature Tour | Pass | #521 | working tree |
| 3 | `CH-stopped-session-prompt-continuity` | `J-13` / `RT-018` | Théo | Interrupt Tour | Skipped | known `BUG-20260825-workspace-agent-unusable-for-sessions` | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### Ada — cancel one prompt, then continue

- Started real Codex turn `turn-8a40d61cac8706d7` in session `sess-a63b87a64ffa9285` and canceled it through the CLI.
- The stream ended with ACP request cancellation, while session status returned to healthy and idle.
- Repeated cancellation through CLI, HTTP, and `compozy__session_prompt_cancel` returned `nothing-in-flight` without stopping the session.
- A later prompt completed `NEXT_TURN_OK` on the same durable session and original ACP session ID.

### Théo — clear and freshly read the same session

- Confirmed the pre-clear transcript contained the canceled turn and the successful later turn.
- `POST .../sessions/sess-a63b87a64ffa9285/clear` returned 200 with the same durable session and a restarted ACP context.
- Independent HTTP and UDS reads returned an empty transcript at epoch 1; neither returned a transient missing-session result.
- The restarted Codex session completed `CLEAR_NEXT_OK`, proving the session remained usable after database replacement.
- A separate Web session rendered `WEB_CLEAR_READY`, completed the clear confirmation dialog, rendered an empty thread, and returned zero transcript entries on an independent API read.
- The adjacent stop/resume canary could not run because workspace-profile agents hit the already-open `BUG-20260825-workspace-agent-unusable-for-sessions`; the in-scope clear and post-clear prompt completed before that independent boundary.

## What Was Fixed

### Issue #522: late prompt cancel could cancel the next turn

- **Symptom:** A repeated or delayed prompt cancel emitted session-scoped ACP cancellation after the original turn had settled.
- **Root cause:** `Driver.Cancel` fell back to `session/cancel`, and prompt-context cancellation also emitted the same session-scoped notification even though the ACP SDK supports request-scoped cancellation.
- **Fix:** working tree.
- **Regression test:** `internal/acp/client_process_lifecycle_test.go` failed before the fix by capturing one unwanted `session/cancel` for both active and late cancellation, then passed with next-turn integrity.
- **Retested:** Pass. CLI, HTTP, native-tool, runtime, and real Codex-provider evidence is recorded under `/Users/pedronauck/dev/qa-labs/compozy-issue-521-522-session-cancel-clear-20260901-183338-907244-lab/qa-artifacts/qa`.

### Issue #521: transcript read raced session clear

- **Symptom:** A transcript poll returned `session not found` during clear and could interfere with the in-progress clear transaction, producing HTTP 500.
- **Root cause:** recorder resolution ignored the conversation-finalization barrier and translated query-pool quiescence into a missing session while the database family was being replaced.
- **Fix:** working tree.
- **Regression test:** `internal/session/manager_clear_test.go` failed before the fix with `session: session not found` during deterministic query-pool quiescence, then passed by waiting for finalization.
- **Retested:** Pass across Web, runtime, API, and UDS in the isolated lab: both clear requests returned 200, fresh reads returned epoch 1 with zero entries, the first durable session accepted a later prompt, and the Web session rendered an empty thread after confirmation. Exact-head PR E2E remains the delivery backstop.

## Paper Cuts

- The out-of-scope stopped-session canary reproduced existing `BUG-20260825-workspace-agent-unusable-for-sessions`: the catalog listed the workspace agent, but resume validation rejected it as unavailable. This did not affect active cancel, clear, fresh read, or the post-clear prompt.

## Runtime Errors Observed

- Recent `ci.yml` runs `33538539190`, `33535868025`, and `33528977779` reproduced issue #521 in Web E2E shard 3/4 at `session-hardening.spec.ts:314`: backup completed, a concurrent transcript read reported `session not found`, and clear returned HTTP 500.
- Other recent E2E failures (Loops builder, OS shell performance, terminal locator, and Desktop teardown) have different logs and are outside this fix.
- The local full Web E2E lane was intentionally stopped before completion; repository policy assigns the heavy E2E matrix to exact-head PR CI. Focused race-enabled regressions and `make gate` remain the local evidence.
- The isolated QA environment was torn down after the strict evidence audit. Its `qa/teardown.json` records `"clean": true` with no surviving processes.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- ACP request cancellation already has an exact request identity; session-wide cancellation belongs only to whole-session stop.
- A database replacement barrier must cover recovery and read-open paths, not only the writer performing the replacement.

## Final Status

- **Local gate:** PASS — `make gate` completed the affected `go-lint` and race-enabled `go-test` lanes with zero lint issues and zero test failures.
- **Final verify log:** `/Users/pedronauck/dev/qa-labs/compozy-issue-521-522-session-cancel-clear-20260901-183338-907244-lab/qa-artifacts/qa/verify-gate.log`
- **Exit gate (full automated suite):** Pending exact-head PR CI by repository policy.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 2/3 passed; 1/3 adjacent canary skipped on a known pre-existing bug.
- Verdict: PASS — both changed behavior contracts passed their public-surface walks and strict evidence review.
- Delivery: PENDING — the PR's exact-head E2E and required CI checks remain the completion authority.
