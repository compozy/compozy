# BUG-20261005-goal-result-loses-fields: A completed Goal loses the conductor's result fields

- **Status:** verified
- **Impact (user-side):** Data-Loss
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-01, inspect the completed orchestrated delivery result
- **Scenarios:** LP-implement-tasks-orchestrated-mode; TA-080
- **Found:** 2026-10-05 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

Bruno sees a completed delivery of three tasks, but its persisted Goal output contains only
the second task's identity. The conductor's final message contains the complete summary
and all three worker identities. Those result fields are recoverable from the transcript,
but the completed Run does not preserve them.

## Reproduction

- **Charter:** CH-implement-tasks-orchestrated-mode · **Tour:** Feature Tour
- **Environment:** desktop, 1512x862, DPR 2, fast Wi-Fi, en-US, America/Los_Angeles

1. Start bundled implement-tasks in orchestrated mode with three dependent tasks.
2. Wait for the conductor and all three workers to finish.
3. Read the completed Run through scoped CLI and HTTP/UDS.
4. Compare the orchestrate output with the conductor's final message in session history.

**Expected:** the Goal output preserves the final summary and all three task/worker identities.
**Actual:** it contains task_02 and its worker_session_id, plus authoritative status complete.

## Evidence

- Run: looprun-741e0ce2ba97875f
- docs/qa/evidence/2026-10-02-untested/loops-orchestrated-status-progress-two.json
- docs/qa/evidence/2026-10-02-untested/loops-orchestrated-conductor-final-history-scoped.json
- docs/qa/evidence/2026-10-02-untested/loops-orchestrated-final-http.json
- docs/qa/evidence/2026-10-02-untested/loops-orchestrated-final-uds.json
- Final message fragments: sequences 354–357. The split occurs inside JSON strings.

## Investigation

Both normal managed Goal output reconstruction and crash recovery insert newlines between
ACP text fragments and discard whitespace-only fragments. The resulting outer JSON becomes
invalid. The existing object extractor can still read an intact nested task object, which the
Goal completion path annotates with its authoritative status. The two reconstruction paths
must preserve the exact streamed bytes. The original transcript and workspace artifacts
remain intact; no task implementation was lost.

## Fix

- **Fix commit:** 4dc707c76eb93e9587cbc6e3fedea3e66236377e
- **Regression test:** existing internal/daemon/loop_goal_executor_test.go, normal and recovery output reconstruction.

## Verification

The owning regression fails before repair and passes with -race for normal and recovery
reads, including a standalone whitespace chunk. Fresh real-provider Run
looprun-61931098226ef84d settles done in generation 1. Its stored Goal result matches the
complete conductor result: status, summary and both task/worker identities. CLI, HTTP,
UDS and Web agree, including after a fresh document reload. Both task files are completed,
the public importer reports zero pending tasks, and both workers are independently
stopped / verified=true. No historical output is rewritten.

Evidence: loops-orchestrated-regressions-{red,green}.json;
loops-orchestrated-fixed-{status-third,final-http,final-uds,final-import,workspace-proof}.json;
loops-orchestrated-fixed-goal-web-verified.json and the inspected
loops-orchestrated-fixed-goal-result-reloaded.png. The 14-frame recording is closed.
Committed closure is recorded below.

## Verified closure

Committed in 4dc707c76eb93e9587cbc6e3fedea3e66236377e. The final make gate passed all affected lanes
(loops-results-final-delivery-gate.json); the commit hook preserved all nine frozen
production/test/site hashes. The real-provider and independent public read evidence above
own this verification. No PR or current-head CI readiness is claimed.
