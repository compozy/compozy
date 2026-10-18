# BUG-20261005-loop-result-drops-whitespace: Loop result readers drop whitespace-only message chunks

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Ada
- **Journey Step:** J-01, inspect the completed default task delivery
- **Scenarios:** TA-080
- **Found:** 2026-10-05 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

The native default implement-tasks Run completes the authored maintenance task, but its
saved summary changes "Added the 59-word" to "Added the59-word". The transcript retains
the space as a separate message chunk. The artifact itself and the rest of the result
remain intact. Dropping content from an otherwise valid JSON string can silently change
result meaning without a parse failure.

## Reproduction

- **Charter:** CH-untested-001-01-ada · **Tour:** Feature Tour
- **Environment:** desktop, 1512x862, DPR 2, fast Wi-Fi, en-US, America/Los_Angeles

1. Start implement-tasks through native compozy__loop_run, omitting mode and implementer.
2. Let the default code_implementer finish the authored maintenance task.
3. Compare its final transcript message with execute_default in CLI, HTTP, UDS and Web.
4. Reload the Run and inspect the saved result again.

**Expected:** all streamed message bytes survive result collection, including standalone spaces.
**Actual:** the standalone space between "the" and "59-word" is removed from the saved summary.

## Evidence

- Run: looprun-63865cd83f00945f; worker: sess_6d534b57a89043826d7af9a104331f68.
- docs/qa/evidence/2026-10-02-untested/loops-native-default-launcher-final-history.json
- docs/qa/evidence/2026-10-02-untested/loops-native-default-worker-history-first.json, sequences 153–155.
- docs/qa/evidence/2026-10-02-untested/loops-native-default-final-status.json
- docs/qa/evidence/2026-10-02-untested/loops-native-default-final-http.json
- docs/qa/evidence/2026-10-02-untested/loops-native-default-final-uds.json
- docs/qa/evidence/2026-10-02-untested/loops-native-default-whitespace-web.json
- docs/qa/evidence/2026-10-02-untested/loops-native-default-result-whitespace-loss-reloaded.png

## Investigation

collectLoopPromptResult concatenates deltas but skips any chunk whose trimmed text is
empty. collectLoopJudgeResult has the same condition at the verdict boundary. Their
existing adapter suites own streamed action answers and verdict bytes respectively;
extend those existing cases with a standalone space inside a JSON string. The managed
Goal reconstruction paths are covered separately by BUG-20261005-goal-result-loses-fields.

## Fix

- **Fix commit:** 4dc707c76eb93e9587cbc6e3fedea3e66236377e
- **Regression test:** existing internal/daemon/loop_runtime_adapters_test.go action and judge collector cases.

## Verification

Both existing collector cases fail before repair and pass with -race after retaining
all chunks. Provider-failure and managed Goal canaries remain green. The test-shape
checker passes the changed adapter suite.

Real-provider Run looprun-7382b45b8c7dbe35 uses the rebuilt daemon and finishes done. Its
final transcript includes a whitespace-only chunk, and the entire stored result equals
the complete final JSON; "Added the 34-word" retains its space. HTTP and UDS match;
Web Details retains it after reload. The first PNG was obscured by the worker's terminal
window, so the unobstructed inspected PNG is the visual evidence. That owned terminal
was then stopped and its exited state independently read back.

The model initially wrote the note at the wrong filename. Independent artifact review
reopened the authored task without relaxing its target or checks. Follow-up Run
looprun-e18745502cde526a corrected the path, removed the duplicate, and settled done.
The full final JSON again matches the transcript. The exact requested file, all three
links, four inventory paths, completed frontmatter and zero pending importer result are
independently verified. Both workers are stopped / verified=true. This operator feedback
is model-output correction, separate from the production stream-reader fix.

Evidence: loops-native-default-whitespace-regression-{red,green}.json;
loops-native-default-fixed-{status-third,worker-final-history,final-http,final-uds}.json;
loops-native-default-fixed-unobstructed-web-verified.json;
loops-native-default-corrected-{workspace-proof,worker-history,worker-stopped,final-import,
final-http,final-uds,final-web}.json. The 18-frame recording is closed and the terminal
and corrected-result PNGs were inspected. Committed closure is recorded below.

## Verified closure

Committed in 4dc707c76eb93e9587cbc6e3fedea3e66236377e. The final make gate passed all affected lanes
(loops-results-final-delivery-gate.json); the commit hook preserved all nine frozen
production/test/site hashes. The real-provider and independent public read evidence above
own this verification. No PR or current-head CI readiness is claimed.
